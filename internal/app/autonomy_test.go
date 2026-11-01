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

func TestAutonomyMailIngressSeparatesReplyWriterFromScalarDecisionModel(t *testing.T) {
	ctx := context.Background()
	s, err := twin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.SavePreferences(ctx, twin.Preferences{Timezone: "UTC", WorkStart: "09:00", WorkEnd: "18:00", Autonomy: twin.AutonomySettings{Enabled: true, Mail: true, Mode: "full_auto"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveContact(ctx, twin.Contact{Name: "Friend", AutonomyPolicy: "default", Aliases: []twin.Alias{{Channel: "mail", Address: "friend@example.com"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveReplyModel(ctx, twin.ReplyModel{Provider: "writer", Model: "reply-writer"}); err != nil {
		t.Fatal(err)
	}
	var calls, scores atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var req struct {
			Model     string             `json:"model"`
			Messages  []provider.Message `json:"messages"`
			Tools     []any              `json:"tools"`
			MaxTokens int                `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if (req.Model != "reply-writer" && req.Model != "score-only") || len(req.Tools) != 0 {
			t.Error("not an isolated decision call", req)
		}
		text := `{"kind":"question","tools":[{"name":"contact_timeline","args":{}}]}`
		if len(req.Messages) > 0 && strings.Contains(fmt.Sprint(req.Messages[0].Content), "Write a reply draft") {
			text = `{"body":"Received for review.","factual":true,"evidence_tools":["contact_timeline"]}`
		}
		if req.Model == "score-only" {
			scores.Add(1)
			text = "0.98"
			if req.MaxTokens != 16 || strings.Contains(fmt.Sprint(req.Messages[0].Content), "Write a reply draft") {
				t.Error("scorer received generation work")
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": text}, "finish_reason": "stop"}}})
	}))
	defer server.Close()
	mgr := provider.NewConfigManager(t.TempDir()+"/providers.json", make([]byte, 32))
	if err := mgr.Set(provider.ProviderConfig{Name: "scorer", Type: provider.ProviderCustom, BaseURL: server.URL, Model: "score-only", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Set(provider.ProviderConfig{Name: "writer", Type: provider.ProviderCustom, BaseURL: server.URL, Model: "reply-writer", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	mailMgr, err := mail.NewManager(t.TempDir() + "/accounts.json")
	if err != nil {
		t.Fatal(err)
	}
	a := &App{mailManager: mailMgr, twinStore: s, providerCfgMgr: mgr, cfg: &config.AppConfig{Learning: config.LearningConfig{DecisionEnabled: true, DecisionProvider: "scorer", DecisionModel: "score-only"}}}
	// Without an actual mail account the safe upload path must fail visibly;
	// it must still retain the local draft, and never attempt SMTP delivery.
	d := &mail.MessageDetail{UID: 91, MessageID: "<incoming@example.com>", From: "Friend <friend@example.com>", To: "owner@example.com", Timestamp: time.Now(), TextBody: "Can you check our conversation?"}
	a.onIncomingMailEvent(ctx, mail.Account{ID: "missing-account"}, d, mail.TriageDecision{AllowAutoReply: true})
	a.onIncomingMailEvent(ctx, mail.Account{ID: "missing-account"}, d, mail.TriageDecision{AllowAutoReply: true})
	actions, err := s.Actions(ctx)
	if err != nil || len(actions) != 1 {
		t.Fatal(actions, err)
	}
	if calls.Load() != 3 || scores.Load() != 1 || actions[0].DecisionScore == nil || *actions[0].DecisionScore != 0.98 || actions[0].State != "failed" || actions[0].Reason != "draft_upload_failed" || actions[0].Body == "" || a.GetAgentEnabled() {
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

func TestReplyModelSelectionValidationAndNoDecisionFallback(t *testing.T) {
	ctx := context.Background()
	s, err := twin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mgr := provider.NewConfigManager(t.TempDir()+"/providers.json", make([]byte, 32))
	for _, cfg := range []provider.ProviderConfig{{Name: "api", Type: provider.ProviderCustom, Enabled: true}, {Name: "off", Type: provider.ProviderCustom}, {Name: "web", Type: provider.ProviderWebChatGPT, Enabled: true}, {Name: "cli", Type: provider.ProviderType("codex-cli"), Enabled: true}} {
		if err = mgr.Set(cfg); err != nil {
			t.Fatal(err)
		}
	}
	a := &App{twinStore: s, providerCfgMgr: mgr, cfg: config.Default()}
	for _, name := range []string{"missing", "off", "web", "cli"} {
		raw, _ := json.Marshal(twin.ReplyModel{Provider: name, Model: "test"})
		if _, err = a.CallTwin(ctx, "reply_model", raw); err == nil {
			t.Fatal("unsafe selection", name)
		}
	}
	if _, err = a.CallTwin(ctx, "reply_model", json.RawMessage(`{"provider":"api","model":"writer"}`)); err != nil {
		t.Fatal(err)
	}
	if m, err := s.ReplyModel(ctx); err != nil || m.Provider != "api" || m.Model != "writer" {
		t.Fatal(m, err)
	}
	if _, err = a.CallTwin(ctx, "reply_model", json.RawMessage(`{"provider":"","model":""}`)); err != nil {
		t.Fatal(err)
	}
	if _, err = a.callAutonomyReplyModel(ctx, twin.ReplyModel{}, "unused", "unused"); err == nil {
		t.Fatal("unset writer used another model")
	}
}

func TestMailSecurityUsesNumericRiskAndClassifierUsesWriter(t *testing.T) {
	ctx := context.Background()
	s, err := twin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.SaveReplyModel(ctx, twin.ReplyModel{Provider: "writer", Model: "reply-writer"}); err != nil {
		t.Fatal(err)
	}
	var scoreCalls, writerCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		text := ""
		switch req.Model {
		case "score-only":
			scoreCalls.Add(1)
			text = "0.01"
			if req.MaxTokens != 16 {
				t.Error("wrong score budget")
			}
		case "reply-writer":
			writerCalls.Add(1)
			text = `{"category":"personal"}`
		default:
			t.Error("unexpected model", req.Model)
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": text}, "finish_reason": "stop"}}})
	}))
	defer server.Close()
	mgr := provider.NewConfigManager(t.TempDir()+"/providers.json", make([]byte, 32))
	for _, m := range []twin.ReplyModel{{Provider: "writer", Model: "reply-writer"}, {Provider: "scorer", Model: "score-only"}} {
		if err = mgr.Set(provider.ProviderConfig{Name: m.Provider, Type: provider.ProviderCustom, BaseURL: server.URL, Model: m.Model, Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	a := &App{twinStore: s, providerCfgMgr: mgr, cfg: &config.AppConfig{Learning: config.LearningConfig{DecisionEnabled: true, DecisionProvider: "scorer", DecisionModel: "score-only"}}}
	if v := a.inspectIncomingMail(ctx, mail.Account{}, &mail.MessageDetail{TextBody: "A normal factual question"}); v.Blocked {
		t.Fatal(v)
	}
	a.cfg.Learning.DecisionEnabled = false
	if raw, err := a.callMailClassifierLLM(ctx, "Classify mail as JSON", "Hello"); err != nil || raw != `{"category":"personal"}` {
		t.Fatal(raw, err)
	}
	if raw, err := a.callAutonomyReplyModel(ctx, twin.ReplyModel{Provider: "writer", Model: "reply-writer"}, "Write a draft", "Hello"); err != nil || raw == "" {
		t.Fatal(raw, err)
	}
	if scoreCalls.Load() != 1 || writerCalls.Load() != 2 {
		t.Fatal(scoreCalls.Load(), writerCalls.Load())
	}
}
