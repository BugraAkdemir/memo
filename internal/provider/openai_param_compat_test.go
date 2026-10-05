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

// reasoningModelServer mimics an OpenAI reasoning model (o-series / gpt-5):
// it names ONE offending parameter per 400, the way the real API does —
// max_tokens first, then a non-default temperature, then top_p.
func reasoningModelServer(t *testing.T) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		bodies = append(bodies, m)
		reject := func(msg string) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":{"message":"` + msg + `","type":"invalid_request_error"}}`))
		}
		if _, ok := m["max_tokens"]; ok {
			reject("Unsupported parameter: 'max_tokens' is not supported with this model. Use 'max_completion_tokens' instead.")
			return
		}
		if _, ok := m["temperature"]; ok {
			reject("Unsupported value: 'temperature' does not support 0.2 with this model. Only the default (1) value is supported.")
			return
		}
		if _, ok := m["top_p"]; ok {
			reject("Unsupported value: 'top_p' does not support 0.9 with this model.")
			return
		}
		if m["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\n")
			io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
			io.WriteString(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":1,\"total_tokens\":8}}\n\n")
			io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"model":"gpt-5","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":1,"total_tokens":8}}`))
	}))
	return srv, &bodies
}

func TestOpenAI_WithdrawsParamsAReasoningModelRefuses(t *testing.T) {
	srv, bodies := reasoningModelServer(t)
	defer srv.Close()
	p := newTestOpenAIProvider(t, srv)
	req := ChatRequest{Model: "gpt-5", Messages: []Message{TextMessage("user", "hi")}, Temperature: 0.2, TopP: 0.9, MaxTokens: 512}

	resp, err := p.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("Content = %q, want ok", resp.Content)
	}
	if len(*bodies) != 4 {
		t.Fatalf("server saw %d requests, want 4 (three refusals, one success)", len(*bodies))
	}
	last := (*bodies)[3]
	if last["max_completion_tokens"] != float64(512) {
		t.Errorf("max_completion_tokens = %v, want 512 carried over from max_tokens", last["max_completion_tokens"])
	}
	if !p.useMaxCompletionTokens.Load() || !p.noTemperature.Load() || !p.noTopP.Load() {
		t.Error("the refusals did not all latch")
	}

	// Latched: the next call — streaming this time — goes out clean at once.
	ch, err := p.ChatCompletionStream(context.Background(), req)
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}
	var got strings.Builder
	var usage *Usage
	for c := range ch {
		got.WriteString(c.Content)
		if c.Usage != nil {
			usage = c.Usage
		}
	}
	if got.String() != "ok" || usage == nil {
		t.Errorf("stream content=%q usage=%v, want ok and the trailing usage", got.String(), usage)
	}
	if len(*bodies) != 5 {
		t.Errorf("server saw %d requests, want 5 — the latched stream must not re-learn anything", len(*bodies))
	}
}

// TestOpenAI_RetryOnlyOnFieldErrors: a 400 that names nothing withdrawable
// (and no stream_options to fall back on) is returned as-is, one request.
func TestOpenAI_UnrelatedBadRequestIsNotRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"context length exceeded"}}`))
	}))
	defer srv.Close()
	p := newTestOpenAIProvider(t, srv)
	_, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}, Temperature: 0.2})
	if err == nil || !strings.Contains(err.Error(), "context length") {
		t.Fatalf("err = %v, want the original error", err)
	}
	if calls != 1 {
		t.Errorf("server saw %d requests, want 1", calls)
	}
	if p.noTemperature.Load() {
		t.Error("an unrelated 400 latched temperature off")
	}
}
