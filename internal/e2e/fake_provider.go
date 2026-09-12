// SPDX-License-Identifier: AGPL-3.0-or-later

// Package e2e is Memo's real end-to-end test layer: it boots an actual
// app.App behind an actual net/http server on a real port (exactly what
// main.go does for a headless backend) and drives it with a real
// net/http.Client hitting the real REST API — no unit-level mocks. The one
// thing it does replace is the LLM itself: FakeProvider stands in for a
// real OpenAI-compatible endpoint, scripted per test, so tests are fast and
// deterministic while everything else (SSE streaming, agent tool-calling,
// the permission-request round trip, the Self-Driving task loop's real
// state machine) runs unmodified, real code.
//
// This deliberately does NOT try to assert on real model output — a real
// LLM's wording isn't deterministic, so the harness's later helper methods
// (Harness.SendAndCollect, etc.) are meant to be asserted against by
// *behavior* (did a permission_request event fire, did the task list reach
// "done", did the SSE stream terminate cleanly) rather than exact text. A
// small, separate "real tiny local model" smoke test tier (Phi-3-mini,
// already present under data/models/ in dev environments) is the natural
// next layer on top of this one for the few cases that specifically need to
// prove real-model output shape, not scripted logic — not built here.
package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// FakeToolCall is one tool call a FakeChatResponse asks the agent pipeline
// to make. Arguments is the raw JSON object text for the call (e.g.
// `{"path":"x.txt","content":"hi"}`) — sent on the wire as a nested JSON
// object, not a string-encoded one, matching how this codebase's own
// internal/provider/openai_test.go constructs tool-call fixtures (see
// TestOpenAIProvider_ChatCompletion_ParsesToolCalls).
type FakeToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// FakeChatResponse is what FakeProvider's script returns for one call. Set
// exactly one of Text or ToolCalls.
type FakeChatResponse struct {
	Text      string
	ToolCalls []FakeToolCall
}

// FakeChatRequest is the decoded request body FakeProvider hands to the
// test's script function, plus the fields most tests need to assert
// against without re-decoding JSON themselves.
type FakeChatRequest struct {
	Model    string
	Stream   bool
	Messages []json.RawMessage
	Tools    []json.RawMessage
	Raw      []byte
}

// FakeProvider is an httptest.Server speaking just enough of the
// OpenAI-compatible /chat/completions wire format (internal/provider/openai.go)
// to drive a real App: both the non-streaming path (used for any
// tool-bearing turn — see internal/app/llm.go's own comment on why) and the
// SSE streaming path (used for plain-text turns).
type FakeProvider struct {
	// Script decides the response for each call. callNum is 1-based and
	// increments across the whole test (not per-chat), so a test scripting
	// a tool-call round trip (call 1: request a tool; call 2: after the
	// tool result comes back as a message, reply with text) can switch on
	// it directly.
	Script func(callNum int, req FakeChatRequest) FakeChatResponse

	Srv *httptest.Server

	mu       sync.Mutex
	callNum  int
	requests []FakeChatRequest
}

// NewFakeProvider starts the server. Script defaults to always replying
// with the fixed text "ok" if left nil after construction — set it before
// or right after calling this.
func NewFakeProvider(t *testing.T) *FakeProvider {
	t.Helper()
	fp := &FakeProvider{
		Script: func(int, FakeChatRequest) FakeChatResponse {
			return FakeChatResponse{Text: "ok"}
		},
	}
	fp.Srv = httptest.NewServer(http.HandlerFunc(fp.handle))
	t.Cleanup(fp.Srv.Close)
	return fp
}

// Requests returns every request received so far, in order — for tests
// that want to assert on what the app actually sent (e.g. that a tool
// result message was included in a follow-up call).
func (fp *FakeProvider) Requests() []FakeChatRequest {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	out := make([]FakeChatRequest, len(fp.requests))
	copy(out, fp.requests)
	return out
}

func (fp *FakeProvider) handle(w http.ResponseWriter, r *http.Request) {
	body, err := decodeChatRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fp.mu.Lock()
	fp.callNum++
	callNum := fp.callNum
	fp.requests = append(fp.requests, body)
	fp.mu.Unlock()

	resp := fp.Script(callNum, body)

	if body.Stream {
		writeStreamingResponse(w, resp)
		return
	}
	writeNonStreamingResponse(w, resp)
}

func decodeChatRequest(r *http.Request) (FakeChatRequest, error) {
	var raw struct {
		Model    string            `json:"model"`
		Stream   bool              `json:"stream"`
		Messages []json.RawMessage `json:"messages"`
		Tools    []json.RawMessage `json:"tools"`
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&raw); err != nil {
		return FakeChatRequest{}, fmt.Errorf("fake provider: decode request: %w", err)
	}
	return FakeChatRequest{
		Model:    raw.Model,
		Stream:   raw.Stream,
		Messages: raw.Messages,
		Tools:    raw.Tools,
	}, nil
}

// --- non-streaming response, matching internal/provider/openai.go's
// openAIResponse/openAIChoice/openAIResponseMsg shapes exactly ---

func writeNonStreamingResponse(w http.ResponseWriter, resp FakeChatResponse) {
	type toolCallFunc struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	type toolCall struct {
		ID       string       `json:"id"`
		Type     string       `json:"type"`
		Function toolCallFunc `json:"function"`
	}
	type respMsg struct {
		Role      string     `json:"role"`
		Content   string     `json:"content"`
		ToolCalls []toolCall `json:"tool_calls,omitempty"`
	}
	type choice struct {
		Index        int     `json:"index"`
		Message      respMsg `json:"message"`
		FinishReason string  `json:"finish_reason"`
	}

	msg := respMsg{Role: "assistant", Content: resp.Text}
	finish := "stop"
	if len(resp.ToolCalls) > 0 {
		finish = "tool_calls"
		for _, tc := range resp.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, toolCall{
				ID:   tc.ID,
				Type: "function",
				Function: toolCallFunc{
					Name:      tc.Name,
					Arguments: json.RawMessage(tc.Arguments),
				},
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":      "fake-resp",
		"object":  "chat.completion",
		"choices": []choice{{Message: msg, FinishReason: finish}},
	})
}

// --- streaming response, matching openai.go's SSE `data: {...}` /
// `data: [DONE]` shape exactly (openAIStreamChunk/openAIStreamChoice/
// openAIStreamDelta) ---

func writeStreamingResponse(w http.ResponseWriter, resp FakeChatResponse) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)

	writeChunk := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	type delta struct {
		Content string `json:"content,omitempty"`
	}
	type streamChoice struct {
		Index        int     `json:"index"`
		Delta        delta   `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	}

	// One content delta per call (real providers stream token-by-token;
	// scripted tests don't need that granularity to exercise the real SSE
	// plumbing end to end).
	if resp.Text != "" {
		writeChunk(map[string]any{"choices": []streamChoice{{Delta: delta{Content: resp.Text}}}})
	}
	stop := "stop"
	writeChunk(map[string]any{"choices": []streamChoice{{FinishReason: &stop}}})
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}
