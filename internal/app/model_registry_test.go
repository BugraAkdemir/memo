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
	if err := a.SetProviderModel("test", "review"); err == nil {
		t.Fatal("decision model used as chat")
	}
	if err := a.SetProviderModel("test", "unknown"); err == nil {
		t.Fatal("unregistered model selected")
	}
	if err := a.SetProviderModel("test", "chat-two"); err != nil {
		t.Fatal(err)
	}
	list, active, err := a.ListProviderModels(context.Background(), "test")
	if err != nil || len(list) != 2 || active != "chat-two" {
		t.Fatalf("list=%v active=%s err=%v", list, active, err)
	}
	if _, err := a.decisionProviderConfig("test", "chat-two"); err == nil {
		t.Fatal("chat model selected for decisions")
	}
	if _, err := a.decisionProviderConfig("test", "review"); err != nil {
		t.Fatal(err)
	}
	cfg.Models = []provider.ConfiguredModel{{ID: "review", Kind: "chat"}}
	cfg.Model = "review"
	a.providerCfgMgr.Set(cfg)
	if _, err := a.decisionProviderConfig("test", "review"); err == nil {
		t.Fatal("reclassified model still accepted")
	}
	cfg.Name = subsProviderName
	cfg.Models = append(cfg.Models, provider.ConfiguredModel{ID: "decision-only", Kind: "decision"})
	a.providerCfgMgr.Set(cfg)
	if err := a.SetProviderModel(subsProviderName, "another-live-chat"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetProviderModel(subsProviderName, "decision-only"); err == nil {
		t.Fatal("subscription decision model used for chat")
	}

}
