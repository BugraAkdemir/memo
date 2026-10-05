package browserengine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestSession_Real_NavigateAndScreenshot launches an actual Chromium
// process (bypassing the newSession test seam used elsewhere in this
// package) and drives it for real. No existing test in this repo was found
// launching a real browser engine before this one, so whether a usable
// Chromium is present is genuinely unverified going in — skip cleanly
// rather than fail when none is found, the same graceful-degradation story
// IsInstalled's doc comment already describes for the Fetch engine (a
// system Chromium/Chrome install is picked up automatically; none present
// just means this particular check can't run here).
func TestSession_Real_NavigateAndScreenshot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chromium test in -short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := startSession(ctx, func(*Session) {})
	if err != nil {
		t.Skipf("no usable Chromium found, skipping real-browser test: %v", err)
	}
	defer s.Close()

	if err := s.Navigate(ctx, "https://example.com"); err != nil {
		t.Fatalf("Navigate: %v", err)
	}

	shot, err := s.Screenshot(ctx)
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	if len(shot) == 0 {
		t.Fatal("Screenshot returned empty bytes")
	}
	if !looksLikePNG(shot) {
		t.Errorf("Screenshot bytes (%d bytes) do not start with the PNG magic number", len(shot))
	}
}

// TestSession_Real_ClickTypeScroll drives a real Chromium tab through a
// page served by a local httptest server (deliberately not a data: URL —
// tried that first and hit a real, reproducible hang: chromedp's
// SendKeys/Click on a data:-URL page never resolved and ran out the full
// actionTimeout every time, while the exact same page served over real
// HTTP works immediately; not chased further since it's not something this
// feature needs — agent-driven pages are always real HTTP(S) URLs anyway)
// and confirms Click/Type/Scroll each actually change real browser state,
// not just "returned no error."
func TestSession_Real_ClickTypeScroll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chromium test in -short mode")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body style="height:3000px">
			<input id="name" type="text">
			<button id="btn" onclick="document.title='clicked'">Click me</button>
		</body></html>`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := startSession(ctx, func(*Session) {})
	if err != nil {
		t.Skipf("no usable Chromium found, skipping real-browser test: %v", err)
	}
	defer s.Close()

	if err := s.Navigate(ctx, srv.URL); err != nil {
		t.Fatalf("Navigate: %v", err)
	}

	if err := s.Type(ctx, "#name", "hello"); err != nil {
		t.Fatalf("Type: %v", err)
	}
	var value string
	if err := chromedp.Run(s.tabCtx, chromedp.Value("#name", &value)); err != nil {
		t.Fatalf("read back input value: %v", err)
	}
	if value != "hello" {
		t.Errorf("input value = %q after Type, want %q", value, "hello")
	}

	if err := s.Click(ctx, "#btn"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	var title string
	if err := chromedp.Run(s.tabCtx, chromedp.Title(&title)); err != nil {
		t.Fatalf("read back document.title: %v", err)
	}
	if title != "clicked" {
		t.Errorf("document.title = %q after Click, want %q — the click did not actually register", title, "clicked")
	}

	if err := s.Scroll(ctx, 0, 500); err != nil {
		t.Fatalf("Scroll: %v", err)
	}
	var scrollY float64
	if err := chromedp.Run(s.tabCtx, chromedp.Evaluate("window.scrollY", &scrollY)); err != nil {
		t.Fatalf("read back window.scrollY: %v", err)
	}
	if scrollY < 400 {
		t.Errorf("window.scrollY = %v after scrolling by 500px, want roughly 500 — the scroll did not actually register", scrollY)
	}

	text, err := s.PageText(ctx)
	if err != nil {
		t.Fatalf("PageText: %v", err)
	}
	if !strings.Contains(text, "Click me") {
		t.Errorf("PageText() = %q, want it to contain the button's visible text %q", text, "Click me")
	}

	url, err := s.CurrentURL(ctx)
	if err != nil {
		t.Fatalf("CurrentURL: %v", err)
	}
	if url != srv.URL+"/" {
		t.Errorf("CurrentURL() = %q, want %q", url, srv.URL+"/")
	}
}

// TestSession_Real_PageTextListsClickablesAndTheirSelectorsActuallyWork is
// the real fix for a live, reported bug: the agent kept calling
// browser_screenshot in a loop and never once called browser_click,
// because it had no reliable way to know what CSS selector would hit a
// given button. This test proves the actual mechanism that closes that
// gap: PageText's clickable-elements list gives back a selector
// (data-memo-ref, injected by findClickablesJS) that — unlike a selector
// the model would have to guess from a screenshot or assumed markup —
// really does resolve to the intended element on a real page, even one
// with no meaningful id/class to guess from.
func TestSession_Real_PageTextListsClickablesAndTheirSelectorsActuallyWork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chromium test in -short mode")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Deliberately no id, no meaningful class — a selector like
		// "#submit" or "button.primary" (the kind a model without this
		// tool tends to guess) would not match anything here.
		_, _ = w.Write([]byte(`<html><body>
			<div class="xj9f2 q7">
				<button onclick="document.title='language-switched'">EN</button>
			</div>
		</body></html>`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := startSession(ctx, func(*Session) {})
	if err != nil {
		t.Skipf("no usable Chromium found, skipping real-browser test: %v", err)
	}
	defer s.Close()

	if err := s.Navigate(ctx, srv.URL); err != nil {
		t.Fatalf("Navigate: %v", err)
	}

	text, err := s.PageText(ctx)
	if err != nil {
		t.Fatalf("PageText: %v", err)
	}
	if !strings.Contains(text, "Clickable elements") {
		t.Fatalf("PageText() did not include a clickable-elements section: %q", text)
	}
	if !strings.Contains(text, `"EN"`) {
		t.Errorf("PageText() clickable list did not mention the button's label \"EN\": %q", text)
	}

	// Extract the selector exactly the way a model would have to: find the
	// line naming the button, pull out the [data-memo-ref="N"] selector.
	selRe := regexp.MustCompile(`<button> "EN" -> (\[data-memo-ref="\d+"\])`)
	m := selRe.FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("could not find a selector for the EN button in PageText() output: %q", text)
	}
	selector := m[1]

	if err := s.Click(ctx, selector); err != nil {
		t.Fatalf("Click(%q): %v", selector, err)
	}
	var title string
	if err := chromedp.Run(s.tabCtx, chromedp.Title(&title)); err != nil {
		t.Fatalf("read back document.title: %v", err)
	}
	if title != "language-switched" {
		t.Errorf("document.title = %q after clicking the found selector, want %q — the selector from PageText did not actually hit the button", title, "language-switched")
	}
}

// TestSession_Real_PageTextCapsClickableElementList is the regression test
// for the clickable-elements list's unbounded size: findClickablesJS used to
// have no upper bound at all, so a data-heavy page (a big table, a long
// nav) could dump thousands of lines into the tool result — the exact
// context-bloat problem maxPageTextRunes already guards against for the
// page's prose, just left open on the (more expensive per line) clickable
// list. Serves a page with well over maxClickableElements buttons and
// checks the list stops at the cap and says so instead of listing them all.
func TestSession_Real_PageTextCapsClickableElementList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chromium test in -short mode")
	}

	const totalButtons = maxClickableElements + 50

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		var b strings.Builder
		b.WriteString("<html><body>")
		for i := 0; i < totalButtons; i++ {
			b.WriteString("<button>btn</button>")
		}
		b.WriteString("</body></html>")
		_, _ = w.Write([]byte(b.String()))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := startSession(ctx, func(*Session) {})
	if err != nil {
		t.Skipf("no usable Chromium found, skipping real-browser test: %v", err)
	}
	defer s.Close()

	if err := s.Navigate(ctx, srv.URL); err != nil {
		t.Fatalf("Navigate: %v", err)
	}

	text, err := s.PageText(ctx)
	if err != nil {
		t.Fatalf("PageText: %v", err)
	}

	refRe := regexp.MustCompile(`data-memo-ref="(\d+)"`)
	matches := refRe.FindAllStringSubmatch(text, -1)
	if len(matches) != maxClickableElements {
		t.Errorf("PageText() listed %d clickable elements for a %d-button page, want exactly the cap of %d", len(matches), totalButtons, maxClickableElements)
	}

	wantOmitted := totalButtons - maxClickableElements
	if !strings.Contains(text, "more visible interactive element(s) not shown") {
		t.Fatalf("PageText() did not note the omitted elements past the cap: %q", text)
	}
	if !strings.Contains(text, strconv.Itoa(wantOmitted)) {
		t.Errorf("PageText() omission note did not mention the expected omitted count %d: %q", wantOmitted, text)
	}
}

// pngSize reads a PNG's pixel size from its IHDR chunk.
func pngSize(t *testing.T, b []byte) (int, int) {
	t.Helper()
	if !looksLikePNG(b) || len(b) < 24 {
		t.Fatalf("not a PNG (%d bytes)", len(b))
	}
	w := int(b[16])<<24 | int(b[17])<<16 | int(b[18])<<8 | int(b[19])
	h := int(b[20])<<24 | int(b[21])<<16 | int(b[22])<<8 | int(b[23])
	return w, h
}

// Every screenshot must be exactly ViewportWidth x ViewportHeight, because
// BrowserPane maps a tap on the image back to page coordinates through that
// size. A WindowSize alone produced 500x757 and every manual click missed.
func TestSession_Real_ScreenshotIsExactlyTheViewport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chromium test in -short mode")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><button style="position:absolute;left:16px;top:128px;width:74px;height:44px" onclick="document.title='hit'">B</button></body></html>`))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := startSession(ctx, func(*Session) {})
	if err != nil {
		t.Skipf("no usable Chromium found: %v", err)
	}
	defer s.Close()

	if err := s.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	shot, err := s.Screenshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := pngSize(t, shot); w != ViewportWidth || h != ViewportHeight {
		t.Fatalf("screenshot is %dx%d, want %dx%d — the pane's tap mapping would be off", w, h, ViewportWidth, ViewportHeight)
	}
	// And a click at the button's page coordinates, as the pane sends it, hits.
	if err := s.ClickAt(ctx, 53, 150); err != nil {
		t.Fatal(err)
	}
	var title string
	if err := chromedp.Run(s.tabCtx, chromedp.Title(&title)); err != nil || title != "hit" {
		t.Errorf("click at the button's page coordinates missed (title %q, err %v)", title, err)
	}
}

