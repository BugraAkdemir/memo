// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// drainStreamUsage collects a stream to completion and returns the accumulated
// content plus the last terminal chunk seen, failing on timeout.
func drainStreamUsage(t *testing.T, ch <-chan StreamChunk) (string, StreamChunk) {
	t.Helper()
	var content strings.Builder
	var terminal StreamChunk
	timeout := time.After(5 * time.Second)
	for {
		select {
		case chunk, ok := <-ch:
			if !ok {
				return content.String(), terminal
			}
			content.WriteString(chunk.Content)
			if chunk.Done {
				terminal = chunk
			}
		case <-timeout:
			t.Fatal("timed out waiting for stream to complete")
		}
	}
}

// TestOpenAIStream_RequestsAndParsesTrailingUsageChunk is the core of the
// streaming half: the usage chunk arrives AFTER the finish_reason chunk and
// carries an empty choices array, so both of processSSE's old shortcuts
// (return on finish_reason, skip chunks with no choices) had to go.
func TestOpenAIStream_RequestsAndParsesTrailingUsageChunk(t *testing.T) {
	var gotBody openAIChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for _, l := range []string{
			`data: {"choices":[{"index":0,"delta":{"content":"hel"}}]}`,
			`data: {"choices":[{"index":0,"delta":{"content":"lo"}}]}`,
			`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: {"choices":[],"usage":{"prompt_tokens":5120,"completion_tokens":2,"total_tokens":5122,"prompt_tokens_details":{"cached_tokens":4096}}}`,
			`data: [DONE]`,
		} {
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
	content, terminal := drainStreamUsage(t, ch)

	if gotBody.StreamOptions == nil || !gotBody.StreamOptions.IncludeUsage {
		t.Errorf("request did not ask for usage: stream_options = %+v", gotBody.StreamOptions)
	}
	if content != "hello" {
		t.Errorf("content = %q, want %q", content, "hello")
	}
	if !terminal.Done {
		t.Fatal("no terminal Done chunk")
	}
	if terminal.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want \"stop\" — holding Done back for usage must not lose it", terminal.FinishReason)
	}
	if terminal.Usage == nil {
		t.Fatal("terminal chunk carried no Usage")
	}
	if terminal.Usage.PromptTokens != 5120 || terminal.Usage.CompletionTokens != 2 {
		t.Errorf("Usage = %+v, want prompt 5120 / completion 2", *terminal.Usage)
	}
	if terminal.Usage.CachedPromptTokens != 4096 {
		t.Errorf("CachedPromptTokens = %d, want 4096", terminal.Usage.CachedPromptTokens)
	}
}

// TestOpenAIStream_NoUsageChunkStillCompletes is the ordinary case for every
// OpenAI-compatible backend that accepts stream_options and ignores it: the
// turn must finish on [DONE] exactly as before, with nil Usage so the caller
// keeps its estimate.
func TestOpenAIStream_NoUsageChunkStillCompletes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for _, l := range []string{
			`data: {"choices":[{"index":0,"delta":{"content":"ok"}}]}`,
			`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		} {
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
	content, terminal := drainStreamUsage(t, ch)
	if content != "ok" {
		t.Errorf("content = %q, want %q", content, "ok")
	}
	if !terminal.Done || terminal.FinishReason != "stop" {
		t.Errorf("terminal = %+v, want Done with finish_reason=stop", terminal)
	}
	if terminal.Usage != nil {
		t.Errorf("Usage = %+v, want nil so the caller keeps its estimate", *terminal.Usage)
	}
}

// TestOpenAIStream_WatchdogEndsTurnWhenPromisedUsageNeverArrives covers the
// failure mode the grace timer exists for: an endpoint accepts
// stream_options, sends finish_reason, then neither emits usage nor [DONE] nor
// closes the connection. The content is already complete, so the turn must
// finish normally — not hang, and not surface an error.
func TestOpenAIStream_WatchdogEndsTurnWhenPromisedUsageNeverArrives(t *testing.T) {
	prev := streamUsageGrace
	streamUsageGrace = 150 * time.Millisecond
	defer func() { streamUsageGrace = prev }()

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for _, l := range []string{
			`data: {"choices":[{"index":0,"delta":{"content":"done thinking"}}]}`,
			`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		} {
			w.Write([]byte(l + "\n\n"))
			flusher.Flush()
		}
		// Hold the response open, exactly like a gateway that never sends the
		// usage chunk it agreed to send.
		<-release
	}))
	defer srv.Close()
	defer close(release)

	p := newTestOpenAIProvider(t, srv)
	ch, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}
	content, terminal := drainStreamUsage(t, ch)
	if content != "done thinking" {
		t.Errorf("content = %q, want %q", content, "done thinking")
	}
	if !terminal.Done {
		t.Fatal("stream never produced a terminal chunk — the watchdog did not fire")
	}
	if terminal.Error != "" {
		t.Errorf("terminal carried an error (%q); closing the body ourselves must not be reported as a stream failure", terminal.Error)
	}
	if terminal.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want \"stop\"", terminal.FinishReason)
	}
}

