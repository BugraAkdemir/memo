package browserengine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"memo/internal/config"
	"memo/internal/logx"
)

// sessionIdleTimeout is how long an interactive Session may sit unused
// before it closes itself. A var (not const) so tests can shrink it instead
// of waiting minutes. Nothing today guarantees the agent calls
// browser_close when it's done, so this is the only backstop against a
// forgotten session holding a Chromium process open indefinitely.
var sessionIdleTimeout = 5 * time.Minute

var errSessionClosed = errors.New("browsersession: session is closed")

// Session is a long-lived, interactive, sandboxed Chromium tab the agent
// drives directly via chromedp — navigate, screenshot, (click/type/scroll
// land in a later checkpoint). Deliberately independent of Manager's
// engine/Fetch machinery in browserengine.go: a brand-new OS process with
// its own dedicated profile directory, same isolation guarantee that
// package's doc comment already states for the Fetch engine, but never
// sharing a process with it — Fetch's one-shot/keep-alive lifecycle and a
// long-lived interactive tab don't mix safely in one browser process.
//
// v1 is deliberately single-session: Manager holds at most one Session at a
// time (see Manager.StartSession below) — no session-id plumbing anywhere
// in the agent tool surface.
type Session struct {
	mu          sync.Mutex
	allocCtx    context.Context
	allocCancel context.CancelFunc
	tabCtx      context.Context
	tabCancel   context.CancelFunc
	profileDir  string
	startedAt   time.Time
	lastUsed    time.Time
	idleTimer   *time.Timer
	onIdle      func()
	closed      bool
}

// newSession sits behind a package var for the same reason newEngine does
// in browserengine.go — tests substitute a fake instead of launching a real
// browser. onIdle is called (with the Session that timed out) once, off the
// idle timer, so Manager can drop its reference to an already-dead session.
var newSession = func(ctx context.Context, onIdle func(*Session)) (*Session, error) {
	return startSession(ctx, onIdle)
}

func startSession(ctx context.Context, onIdle func(*Session)) (*Session, error) {
	baseDir := config.DataPath("browsersession")
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, fmt.Errorf("browsersession: create base dir: %w", err)
	}
	profileDir, err := os.MkdirTemp(baseDir, "session-")
	if err != nil {
		return nil, fmt.Errorf("browsersession: create profile dir: %w", err)
	}

	opts := append(append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...),
		chromedp.UserDataDir(profileDir),
		chromedp.WindowSize(1280, 800),
		chromedp.Flag("disable-extensions", true),
	)
	// allocCtx is deliberately NOT derived from ctx as-is (ctx is normally an
	// HTTP request/tool-call context) — the session must outlive the single
	// call that started it, the same reasoning StartInstall's
	// context.WithoutCancel already documents in browserengine.go.
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.WithoutCancel(ctx), opts...)
	tabCtx, tabCancel := chromedp.NewContext(allocCtx)

	if err := chromedp.Run(tabCtx); err != nil {
		tabCancel()
		allocCancel()
		_ = os.RemoveAll(profileDir)
		return nil, fmt.Errorf("browsersession: start: %w", err)
	}

	s := &Session{
		allocCtx:    allocCtx,
		allocCancel: allocCancel,
		tabCtx:      tabCtx,
		tabCancel:   tabCancel,
		profileDir:  profileDir,
		startedAt:   time.Now(),
		lastUsed:    time.Now(),
	}
	s.onIdle = func() { onIdle(s) }
	s.armIdleTimer()
	logx.Info("BROWSERSESSION: started", "profile", profileDir)
	return s, nil
}

// armIdleTimer locks: time.AfterFunc arms the timer (and thus can start
// racing toward calling onIdleTimeout, in its own goroutine) before it
// returns the *Timer this assigns into s.idleTimer — without the lock here,
// that assignment can race against close()'s locked read of the same field
// if the timeout is short enough to fire near-immediately (caught by
// -race, not just theoretical).
func (s *Session) armIdleTimer() {
	s.mu.Lock()
	s.idleTimer = time.AfterFunc(sessionIdleTimeout, s.onIdleTimeout)
	s.mu.Unlock()
}

