// SPDX-License-Identifier: AGPL-3.0-or-later

package webchat

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"

	"memo/internal/logx"
)

//go:embed page.js
var pageJS string

// Timings. Package vars so tests can shrink them; production values are what
// the sites need in practice.
var (
	pollEvery        = 150 * time.Millisecond
	readyTimeout     = 30 * time.Second // page loaded far enough to type into
	startTimeout     = 60 * time.Second // from "send" to the first sign of a reply
	stableFor        = 1200 * time.Millisecond
	idleGenTimeout   = 180 * time.Second // generating, but nothing new for this long
	humanCheckWindow = 4 * time.Minute   // how long a person gets to finish a human check
	hardCap          = 20 * time.Minute
)

// Service drives one site. At most one message is in flight per site: the
// page has a single composer, so two concurrent turns would type into each
// other.
type Service struct {
	site *Site

	// resolve finds the browser binary (seam for tests).
	resolve func(context.Context) (string, error)

	mu   sync.Mutex // guards t, connecting, lastErr
	t    *tab
	busy sync.Mutex // held for the length of one turn

	connecting    bool   // a background Connect is waiting for the person
	lastErr       string // sentence from the last failed Connect, shown by Status
	connectCancel context.CancelFunc
	connectDone   chan struct{} // closed when the latest background Connect has ended
}

var (
	servicesMu sync.Mutex
	services   = map[string]*Service{}
)

// For returns the Service for a site, creating it on first use. Creating one
// never starts a browser.
func For(site *Site) *Service {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	if s, ok := services[site.ID]; ok {
		return s
	}
	s := &Service{site: site, resolve: resolveBrowser}
	services[site.ID] = s
	return s
}

// CloseAll shuts every browser down (application exit, Beta switched off).
func CloseAll() {
	servicesMu.Lock()
	list := make([]*Service, 0, len(services))
	for _, s := range services {
		list = append(list, s)
	}
	servicesMu.Unlock()
	for _, s := range list {
		s.Close()
	}
}

// Close stops this site's browser. The profile folder (and therefore the
// sign-in) stays on disk.
func (s *Service) Close() {
	s.mu.Lock()
	t := s.t
	s.t = nil
	s.mu.Unlock()
	t.close()
}

// ensure returns a live tab, starting (or restarting, after the browser was
// closed by hand or crashed) one when needed.
func (s *Service) ensure(ctx context.Context) (*tab, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.t.alive() {
		return s.t, nil
	}
	s.t.close()
	t, err := startTab(ctx, s.site.ID, s.resolve)
	if err != nil {
		s.t = nil
		var e *Err
		if asErr(err, &e) {
			e.Site = s.site.Name
		}
		return nil, err
	}
	s.t = t
	return t, nil
}

// reset drops the browser after a failure that may have left it in a state
// the next turn cannot trust. The next call starts clean.
func (s *Service) reset() { s.Close() }

// ---- page protocol ---------------------------------------------------------

type probeResult struct {
	State    string `json:"state"` // ready | login | challenge | unknown
	Host     string `json:"host"`
	URL      string `json:"url"`
	Composer string `json:"composer"`
	Error    string `json:"error"`
}

type snapResult struct {
	Count      int    `json:"count"`
	Last       string `json:"last"`
	Generating bool   `json:"generating"`
	Matched    string `json:"matched"`
	URL        string `json:"url"`
	Error      string `json:"error"`
}

type fillResult struct {
	OK     bool   `json:"ok"`
	Len    int    `json:"len"`
	Reason string `json:"reason"`
	Error  string `json:"error"`
}

type sendResult struct {
	Clicked  bool   `json:"clicked"`
	Disabled bool   `json:"disabled"`
	Error    string `json:"error"`
}

type diagResult struct {
	Quota     bool        `json:"quota"`
	QuotaLine string      `json:"quotaLine"`
	Probe     probeResult `json:"probe"`
	Error     string      `json:"error"`
}

// call runs one page operation and decodes its JSON answer into out. A
// protocol failure and an error reported by the script itself both come back
// as a plain error; callers decide what it means.
func (s *Service) call(ctx context.Context, t *tab, op string, arg any, out any) error {
	cfg, _ := json.Marshal(s.site.pageConfig())
	argJSON, _ := json.Marshal(arg)
	opJSON, _ := json.Marshal(op)
	expr := fmt.Sprintf("(%s)(%s,%s,%s)", pageJS, cfg, opJSON, argJSON)
	var raw string
	if err := t.run(ctx, 15*time.Second, chromedp.Evaluate(expr, &raw)); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return fmt.Errorf("page answer unreadable: %w", err)
	}
	return nil
}

