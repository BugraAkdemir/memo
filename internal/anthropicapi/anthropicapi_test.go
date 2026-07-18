// SPDX-License-Identifier: AGPL-3.0-or-later

package anthropicapi

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"memo/internal/provider"
)

func TestParseRequest_StringContent(t *testing.T) {
	body := []byte(`{
		"model": "local/qwen2.5",
		"max_tokens": 512,
		"system": "You are a helpful assistant.",
		"messages": [{"role": "user", "content": "hello"}]
	}`)
	req, err := ParseRequest(body)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	if req.Model != "local/qwen2.5" {
		t.Errorf("Model = %q", req.Model)
	}
	if req.MaxTokens != 512 {
		t.Errorf("MaxTokens = %d", req.MaxTokens)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" {
		t.Fatalf("Messages = %+v", req.Messages)
	}

	chatReq := ToChatRequest(req)
	if len(chatReq.Messages) != 2 {
		t.Fatalf("ToChatRequest messages = %+v, want system+user", chatReq.Messages)
	}
	if chatReq.Messages[0].Role != "system" || chatReq.Messages[0].Content != "You are a helpful assistant." {
		t.Errorf("system message = %+v", chatReq.Messages[0])
	}
	if chatReq.Messages[1].Role != "user" || chatReq.Messages[1].Content != "hello" {
		t.Errorf("user message = %+v", chatReq.Messages[1])
	}
	if chatReq.MaxTokens != 512 {
		t.Errorf("MaxTokens = %d", chatReq.MaxTokens)
	}
}

func TestParseRequest_BlockContent(t *testing.T) {
	body := []byte(`{
		"model": "openai/gpt-4o",
		"messages": [{"role": "user", "content": [{"type":"text","text":"part one"},{"type":"text","text":"part two"}]}]
	}`)
	req, err := ParseRequest(body)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	chatReq := ToChatRequest(req)
	if len(chatReq.Messages) != 1 {
		t.Fatalf("Messages = %+v", chatReq.Messages)
	}
	want := "part one\npart two"
	if chatReq.Messages[0].Content != want {
		t.Errorf("Content = %q, want %q", chatReq.Messages[0].Content, want)
	}
}

func TestToChatRequest_DefaultsMaxTokens(t *testing.T) {
	req := Request{Messages: []Message{{Role: "user", Content: json.RawMessage(`"hi"`)}}}
	chatReq := ToChatRequest(req)
	if chatReq.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want default 4096", chatReq.MaxTokens)
	}
}

func TestWriteNonStream(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := provider.ChatResponse{
		Content: "hello there",
		Usage:   &provider.Usage{PromptTokens: 10, CompletionTokens: 3},
	}
	if err := WriteNonStream(rec, "local/qwen2.5", resp, "stop"); err != nil {
		t.Fatalf("WriteNonStream: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v (body: %s)", err, rec.Body.String())
	}
	if decoded["type"] != "message" || decoded["role"] != "assistant" {
		t.Errorf("decoded = %+v", decoded)
	}
	if decoded["stop_reason"] != "end_turn" {
		t.Errorf("stop_reason = %v, want end_turn", decoded["stop_reason"])
	}
	content, ok := decoded["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("content = %+v", decoded["content"])
	}
	block := content[0].(map[string]any)
	if block["text"] != "hello there" {
		t.Errorf("block text = %v", block["text"])
	}
	usage := decoded["usage"].(map[string]any)
	if usage["input_tokens"] != float64(10) || usage["output_tokens"] != float64(3) {
		t.Errorf("usage = %+v", usage)
	}
}

// flushRecorder adapts httptest.ResponseRecorder (which doesn't implement
// http.Flusher) so StreamSSE's flusher.Flush() call doesn't panic.
type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f flushRecorder) Flush() {}

func TestStreamSSE_EventSequence(t *testing.T) {
	ch := make(chan provider.StreamChunk, 4)
	ch <- provider.StreamChunk{Content: "Hello "}
	ch <- provider.StreamChunk{Content: "world"}
	ch <- provider.StreamChunk{Done: true, FinishReason: "stop"}
	close(ch)

	rec := flushRecorder{httptest.NewRecorder()}
	fullText := StreamSSE(context.Background(), rec, rec, "local/qwen2.5", 7, ch)
	if fullText != "Hello world" {
		t.Errorf("accumulated text = %q, want %q", fullText, "Hello world")
	}

	var events []string
	scanner := bufio.NewScanner(strings.NewReader(rec.Body.String()))
	for scanner.Scan() {
		line := scanner.Text()
		if name, ok := strings.CutPrefix(line, "event: "); ok {
			events = append(events, name)
		}
	}
	want := []string{
		"message_start", "content_block_start",
		"content_block_delta", "content_block_delta",
		"content_block_stop", "message_delta", "message_stop",
	}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i, e := range want {
		if events[i] != e {
			t.Errorf("event[%d] = %q, want %q", i, events[i], e)
		}
	}

	if !strings.Contains(rec.Body.String(), `"text":"Hello "`) {
		t.Errorf("missing first delta text in body: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"input_tokens":7`) {
		t.Errorf("missing prompt token count in message_start: %s", rec.Body.String())
	}
}

func TestStreamSSE_PropagatesError(t *testing.T) {
	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Error: "boom", Done: true}
	close(ch)

	rec := flushRecorder{httptest.NewRecorder()}
	StreamSSE(context.Background(), rec, rec, "local/qwen2.5", 0, ch)

	body := rec.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Errorf("expected an error event, got: %s", body)
	}
	if !strings.Contains(body, "boom") {
		t.Errorf("expected error message in body, got: %s", body)
	}
	// A propagated error must stop before the normal completion sequence —
	// message_stop should never appear after an error event.
	if idx := strings.Index(body, "event: error"); idx >= 0 && strings.Contains(body[idx:], "message_stop") {
		t.Errorf("message_stop appeared after error event: %s", body)
	}
}
