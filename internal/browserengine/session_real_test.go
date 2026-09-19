package browserengine

import (
	"context"
	"testing"
	"time"
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
