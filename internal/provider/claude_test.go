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

func newTestClaudeProvider(t *testing.T, srv *httptest.Server, configuredModel string) *claudeProvider {
	t.Helper()
	p, err := newClaudeProvider(ProviderConfig{
		BaseURL: srv.URL,
		Model:   configuredModel,
		APIKey:  "test-key-123",
	})
	if err != nil {
		t.Fatalf("newClaudeProvider() error = %v", err)
	}
	return p
}

// TestClaudeProvider_ChatCompletion_FallsBackToConfiguredModel is the
// regression test for a real bug found while writing this file: ChatRequest
// omitting Model (exactly what internal/app/llm.go's main chat streaming
// path does — it never sets Model at all) used to send Anthropic's API an
// empty "model" field on every request. ChatCompletion/ChatCompletionStream
// computed a model-with-fallback local variable but then called
// buildClaudeRequest(req, ...), which read req.Model directly — silently
// discarding that fallback. Every plain chat message sent while Claude was
// the active provider went out with model="".
func TestClaudeProvider_ChatCompletion_FallsBackToConfiguredModel(t *testing.T) {
	var gotBody claudeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(claudeResponse{Content: []claudeBlock{{Type: "text", Text: "ok"}}})
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-3-5-sonnet-20241022")
	// Deliberately empty Model — the real-world case that broke.
	if _, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}}); err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	if gotBody.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("request Model = %q, want the provider's configured default %q", gotBody.Model, "claude-3-5-sonnet-20241022")
	}
}

func TestClaudeProvider_ChatCompletionStream_FallsBackToConfiguredModel(t *testing.T) {
	var gotBody claudeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		w.Write([]byte(`data: {"type":"message_stop"}` + "\n\n"))
		flusher.Flush()
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-3-5-sonnet-20241022")
	ch, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}
	drainStream(t, ch)

	if gotBody.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("request Model = %q, want the provider's configured default %q", gotBody.Model, "claude-3-5-sonnet-20241022")
	}
}

func TestClaudeProvider_ChatCompletion_RequestModelOverridesConfigured(t *testing.T) {
	var gotBody claudeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(claudeResponse{Content: []claudeBlock{{Type: "text", Text: "ok"}}})
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-3-5-sonnet-20241022")
	if _, err := p.ChatCompletion(context.Background(), ChatRequest{
		Model:    "claude-3-opus-20240229",
		Messages: []Message{TextMessage("user", "hi")},
	}); err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if gotBody.Model != "claude-3-opus-20240229" {
		t.Errorf("request Model = %q, want the per-request override", gotBody.Model)
	}
}

func TestClaudeProvider_ChatCompletion_SplitsSystemMessagesFromContent(t *testing.T) {
	var gotBody claudeRequest
	var gotVersionHeader, gotAuthHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersionHeader = r.Header.Get("anthropic-version")
		gotAuthHeader = r.Header.Get("x-api-key")
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(claudeResponse{Content: []claudeBlock{{Type: "text", Text: "ok"}}})
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-3-5-sonnet-20241022")
	_, err := p.ChatCompletion(context.Background(), ChatRequest{
		Messages: []Message{
			TextMessage("system", "You are helpful."),
			TextMessage("user", "hi"),
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	if gotBody.System != "You are helpful." {
		t.Errorf("System = %q, want the system message extracted out of Messages", gotBody.System)
	}
	if len(gotBody.Messages) != 1 || gotBody.Messages[0].Role != "user" {
		t.Errorf("Messages = %+v, want only the user message (system pulled out)", gotBody.Messages)
	}
	if gotVersionHeader != "2023-06-01" {
		t.Errorf("anthropic-version header = %q, want %q", gotVersionHeader, "2023-06-01")
	}
	if gotAuthHeader != "test-key-123" {
		t.Errorf("x-api-key header = %q, want %q (Claude uses x-api-key, not Authorization)", gotAuthHeader, "test-key-123")
	}
}

func TestClaudeProvider_ChatCompletion_DefaultsMaxTokensWhenUnset(t *testing.T) {
	var gotBody claudeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(claudeResponse{Content: []claudeBlock{{Type: "text", Text: "ok"}}})
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-3-5-sonnet-20241022")
	if _, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}}); err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if gotBody.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want the 4096 default when unset (Anthropic requires this field)", gotBody.MaxTokens)
	}
}

