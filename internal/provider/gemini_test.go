package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestGeminiProvider(t *testing.T, srv *httptest.Server, configuredModel string) *geminiProvider {
	t.Helper()
	p, err := newGeminiProvider(ProviderConfig{
		BaseURL: srv.URL,
		Model:   configuredModel,
		APIKey:  "test-key-123",
	})
	if err != nil {
		t.Fatalf("newGeminiProvider() error = %v", err)
	}
	return p
}

// TestGeminiProvider_ChatCompletion_FallsBackToConfiguredModelInURL confirms
// Gemini does NOT have claude.go's model-fallback-discarded bug: unlike
// Claude (which puts model in the request body via buildClaudeRequest),
// Gemini's resolved model goes into the request URL path directly in
// ChatCompletion/ChatCompletionStream themselves, so an empty
// ChatRequest.Model correctly falls through to the provider's configured
// default here.
func TestGeminiProvider_ChatCompletion_FallsBackToConfiguredModelInURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(geminiResponse{Candidates: []geminiCandidate{{Content: geminiContent{Parts: []geminiPart{{Text: "ok"}}}}}})
	}))
	defer srv.Close()

	p := newTestGeminiProvider(t, srv, "gemini-1.5-pro")
	if _, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}}); err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	if !strings.Contains(gotPath, "gemini-1.5-pro") {
		t.Errorf("request path = %q, want it to contain the configured model %q", gotPath, "gemini-1.5-pro")
	}
}

func TestGeminiProvider_ChatCompletion_ModelGetsModelsPrefixWhenMissing(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(geminiResponse{Candidates: []geminiCandidate{{Content: geminiContent{Parts: []geminiPart{{Text: "ok"}}}}}})
	}))
	defer srv.Close()

	p := newTestGeminiProvider(t, srv, "gemini-1.5-pro")
	if _, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}}); err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if !strings.Contains(gotPath, "/models/gemini-1.5-pro:generateContent") {
		t.Errorf("request path = %q, want a \"models/\" prefix added to a bare model name", gotPath)
	}
}

func TestGeminiProvider_ChatCompletion_DoesNotDoublePrefixAlreadyQualifiedModel(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(geminiResponse{Candidates: []geminiCandidate{{Content: geminiContent{Parts: []geminiPart{{Text: "ok"}}}}}})
	}))
	defer srv.Close()

	p := newTestGeminiProvider(t, srv, "models/gemini-1.5-pro")
	if _, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}}); err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if strings.Contains(gotPath, "models/models/") {
		t.Errorf("request path = %q, double-prefixed an already-qualified model", gotPath)
	}
}

func TestGeminiProvider_ChatCompletion_MapsAssistantRoleToModel(t *testing.T) {
	var gotBody geminiRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(geminiResponse{Candidates: []geminiCandidate{{Content: geminiContent{Parts: []geminiPart{{Text: "ok"}}}}}})
	}))
	defer srv.Close()

	p := newTestGeminiProvider(t, srv, "gemini-1.5-pro")
	_, err := p.ChatCompletion(context.Background(), ChatRequest{
		Messages: []Message{
			TextMessage("system", "be nice"),
			TextMessage("user", "hi"),
			TextMessage("assistant", "hello there"),
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	if gotBody.SystemInstruction == nil || len(gotBody.SystemInstruction.Parts) == 0 || gotBody.SystemInstruction.Parts[0].Text != "be nice" {
		t.Errorf("SystemInstruction = %+v, want the system message extracted there", gotBody.SystemInstruction)
	}
	if len(gotBody.Contents) != 2 {
		t.Fatalf("len(Contents) = %d, want 2 (system message excluded)", len(gotBody.Contents))
	}
	if gotBody.Contents[0].Role != "user" {
		t.Errorf("Contents[0].Role = %q, want %q", gotBody.Contents[0].Role, "user")
	}
	if gotBody.Contents[1].Role != "model" {
		t.Errorf("Contents[1].Role = %q, want %q (Gemini calls the assistant role \"model\")", gotBody.Contents[1].Role, "model")
	}
}

func TestGeminiProvider_ChatCompletion_ParsesUsageAndConcatenatesParts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(geminiResponse{
			Candidates: []geminiCandidate{{
				Content:      geminiContent{Parts: []geminiPart{{Text: "merhaba "}, {Text: "dunya"}}},
				FinishReason: "STOP",
			}},
			Usage: &geminiUsage{PromptTokenCount: 10, CandidatesTokenCount: 5, TotalTokenCount: 15},
		})
	}))
	defer srv.Close()

	p := newTestGeminiProvider(t, srv, "gemini-1.5-pro")
	resp, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if resp.Content != "merhaba dunya" {
		t.Errorf("resp.Content = %q, want %q", resp.Content, "merhaba dunya")
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 15 {
		t.Errorf("resp.Usage = %+v, want TotalTokens=15", resp.Usage)
	}
}

