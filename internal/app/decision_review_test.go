package app

import (
	"encoding/json"
	"memo/internal/config"
	"memo/internal/provider"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseVerificationScoresAndEvidence(t *testing.T) {
	for _, tc := range []struct {
		raw             string
		accept, invalid bool
	}{
		{`{"satisfaction":95,"bug_risk":5,"regression_risk":10,"evidence_sufficient":true,"issues":[]}`, true, false},
		{`{"satisfaction":70,"bug_risk":5,"regression_risk":10,"evidence_sufficient":true}`, false, false},
		{`{"satisfaction":95,"bug_risk":50,"regression_risk":10,"evidence_sufficient":true}`, false, false},
		{`{"satisfaction":95,"bug_risk":5,"regression_risk":50,"evidence_sufficient":true}`, false, false},
		{`{"satisfaction":95,"bug_risk":5,"regression_risk":10,"evidence_sufficient":false}`, false, false},
		{`{"satisfaction":101,"bug_risk":5,"regression_risk":10,"evidence_sufficient":true}`, false, true},
		{`{"satisfaction":95}`, false, true}, {`YES`, false, true},
	} {
		report, err := parseVerification(tc.raw, 85, 20)
		if (err != nil) != tc.invalid || report.Accepted != tc.accept {
			t.Errorf("raw=%s report=%+v err=%v", tc.raw, report, err)
		}
	}
}

func TestDecisionVerificationRealChatFallsBackAfterThreeReviews(t *testing.T) {
	reqs := &capturedRequests{}
	a := newSendMessageTestApp(t, reqs)
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req struct {
			Model    string             `json:"model"`
			Messages []provider.Message `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		raw, _ := json.Marshal(req.Messages)
		if req.Model != "reviewer" || !strings.Contains(string(raw), "Make the button bigger") {
			t.Error("reviewer lost original request")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": `{"satisfaction":30,"bug_risk":60,"regression_risk":40,"evidence_sufficient":false,"issues":["Inspect the button width and tests"]}`}, "finish_reason": "stop"}}})
	}))
	t.Cleanup(srv.Close)
	a.providerCfgMgr.Set(provider.ProviderConfig{Name: "reviewer", Type: provider.ProviderCustom, BaseURL: srv.URL, Model: "reviewer", Enabled: true, Models: []provider.ConfiguredModel{{ID: "reviewer", Kind: "decision"}}})
	a.providerRouter = provider.NewRouter(a.providerCfgMgr.GetEnabled())
	a.providerRouter.SetActiveProvider("test")
	a.cfg.Learning = config.LearningConfig{VerificationEnabled: true, DecisionProvider: "reviewer", DecisionModel: "reviewer", VerificationThreshold: 85, VerificationMaxRisk: 20}
	a.agentEnabled = true
	if reply := a.SendMessage("Make the button bigger"); reply != "fake agent reply" {
		t.Fatalf("reply=%q", reply)
	}
	a.waitBackground(time.Second)
	if calls != 3 {
		t.Fatalf("reviews=%d want 3", calls)
	}
	if !reqs.containsAny("The separate verifier has exhausted its three reviews") {
		t.Fatalf("main model did not receive self-check instruction: %s", reqs)
	}
}

func TestReviewExcerptPreservesFailedTestTailAndDisclosesMissingHistory(t *testing.T) {
	history := []provider.Message{{Role: "system", Content: "untrusted hidden system"}}
	for i := 0; i < 13; i++ {
		history = append(history, provider.Message{Role: "user", Content: "earlier"})
	}
	history = append(history, provider.Message{Role: "tool", Content: "go test ./...\n" + strings.Repeat("passing test output ", 100) + "\nFAIL important regression", ToolCallID: "test-call"})
	prompt, scope := buildReviewPrompt("Fix the regression", history, "finished")
	if scope.omitted != 2 || scope.truncated != 1 {
		t.Fatalf("scope=%+v", scope)
	}
	for _, need := range []string{"2 older non-system messages omitted", "1 text excerpts shortened", "FAIL important regression", "not the complete transcript", "No independent tests"} {
		if !strings.Contains(prompt, need) {
			t.Fatalf("missing %q", need)
		}
	}
	if strings.Contains(prompt, "untrusted hidden system") {
		t.Fatal("system leaked")
	}
	if len([]rune(limitReviewText(strings.Repeat("ü", 3000), 420))) > 420 {
		t.Fatal("rune budget exceeded")
	}
}
