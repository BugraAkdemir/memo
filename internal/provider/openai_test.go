package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestOpenAIProvider points an openAIProvider at an httptest server
// instead of the real OpenAI API — none of the 7 providers built on top of
// openAIProvider (openai itself, grok, groq, ollama, llama.cpp, opencode-zen,
// opencode-go, openrouter) had any test coverage of their shared
// request-building/response-parsing/SSE logic before this file.
func newTestOpenAIProvider(t *testing.T, srv *httptest.Server) *openAIProvider {
	t.Helper()
	p, err := newOpenAIProvider(ProviderConfig{
		Type:    ProviderOpenAI,
		BaseURL: srv.URL,
		Model:   "gpt-4o",
		APIKey:  "test-key-123",
	})
	if err != nil {
		t.Fatalf("newOpenAIProvider() error = %v", err)
	}
	return p
}

func TestOpenAIProvider_ChatCompletion_SendsCorrectRequestAndParsesResponse(t *testing.T) {
	var gotAuth string
	var gotBody openAIChatRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResponse{
			Model: "gpt-4o",
			Choices: []openAIChoice{
				{Message: openAIResponseMsg{Role: "assistant", Content: "merhaba!"}, FinishReason: "stop"},
			},
			Usage: openAIUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		})
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	resp, err := p.ChatCompletion(context.Background(), ChatRequest{
		Messages:    []Message{TextMessage("user", "selam")},
		Temperature: 0.7,
		MaxTokens:   100,
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	if gotAuth != "Bearer test-key-123" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer test-key-123")
	}
	if gotBody.Model != "gpt-4o" {
		t.Errorf("request Model = %q, want %q (fell back to configured model)", gotBody.Model, "gpt-4o")
	}
	if gotBody.Stream {
		t.Error("request Stream = true, want false for ChatCompletion")
	}
	if len(gotBody.Messages) != 1 || gotBody.Messages[0].Content != "selam" {
		t.Errorf("request Messages = %+v, want a single \"selam\" user message", gotBody.Messages)
	}

	if resp.Content != "merhaba!" {
		t.Errorf("resp.Content = %q, want %q", resp.Content, "merhaba!")
	}
	if resp.Model != "gpt-4o" {
		t.Errorf("resp.Model = %q, want %q", resp.Model, "gpt-4o")
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 15 {
		t.Errorf("resp.Usage = %+v, want TotalTokens=15", resp.Usage)
	}
}

func TestOpenAIProvider_ChatCompletion_RequestModelOverridesConfiguredModel(t *testing.T) {
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body openAIChatRequest
		json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		json.NewEncoder(w).Encode(openAIResponse{Choices: []openAIChoice{{Message: openAIResponseMsg{Content: "ok"}}}})
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv) // configured with "gpt-4o"
	if _, err := p.ChatCompletion(context.Background(), ChatRequest{
		Model:    "gpt-4o-mini",
		Messages: []Message{TextMessage("user", "hi")},
	}); err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	if gotModel != "gpt-4o-mini" {
		t.Errorf("request Model = %q, want the per-request override %q, not the provider's configured default", gotModel, "gpt-4o-mini")
	}
}

func TestOpenAIProvider_ChatCompletion_ParsesToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(openAIResponse{
			Choices: []openAIChoice{{
				Message: openAIResponseMsg{
					Role: "assistant",
					ToolCalls: []ToolCall{
						{ID: "call_1", Type: "function", Function: ToolCallFunction{Name: "read_file", Arguments: json.RawMessage(`{"path":"x.txt"}`)}},
					},
				},
				FinishReason: "tool_calls",
			}},
		})
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	resp, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "read x.txt")}})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("resp.ToolCalls = %+v, want a single read_file call", resp.ToolCalls)
	}
}

func TestOpenAIProvider_ChatCompletion_EmptyChoicesReturnsEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(openAIResponse{Model: "gpt-4o", Choices: []openAIChoice{}})
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	resp, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if resp.Content != "" {
		t.Errorf("resp.Content = %q, want empty for a response with zero choices", resp.Content)
	}
}

