// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"memo/internal/config"
	"memo/internal/provider"
)

func isolatedDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MEMO_DATA_DIR", dir)
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)
	return dir
}

// The per-vendor subscription providers each left an encrypted OAuth token
// under their own data dir. Nothing reads them now, so they are removed — and
// nothing else (the sidecar's own dir in particular) is touched.
func TestPurgeLegacySubscriptionData(t *testing.T) {
	dir := isolatedDataDir(t)
	for _, f := range []string{"geminisub/token.enc", "claudesub/token.enc", "cliproxy/state.json", "sessions/keep.json"} {
		p := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	purgeLegacySubscriptionData()
	purgeLegacySubscriptionData() // idempotent

	for _, gone := range []string{"geminisub", "claudesub"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("%s was not removed", gone)
		}
	}
	for _, kept := range []string{"cliproxy/state.json", "sessions/keep.json"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s was removed: %v", kept, err)
		}
	}
}

// After the legacy providers are dropped, a config whose active provider named
// one of them must not stay pointed at it: the router would hold an active name
// matching nothing and every chat would fail instead of using the local model.
func TestReinit_ClearsAnActiveProviderThatIsGone(t *testing.T) {
	isolatedDataDir(t)

	seed := provider.NewConfigManager(config.DataPath("providers.json"), nil)
	seed.Set(provider.ProviderConfig{Type: provider.ProviderOpenAI, Name: "My OpenAI", APIKey: "k", Model: "gpt-4o", Enabled: true})
	seed.Set(provider.ProviderConfig{Type: "gemini-sub", Name: "Google — Gemini (subscription)", Model: "gemini-2.5-pro", Enabled: true})
	seed.Save()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &App{cfg: &config.AppConfig{ActiveProvider: "Google — Gemini (subscription)"}, lifecycleCtx: ctx}
	a.reinitProviderAndOrchestra()

	if got := a.GetActiveProvider(); got != "" {
		t.Errorf("active provider = %q, want it cleared (the provider no longer exists)", got)
	}
	if a.cfg.ActiveProvider != "" {
		t.Errorf("cfg.ActiveProvider = %q, want it cleared", a.cfg.ActiveProvider)
	}
}

// ...but an active provider that DOES still exist is left alone.
func TestReinit_KeepsAnActiveProviderThatExists(t *testing.T) {
	isolatedDataDir(t)

	seed := provider.NewConfigManager(config.DataPath("providers.json"), nil)
	seed.Set(provider.ProviderConfig{Type: provider.ProviderOpenAI, Name: "My OpenAI", APIKey: "k", Model: "gpt-4o", Enabled: true})
	seed.Save()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &App{cfg: &config.AppConfig{ActiveProvider: "My OpenAI"}, lifecycleCtx: ctx}
	a.reinitProviderAndOrchestra()

	if got := a.GetActiveProvider(); got != "My OpenAI" {
		t.Errorf("active provider = %q, want My OpenAI kept", got)
	}
}
