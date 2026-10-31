package app

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/config"
	"memo/internal/mail"
	"memo/internal/provider"
	"memo/internal/twin"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAutonomyMailIngressUsesSelectedDecisionProviderAndOwnerQueue(t *testing.T) {
	ctx := context.Background()
	s, err := twin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.SavePreferences(ctx, twin.Preferences{Timezone: "UTC", WorkStart: "09:00", WorkEnd: "18:00", Autonomy: twin.AutonomySettings{Enabled: true, Mail: true, Mode: "draft_only"}}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var req struct {
			Model    string             `json:"model"`
			Messages []provider.Message `json:"messages"`
			Tools    []any              `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req.Model != "decision" || len(req.Tools) != 0 {
			t.Error("not an isolated decision call", req)
		}
		text := `{"kind":"question","tools":[{"name":"contact_timeline","args":{}}]}`
		if len(req.Messages) > 0 && strings.Contains(fmt.Sprint(req.Messages[0].Content), "Write a reply draft") {
			text = `{"body":"Received for review.","factual":false,"confidence":0.5}`
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": text}, "finish_reason": "stop"}}})
	}))
	defer server.Close()
	mgr := provider.NewConfigManager(t.TempDir()+"/providers.json", make([]byte, 32))
	if err := mgr.Set(provider.ProviderConfig{Name: "cheap", Type: provider.ProviderCustom, BaseURL: server.URL, Model: "decision", Enabled: true, Models: []provider.ConfiguredModel{{ID: "decision", Kind: "decision"}}}); err != nil {
		t.Fatal(err)
	}
	mailMgr, err := mail.NewManager(t.TempDir() + "/accounts.json")
	if err != nil {
		t.Fatal(err)
	}
	a := &App{mailManager: mailMgr, twinStore: s, providerCfgMgr: mgr, cfg: &config.AppConfig{Learning: config.LearningConfig{DecisionEnabled: true, DecisionProvider: "cheap", DecisionModel: "decision"}}}
	// Without an actual mail account the safe upload path must fail visibly;
	// it must still retain the local draft, and never attempt SMTP delivery.
	d := &mail.MessageDetail{UID: 91, MessageID: "<incoming@example.com>", From: "Friend <friend@example.com>", To: "owner@example.com", Timestamp: time.Now(), TextBody: "Can you check our conversation?"}
	a.onIncomingMailEvent(ctx, mail.Account{ID: "missing-account"}, d, mail.TriageDecision{})
	a.onIncomingMailEvent(ctx, mail.Account{ID: "missing-account"}, d, mail.TriageDecision{})
	actions, err := s.Actions(ctx)
	if err != nil || len(actions) != 1 {
		t.Fatal(actions, err)
	}
	if calls.Load() != 2 || actions[0].State != "failed" || actions[0].Reason != "draft_upload_failed" || actions[0].Body == "" || a.GetAgentEnabled() {
		t.Fatal(calls.Load(), actions)
	}
	raw, err := a.CallTwin(ctx, "state", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(raw)
	if !strings.Contains(string(b), "draft_upload_failed") {
		t.Fatal(string(b))
	}
}