func TestOpenAIProvider_ChatCompletion_RateLimitedWrapsErrRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "slow down"}})
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	_, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err == nil {
		t.Fatal("expected an error for a 429 response")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("error = %v, want it to wrap ErrRateLimited", err)
	}
	if !strings.Contains(err.Error(), "slow down") {
		t.Errorf("error = %v, want it to include the provider's message", err)
	}
}

func TestOpenAIProvider_ChatCompletion_AuthErrorReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "invalid api key"}})
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	_, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err == nil {
		t.Fatal("expected an error for a 401 response")
	}
	if !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("error = %v, want it to include the provider's message", err)
	}
}

func TestOpenAIProvider_ChatCompletionStream_ParsesSSEChunksAndSendsDone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		lines := []string{
			`data: {"choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
			`data: {"choices":[{"index":0,"delta":{"content":"me"}}]}`,
			`data: {"choices":[{"index":0,"delta":{"content":"rhaba"}}]}`,
			`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}
		for _, l := range lines {
			w.Write([]byte(l + "\n\n"))
			flusher.Flush()
		}
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	ch, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}

	var content strings.Builder
	var sawFinishReasonDone bool
	timeout := time.After(2 * time.Second)
	for {
		select {
		case chunk, ok := <-ch:
			if !ok {
				if !sawFinishReasonDone {
					t.Error("channel closed without ever seeing the finish_reason=stop Done chunk")
				}
				if content.String() != "merhaba" {
					t.Errorf("accumulated content = %q, want %q", content.String(), "merhaba")
				}
				return
			}
			content.WriteString(chunk.Content)
			if chunk.Done && chunk.FinishReason == "stop" {
				sawFinishReasonDone = true
			}
		case <-timeout:
			t.Fatal("timed out waiting for stream to complete")
		}
	}
}

func TestOpenAIProvider_ChatCompletionStream_SkipsMalformedChunkWithoutAborting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		lines := []string{
			`data: not valid json at all`,
			`data: {"choices":[{"index":0,"delta":{"content":"ok"}}]}`,
			`data: [DONE]`,
		}
		for _, l := range lines {
			w.Write([]byte(l + "\n\n"))
			flusher.Flush()
		}
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	ch, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}

	var content strings.Builder
	timeout := time.After(2 * time.Second)
	for {
		select {
		case chunk, ok := <-ch:
			if !ok {
				if content.String() != "ok" {
					t.Errorf("accumulated content = %q, want %q (malformed chunk should be skipped, not abort the stream)", content.String(), "ok")
				}
				return
			}
			content.WriteString(chunk.Content)
		case <-timeout:
			t.Fatal("timed out waiting for stream to complete")
		}
	}
}

func TestOpenAIProvider_ChatCompletionStream_NonOKStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "server exploded"}})
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	_, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err == nil {
		t.Fatal("expected an error for a 500 stream-open response")
	}
	if !strings.Contains(err.Error(), "server exploded") {
		t.Errorf("error = %v, want it to include the provider's message", err)
	}
}

func TestOpenAIProvider_ListModels_ParsesModelIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("path = %q, want /models", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "gpt-4o"},
				{"id": "gpt-4o-mini"},
			},
		})
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 2 || models[0] != "gpt-4o" || models[1] != "gpt-4o-mini" {
		t.Errorf("ListModels() = %v, want [gpt-4o gpt-4o-mini]", models)
	}
}

func TestOpenAIProvider_ToOpenAIMessages_PreservesToolCallFields(t *testing.T) {
	p := &openAIProvider{}
	msgs := []Message{
		TextMessage("user", "hi"),
		{Role: "tool", Content: "result", ToolCallID: "call_1"},
		{Role: "assistant", Content: "", ToolCalls: []ToolCall{{ID: "call_1", Type: "function"}}},
	}
	got := p.toOpenAIMessages(msgs)
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	if got[1].ToolCallID != "call_1" {
		t.Errorf("got[1].ToolCallID = %q, want %q", got[1].ToolCallID, "call_1")
	}
	if len(got[2].ToolCalls) != 1 {
		t.Errorf("got[2].ToolCalls = %+v, want 1 entry", got[2].ToolCalls)
	}
}
