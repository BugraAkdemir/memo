// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"testing"

	"memo/internal/config"
	"memo/internal/provider"
)

func TestGeminiSubMarkerConfig(t *testing.T) {
	cfg := geminiSubMarkerConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("marker config fails Validate: %v", err)
	}
	if cfg.Type != provider.ProviderGeminiSub {
		t.Errorf("Type = %q, want gemini-sub", cfg.Type)
	}
	if cfg.Name != geminiSubProviderName {
		t.Errorf("Name = %q, want %q", cfg.Name, geminiSubProviderName)
	}
	if !cfg.Enabled {
		t.Error("marker config is not Enabled — it would never route")
	}
	if cfg.APIKey != "" {
		t.Error("marker config carries a secret; it must not")
	}
}

func TestGoogleAccountState(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	if c, e := a.GoogleAccountState(); c || e != "" {
		t.Errorf("fresh state = (%v, %q), want (false, \"\")", c, e)
	}
	a.cfg.DevGateway.GeminiSub = config.GeminiSubState{Connected: true, Email: "x@y.z"}
	if c, e := a.GoogleAccountState(); !c || e != "x@y.z" {
		t.Errorf("state = (%v, %q), want (true, x@y.z)", c, e)
	}
}

func TestDisconnectGoogleAccount_NotConnectedIsNoop(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	if err := a.DisconnectGoogleAccount(); err != nil {
		t.Errorf("Disconnect when not connected = %v, want nil no-op", err)
	}
}