// TestOpenAIStream_RetriesWithoutStreamOptionsOn400 is the compatibility
// valve: a strict endpoint that rejects the unknown field must not break chat.
// The retry drops stream_options and the refusal latches, so only the first
// request of the session pays for it.
func TestOpenAIStream_RetriesWithoutStreamOptionsOn400(t *testing.T) {
	var requests []openAIChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body openAIChatRequest
		json.Unmarshal(raw, &body)
		requests = append(requests, body)

		if body.StreamOptions != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":{"message":"unrecognized request argument supplied: stream_options"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for _, l := range []string{
			`data: {"choices":[{"index":0,"delta":{"content":"fine"}}]}`,
			`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		} {
			w.Write([]byte(l + "\n\n"))
			flusher.Flush()
		}
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	ch, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v — a rejected stream_options must be retried, not surfaced", err)
	}
	content, terminal := drainStreamUsage(t, ch)
	if content != "fine" {
		t.Errorf("content = %q, want %q", content, "fine")
	}
	if !terminal.Done {
		t.Error("no terminal chunk after the retry")
	}
	if len(requests) != 2 {
		t.Fatalf("server saw %d requests, want 2 (one rejected, one retried)", len(requests))
	}
	if requests[0].StreamOptions == nil || requests[1].StreamOptions != nil {
		t.Errorf("retry did not drop stream_options: %+v then %+v", requests[0].StreamOptions, requests[1].StreamOptions)
	}
	if !p.noStreamUsage.Load() {
		t.Error("the refusal did not latch — every later message would pay the same wasted round trip")
	}

	// Second call must go straight out without the field.
	ch2, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "again")}})
	if err != nil {
		t.Fatalf("second ChatCompletionStream() error = %v", err)
	}
	drainStreamUsage(t, ch2)
	if len(requests) != 3 {
		t.Fatalf("server saw %d requests, want 3", len(requests))
	}
	if requests[2].StreamOptions != nil {
		t.Error("third request still carried stream_options despite the latch")
	}
}

// TestOpenAIStream_Non400ErrorDoesNotLatchOrRetry keeps the valve narrow. A
// 401 or a 500 says nothing about field support; retrying on it would both
// double the failed requests and permanently disable usage reporting because
// of an expired API key.
func TestOpenAIStream_Non400ErrorDoesNotLatchOrRetry(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(status)
				w.Write([]byte(`{"error":{"message":"nope"}}`))
			}))
			defer srv.Close()

			p := newTestOpenAIProvider(t, srv)
			if _, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}}); err == nil {
				t.Fatal("expected an error")
			}
			if calls != 1 {
				t.Errorf("server saw %d requests, want 1 (%s says nothing about field support, so no retry)", calls, http.StatusText(status))
			}
			if p.noStreamUsage.Load() {
				t.Error("a non-field error latched the opt-out")
			}
		})
	}
}

// TestClaudeStream_EmitsUsageWithCacheSplitOnTerminalChunk: Anthropic splits a
// streamed turn's accounting across two events — message_start carries
// input_tokens and the (already final) cache figures, message_delta carries
// the running output_tokens. Both were previously read only for a log line.
func TestClaudeStream_EmitsUsageWithCacheSplitOnTerminalChunk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for _, l := range []string{
			`data: {"type":"message_start","message":{"usage":{"input_tokens":300,"output_tokens":1,"cache_read_input_tokens":7800,"cache_creation_input_tokens":0}}}`,
			`data: {"type":"content_block_delta","delta":{"text":"hi"}}`,
			`data: {"type":"message_delta","delta":{},"usage":{"output_tokens":42}}`,
			`data: {"type":"message_stop"}`,
		} {
			w.Write([]byte(l + "\n\n"))
			flusher.Flush()
		}
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-opus-4-6")
	ch, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}
	content, terminal := drainStreamUsage(t, ch)
	if content != "hi" {
		t.Errorf("content = %q, want %q", content, "hi")
	}
	if terminal.Usage == nil {
		t.Fatal("terminal chunk carried no Usage")
	}
	if terminal.Usage.PromptTokens != 8100 {
		t.Errorf("PromptTokens = %d, want 8100 (300 fresh + 7800 from cache)", terminal.Usage.PromptTokens)
	}
	if terminal.Usage.CachedPromptTokens != 7800 {
		t.Errorf("CachedPromptTokens = %d, want 7800", terminal.Usage.CachedPromptTokens)
	}
	if terminal.Usage.CompletionTokens != 42 {
		t.Errorf("CompletionTokens = %d, want 42 (message_delta's running total, not message_start's 1)", terminal.Usage.CompletionTokens)
	}
}

// TestGeminiStream_EmitsUsageFromUsageMetadata: Gemini repeats usageMetadata
// on every chunk with growing counts, so the last one is the total. The final
// accounting chunk can also arrive with no candidates at all, which the old
// candidates guard would have skipped.
func TestGeminiStream_EmitsUsageFromUsageMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for _, l := range []string{
			`data: {"candidates":[{"content":{"parts":[{"text":"he"}]}}],"usageMetadata":{"promptTokenCount":4000,"candidatesTokenCount":1,"totalTokenCount":4001,"cachedContentTokenCount":3200}}`,
			`data: {"candidates":[{"content":{"parts":[{"text":"y"}]}}]}`,
			`data: {"usageMetadata":{"promptTokenCount":4000,"candidatesTokenCount":9,"totalTokenCount":4009,"cachedContentTokenCount":3200}}`,
			`data: [DONE]`,
		} {
			w.Write([]byte(l + "\n\n"))
			flusher.Flush()
		}
	}))
	defer srv.Close()

	p := newTestGeminiProvider(t, srv, "gemini-2.5-pro")
	ch, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletionStream() error = %v", err)
	}
	content, terminal := drainStreamUsage(t, ch)
	if content != "hey" {
		t.Errorf("content = %q, want %q", content, "hey")
	}
	if terminal.Usage == nil {
		t.Fatal("terminal chunk carried no Usage")
	}
	if terminal.Usage.CompletionTokens != 9 {
		t.Errorf("CompletionTokens = %d, want 9 (the last usageMetadata wins)", terminal.Usage.CompletionTokens)
	}
	if terminal.Usage.CachedPromptTokens != 3200 {
		t.Errorf("CachedPromptTokens = %d, want 3200", terminal.Usage.CachedPromptTokens)
	}
	if got := terminal.Usage.FreshPromptTokens(); got != 800 {
		t.Errorf("FreshPromptTokens() = %d, want 800", got)
	}
}
