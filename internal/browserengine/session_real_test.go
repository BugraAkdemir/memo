package browserengine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
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
