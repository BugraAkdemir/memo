package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/provider"
)

func TestDecisionModelUsesSelectedProviderAndFailsOpen(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		status         int
		enabled, want  bool
	}{
		{"no", "NO", 200, true, false}, {"yes", "YES", 200, true, true},
		{"ambiguous", "NO because I think so", 200, true, true}, {"empty", "", 200, true, true},
		{"provider error", "", 500, true, true}, {"disabled", "NO", 200, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var req struct {
					Model     string `json:"model"`
					Tools     []any  `json:"tools"`
					MaxTokens int    `json:"max_tokens"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
				}
				if req.Model != "tiny-decision" || len(req.Tools) != 0 {
					t.Errorf("wrong request: %+v", req)
				}
				if tc.status != 200 {
					http.Error(w, "unavailable", tc.status)
					return
				}
				json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": tc.response}, "finish_reason": "stop"}}})
			}))
			defer server.Close()
			router := provider.NewRouter([]provider.ProviderConfig{{Name: "cheap", Type: provider.ProviderCustom, BaseURL: server.URL, Model: "default", Enabled: true}})
			router.SetActiveProvider("different-chat-provider")
			a := &App{cfg: &config.AppConfig{Learning: config.LearningConfig{DecisionEnabled: tc.enabled, DecisionProvider: "cheap", DecisionModel: "tiny-decision"}}, providerRouter: router}
			if got := a.decisionAllows(context.Background(), agentDecisionPrompt, "Explain recursion"); got != tc.want {
				t.Fatalf("allows=%v want %v", got, tc.want)
			}
			if tc.enabled && calls != 1 {
				t.Errorf("calls=%d", calls)
			}
			if !tc.enabled && calls != 0 {
				t.Errorf("disabled called provider")
			}
		})
	}
}

func TestDecisionModelUnavailablePreservesBehavior(t *testing.T) {
	a := &App{cfg: &config.AppConfig{Learning: config.LearningConfig{DecisionEnabled: true, DecisionProvider: "missing", DecisionModel: "tiny"}}}
	if !a.decisionAllows(context.Background(), memoryDecisionPrompt, "My name is Ada") {
		t.Fatal("missing provider must preserve memory")
	}
}

func TestDecisionAgentRoutingPreservesExplicitAgentChats(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		agent, forced, tools bool
		decisions            int
	}{
		{"ordinary text skips optional tools", true, false, false, 1},
		{"disabled agent stays disabled", false, false, false, 0},
		{"explicit agent chat stays agent", false, true, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reqs := &capturedRequests{}
			a := newSendMessageTestApp(t, reqs)
			config.ResetForTests()
			t.Cleanup(config.ResetForTests)
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"NO"},"finish_reason":"stop"}]}`))
			}))
			t.Cleanup(srv.Close)
			a.providerCfgMgr.Set(provider.ProviderConfig{Name: "decision", Type: provider.ProviderCustom, BaseURL: srv.URL, Model: "tiny", Enabled: true})
			a.providerRouter = provider.NewRouter(a.providerCfgMgr.GetEnabled())
			a.providerRouter.SetActiveProvider("test")
			a.cfg.Learning = config.LearningConfig{DecisionEnabled: true, DecisionProvider: "decision", DecisionModel: "tiny"}
			a.agentEnabled = tc.agent
			if tc.forced {
				a.sessions.NewAgentChat(t.TempDir())
				drainBackground(t, a)
			}
			if got := a.SendMessage("Explain recursion"); got != "fake agent reply" {
				t.Fatalf("reply=%q", got)
			}
			a.waitBackground(time.Second)
			if got := reqs.containsAny(`"tools"`); got != tc.tools {
				t.Fatalf("tools=%v want=%v; requests=%s", got, tc.tools, reqs)
			}
			if calls != tc.decisions {
				t.Fatalf("decisions=%d want=%d", calls, tc.decisions)
			}
		})
	}
}
