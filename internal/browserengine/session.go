package browserengine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/BugraAkdemir/gosearch/browser"
	"github.com/chromedp/chromedp"
	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/truncate"
)

// sessionIdleTimeout is how long an interactive Session may sit unused
// before it closes itself. A var (not const) so tests can shrink it instead
// of waiting minutes. Nothing today guarantees the agent calls
// browser_close when it's done, so this is the only backstop against a
// forgotten session holding a Chromium process open indefinitely.
var sessionIdleTimeout = 5 * time.Minute

// actionTimeout bounds every single chromedp action below (Navigate, Click,
// Type, Scroll, Screenshot) and the initial launch handshake. A var (not
// const) so tests can shrink it. This matters more than it looks: every
// action runs against s.tabCtx, which is deliberately NOT derived from the
// per-call ctx argument (see startSession's doc comment — the session must
// outlive a single HTTP request), so without a bound of its own here, a
// hung chromedp action (a click that never resolves, a page that never
// finishes loading) would block forever with nothing — not even the
// caller's own context deadline — able to stop it. Caught empirically: an
// early version of this file had no such bound and a real Chromium click
// test hung indefinitely under go test's own -race build until killed by
// hand.
var actionTimeout = 30 * time.Second

var errSessionClosed = errors.New("browsersession: session is closed")

// ErrBrowserNotInstalled wraps a resolveExecutable failure — no
// Chromium-family browser could be found at all (system discovery came up
// empty, and nothing was installed via Settings' one-click browser-engine
// install either). Exported so callers (e.g. the agent tools) can detect
// this specific case with errors.Is and surface an actionable message
// instead of a raw exec error.
var ErrBrowserNotInstalled = errors.New("browsersession: no chromium-family browser found")

// resolveExecutable finds the binary Session should launch — a package var
// (not a direct call) so tests can substitute a fake, same convention as
// newEngine/installFn in browserengine.go. Deliberately reuses
// gosearch/browser's OWN resolution ("explicit path > embedded archive >
// system discovery > opt-in download", per browser.New's doc comment)
// instead of a second, independent chromedp discovery — a user who already
// installed the engine via Settings' one-click flow must not need a second,
// separate Chromium found some other way, and one found via plain system
// PATH discovery should resolve identically either way. Cheap: browser.New
// resolves the path but does not launch a process ("The browser process
// starts lazily on first use, not in New" — its own doc comment), so this
// never pays real browser-startup cost just to read a path.
var resolveExecutable = func(ctx context.Context) (string, error) {
	e, err := browser.New(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrBrowserNotInstalled, err)
	}
	defer e.Close()
	return e.Executable(), nil
}

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
	execPath, err := resolveExecutable(ctx)
	if err != nil {
		return nil, err
	}

	baseDir := config.DataPath("browsersession")
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, fmt.Errorf("browsersession: create base dir: %w", err)
	}
	profileDir, err := os.MkdirTemp(baseDir, "session-")
	if err != nil {
		return nil, fmt.Errorf("browsersession: create profile dir: %w", err)
	}

	opts := append(append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...),
		chromedp.ExecPath(execPath),
		chromedp.UserDataDir(profileDir),
		// Matches BrowserPane's own width in frontend/lib/widgets/agent/
		// browser_pane.dart (kept in sync by hand — Go and Dart can't share
		// a literal). A fixed 1280x800 desktop viewport here was the first
		// version, and it looked broken in the pane: a responsive page
		// always rendered its desktop layout regardless of how narrow the
		// pane actually displays it, then got squeezed down to ~30% size
		// with a large dead-space letterbox below it once BoxFit.contain
		// fit its now-oversized aspect ratio into the pane's tall, narrow
		// one. Matching the aspect ratio here means a responsive site
		// renders the same narrow layout the pane actually shows, and fills
		// the available space instead of shrinking into a corner of it.
		chromedp.WindowSize(420, 900),
		chromedp.Flag("disable-extensions", true),
	)
	// allocCtx is deliberately NOT derived from ctx as-is (ctx is normally an
	// HTTP request/tool-call context) — the session must outlive the single
	// call that started it, the same reasoning StartInstall's
	// context.WithoutCancel already documents in browserengine.go.
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.WithoutCancel(ctx), opts...)
	tabCtx, tabCancel := chromedp.NewContext(allocCtx)

	// This first Run call is deliberately NOT given a bounded/derived child
	// context the way every later action below is (see runCtx) — chromedp
	// ties actual browser-process allocation to whichever context object is
	// passed to the Run call that triggers it (the first one), not just to
	// tabCtx as a logical parent. A child context cancelled right after this
	// call returns — even a WithTimeout one, cancelled via its own
	// CancelFunc immediately on success — kills the underlying CDP
	// connection along with it, so every later action then fails with
	// "context canceled" despite tabCtx itself being untouched. Caught
	// empirically: an earlier version of this function bounded this exact
	// call and broke every real-Chromium test that followed it.
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