func (s *Service) probe(ctx context.Context, t *tab) (probeResult, error) {
	var p probeResult
	err := s.call(ctx, t, "probe", nil, &p)
	if err == nil && p.Error != "" {
		err = fmt.Errorf("probe: %s", p.Error)
	}
	return p, err
}

func (s *Service) snap(ctx context.Context, t *tab) (snapResult, error) {
	var r snapResult
	err := s.call(ctx, t, "snapshot", nil, &r)
	if err == nil && r.Error != "" {
		err = fmt.Errorf("snapshot: %s", r.Error)
	}
	return r, err
}

// ---- navigation and state --------------------------------------------------

func (s *Service) navigate(ctx context.Context, t *tab, url string) error {
	return t.run(ctx, 30*time.Second, chromedp.Navigate(url))
}

// waitState polls until the page is in a decisive state (anything other than
// "unknown") or the timeout passes. A page still loading reads "unknown", so
// "unknown" is only reported once the timeout ends.
func (s *Service) waitState(ctx context.Context, t *tab, timeout time.Duration) (probeResult, error) {
	deadline := time.Now().Add(timeout)
	var last probeResult
	for {
		p, err := s.probe(ctx, t)
		if err == nil {
			last = p
			if p.State != "unknown" {
				return p, nil
			}
		}
		if ctx.Err() != nil {
			return last, ctx.Err()
		}
		if time.Now().After(deadline) {
			if err != nil {
				return last, err
			}
			return last, nil
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(pollEvery * 2):
		}
	}
}

// openFresh loads the site's start page and returns once it is usable. It
// is the single place that turns "what the page shows" into an error kind.
// wait=true gives a person time to finish sign-in or a human check in the
// visible window (Connect); wait=false fails fast (a chat turn).
func (s *Service) openFresh(ctx context.Context, t *tab, wait bool) error {
	if err := s.navigate(ctx, t, s.site.StartURL); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return newErr(KindBrowser, s.site.Name, err)
	}
	p, err := s.waitState(ctx, t, readyTimeout)
	if err != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	switch p.State {
	case "ready":
		return nil
	case "login", "challenge":
		if !wait {
			t.bringToFront(ctx)
			kind := KindNeedsLogin
			if p.State == "challenge" {
				kind = KindChallenge
			}
			return newErr(kind, s.site.Name, nil)
		}
		return s.waitForHuman(ctx, t, p.State)
	default:
		return newErr(KindPageChanged, s.site.Name, fmt.Errorf("no recognisable state at %s (%v)", p.URL, err))
	}
}

// waitForHuman leaves the window in front and polls until the person has
// signed in / finished the check, up to humanCheckWindow. Nothing is clicked
// or typed on their behalf.
func (s *Service) waitForHuman(ctx context.Context, t *tab, first string) error {
	t.bringToFront(ctx)
	deadline := time.Now().Add(humanCheckWindow)
	state := first
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
		// The person closed the window (or the browser died): stop waiting at
		// once instead of polling a dead page for the rest of the window, which
		// left Connect "running" and made pressing it again do nothing.
		if !t.alive() {
			return newErr(KindClosed, s.site.Name, nil)
		}
		p, err := s.probe(ctx, t)
		if err != nil {
			continue
		}
		state = p.State
		if p.State == "ready" {
			return nil
		}
	}
	kind := KindNeedsLogin
	if state == "challenge" {
		kind = KindChallenge
	}
	return newErr(kind, s.site.Name, nil)
}

// ---- public operations -----------------------------------------------------

// Status describes what is known without starting a browser.
type Status struct {
	Site      string `json:"site"`
	Name      string `json:"name"`
	Running   bool   `json:"running"`  // a browser is open
	State     string `json:"state"`    // "idle" (not started), or ready | login | challenge | unknown
	Verified  bool   `json:"verified"` // selectors confirmed against a real signed-in page
	Installed bool   `json:"installed"`
	// Connecting is true while the window is open waiting for the person to
	// sign in; Error holds the sentence from the last Connect that failed.
	Connecting bool   `json:"connecting"`
	Error      string `json:"error,omitempty"`
}

// Status never launches anything; with no browser open the state is "idle"
// (nobody knows whether the stored sign-in is still good until it is used).
func (s *Service) Status(ctx context.Context) Status {
	st := Status{Site: s.site.ID, Name: s.site.Name, Verified: s.site.Verified, State: "idle"}
	if _, err := s.resolve(ctx); err == nil {
		st.Installed = true
	}
	s.mu.Lock()
	t := s.t
	st.Connecting, st.Error = s.connecting, s.lastErr
	s.mu.Unlock()
	if t.alive() && !st.Connecting {
		st.Running = true
		if p, err := s.probe(ctx, t); err == nil {
			st.State = p.State
		} else {
			st.State = "unknown"
		}
	}
	return st
}

