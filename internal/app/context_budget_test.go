package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"memo/internal/config"
	"memo/internal/provider"
)

// A provider's name or type says nothing about its window: only a published
// figure (the user's own, the vendor's, the catalogue's) does. Without one the
// prompt is budgeted against the internal safety margin and the gauge says
// "unknown" (0).
func TestContextBudgetFor_Precedence(t *testing.T) {
	provider.ResetContextWindowCache()
	t.Cleanup(provider.ResetContextWindowCache)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"big-model","context_length":1000000}]}`))
	}))
	defer srv.Close()

	cfgMgr := provider.NewConfigManager(t.TempDir()+"/providers.json", nil)
	published := provider.ProviderConfig{Type: provider.ProviderCustom, Name: "published", BaseURL: srv.URL + "/v1", Model: "big-model", Enabled: true}
	cfgMgr.Set(published)
	cfgMgr.Set(provider.ProviderConfig{Type: provider.ProviderGemini, Name: "gemini", BaseURL: "http://127.0.0.1:1/v1beta", Model: "gemini-x", Enabled: true})
	cfgMgr.Set(provider.ProviderConfig{Type: provider.ProviderClaude, Name: "claude", BaseURL: "http://127.0.0.1:1/v1", Model: "claude-x", Enabled: true, ContextTokens: 77000})
	a := &App{cfg: &config.AppConfig{}, providerCfgMgr: cfgMgr}

	// The vendor's published figure.
	if n, src := provider.WarmContextWindow(context.Background(), published, "big-model"); n != 1000000 || src != "api" {
		t.Fatalf("warm = (%d,%q)", n, src)
	}
	if got := a.contextBudgetFor("published"); got != 1000000 {
		t.Fatalf("published window = %d", got)
	}

	// The provider NAME gemini/claude no longer implies 1M/200K: nothing
	// published -> internal safety budget, and the gauge's figure is unknown.
	if got := a.contextBudgetFor("gemini"); got != unknownContextBudget {
		t.Fatalf("unpublished gemini budget = %d, want the safety budget", got)
	}
	if got := a.knownContextWindowFor("gemini", ""); got != 0 {
		t.Fatalf("unpublished gemini window = %d, want 0 (unknown)", got)
	}
	if got := a.contextBudgetFor("something-else"); got != unknownContextBudget {
		t.Fatalf("unknown provider budget = %d", got)
	}

	// The user's per-provider figure.
	if got := a.contextBudgetFor("claude"); got != 77000 {
		t.Fatalf("configured window = %d", got)
	}

	// An explicit global override beats everything, published or not.
	a.cfg.Llama.MaxContextTokens = 9
	if got := a.contextBudgetFor("published"); got != 9 {
		t.Fatalf("global override = %d, want 9", got)
	}
}