// runCtx builds the context a single chromedp action actually runs under:
// rooted in the session's own tabCtx (so Session.Close cancels it right
// away, and it survives past whatever single call's ctx started the
// session — see startSession's doc comment), capped at actionTimeout so a
// hung action can never block forever, and additionally cancelled early if
// the caller's ctx is cancelled first (e.g. the HTTP request driving this
// tool call was aborted). Always pair with `defer cancel()`.
func (s *Session) runCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	runCtx, cancel := context.WithTimeout(s.tabCtx, actionTimeout)
	stop := context.AfterFunc(ctx, cancel)
	return runCtx, func() {
		stop()
		cancel()
	}
}

// Navigate loads url in the session's tab.
func (s *Session) Navigate(ctx context.Context, rawURL string) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	s.touch()
	runCtx, cancel := s.runCtx(ctx)
	defer cancel()
	if err := chromedp.Run(runCtx, chromedp.Navigate(rawURL)); err != nil {
		return fmt.Errorf("browsersession: navigate %s: %w", rawURL, err)
	}
	return nil
}

// Click clicks the first element matching a CSS selector. Waits for the
// element to be visible first (chromedp.Click's default behavior) so a
// click right after Navigate on a still-rendering page doesn't just miss.
func (s *Session) Click(ctx context.Context, selector string) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	s.touch()
	runCtx, cancel := s.runCtx(ctx)
	defer cancel()
	if err := chromedp.Run(runCtx, chromedp.Click(selector, chromedp.ByQuery)); err != nil {
		return fmt.Errorf("browsersession: click %q: %w", selector, err)
	}
	return nil
}

// ClickAt clicks at fixed viewport coordinates — for elements with no
// stable selector to target (canvas content, an icon inside a shadow DOM,
// etc.), the same escape hatch Claude Code's own browser tools offer
// alongside a selector-based click.
func (s *Session) ClickAt(ctx context.Context, x, y float64) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	s.touch()
	runCtx, cancel := s.runCtx(ctx)
	defer cancel()
	if err := chromedp.Run(runCtx, chromedp.MouseClickXY(x, y)); err != nil {
		return fmt.Errorf("browsersession: click at (%.0f, %.0f): %w", x, y, err)
	}
	return nil
}

// Type focuses the first element matching selector and sends text as key
// events (so it behaves like real typing — triggers input/keydown
// listeners a page might rely on, not just a value assignment).
func (s *Session) Type(ctx context.Context, selector, text string) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	s.touch()
	runCtx, cancel := s.runCtx(ctx)
	defer cancel()
	if err := chromedp.Run(runCtx, chromedp.SendKeys(selector, text, chromedp.ByQuery)); err != nil {
		return fmt.Errorf("browsersession: type into %q: %w", selector, err)
	}
	return nil
}

// Scroll scrolls the page by (dx, dy) pixels from its current position.
func (s *Session) Scroll(ctx context.Context, dx, dy int) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	s.touch()
	runCtx, cancel := s.runCtx(ctx)
	defer cancel()
	if err := chromedp.Run(runCtx, chromedp.Evaluate(fmt.Sprintf("window.scrollBy(%d, %d)", dx, dy), nil)); err != nil {
		return fmt.Errorf("browsersession: scroll: %w", err)
	}
	return nil
}

// Screenshot captures the current viewport as PNG bytes.
func (s *Session) Screenshot(ctx context.Context) ([]byte, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	s.touch()
	runCtx, cancel := s.runCtx(ctx)
	defer cancel()
	var buf []byte
	if err := chromedp.Run(runCtx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return nil, fmt.Errorf("browsersession: screenshot: %w", err)
	}
	return buf, nil
}