func TestGeminiProvider_ChatCompletion_EmptyCandidatesReturnsEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(geminiResponse{Candidates: []geminiCandidate{}})
	}))
	defer srv.Close()

	p := newTestGeminiProvider(t, srv, "gemini-1.5-pro")
	resp, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if resp.Content != "" {
		t.Errorf("resp.Content = %q, want empty for zero candidates", resp.Content)
	}
}

func TestGeminiProvider_ChatCompletion_APIKeySentAsHeaderNotAuthorization(t *testing.T) {
	var gotHeader, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("x-goog-api-key")
		gotAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(geminiResponse{Candidates: []geminiCandidate{{Content: geminiContent{Parts: []geminiPart{{Text: "ok"}}}}}})
	}))
	defer srv.Close()

	p := newTestGeminiProvider(t, srv, "gemini-1.5-pro")
	if _, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}}); err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if gotHeader != "test-key-123" {
		t.Errorf("x-goog-api-key header = %q, want %q", gotHeader, "test-key-123")
	}
	if gotAuth != "" {
		t.Errorf("Authorization header = %q, want empty (Gemini doesn't use it)", gotAuth)
	}
}

func TestGeminiProvider_ChatCompletionStream_SkipsSTOPFinishReasonButSendsDoneAtEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		lines := []string{
			`data: {"candidates":[{"content":{"parts":[{"text":"me"}]}}]}`,
			`data: {"candidates":[{"content":{"parts":[{"text":"rhaba"}]},"finishReason":"STOP"}]}`,
		}
		for _, l := range lines {
			w.Write([]byte(l + "\n\n"))
			flusher.Flush()
		}
	}))
	defer srv.Close()

	p := newTestGeminiProvider(t, srv, "gemini-1.5-pro")
	ch, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}
	content, sawDone := drainStream(t, ch)
	if content != "merhaba" {
		t.Errorf("accumulated content = %q, want %q", content, "merhaba")
	}
	// finishReason=="STOP" is deliberately NOT treated as a Done-with-reason
	// signal (only a non-STOP reason is, e.g. SAFETY/MAX_TOKENS) — but the
	// stream must still send a final Done once it naturally ends.
	if !sawDone {
		t.Error("expected a final Done chunk once the stream ends")
	}
}

func TestGeminiProvider_ChatCompletionStream_NonStopFinishReasonEndsStreamImmediately(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		w.Write([]byte(`data: {"candidates":[{"content":{"parts":[{"text":"cut off"}]},"finishReason":"MAX_TOKENS"}]}` + "\n\n"))
		flusher.Flush()
	}))
	defer srv.Close()

	p := newTestGeminiProvider(t, srv, "gemini-1.5-pro")
	ch, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}

	var gotFinishReason string
	for chunk := range ch {
		if chunk.Done {
			gotFinishReason = chunk.FinishReason
		}
	}
	if gotFinishReason != "MAX_TOKENS" {
		t.Errorf("FinishReason = %q, want %q", gotFinishReason, "MAX_TOKENS")
	}
}

func TestGeminiProvider_ListModels_ParsesModelNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]string{
				{"name": "models/gemini-1.5-pro", "displayName": "Gemini 1.5 Pro"},
				{"name": "models/gemini-1.5-flash", "displayName": "Gemini 1.5 Flash"},
			},
		})
	}))
	defer srv.Close()

	p := newTestGeminiProvider(t, srv, "gemini-1.5-pro")
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 2 || models[0] != "models/gemini-1.5-pro" {
		t.Errorf("ListModels() = %v, want [models/gemini-1.5-pro models/gemini-1.5-flash]", models)
	}
}
