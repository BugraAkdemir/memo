package app

import (
	"context"
	"memo/internal/config"
	"memo/internal/provider"
	"testing"
)

func TestRegisteredChatSwitchAndDecisionSelection(t *testing.T) {
	a := newSendMessageTestApp(t, &capturedRequests{})
	ctx, cancel := context.WithCancel(context.Background())
	a.lifecycleCtx = ctx
	t.Cleanup(cancel)
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)
	var cfg provider.ProviderConfig
	for _, p := range a.providerCfgMgr.GetAll() {
		if p.Name == "test" {
			cfg = p
		}
	}
	cfg.Models = []provider.ConfiguredModel{{ID: cfg.Model, Kind: "chat"}, {ID: "chat-two", Kind: "chat"}, {ID: "review", Kind: "decision"}}
	a.providerCfgMgr.Set(cfg)
	if err := a.SetProviderModel("test", "review"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetProviderModel("test", "unknown"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetProviderModel("test", "chat-two"); err != nil {
		t.Fatal(err)
	}
	list, active, err := a.ListProviderModels(context.Background(), "test")
	if err != nil || len(list) != 4 || active != "chat-two" {
		t.Fatalf("list=%v active=%s err=%v", list, active, err)
	}
	if _, err := a.decisionProviderConfig("test", "chat-two"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.decisionProviderConfig("test", "review"); err != nil {
		t.Fatal(err)
	}
	cfg.Models = []provider.ConfiguredModel{{ID: "review", Kind: "chat"}}
	cfg.Model = "review"
	a.providerCfgMgr.Set(cfg)
	if _, err := a.decisionProviderConfig("test", "review"); err != nil {
		t.Fatal(err)
	}
	cfg.Name = subsProviderName
	cfg.Models = append(cfg.Models, provider.ConfiguredModel{ID: "decision-only", Kind: "decision"})
	a.providerCfgMgr.Set(cfg)
	if err := a.SetProviderModel(subsProviderName, "another-live-chat"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetProviderModel(subsProviderName, "decision-only"); err != nil {
		t.Fatal(err)
	}

	// A legacy client changing only Model must not discard the previous choice.
	cfg.Name = "legacy-editor"
	cfg.Model = "old-model"
	cfg.Models = nil
	a.providerCfgMgr.Set(cfg)
	cfg.Model = "replacement"
	if err := a.UpdateProvider(cfg); err != nil {
		t.Fatal(err)
	}
	for _, p := range a.providerCfgMgr.GetAll() {
		if p.Name == cfg.Name {
			if len(p.Models) != 2 || p.Models[0].ID != "old-model" || p.Models[1].ID != "replacement" {
				t.Fatalf("lost legacy choices: %+v", p.Models)
			}
		}
	}

}
