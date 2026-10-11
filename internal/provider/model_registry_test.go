package provider

import (
	"path/filepath"
	"testing"
)

func TestConfiguredModelsPersistWithoutRoleRestrictions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.json")
	key := make([]byte, 32)
	cm := NewConfigManager(path, key)
	cfg := ProviderConfig{Type: ProviderCustom, Name: "multi", BaseURL: "http://localhost:1", APIKey: "secret", Model: "chat-one", Enabled: true, Models: []ConfiguredModel{{ID: "chat-one", Kind: "chat"}, {ID: "chat-two", Kind: "chat"}, {ID: "review", Kind: "decision"}}}
	cm.Set(cfg)
	cfg.Models[0].ID = "mutated"
	got := NewConfigManager(path, key).GetAll()[0]
	if got.APIKey != "secret" || got.ModelKind("chat-two") != "chat" || got.ModelKind("review") != "chat" {
		t.Fatalf("lost configuration: %+v", got.Models)
	}
	copy := cm.GetAll()
	copy[0].Models[0].ID = "mutated"
	if cm.GetAll()[0].Models[0].ID != "chat-one" {
		t.Fatal("shared mutable model slice")
	}
	got.Name = "decision-only"
	got.Model = "review"
	got.Models = []ConfiguredModel{{ID: "review", Kind: "decision"}}
	cm.Set(got)
	if len(cm.GetEnabled()) != 2 {
		t.Fatal("legacy role blocked an enabled API provider")
	}
	rt := NewRouter(cm.GetAll())
	if _, ok := rt.GetProvider("decision-only"); !ok {
		t.Fatal("legacy role excluded a valid provider")
	}
}

func TestConfiguredModelRoleValidationAndLegacy(t *testing.T) {
	cfg := ProviderConfig{Type: ProviderOpenAI, Model: "old", Enabled: true}
	if !cfg.HasChatModel() || cfg.ModelKind("old") != "chat" || cfg.ModelKind("unknown") != "" {
		t.Fatal("legacy model migration")
	}
	for _, models := range [][]ConfiguredModel{
		{{ID: "old", Kind: "chat"}, {ID: "old", Kind: "decision"}}, {{ID: "", Kind: "chat"}}, {{ID: "other", Kind: "chat"}},
	} {
		cfg.Models = models
		if cfg.Validate() == nil {
			t.Fatalf("accepted invalid models: %+v", models)
		}
	}
}
