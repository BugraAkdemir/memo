package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestEffortLevelsForType guards the "no static guessing, ever" invariant
// (effort.go's package doc comment): every provider type either has no
// effort concept, or is discovered live per-model in
// internal/webserver/handlers_oauth.go — never a hand-maintained table in
// this package. OpenAI is the sharpest regression case: verified live
// (2026-08-18) that sending reasoning_effort to a non-reasoning OpenAI
// model like gpt-4o gets a hard 400, so a static per-type list here would
// be actively unsafe, not just occasionally wrong.
func TestEffortLevelsForType(t *testing.T) {
	types := []ProviderType{
		ProviderOpenAI, ProviderClaude, ProviderGrok, ProviderGroq,
		ProviderOllama, ProviderLlamaCPP, ProviderGemini, ProviderOpenRouter,
		ProviderOpenCodeZen, ProviderOpenCodeGo, ProviderCustom,
		ProviderClaudeCodeCLI, ProviderCodexCLI, ProviderType("nonexistent"),
	}
	for _, pt := range types {
		t.Run(string(pt), func(t *testing.T) {
			if got := EffortLevelsForType(pt); len(got) != 0 {
				t.Errorf("EffortLevelsForType(%q) = %v, want empty — no type gets a static guessed list anymore", pt, got)
			}
		})
	}
}

func TestGeminiThinkingBudgetForLevel(t *testing.T) {
	tests := []struct {
		level   string
		wantOK  bool
		wantMin int // budget must be > 0 when wantOK
	}{
		{"minimal", true, 1},
		{"low", true, 1},
		{"medium", true, 1},
		{"high", true, 1},
		{"max", true, 1},
		{"", false, 0},
		{"nonexistent", false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			budget, ok := GeminiThinkingBudgetForLevel(tt.level)
			if ok != tt.wantOK {
				t.Fatalf("GeminiThinkingBudgetForLevel(%q) ok = %v, want %v", tt.level, ok, tt.wantOK)
			}
			if ok && budget <= 0 {
				t.Errorf("GeminiThinkingBudgetForLevel(%q) budget = %d, want > 0", tt.level, budget)
			}
		})
	}
}

// TestGeminiThinkingBudgetForLevel_AscendingByLevel is a sanity check on
// the hand-picked constants: "more effort" must mean "more budget," or the
// mapping contradicts the UI's own low→high ordering.
func TestGeminiThinkingBudgetForLevel_AscendingByLevel(t *testing.T) {
	levels := EffortLevelsForGemini()
	prev := -1
	for _, level := range levels {
		budget, ok := GeminiThinkingBudgetForLevel(level)
		if !ok {
			t.Fatalf("EffortLevelsForGemini() included %q, but GeminiThinkingBudgetForLevel has no mapping for it", level)
		}
		if budget <= prev {
			t.Errorf("level %q budget %d is not greater than the previous level's %d — mapping isn't ascending", level, budget, prev)
		}
		prev = budget
	}
}

// TestOpenAIProvider_ChatCompletion_SetsReasoningEffort is the H-provider
// regression for the flat reasoning_effort field shared by openai/grok/
// groq/ollama/llama.cpp/opencode-zen/opencode-go (all built on
// openAIProvider — see applyEffortLevel's doc comment).
func TestOpenAIProvider_ChatCompletion_SetsReasoningEffort(t *testing.T) {
	var gotBody openAIChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResponse{Choices: []openAIChoice{{Message: openAIResponseMsg{Content: "ok"}}}})
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	_, err := p.ChatCompletion(context.Background(), ChatRequest{
		Messages:    []Message{TextMessage("user", "hi")},
		EffortLevel: "high",
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if gotBody.ReasoningEffort != "high" {
		t.Errorf("request ReasoningEffort = %q, want %q", gotBody.ReasoningEffort, "high")
	}
	if gotBody.Reasoning != nil {
		t.Errorf("request Reasoning (nested, OpenRouter-only shape) = %+v, want nil for a plain openai-type provider", gotBody.Reasoning)
	}
}

// TestOpenAIProvider_ChatCompletion_OmitsReasoningEffortWhenEmpty ensures
// an unset EffortLevel produces the exact same wire request as before this
// feature existed — the field must not appear in the JSON at all (thanks
// to omitempty), not just be sent as an empty string.
func TestOpenAIProvider_ChatCompletion_OmitsReasoningEffortWhenEmpty(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResponse{Choices: []openAIChoice{{Message: openAIResponseMsg{Content: "ok"}}}})
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	_, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if strings.Contains(string(rawBody), "reasoning_effort") || strings.Contains(string(rawBody), `"reasoning"`) {
		t.Errorf("request body contains a reasoning field with EffortLevel unset: %s", rawBody)
	}
}

