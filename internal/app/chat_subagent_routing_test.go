package app

import (
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/provider"
	"memo/internal/subagents"
)

func TestChatDelegationRouteKeepsTurnProviderAfterGlobalSelectionChanges(t *testing.T) {
	selected := provider.ProviderConfig{Type: provider.ProviderCustom, Name: "selected", BaseURL: "http://selected.invalid", Model: "selected-model", Enabled: true}
	other := provider.ProviderConfig{Type: provider.ProviderCustom, Name: "other", BaseURL: "http://other.invalid", Model: "other-model", Enabled: true, Priority: 100}
	router := provider.NewRouter([]provider.ProviderConfig{other, selected})
	router.SetActiveProvider("selected")
	route := newChatDelegationRoute(router, "selected-model", "high")
	router.SetActiveProvider("other")
	got, ok := route.router.SelectedConfig()
	if !ok || got.Name != "selected" || got.Model != "selected-model" || route.model != "selected-model" || route.effort != "high" {
		t.Fatalf("delegated route moved with global selection: config=%+v model=%s effort=%s", got, route.model, route.effort)
	}
}

func TestChatSubAgentRouterReportsInvalidProviderModelPair(t *testing.T) {
	mgr := provider.NewConfigManager(filepath.Join(t.TempDir(), "providers.json"), make([]byte, 32))
	if err := mgr.Set(provider.ProviderConfig{Type: provider.ProviderCustom, Name: "wrong-vendor", BaseURL: "http://unused.invalid", Model: "vendor-model", Models: []provider.ConfiguredModel{{ID: "vendor-model"}}, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	a := &App{providerCfgMgr: mgr}
	_, _, _, err := a.chatSubAgentRouter(subagents.Job{ProviderName: "wrong-vendor", Model: "another-vendor-model"})
	if err == nil || !strings.Contains(err.Error(), "wrong-vendor") || !strings.Contains(err.Error(), "another-vendor-model") {
		t.Fatalf("missing failed route in error: %v", err)
	}
}