func TestClaudeProvider_ChatCompletion_ParsesTextBlocksAndUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(claudeResponse{
			Model: "claude-3-5-sonnet-20241022",
			Content: []claudeBlock{
				{Type: "text", Text: "merhaba "},
				{Type: "text", Text: "dunya"},
			},
			Usage: &claudeUsage{InputTokens: 10, OutputTokens: 5},
		})
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-3-5-sonnet-20241022")
	resp, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if resp.Content != "merhaba dunya" {
		t.Errorf("resp.Content = %q, want concatenated text blocks %q", resp.Content, "merhaba dunya")
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 15 {
		t.Errorf("resp.Usage = %+v, want TotalTokens=15 (input+output)", resp.Usage)
	}
}

func TestClaudeProvider_ChatCompletion_RateLimitedWrapsErrRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "rate limited"}})
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-3-5-sonnet-20241022")
	_, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err == nil {
		t.Fatal("expected an error for a 429 response")
	}
	if !strings.Contains(err.Error(), "429") && !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("error = %v, want it to reference the 429/rate-limit condition", err)
	}
}

func TestClaudeProvider_ChatCompletionStream_ParsesContentBlockDeltasAndMessageStop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		lines := []string{
			`data: {"type":"content_block_delta","delta":{"text":"me"}}`,
			`data: {"type":"content_block_delta","delta":{"text":"rhaba"}}`,
			`data: {"type":"message_stop"}`,
		}
		for _, l := range lines {
			w.Write([]byte(l + "\n\n"))
			flusher.Flush()
		}
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-3-5-sonnet-20241022")
	ch, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}
	content, sawDone := drainStream(t, ch)
	if content != "merhaba" {
		t.Errorf("accumulated content = %q, want %q", content, "merhaba")
	}
	if !sawDone {
		t.Error("expected a Done chunk after message_stop")
	}
}

func TestClaudeProvider_ChatCompletionStream_ErrorEventSurfacesAsChunkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		w.Write([]byte(`data: {"type":"error","error":{"type":"overloaded_error","message":"servers overloaded"}}` + "\n\n"))
		flusher.Flush()
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-3-5-sonnet-20241022")
	ch, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}

	timeout := time.After(2 * time.Second)
	select {
	case chunk := <-ch:
		if chunk.Error != "servers overloaded" {
			t.Errorf("chunk.Error = %q, want %q", chunk.Error, "servers overloaded")
		}
		if !chunk.Done {
			t.Error("expected the error chunk to also carry Done=true")
		}
	case <-timeout:
		t.Fatal("timed out waiting for the error chunk")
	}
}

func TestClaudeProvider_ListModels_FiltersToModelTypeOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"type": "model", "id": "claude-3-5-sonnet-20241022"},
				{"type": "something-else", "id": "not-a-model"},
			},
		})
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-3-5-sonnet-20241022")
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 1 || models[0] != "claude-3-5-sonnet-20241022" {
		t.Errorf("ListModels() = %v, want only the type=\"model\" entry", models)
	}
}

func TestClaudeProvider_ChatCompletion_TimeoutWrapsErrTimeout(t *testing.T) {
	p := &claudeProvider{}
	err := p.wrapError(errors.New("context deadline exceeded"))
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("wrapError(deadline exceeded) = %v, want it to wrap ErrTimeout", err)
	}
}

// drainStream reads chunks until ch closes, returning accumulated content
// and whether a Done chunk was ever seen.
func drainStream(t *testing.T, ch <-chan StreamChunk) (string, bool) {
	t.Helper()
	var content strings.Builder
	var sawDone bool
	timeout := time.After(2 * time.Second)
	for {
		select {
		case chunk, ok := <-ch:
			if !ok {
				return content.String(), sawDone
			}
			content.WriteString(chunk.Content)
			if chunk.Done {
				sawDone = true
			}
		case <-timeout:
			t.Fatal("timed out waiting for stream to complete")
			return "", false
		}
	}
}
