// SPDX-License-Identifier: AGPL-3.0-or-later

package cliproxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"memo/internal/logx"
)

// loginFlag maps a provider to the sidecar's own login switch.
var loginFlag = map[string]string{
	ProviderAntigravity: "-antigravity-login",
	ProviderClaude:      "-claude-login",
	ProviderCodex:       "-codex-login",
}

// Login timing — vars so tests can shorten them.
var (
	loginURLTimeout = 25 * time.Second
	loginTimeout    = 5 * time.Minute
)

// LoginState is what the UI polls while a browser login is in flight.
type LoginState struct {
	Provider string `json:"provider"`
	Running  bool   `json:"running"`
	URL      string `json:"url,omitempty"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

type loginSession struct {
	provider string
	url      string
	cancel   context.CancelFunc

	mu      sync.Mutex
	done    bool
	err     string
	tail    []string // last output lines, for an error message
	cmd     *exec.Cmd
	started time.Time
}

var urlRe = regexp.MustCompile(`https://[^\s"'<>]+`)

// loginURL picks the vendor's authorize URL out of a line of output. The
// sidecar prints other https links too (docs, update hints); the authorize
// URL is the one carrying OAuth parameters.
func loginURL(line string) string {
	for _, u := range urlRe.FindAllString(line, -1) {
		if strings.Contains(u, "client_id=") || strings.Contains(u, "/oauth") {
			return strings.TrimRight(u, ".,;)")
		}
	}
	return ""
}

// StartLogin runs the sidecar's browser login for provider and returns the URL
// the user must open. The sidecar opens the browser itself when it can; the URL
// is returned so the UI can show a copyable fallback.
//
// -no-browser is deliberately NOT passed: that mode makes the sidecar look up
// the machine's public IP on an external service to print an SSH hint, and
// Memo has no business leaking that.
//
// The login ends when the sidecar exits. Poll LoginStatus; Accounts() shows the
// new credential once it has succeeded. A second StartLogin replaces the first
// (both would want the same loopback callback port).
func (m *Manager) StartLogin(ctx context.Context, provider string) (string, error) {
	flag, ok := loginFlag[provider]
	if !ok {
		return "", ErrUnknownProvider
	}
	bin, _, err := m.Binary()
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	if err := m.ensureIdentityLocked(); err != nil { // writes the same config the server uses
		m.mu.Unlock()
		return "", err
	}
	if old := m.login; old != nil {
		old.cancel()
		m.login = nil
	}
	if err := os.MkdirAll(m.authDir(), 0o700); err != nil {
		m.mu.Unlock()
		return "", err
	}
	if err := os.WriteFile(m.configPath(), []byte(m.renderConfigLocked()), 0o600); err != nil {
		m.mu.Unlock()
		return "", err
	}
	m.mu.Unlock()

	lctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	cmd := exec.CommandContext(lctx, bin, "-config", m.configPath(), flag)
	cmd.Dir = m.dir
	cmd.Env = m.loginEnv()
	cmd.SysProcAttr = sysProcAttr()
	cmd.Cancel = func() error { killTree(cmd); return nil }

	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	s := &loginSession{provider: provider, cancel: cancel, cmd: cmd, started: time.Now()}

	if err := cmd.Start(); err != nil {
		cancel()
		return "", fmt.Errorf("cliproxy: start %s login: %w", provider, err)
	}
	m.mu.Lock()
	m.login = s
	m.mu.Unlock()

	urlCh := make(chan string, 1)
	go func() { // read output, find the URL, keep the tail
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 1<<20)
		sent := false
		for sc.Scan() {
			line := sc.Text()
			s.mu.Lock()
			s.tail = append(s.tail, redactLine(line))
			if len(s.tail) > 12 {
				s.tail = s.tail[len(s.tail)-12:]
			}
			s.mu.Unlock()
			if !sent {
				if u := loginURL(line); u != "" {
					sent = true
					s.mu.Lock()
					s.url = u
					s.mu.Unlock()
					urlCh <- u
				}
			}
		}
	}()
	go func() { // reap and record the outcome
		werr := cmd.Wait()
		_ = pw.Close()
		s.mu.Lock()
		s.done = true
		if werr != nil && lctx.Err() != context.Canceled {
			s.err = loginErrorMessage(werr, s.tail, lctx.Err())
		}
		s.mu.Unlock()
		cancel()
		m.invalidateModels()
		if werr == nil {
			logx.Printf("cliproxy: %s login finished", provider)
		}
	}()

	select {
	case u := <-urlCh:
		openLoginPage(u)
		return u, nil
	case <-time.After(loginURLTimeout):
		cancel()
		s.mu.Lock()
		msg := strings.Join(s.tail, " | ")
		s.mu.Unlock()
		return "", fmt.Errorf("cliproxy: %s login printed no sign-in URL within %s: %s", provider, loginURLTimeout, msg)
	case <-ctx.Done():
		cancel()
		return "", ctx.Err()
	}
}

// LoginStatus reports the login in flight (or the last one finished).
func (m *Manager) LoginStatus() LoginState {
	m.mu.Lock()
	s := m.login
	m.mu.Unlock()
	if s == nil {
		return LoginState{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return LoginState{Provider: s.provider, Running: !s.done, URL: s.url, Done: s.done, Error: s.err}
}

// CancelLogin abandons a login in flight.
func (m *Manager) CancelLogin() {
	m.mu.Lock()
	s := m.login
	m.login = nil
	m.mu.Unlock()
	if s != nil {
		s.cancel()
	}
}

func loginErrorMessage(werr error, tail []string, ctxErr error) string {
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		return "the sign-in was not completed in time"
	}
	if len(tail) > 0 {
		return strings.Join(tail[max(0, len(tail)-4):], " | ")
	}
	return werr.Error()
}

// redactLine drops the query string of any URL in a log line: authorize URLs
// carry the OAuth state and PKCE challenge, which have no business in an error
// message that may be shown or logged.
func redactLine(line string) string {
	return urlRe.ReplaceAllStringFunc(line, func(u string) string {
		if i := strings.IndexByte(u, '?'); i >= 0 {
			return u[:i] + "?…"
		}
		return u
	})
}