// Connect opens the site in the Memo window and waits for the person to
// sign in (or finish a check), then tucks the window away. It returns nil
// once the composer is usable.
func (s *Service) Connect(ctx context.Context) (err error) {
	defer recoverTo(&err, s)
	if !s.busy.TryLock() {
		return newErr(KindBusy, s.site.Name, nil)
	}
	defer s.busy.Unlock()
	t, err := s.ensure(ctx)
	if err != nil {
		return err
	}
	t.bringToFront(ctx)
	if err := s.openFresh(ctx, t, true); err != nil {
		// Whatever the step that failed, a window that is gone means the
		// person closed it: say that, not a generic browser failure.
		if !t.alive() && !isCtxErr(err) {
			return newErr(KindClosed, s.site.Name, err)
		}
		return s.classify(err)
	}
	t.setWindow(ctx, browser.WindowStateMinimized)
	return nil
}

// StartConnect begins Connect in the background and returns at once; the
// person watches Status. Pressing it again while a connect is still waiting
// replaces that connect (the old one is cancelled and fully ended first), so a
// stuck or abandoned attempt can never block a new one.
func (s *Service) StartConnect() {
	ctx, cancel := context.WithTimeout(context.Background(), humanCheckWindow+readyTimeout+30*time.Second)
	done := make(chan struct{})

	s.mu.Lock()
	if s.connectCancel != nil {
		s.connectCancel()
	}
	prev := s.connectDone
	s.connectCancel, s.connectDone = cancel, done
	s.connecting, s.lastErr = true, ""
	s.mu.Unlock()

	go func() {
		defer close(done)
		defer cancel()
		if prev != nil {
			select {
			case <-prev:
			case <-time.After(15 * time.Second):
			}
		}
		err := s.Connect(ctx)
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.connectDone != done {
			return // superseded: the newer connect owns the status now
		}
		s.connecting = false
		if err != nil && !isCtxErr(err) {
			if e := (*Err)(nil); asErr(err, &e) {
				s.lastErr = e.Message()
			} else {
				s.lastErr = newErr(KindBrowser, s.site.Name, err).Message()
			}
		}
	}()
}

// Disconnect closes the browser. The saved sign-in stays unless forget is
// true, which also removes the profile folder.
func (s *Service) Disconnect(forget bool) error {
	s.mu.Lock()
	if s.connectCancel != nil {
		s.connectCancel() // the person asked to stop: no "window closed" complaint
	}
	s.mu.Unlock()
	s.Close()
	if forget {
		return removeProfile(s.site.ID)
	}
	return nil
}

// Ask sends prompt as a new conversation and streams the reply's markdown
// through emit as it grows. emit is called from the caller's goroutine.
// Success returns nil after the last delta.
func (s *Service) Ask(ctx context.Context, prompt string, emit func(delta string)) (err error) {
	defer recoverTo(&err, s)
	if strings.TrimSpace(prompt) == "" {
		return newErr(KindPageChanged, s.site.Name, fmt.Errorf("empty prompt"))
	}
	if !s.busy.TryLock() {
		return newErr(KindBusy, s.site.Name, nil)
	}
	defer s.busy.Unlock()

	ctx, cancel := context.WithTimeout(ctx, hardCap)
	defer cancel()

	t, err := s.ensure(ctx)
	if err != nil {
		return err
	}
	if err := s.turn(ctx, t, prompt, emit); err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil {
			// The overall cap ended the turn (the caller's own context, had it
			// ended first, would be cancelled/deadline too — both mean "too long").
			err = newErr(KindTimeout, s.site.Name, err)
		}
		err = s.classify(err)
		// A turn that failed because the browser or the page misbehaved may
		// have left a half-typed composer or a dead tab; start clean next
		// time. Quota, login and human-check outcomes say nothing about the
		// browser's health, so the window stays.
		switch KindOf(err) {
		case KindBrowser, KindPageChanged, KindTimeout:
			s.reset()
		}
		return err
	}
	return nil
}

func (s *Service) turn(ctx context.Context, t *tab, prompt string, emit func(string)) error {
	if err := s.openFresh(ctx, t, false); err != nil {
		return err
	}
	before, err := s.snap(ctx, t)
	if err != nil {
		return newErr(KindBrowser, s.site.Name, err)
	}

	var fr fillResult
	if err := s.call(ctx, t, "fill", prompt, &fr); err != nil {
		return newErr(KindBrowser, s.site.Name, err)
	}
	// A composer that took less than most of the text (a page that swallowed
	// the paste) would send a truncated question and look like a model fault.
	if !fr.OK || fr.Len < len(strings.TrimSpace(prompt))/2 {
		return newErr(KindPageChanged, s.site.Name, fmt.Errorf("composer filled %d of %d chars (%s%s)", fr.Len, len(prompt), fr.Reason, fr.Error))
	}

	if err := s.submit(ctx, t); err != nil {
		return err
	}
	return s.stream(ctx, t, before, emit)
}

