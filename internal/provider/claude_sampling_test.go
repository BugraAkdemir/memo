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

// opus47Server mimics a Claude model from Opus 4.7 onward: any request that
// carries temperature or top_p is a 400 (Anthropic removed both). It records
// every raw body so the tests can see exactly what was sent.
func opus47Server(t *testing.T, errMsg string, stream bool) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		bodies = append(bodies, m)
		_, hasT := m["temperature"]
		_, hasP := m["top_p"]
		if hasT || hasP {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"` + errMsg + `"}}`))
			return
		}
		if stream {
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":5,\"output_tokens\":1}}}\n\n")
			io.WriteString(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"ok\"}}\n\n")
			io.WriteString(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"model":"claude-opus-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":5,"output_tokens":1}}`))
	}))
	return srv, &bodies
}

// TestClaude_RetriesWithoutSamplingWhenModelRejectsIt is S3 (BUG_REPORT
// 2026-09-28): every caller sends temperature (the agent pipeline pins 0.2),
// so against Opus 4.7+ every single turn used to fail with a 400.
func TestClaude_RetriesWithoutSamplingWhenModelRejectsIt(t *testing.T) {
	srv, bodies := opus47Server(t, "temperature: This model does not support the temperature parameter.", false)
	defer srv.Close()
	p := newTestClaudeProvider(t, srv, "claude-opus-5")
	req := ChatRequest{
		Messages:    []Message{TextMessage("system", "persona"), TextMessage("user", "hi")},
		Temperature: 0.2,
		Tools: []ToolDefinition{
			{Type: "function", Function: ToolFunction{Name: "a", Description: "d", Parameters: []byte(`{"type":"object"}`)}},
		},
	}
	resp, err := p.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v — a rejected temperature must be withdrawn and retried", err)
	}
	if resp.Content != "ok" {
		t.Errorf("Content = %q, want ok", resp.Content)
	}
	if len(*bodies) != 2 {
		t.Fatalf("server saw %d requests, want 2", len(*bodies))
	}
	if _, ok := (*bodies)[1]["temperature"]; ok {
		t.Error("the retry still carried temperature")
	}
	// The error named temperature, not cache_control: caching must survive.
	if !strings.Contains(mustJSON(t, (*bodies)[1]), "cache_control") {
		t.Error("the retry dropped cache_control although the rejection was about temperature")
	}
	if !p.noSampling.Load() {
		t.Error("the refusal did not latch")
	}
	if p.noCacheControl.Load() {
		t.Error("a sampling rejection latched prompt caching off")
	}
	if _, err := p.ChatCompletion(context.Background(), req); err != nil {
		t.Fatalf("second ChatCompletion() error = %v", err)
	}
	if len(*bodies) != 3 {
		t.Fatalf("server saw %d requests after the latch, want 3 (one clean request)", len(*bodies))
	}
}

// TestClaude_SamplingRetryOnPlainChatStream covers the path most turns take:
// a streamed plain-chat turn with no tools and so no cache_control, rejected
// with an error text that names neither field.
func TestClaude_SamplingRetryOnPlainChatStream(t *testing.T) {
	srv, bodies := opus47Server(t, "invalid request", true)
	defer srv.Close()
	p := newTestClaudeProvider(t, srv, "claude-opus-5")
	ch, err := p.ChatCompletionStream(context.Background(), ChatRequest{
		Messages:    []Message{TextMessage("user", "hi")},
		Temperature: 0.7,
	})
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}
	var got strings.Builder
	for c := range ch {
		got.WriteString(c.Content)
		if c.Error != "" {
			t.Fatalf("stream error: %s", c.Error)
		}
	}
	if got.String() != "ok" {
		t.Errorf("content = %q, want ok", got.String())
	}
	if len(*bodies) != 2 || !p.noSampling.Load() {
		t.Errorf("requests=%d latched=%v, want 2 and true", len(*bodies), p.noSampling.Load())
	}
}

// TestClaude_SamplingRetryThatStillFailsReportsOriginalAndDoesNotLatch keeps
// the valve honest: when withdrawing sampling doesn't help, the caller sees the
// first error and nothing is disabled for the session.
func TestClaude_SamplingRetryThatStillFailsReportsOriginalAndDoesNotLatch(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		if calls == 1 {
			w.Write([]byte(`{"error":{"message":"first: temperature out of range"}}`))
			return
		}
		w.Write([]byte(`{"error":{"message":"second"}}`))
	}))
	defer srv.Close()
	p := newTestClaudeProvider(t, srv, "claude-x")
	_, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}, Temperature: 0.2})
	if err == nil || !strings.Contains(err.Error(), "first") {
		t.Fatalf("err = %v, want the original error", err)
	}
	if p.noSampling.Load() {
		t.Error("a failed retry latched sampling off")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