// When the session's Chromium dies (crash, killed, closed by hand), the
// Manager must notice, report no session, and start a fresh one on the next
// navigate. It used to hand the dead session back forever: every action
// failed with "context canceled" and status kept saying active.
func TestManager_Real_DeadBrowserIsReplaced(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chromium test in -short mode")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body>ok</body></html>`))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	m := New(false)
	defer m.StopSession()
	s, err := m.StartSession(ctx)
	if err != nil {
		t.Skipf("no usable Chromium found: %v", err)
	}
	if err := s.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	proc := chromedp.FromContext(s.tabCtx).Browser.Process()
	if proc == nil {
		t.Fatal("no browser process to kill")
	}
	if err := proc.Kill(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for !s.dead() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if !s.dead() {
		t.Fatal("the session did not notice its browser died")
	}
	if _, ok := m.GetSession(); ok {
		t.Error("GetSession still reports the dead session as active")
	}
	s2, err := m.StartSession(ctx)
	if err != nil {
		t.Fatalf("StartSession after the browser died: %v", err)
	}
	if s2 == s {
		t.Fatal("StartSession handed back the dead session")
	}
	if err := s2.Navigate(ctx, srv.URL); err != nil {
		t.Errorf("the replacement session cannot navigate: %v", err)
	}
}

// The pane's keyboard: text goes to whatever the user focused by clicking,
// and Enter submits.
func TestSession_Real_TypeTextGoesToTheFocusedElement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chromium test in -short mode")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/done" {
			_, _ = w.Write([]byte(`<html><head><title>submitted:` + r.URL.Query().Get("q") + `</title></head></html>`))
			return
		}
		_, _ = w.Write([]byte(`<html><body><form action="/done"><input id="q" name="q" style="position:absolute;left:10px;top:10px;width:200px;height:30px"></form></body></html>`))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := startSession(ctx, func(*Session) {})
	if err != nil {
		t.Skipf("no usable Chromium found: %v", err)
	}
	defer s.Close()
	if err := s.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	if err := s.ClickAt(ctx, 50, 25); err != nil {
		t.Fatal(err)
	}
	if err := s.TypeText(ctx, "merhaba", true); err != nil {
		t.Fatal(err)
	}
	s.Settle(ctx)
	var title string
	if err := chromedp.Run(s.tabCtx, chromedp.Title(&title)); err != nil || title != "submitted:merhaba" {
		t.Errorf("typed text + Enter did not submit the focused field (title %q, err %v)", title, err)
	}
}

// What the pane's address bar actually receives: an address with no scheme.
// It used to be refused outright ("has no scheme").
func TestSession_Real_NavigateWithoutAScheme(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chromium test in -short mode")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>reached</title></head></html>`))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := startSession(ctx, func(*Session) {})
	if err != nil {
		t.Skipf("no usable Chromium found: %v", err)
	}
	defer s.Close()
	if err := s.Navigate(ctx, strings.TrimPrefix(srv.URL, "http://")); err != nil {
		t.Fatalf("a scheme-less address was refused: %v", err)
	}
	var title string
	if err := chromedp.Run(s.tabCtx, chromedp.Title(&title)); err != nil || title != "reached" {
		t.Errorf("title %q, err %v", title, err)
	}
}