// submit presses the send button once it is enabled (it stays disabled until
// the page has registered the text), falling back to Enter.
func (s *Service) submit(ctx context.Context, t *tab) error {
	deadline := time.Now().Add(6 * time.Second)
	for {
		var sr sendResult
		if err := s.call(ctx, t, "send", nil, &sr); err != nil {
			return newErr(KindBrowser, s.site.Name, err)
		}
		if sr.Clicked {
			return nil
		}
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollEvery):
		}
	}
	// No usable send button: Enter submits in both supported composers.
	if err := t.run(ctx, 5*time.Second, chromedp.KeyEvent(kb.Enter)); err != nil {
		return newErr(KindBrowser, s.site.Name, err)
	}
	return nil
}

// stream watches the newest assistant message and emits what is new. A
// reply is finished when the page is not generating AND the text has stopped
// changing for stableFor (a stop button that blinks off between tool steps
// must not end the turn early).
func (s *Service) stream(ctx context.Context, t *tab, before snapResult, emit func(string)) error {
	var (
		emitted    string // what has been handed to emit
		lastText   string
		lastChange = time.Now()
		sawReply   bool
		started    = time.Now()
	)
	for {
		if ctx.Err() != nil {
			// Ask the page to stop, on a context that outlives the cancelled one.
			sctx, c := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			var ignore map[string]any
			_ = s.call(sctx, t, "stop", nil, &ignore)
			c()
			return ctx.Err()
		}
		cur, err := s.snap(ctx, t)
		if err != nil {
			if ctx.Err() != nil {
				continue
			}
			return newErr(KindBrowser, s.site.Name, err)
		}

		// A reply exists when there are more assistant messages than before,
		// or (reused view) the last one's text differs from what it was.
		isNew := cur.Count > before.Count || (cur.Count > 0 && cur.Last != before.Last && cur.Last != "")
		if isNew && (cur.Last != "" || cur.Generating) {
			sawReply = true
		}
		if sawReply {
			if cur.Last != lastText {
				lastText = cur.Last
				lastChange = time.Now()
			}
			// Hold back the unfinished last line: markdown conversion can
			// still rewrite it (a list item gaining its bullet, a fence
			// closing), and emitted text cannot be taken back.
			safe := lastText
			if cur.Generating {
				safe = stablePrefix(lastText)
			}
			if len(safe) > len(emitted) && strings.HasPrefix(safe, emitted) {
				emit(safe[len(emitted):])
				emitted = safe
			}
			if !cur.Generating && time.Since(lastChange) >= stableFor && lastText != "" {
				if len(lastText) > len(emitted) && strings.HasPrefix(lastText, emitted) {
					emit(lastText[len(emitted):])
				}
				return nil
			}
			if cur.Generating && time.Since(lastChange) > idleGenTimeout {
				return newErr(KindTimeout, s.site.Name, nil)
			}
		} else if time.Since(started) > startTimeout {
			return s.whyNoReply(ctx, t)
		}

		select {
		case <-ctx.Done():
		case <-time.After(pollEvery):
		}
	}
}

// whyNoReply explains a reply that never started, from what the page says.
func (s *Service) whyNoReply(ctx context.Context, t *tab) error {
	var d diagResult
	if err := s.call(ctx, t, "diagnose", nil, &d); err == nil {
		switch {
		case d.Probe.State == "login":
			return newErr(KindNeedsLogin, s.site.Name, nil)
		case d.Probe.State == "challenge":
			t.bringToFront(ctx)
			return newErr(KindChallenge, s.site.Name, nil)
		case d.Quota:
			e := newErr(KindQuota, s.site.Name, nil)
			e.Extra = d.QuotaLine
			return e
		}
	}
	return newErr(KindPageChanged, s.site.Name, fmt.Errorf("no assistant message appeared after sending"))
}

// stablePrefix is text up to and including its last newline: whole lines
// only. A reply with no newline yet is held until it has one or finishes.
func stablePrefix(s string) string {
	i := strings.LastIndex(s, "\n")
	if i < 0 {
		return ""
	}
	return s[:i+1]
}

// classify turns whatever a step returned into an *Err the caller can show.
// Context cancellation is returned as is — it is the caller's own decision,
// not a failure to explain.
func (s *Service) classify(err error) error {
	if err == nil {
		return nil
	}
	var e *Err
	if asErr(err, &e) {
		if e.Site == "" {
			e.Site = s.site.Name
		}
		return e
	}
	if isCtxErr(err) {
		return err
	}
	logx.Info("WEBCHAT: step failed", "site", s.site.ID, "error", err)
	return newErr(KindBrowser, s.site.Name, err)
}