// TestOpenRouterShape_UsesNestedReasoningObject is the regression for the
// one vendor in this package that rejects the flat reasoning_effort field
// outright (verified against current OpenRouter docs, 2026-08-18) —
// applyEffortLevel must route EffortLevel into the nested {"reasoning":
// {"effort":...}} shape specifically when provType==ProviderOpenRouter,
// not the shape every other openAIProvider-based vendor uses.
func TestOpenRouterShape_UsesNestedReasoningObject(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResponse{Choices: []openAIChoice{{Message: openAIResponseMsg{Content: "ok"}}}})
	}))
	defer srv.Close()

	p, err := newOpenAIProvider(ProviderConfig{
		Type:    ProviderOpenRouter,
		BaseURL: srv.URL,
		Model:   "test-model",
	})
	if err != nil {
		t.Fatalf("newOpenAIProvider() error = %v", err)
	}
	_, err = p.ChatCompletion(context.Background(), ChatRequest{
		Messages:    []Message{TextMessage("user", "hi")},
		EffortLevel: "medium",
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if _, present := parsed["reasoning_effort"]; present {
		t.Errorf("request body has flat reasoning_effort for OpenRouter — OpenRouter's API rejects this shape entirely: %s", rawBody)
	}
	reasoning, ok := parsed["reasoning"].(map[string]interface{})
	if !ok {
		t.Fatalf("request body missing nested \"reasoning\" object: %s", rawBody)
	}
	if reasoning["effort"] != "medium" {
		t.Errorf("reasoning.effort = %v, want %q", reasoning["effort"], "medium")
	}
}

// TestClaudeProvider_ChatCompletion_SetsAdaptiveThinking is the regression
// for claude.go's effort wiring: EffortLevel must produce
// thinking:{type:"adaptive"} + output_config:{effort:...} — the vendor's
// current mechanism (see claudeThinking's doc comment for the older
// manual-budget mode this deliberately does NOT use, and the known
// model-generation compatibility caveat).
func TestClaudeProvider_ChatCompletion_SetsAdaptiveThinking(t *testing.T) {
	var gotBody claudeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(claudeResponse{Content: []claudeBlock{{Type: "text", Text: "ok"}}})
	}))
	defer srv.Close()

	p, err := newClaudeProvider(ProviderConfig{BaseURL: srv.URL, Model: "claude-test", APIKey: "k"})
	if err != nil {
		t.Fatalf("newClaudeProvider() error = %v", err)
	}
	_, err = p.ChatCompletion(context.Background(), ChatRequest{
		Messages:    []Message{TextMessage("user", "hi")},
		EffortLevel: "high",
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if gotBody.Thinking == nil || gotBody.Thinking.Type != "adaptive" {
		t.Fatalf("request Thinking = %+v, want {Type: \"adaptive\"}", gotBody.Thinking)
	}
	if gotBody.OutputConfig == nil || gotBody.OutputConfig.Effort != "high" {
		t.Fatalf("request OutputConfig = %+v, want {Effort: \"high\"}", gotBody.OutputConfig)
	}
}

// TestClaudeProvider_ChatCompletion_OmitsThinkingWhenEffortUnset guards
// the "don't change existing behavior for callers that never set
// EffortLevel" invariant — every other Claude test in this package
// predates this feature and asserts the old request shape.
func TestClaudeProvider_ChatCompletion_OmitsThinkingWhenEffortUnset(t *testing.T) {
	var gotBody claudeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(claudeResponse{Content: []claudeBlock{{Type: "text", Text: "ok"}}})
	}))
	defer srv.Close()

	p, err := newClaudeProvider(ProviderConfig{BaseURL: srv.URL, Model: "claude-test", APIKey: "k"})
	if err != nil {
		t.Fatalf("newClaudeProvider() error = %v", err)
	}
	_, err = p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if gotBody.Thinking != nil {
		t.Errorf("request Thinking = %+v, want nil when EffortLevel is unset", gotBody.Thinking)
	}
	if gotBody.OutputConfig != nil {
		t.Errorf("request OutputConfig = %+v, want nil when EffortLevel is unset", gotBody.OutputConfig)
	}
}

// TestGeminiProvider_ChatCompletion_SetsThinkingBudget is the regression
// for gemini.go's effort wiring — the label must resolve through
// GeminiThinkingBudgetForLevel into generationConfig.thinkingConfig.
func TestGeminiProvider_ChatCompletion_SetsThinkingBudget(t *testing.T) {
	var gotBody geminiRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(geminiResponse{Candidates: []geminiCandidate{{Content: geminiContent{Parts: []geminiPart{{Text: "ok"}}}}}})
	}))
	defer srv.Close()

	p, err := newGeminiProvider(ProviderConfig{BaseURL: srv.URL, Model: "gemini-test", APIKey: "k"})
	if err != nil {
		t.Fatalf("newGeminiProvider() error = %v", err)
	}
	_, err = p.ChatCompletion(context.Background(), ChatRequest{
		Messages:    []Message{TextMessage("user", "hi")},
		EffortLevel: "high",
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	wantBudget, _ := GeminiThinkingBudgetForLevel("high")
	if gotBody.GenerationConfig == nil || gotBody.GenerationConfig.ThinkingConfig == nil {
		t.Fatalf("request GenerationConfig.ThinkingConfig is nil, want ThinkingBudget=%d", wantBudget)
	}
	if gotBody.GenerationConfig.ThinkingConfig.ThinkingBudget != wantBudget {
		t.Errorf("ThinkingBudget = %d, want %d", gotBody.GenerationConfig.ThinkingConfig.ThinkingBudget, wantBudget)
	}
}

// TestGeminiProvider_ChatCompletion_OmitsThinkingConfigForUnknownLevel
// guards the "0 is a real value, not a safe unset sentinel" note on
// GeminiThinkingBudgetForLevel — an unset/unrecognized EffortLevel must
// omit thinkingConfig entirely, not send ThinkingBudget:0.
func TestGeminiProvider_ChatCompletion_OmitsThinkingConfigForUnknownLevel(t *testing.T) {
	var gotBody geminiRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(geminiResponse{Candidates: []geminiCandidate{{Content: geminiContent{Parts: []geminiPart{{Text: "ok"}}}}}})
	}))
	defer srv.Close()

	p, err := newGeminiProvider(ProviderConfig{BaseURL: srv.URL, Model: "gemini-test", APIKey: "k"})
	if err != nil {
		t.Fatalf("newGeminiProvider() error = %v", err)
	}
	_, err = p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if gotBody.GenerationConfig != nil && gotBody.GenerationConfig.ThinkingConfig != nil {
		t.Errorf("ThinkingConfig = %+v, want nil when EffortLevel is unset", gotBody.GenerationConfig.ThinkingConfig)
	}
}