// CurrentURL reports the tab's current address — used by the direct
// (non-agent) manual-control endpoints so the Flutter pane's URL bar can
// sync to wherever a click/navigate actually landed (a click can itself
// navigate, e.g. following a link), without the agent's own tool-call
// AgentEvents being the only source of the URL.
func (s *Session) CurrentURL(ctx context.Context) (string, error) {
	if err := s.checkOpen(); err != nil {
		return "", err
	}
	runCtx, cancel := s.runCtx(ctx)
	defer cancel()
	var url string
	if err := chromedp.Run(runCtx, chromedp.Location(&url)); err != nil {
		return "", fmt.Errorf("browsersession: current url: %w", err)
	}
	return url, nil
}

// maxPageTextRunes bounds the visible-text portion of PageText's return
// value — same order of magnitude as websearch's own maxFetchContentRunes
// (8000), just enough for the model to know what's on a page without
// itself becoming a second context-bloat source (see
// tools.BrowserScreenshot's doc comment for the first one, and why it
// mattered).
const maxPageTextRunes = 6000

// maxClickableElements bounds how many entries findClickablesJS will emit.
// This list used to be deliberately unbounded — the reasoning was that,
// unlike the prose above it, this is what the model actually needs to
// reliably act, so trimming a long page's text must never also eat into it.
// That's still true for an ordinary page, but it assumed the list itself
// stayed small; a data-heavy page (a big table, a long nav, an infinite-
// scroll feed) can have thousands of matching elements, and at that point
// the list becomes exactly the same context-bloat problem the prose bound
// above already guards against — worse, since each entry costs more tokens
// than a line of prose. 200 is generously above what a real page needs a
// model to act on in one turn; a page with more just says so instead of
// dumping all of them.
const maxClickableElements = 200

// findClickablesJS queries a reasonable set of interactive elements,
// skips anything not actually visible, and stamps each survivor (up to
// maxClickableElements) with a data-memo-ref attribute — a real, unique
// attribute this call just wrote onto the live DOM, not a description or a
// guess. Returns one line per element: its tag, its visible label, and the
// exact CSS selector (`[data-memo-ref="N"]`) that will hit it. This is the
// actual fix for "the agent never clicks anything": browser_get_text alone
// gives prose, which is not something a model can turn into a reliable CSS
// selector — Click #email or button.submit is a guess at best on a real
// page's class soup. Handing back a selector guaranteed to resolve to the
// exact element the model just read the label of closes that gap directly.
var findClickablesJS = fmt.Sprintf(`(function() {
	var MAX = %d;
	var els = document.querySelectorAll('a, button, input, select, textarea, [role="button"], [onclick]');
	var lines = [];
	var i = 0;
	var omitted = 0;
	els.forEach(function(el) {
		var rect = el.getBoundingClientRect();
		var style = window.getComputedStyle(el);
		if (rect.width === 0 || rect.height === 0) return;
		if (style.visibility === 'hidden' || style.display === 'none') return;
		if (i >= MAX) { omitted++; return; }
		i++;
		el.setAttribute('data-memo-ref', String(i));
		var label = (el.innerText || el.value || el.placeholder || el.getAttribute('aria-label') || '').trim().replace(/\s+/g, ' ').slice(0, 60);
		lines.push(i + '. <' + el.tagName.toLowerCase() + '> "' + label + '" -> [data-memo-ref="' + i + '"]');
	});
	if (omitted > 0) {
		lines.push('... and ' + omitted + ' more visible interactive element(s) not shown (cap: ' + MAX + ') — narrow your search or scroll before relying on this list.');
	}
	return lines.join('\n');
})()`, maxClickableElements)

// PageText returns the tab's visible text (document.body.innerText,
// truncated) plus a list of every visible clickable element with a
// ready-to-use CSS selector — see findClickablesJS's doc comment. This is
// the model's only real grounding for what's on a page AND what it can
// act on, given there is no visual/multimodal channel wired up (see
// tools.BrowserScreenshot's doc comment).
func (s *Session) PageText(ctx context.Context) (string, error) {
	if err := s.checkOpen(); err != nil {
		return "", err
	}
	runCtx, cancel := s.runCtx(ctx)
	defer cancel()
	var bodyText, clickables string
	if err := chromedp.Run(runCtx,
		chromedp.Evaluate("document.body.innerText", &bodyText),
		chromedp.Evaluate(findClickablesJS, &clickables),
	); err != nil {
		return "", fmt.Errorf("browsersession: page text: %w", err)
	}
	bodyText = truncate.Text(bodyText, maxPageTextRunes)
	if clickables == "" {
		return bodyText, nil
	}
	return bodyText + "\n\n--- Clickable elements on this page (use the exact selector shown to click one with browser_click) ---\n" + clickables, nil
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
