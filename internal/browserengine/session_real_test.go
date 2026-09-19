package browserengine

import (
	"context"
	"net/http"
	"net/http/httptest"
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
}
