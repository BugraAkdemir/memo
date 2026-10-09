// SPDX-License-Identifier: AGPL-3.0-or-later

package cliproxy

import (
	"bytes"
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
	// NeedsPaste is true when the sidecar is waiting for the callback URL to
	// be typed in. That is the only way in from a machine that is not the
	// browser's: see SubmitCallbackURL.
	NeedsPaste bool `json:"needs_paste,omitempty"`
}

type loginSession struct {
	provider string
	url      string
	cancel   context.CancelFunc

	mu         sync.Mutex
	done       bool
	err        string
	needsPaste bool
	tail       []string // last output lines, for an error message
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	started    time.Time
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

// pasteRe is the sidecar's prompt for the manual callback hand-back. All three
// vendors print one ("Paste the Codex callback URL (or press Enter to keep
// waiting):" and the Claude / antigravity equivalents, verified against the
// pinned binary) once their loopback listener has waited a while without a
// browser turning up locally.
var pasteRe = regexp.MustCompile(`(?i)paste the .*callback URL`)

// wantsPaste reports whether an output line is the sidecar asking to be handed
// the callback URL on stdin.
func wantsPaste(line string) bool { return pasteRe.MatchString(line) }

// handleLoginLine records one line of the login process's output and picks out
// the two things it can tell us: the authorize URL (reported once) and the
// hand-off prompt.
func (s *loginSession) handleLoginLine(line string, urlCh chan<- string) {
	s.mu.Lock()
	s.tail = append(s.tail, redactLine(line))
	if len(s.tail) > 12 {
		s.tail = s.tail[len(s.tail)-12:]
	}
	first := s.url == ""
	if first {
		if u := loginURL(line); u != "" {
			s.url = u
		}
	}
	if wantsPaste(line) {
		s.needsPaste = true
	}
	u := s.url
	s.mu.Unlock()
	if first && u != "" {
		select {
		case urlCh <- u:
		default:
		}
	}
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

	// Stdin must come from cmd.StdinPipe(), NOT from an io.Pipe handed over as
	// cmd.Stdin: a Stdin that is not an *os.File makes exec.Cmd start a copy
	// goroutine of its own, and Wait() does not return until that goroutine
	// stops — which it cannot, because it is blocked reading a pipe nobody
	// writes to. The sign-in finished (the sidecar printed "Authentication
	// successful") and cmd.Wait() still hung, so Done never became true and the
	// UI polled a login that was already over — which is exactly what made a
	// pasted callback URL look like it had done nothing.
	// Output, likewise, is a real *os.File: anything else starts a second copy
	// goroutine, with the same effect. Both streams share one write end so the
	// sidecar's interleaved stdout/stderr progress is read as one stream.
	inw, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return "", fmt.Errorf("cliproxy: stdin for the login: %w", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		cancel()
		return "", fmt.Errorf("cliproxy: pipe for the login output: %w", err)
	}
	pr := outR
	cmd.Stdout, cmd.Stderr = outW, outW
	s := &loginSession{provider: provider, cancel: cancel, cmd: cmd, stdin: inw, started: time.Now()}

	if err := cmd.Start(); err != nil {
		cancel()
		return "", fmt.Errorf("cliproxy: start %s login: %w", provider, err)
	}
	m.mu.Lock()
	m.login = s
	m.mu.Unlock()

	urlCh := make(chan string, 1)
	waited := make(chan error, 1)
	go func() { // read output, find the URL, keep the tail
		// Lines are handled as they arrive, but the paste prompt must be
		// recognised WITHOUT waiting for its newline: the sidecar prints it
		// with a bare Print ("Paste the Codex callback URL (or press Enter to
		// keep waiting): ") and then blocks on stdin, so any line-oriented
		// reader — a Scanner, or ReadString — waits forever and the UI never
		// learns the login is waiting for the URL, which is the one thing it
		// most needs to show. So the tail of an unfinished line is matched as
		// soon as it changes, and only re-reported once when it completes.
		var pending []byte
		buf := make([]byte, 512)
		seenPaste := false
		handle := func(final bool) {
			if len(pending) == 0 {
				return
			}
			line := strings.TrimRight(string(pending), " \r")
			paste := wantsPaste(line)
			if paste && seenPaste {
				// Already reported from an earlier, unfinished read.
				pending = pending[:0]
				seenPaste = false
				return
			}
			if !paste && !final {
				// Not a finished line yet and not the prompt: wait for more.
				return
			}
			if paste {
				seenPaste = true
			}
			s.handleLoginLine(line, urlCh)
			pending = pending[:0]
		}
		for {
			n, err := pr.Read(buf)
			chunk := buf[:n]
			if i := bytes.IndexByte(chunk, '\n'); i >= 0 {
				pending = append(pending, chunk[:i]...)
				seenPaste = false
				handle(true)
				rest := chunk[i+1:]
				for {
					j := bytes.IndexByte(rest, '\n')
					if j < 0 {
						// The chunk ends mid-line. Offer it now: if it is the
						// paste prompt this is the ONLY chance, because the
						// sidecar blocks on stdin right after and no further
						// Read will ever return.
						pending = append(pending, rest...)
						handle(false)
						break
					}
					s.handleLoginLine(strings.TrimRight(string(rest[:j]), " \r"), urlCh)
					rest = rest[j+1:]
				}
			} else {
				pending = append(pending, chunk...)
				handle(false)
			}
			if err != nil {
				// The child closed its output (it exited or closed stdout):
				// finish the line and let the Wait below proceed, which needs
				// this goroutine to be done reading before it can reap.
				seenPaste = false
				handle(true)
				return
			}
		}
	}()
	// Wait for the child and close the write end so the reader above sees EOF.
	// Both streams are an *os.File precisely so that exec.Cmd runs no copy
	// goroutine here: with an io.Pipe it starts one per stream and Wait() does
	// not return until they finish, which cannot happen while the reader is
	// still holding the other end open. That deadlock left Done false forever
	// on a sign-in that had already succeeded ("Authentication successful" was
	// printed), so the UI kept polling a finished login — which is exactly what
	// made a pasted callback URL look like it had done nothing.
	go func() {
		werr := cmd.Wait()
		_ = outW.Close() // the reader above sees EOF once the child is gone
		_ = inw.Close()
		waited <- werr
	}()
	go func() { // record the outcome
		werr := <-waited
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
	return LoginState{
		Provider:   s.provider,
		Running:    !s.done,
		URL:        s.url,
		Done:       s.done,
		Error:      s.err,
		NeedsPaste: s.needsPaste && !s.done,
	}
}

// SubmitCallbackURL hands the sidecar the callback URL the browser could not
// deliver.
//
// Every vendor's OAuth client registers a loopback redirect (Codex
// `http://localhost:1455/auth/callback`, Claude `:54545/callback`, Antigravity
// `:51121/oauth-callback`), and the provider checks redirect_uri against that
// registration — rewriting it to a LAN or public address is refused outright
// ("redirect_uri_mismatch", verified against the live endpoints). So the code
// can only ever be delivered to the loopback interface of the machine running
// the sidecar, and a browser on another machine cannot reach it. The sidecar
// knows this: it prints "To authenticate from a remote machine, an SSH tunnel
// may be required" and then asks for the callback URL on stdin, which is what
// this feeds.
//
// The user finishes the sign-in in their browser, whose address bar ends up on
// the (empty) localhost callback page with `?code=…&state=…`; copying that
// whole address back here hands the sidecar the code and it exchanges the token
// itself.
func (m *Manager) SubmitCallbackURL(callbackURL string) error {
	callbackURL = strings.TrimSpace(callbackURL)
	if callbackURL == "" {
		return errors.New("cliproxy: empty callback URL")
	}
	m.mu.Lock()
	s := m.login
	m.mu.Unlock()
	if s == nil {
		return errors.New("cliproxy: no login in progress")
	}
	s.mu.Lock()
	done, in := s.done, s.stdin
	s.mu.Unlock()
	if done {
		return errors.New("cliproxy: the login has already finished")
	}
	if in == nil {
		return errors.New("cliproxy: the login cannot accept a pasted URL")
	}
	// A newline is what the sidecar's reader is waiting for; without it the
	// value sits in the pipe and the login times out looking busy.
	if _, werr := io.WriteString(in, callbackURL+"\n"); werr != nil {
		return fmt.Errorf("cliproxy: hand the callback URL over: %w", werr)
	}
	logx.Printf("cliproxy: %s callback URL handed over by the user", s.provider)
	return nil
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
