package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/mail"
	"memo/internal/provider"
	"memo/internal/twin"
)

// Found live: while several mails were processed at once the shared, slow model
// answered the 5 s risk review too late (proxy 499) and ordinary mail was held
// as review_unavailable. The background mail review now has its own longer
// limit, while the chat-path decision keeps the short one (a user is waiting
// there and a slow model only means "keep the old behaviour").
func TestMailScorerWaitsLongerThanTheChatPathDecision(t *testing.T) {
	prevChat, prevMail := decisionCallTimeout, mailScorerTimeout
	decisionCallTimeout, mailScorerTimeout = 80*time.Millisecond, 3*time.Second
	t.Cleanup(func() { decisionCallTimeout, mailScorerTimeout = prevChat, prevMail })

	s, err := twin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(400 * time.Millisecond) // slower than the chat limit, well inside the mail limit
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "0.02"}, "finish_reason": "stop"}}})
	}))
	t.Cleanup(server.Close)
	mgr := provider.NewConfigManager(t.TempDir()+"/providers.json", make([]byte, 32))
	if err = mgr.Set(provider.ProviderConfig{Name: "scorer", Type: provider.ProviderCustom, BaseURL: server.URL, Model: "score-only", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	a := &App{twinStore: s, providerCfgMgr: mgr, cfg: &config.AppConfig{Learning: config.LearningConfig{DecisionEnabled: true, DecisionProvider: "scorer", DecisionModel: "score-only"}}}

	v := a.inspectIncomingMail(context.Background(), mail.Account{}, &mail.MessageDetail{TextBody: "Merhaba, takvimde yarın boş bir saat var mı?"})
	if v.Blocked {
		t.Fatalf("an ordinary mail was held although the model answered inside the mail limit: %+v", v)
	}
	if _, err := a.callDecisionModel(context.Background(), "Answer a number.", "x", 8); err == nil {
		t.Fatal("the chat-path decision must keep its short limit")
	}
}