func (s *Session) onIdleTimeout() {
	logx.Info("BROWSERSESSION: idle timeout, closing", "profile", s.profileDir)
	_ = s.close()
	if s.onIdle != nil {
		s.onIdle()
	}
}

// touch resets the idle timer — called at the start of every public action
// below so an actively-used session never times out mid-task.
func (s *Session) touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.lastUsed = time.Now()
	if s.idleTimer != nil {
		s.idleTimer.Reset(sessionIdleTimeout)
	}
}

func (s *Session) checkOpen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errSessionClosed
	}
	return nil
}

// Navigate loads url in the session's tab.
func (s *Session) Navigate(ctx context.Context, rawURL string) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	s.touch()
	if err := chromedp.Run(s.tabCtx, chromedp.Navigate(rawURL)); err != nil {
		return fmt.Errorf("browsersession: navigate %s: %w", rawURL, err)
	}
	return nil
}

// Screenshot captures the current viewport as PNG bytes.
func (s *Session) Screenshot(ctx context.Context) ([]byte, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	s.touch()
	var buf []byte
	if err := chromedp.Run(s.tabCtx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return nil, fmt.Errorf("browsersession: screenshot: %w", err)
	}
	return buf, nil
}

// Close tears the session down: cancels the tab and allocator contexts
// (which stops the Chromium process) and removes its dedicated profile
// directory. Idempotent — safe to call more than once (e.g. once from the
// agent's browser_close tool and again, harmlessly, from the idle timer or
// App shutdown racing it).
func (s *Session) Close() error {
	return s.close()
}

func (s *Session) close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	if s.idleTimer != nil {
		s.idleTimer.Stop()
	}
	s.mu.Unlock()

	if s.tabCancel != nil {
		s.tabCancel()
	}
	if s.allocCancel != nil {
		s.allocCancel()
	}
	var err error
	if s.profileDir != "" {
		err = os.RemoveAll(s.profileDir)
	}
	logx.Info("BROWSERSESSION: closed", "profile", s.profileDir)
	return err
}

var pngMagic = []byte{0x89, 0x50, 0x4E, 0x47}

// looksLikePNG is a cheap sanity check used by tests/callers that want to
// confirm Screenshot returned real image bytes without decoding the image.
func looksLikePNG(b []byte) bool {
	return len(b) >= 4 && bytes.Equal(b[:4], pngMagic)
}

// StartSession launches (or returns the existing) interactive browser
// session. Only one may run at a time for v1: a second call while one is
// already running just hands back the existing Session — a browser_navigate
// followed by a browser_click in the same agent turn must find the same
// session without the model tracking a session id it was never given.
func (m *Manager) StartSession(ctx context.Context) (*Session, error) {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()
	if m.session != nil {
		return m.session, nil
	}
	s, err := newSession(ctx, m.dropIdleSession)
	if err != nil {
		return nil, err
	}
	m.session = s
	return s, nil
}

// GetSession returns the current interactive session, if one is running.
func (m *Manager) GetSession() (*Session, bool) {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()
	return m.session, m.session != nil
}

// StopSession closes the interactive session, if one is running. Idempotent
// no-op otherwise — meant to be wired into App shutdown unconditionally,
// the same way Stop (for the Fetch engine, above) already is.
func (m *Manager) StopSession() error {
	m.sessionMu.Lock()
	s := m.session
	m.session = nil
	m.sessionMu.Unlock()
	if s == nil {
		return nil
	}
	return s.close()
}

// dropIdleSession is the idle timer's callback (threaded through newSession
// above) — clears Manager's reference once a session closes itself after
// sessionIdleTimeout of inactivity, so the next StartSession launches a
// fresh one instead of handing back a handle to an already-dead process.
// Checks identity first: if Manager's session has already moved on (e.g.
// StopSession raced the idle timer), there's nothing to clear.
func (m *Manager) dropIdleSession(s *Session) {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()
	if m.session == s {
		m.session = nil
	}
}
