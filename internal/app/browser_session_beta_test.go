// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"strings"
	"testing"

	"memo/internal/config"
)

// The browser panel's direct actions are part of the beta browser feature.
// With Beta off they refuse before touching the engine; with it on they get as
// far as the engine (nil here, so the engine's own error proves the Beta check
// passed first).
func TestBrowserPanelActions_RefuseWhileBetaIsOff(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	ctx := context.Background()

	if _, _, err := a.NavigateBrowserSession(ctx, "https://example.com"); err == nil || !strings.Contains(strings.ToLower(err.Error()), "beta") {
		t.Errorf("Navigate with Beta off = %v, want a Beta refusal", err)
	}
	if _, _, err := a.ClickBrowserSession(ctx, 1, 2); err == nil || !strings.Contains(strings.ToLower(err.Error()), "beta") {
		t.Errorf("Click with Beta off = %v, want a Beta refusal", err)
	}
	if _, _, err := a.ScrollBrowserSession(ctx, 0, 100); err == nil || !strings.Contains(strings.ToLower(err.Error()), "beta") {
		t.Errorf("Scroll with Beta off = %v, want a Beta refusal", err)
	}
	if _, _, err := a.TypeBrowserSession(ctx, "x", false); err == nil || !strings.Contains(strings.ToLower(err.Error()), "beta") {
		t.Errorf("Type with Beta off = %v, want a Beta refusal", err)
	}
	if active, _ := a.BrowserSessionStatus(ctx); active {
		t.Error("BrowserSessionStatus reports a session with Beta off")
	}
	if err := a.CloseBrowserSession(); err != nil {
		t.Errorf("CloseBrowserSession must stay allowed so a leftover session can be torn down: %v", err)
	}

	a.cfg.Beta = true
	if _, _, err := a.NavigateBrowserSession(ctx, "https://example.com"); err == nil || strings.Contains(strings.ToLower(err.Error()), "beta") {
		t.Errorf("Navigate with Beta on = %v, want it to get past the Beta check (to the engine's own error)", err)
	}
}

func TestSetBeta_ReportsBackWhatWasSet(t *testing.T) {
	t.Setenv("MEMO_DATA_DIR", t.TempDir())
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)
	a := &App{cfg: config.Default()}

	if a.GetBeta() {
		t.Fatal("beta on by default")
	}
	if err := a.SetBeta(true); err != nil || !a.GetBeta() {
		t.Fatalf("SetBeta(true): err=%v GetBeta=%v", err, a.GetBeta())
	}
	if err := a.SetBeta(false); err != nil || a.GetBeta() {
		t.Fatalf("SetBeta(false): err=%v GetBeta=%v", err, a.GetBeta())
	}
}
