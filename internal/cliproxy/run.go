// SPDX-License-Identifier: AGPL-3.0-or-later

package cliproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"memo/internal/logx"
)

// process is one running sidecar. done closes when it has exited.
type process struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error // valid after done closes

	stopping bool // set under Manager.mu before an intentional stop
}

func (p *process) alive() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

// Timing knobs — vars so tests can shorten them.
var (
	healthTimeout   = 45 * time.Second
	healthInterval  = 150 * time.Millisecond
	restartBackoff  = 2 * time.Second
	restartWindow   = 10 * time.Minute
	maxRestartsInWd = 5
)

// Running reports whether the sidecar process is alive.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.proc != nil && m.proc.alive()
}

// Start launches the sidecar if it is not already running and returns once it
// answers /v1/models (or fails). It is safe to call repeatedly.
func (m *Manager) Start(ctx context.Context) error {
	bin, _, err := m.Binary()
	if err != nil {
		return err
	}

	m.mu.Lock()
	if m.proc != nil && m.proc.alive() {
		m.mu.Unlock()
		return nil
	}
	if err := m.ensureIdentityLocked(); err != nil {
		m.mu.Unlock()
		return err
	}
	p, err := m.spawnLocked(bin)
	m.mu.Unlock()
	if err != nil {
		return err
	}

	logx.GoRecover("cliproxy.monitor", func() { m.monitor(p, bin) })
	return m.waitHealthy(ctx, p)
}

// spawnLocked writes the config and execs the binary.
func (m *Manager) spawnLocked(bin string) (*process, error) {
	if err := os.MkdirAll(m.authDir(), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(m.configPath(), []byte(m.renderConfigLocked()), 0o600); err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, "-config", m.configPath())
	cmd.Dir = m.dir
	cmd.SysProcAttr = sysProcAttr()
	out := &lineLogger{prefix: "cliproxy"}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("cliproxy: start: %w", err)
	}
	p := &process{cmd: cmd, done: make(chan struct{})}
	m.proc = p
	go func() { // reap — an unwaited child is a zombie
		p.err = cmd.Wait()
		close(p.done)
	}()
	return p, nil
}

// renderConfigLocked is the whole config CLIProxyAPI sees: loopback only, one
// client key, our own auth dir, management API off (an empty secret disables
// it), plugins and usage statistics at their disabled defaults.
func (m *Manager) renderConfigLocked() string {
	return fmt.Sprintf(`config-version: 8
server:
  host: "127.0.0.1"
  port: %d
management:
  allow-remote: false
  secret-key: ""
  disable-control-panel: true
access:
  api-keys:
    - %s
oauth:
  auth-dir: %s
`, m.st.Port, strconv.Quote(m.st.APIKey), strconv.Quote(m.authDir()))
}

// waitHealthy polls /v1/models until it answers 200, the process dies, the
// context ends or healthTimeout passes.
func (m *Manager) waitHealthy(ctx context.Context, p *process) error {
	deadline := time.Now().Add(healthTimeout)
	for {
		if !p.alive() {
			return fmt.Errorf("cliproxy: sidecar exited during startup: %v", p.err)
		}
		if m.ping(ctx) == nil {
			return nil
		}
		if time.Now().After(deadline) {
			m.Stop()
			return fmt.Errorf("cliproxy: sidecar did not become healthy within %s", healthTimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(healthInterval):
		}
	}
}

func (m *Manager) ping(ctx context.Context) error {
	base, key := m.BaseURL(), m.APIKey()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// monitor restarts the sidecar if it dies on its own, a few times, so a crash
// is not a dead chat. An intentional Stop is never "restarted".
func (m *Manager) monitor(p *process, bin string) {
	for {
		<-p.done

		m.mu.Lock()
		if p.stopping || m.proc != p {
			m.mu.Unlock()
			return
		}
		now := time.Now().Unix()
		kept := m.restarts[:0]
		for _, t := range m.restarts {
			if now-t < int64(restartWindow/time.Second) {
				kept = append(kept, t)
			}
		}
		m.restarts = kept
		if len(m.restarts) >= maxRestartsInWd {
			logx.Printf("cliproxy: sidecar keeps dying (%d restarts in %s) — giving up", len(m.restarts), restartWindow)
			m.proc = nil
			m.mu.Unlock()
			return
		}
		m.restarts = append(m.restarts, now)
		m.mu.Unlock()

		logx.Printf("cliproxy: sidecar exited (%v) — restarting in %s", p.err, restartBackoff)
		time.Sleep(restartBackoff)

		m.mu.Lock()
		if m.proc != p || p.stopping {
			m.mu.Unlock()
			return
		}
		np, err := m.spawnLocked(bin)
		m.mu.Unlock()
		if err != nil {
			logx.Printf("cliproxy: restart failed: %v", err)
			return
		}
		p = np
	}
}

// Stop ends the sidecar (and any login helper). Idempotent.
func (m *Manager) Stop() {
	m.mu.Lock()
	p := m.proc
	if p != nil {
		p.stopping = true
	}
	m.proc = nil
	l := m.login
	m.login = nil
	m.mu.Unlock()

	if l != nil {
		l.cancel()
	}
	if p != nil && p.alive() {
		killTree(p.cmd)
		select {
		case <-p.done:
		case <-time.After(5 * time.Second):
		}
	}
}

// lineLogger forwards the child's output to Memo's log, one line at a time.
// CLIProxyAPI does not print tokens at its default log level; request logging
// is off in our config.
type lineLogger struct {
	prefix string
	mu     sync.Mutex
	buf    []byte
}

func (l *lineLogger) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf = append(l.buf, p...)
	for {
		i := strings.IndexByte(string(l.buf), '\n')
		if i < 0 {
			break
		}
		if line := strings.TrimRight(string(l.buf[:i]), "\r"); line != "" {
			logx.Printf("%s: %s", l.prefix, line)
		}
		l.buf = l.buf[i+1:]
	}
	return len(p), nil
}
