// SPDX-License-Identifier: AGPL-3.0-or-later

package openaiapi

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
		"messages": [
			{"role": "system", "content": "You are a helpful assistant."},
			{"role": "user", "content": "hello"}
		]
	}`)
	req, err := ParseRequest(body)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	if req.Model != "local/qwen2.5" {
		t.Errorf("Model = %q", req.Model)
	}

	chatReq := ToChatRequest(req)
	if len(chatReq.Messages) != 2 {
		t.Fatalf("Messages = %+v", chatReq.Messages)
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

func TestParseRequest_ArrayContent(t *testing.T) {
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

func TestToChatRequest_Tools(t *testing.T) {
	body := []byte(`{
		"model": "openai/gpt-4o",
		"messages": [{"role": "user", "content": "what's the weather in SF?"}],
		"tools": [{"type":"function","function":{"name": "get_weather", "description": "Get the weather", "parameters": {"type":"object","properties":{"location":{"type":"string"}},"required":["location"]}}}]
	}`)
	req, err := ParseRequest(body)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	chatReq := ToChatRequest(req)
	if len(chatReq.Tools) != 1 {
		t.Fatalf("Tools = %+v", chatReq.Tools)
	}
	tool := chatReq.Tools[0]
	if tool.Type != "function" || tool.Function.Name != "get_weather" || tool.Function.Description != "Get the weather" {
		t.Errorf("tool = %+v", tool)
	}
	var schema map[string]any
	if err := json.Unmarshal(tool.Function.Parameters, &schema); err != nil {
		t.Fatalf("Parameters not valid JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("schema = %+v", schema)
	}
}

func TestToChatRequest_AssistantToolCalls(t *testing.T) {
	body := []byte(`{
		"model": "openai/gpt-4o",
		"messages": [
			{"role": "user", "content": "what's the weather in SF?"},
			{"role": "assistant", "content": "Let me check.", "tool_calls": [
				{"id": "call_01ABC", "type": "function", "function": {"name": "get_weather", "arguments": "{\"location\":\"SF\"}"}}
			]}
		]
	}`)
	req, err := ParseRequest(body)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	chatReq := ToChatRequest(req)
	if len(chatReq.Messages) != 2 {
		t.Fatalf("Messages = %+v", chatReq.Messages)
	}
	asst := chatReq.Messages[1]
	if asst.Role != "assistant" || asst.Content != "Let me check." {
		t.Errorf("assistant message = %+v", asst)
	}
	if len(asst.ToolCalls) != 1 {
		t.Fatalf("ToolCalls = %+v", asst.ToolCalls)
	}
	tc := asst.ToolCalls[0]
	if tc.ID != "call_01ABC" || tc.Function.Name != "get_weather" {
		t.Errorf("tool call = %+v", tc)
	}
	// Arguments is copied through verbatim as raw wire bytes — OpenAI's
	// function.arguments is itself a JSON string containing the object's
	// text, so tc.Function.Arguments must still be that JSON string (quotes
	// and all), not already-unwrapped into a plain object.
	var argsText string
	if err := json.Unmarshal(tc.Function.Arguments, &argsText); err != nil {
		t.Fatalf("Arguments is not a JSON string: %v (raw: %s)", err, tc.Function.Arguments)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(argsText), &args); err != nil {
		t.Fatalf("Arguments string content not valid JSON: %v", err)
	}
	if args["location"] != "SF" {
		t.Errorf("args = %+v", args)
	}
}

func TestToChatRequest_ToolResultMessage(t *testing.T) {
	body := []byte(`{
		"model": "openai/gpt-4o",
		"messages": [
			{"role": "tool", "tool_call_id": "call_01ABC", "content": "Sunny, 65F"}
		]
	}`)
	req, err := ParseRequest(body)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	chatReq := ToChatRequest(req)
	if len(chatReq.Messages) != 1 {
		t.Fatalf("Messages = %+v", chatReq.Messages)
	}
	toolMsg := chatReq.Messages[0]
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != "call_01ABC" || toolMsg.Content != "Sunny, 65F" {
		t.Errorf("tool message = %+v", toolMsg)
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
	if decoded["object"] != "chat.completion" {
		t.Errorf("object = %v, want chat.completion", decoded["object"])
	}
	choices := decoded["choices"].([]any)
	if len(choices) != 1 {
		t.Fatalf("choices = %+v", choices)
	}
	choice := choices[0].(map[string]any)
	if choice["finish_reason"] != "stop" {
		t.Errorf("finish_reason = %v, want stop", choice["finish_reason"])
	}
	message := choice["message"].(map[string]any)
	if message["role"] != "assistant" || message["content"] != "hello there" {
		t.Errorf("message = %+v", message)
	}
	usage := decoded["usage"].(map[string]any)
	if usage["prompt_tokens"] != float64(10) || usage["completion_tokens"] != float64(3) || usage["total_tokens"] != float64(13) {
		t.Errorf("usage = %+v", usage)
	}
}

func TestWriteNonStream_ToolCalls(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := provider.ChatResponse{
		// Deliberately no text — a pure tool-call turn, which must serialize
		// content as null, not "".
		ToolCalls: []provider.ToolCall{
			// Arguments is JSON-string-encoded (`"{\"location\":\"SF\"}"`),
			// matching the real OpenAI wire format for function.arguments —
			// not a raw embedded object. This is the invariant
			// toWireToolCalls' comment documents: RawMessage embeds these
			// bytes verbatim, so passing a raw object here (instead of a
			// JSON string) would be testing an input shape that never
			// actually occurs on the wire.
			{ID: "call_01ABC", Type: "function", Function: provider.ToolCallFunction{
				Name: "get_weather", Arguments: json.RawMessage(`"{\"location\":\"SF\"}"`),
			}},
		},
	}
	// finishReason "stop" is deliberately passed — a tool call must still win
	// over whatever finish reason the backend reported.
	if err := WriteNonStream(rec, "openai/gpt-4o", resp, "stop"); err != nil {
		t.Fatalf("WriteNonStream: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	choice := decoded["choices"].([]any)[0].(map[string]any)
	if choice["finish_reason"] != "tool_calls" {
		t.Errorf("finish_reason = %v, want tool_calls", choice["finish_reason"])
	}
	message := choice["message"].(map[string]any)
	if message["content"] != nil {
		t.Errorf("content = %v, want nil for a pure tool-call turn", message["content"])
	}
	toolCalls := message["tool_calls"].([]any)
	if len(toolCalls) != 1 {
		t.Fatalf("tool_calls = %+v", toolCalls)
	}
	tc := toolCalls[0].(map[string]any)
	fn := tc["function"].(map[string]any)
	// fn["arguments"] decodes back to a plain Go string (not nested JSON) —
	// proof the value was embedded as a JSON string on the wire, exactly
	// once-encoded, not double-encoded or left as a raw object.
	if fn["name"] != "get_weather" || fn["arguments"] != `{"location":"SF"}` {
		t.Errorf("function = %+v", fn)
	}
}

// flushRecorder adapts httptest.ResponseRecorder (which doesn't implement
// http.Flusher) so StreamSSE's flusher.Flush() call doesn't panic.
type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f flushRecorder) Flush() {}

func TestStreamSSE_ChunkSequence(t *testing.T) {
	ch := make(chan provider.StreamChunk, 4)
	ch <- provider.StreamChunk{Content: "Hello "}
	ch <- provider.StreamChunk{Content: "world"}
	ch <- provider.StreamChunk{Done: true, FinishReason: "stop"}
	close(ch)

	rec := flushRecorder{httptest.NewRecorder()}
	fullText := StreamSSE(context.Background(), rec, rec, "local/qwen2.5", ch)
	if fullText != "Hello world" {
		t.Errorf("accumulated text = %q, want %q", fullText, "Hello world")
	}

	body := rec.Body.String()
	var dataLines []string
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		if data, ok := strings.CutPrefix(scanner.Text(), "data: "); ok {
			dataLines = append(dataLines, data)
		}
	}
	// role chunk, "Hello " chunk, "world" chunk, finish_reason chunk, [DONE]
	if len(dataLines) != 5 {
		t.Fatalf("data lines = %v, want 5", dataLines)
	}
	if dataLines[len(dataLines)-1] != "[DONE]" {
		t.Errorf("last line = %q, want [DONE]", dataLines[len(dataLines)-1])
	}
	if !strings.Contains(body, `"content":"Hello "`) {
		t.Errorf("missing first content delta in body: %s", body)
	}
	if !strings.Contains(body, `"finish_reason":"stop"`) {
		t.Errorf("missing finish_reason in body: %s", body)
	}
}

func TestStreamSSE_PropagatesError(t *testing.T) {
	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Error: "boom", Done: true}
	close(ch)

	rec := flushRecorder{httptest.NewRecorder()}
	StreamSSE(context.Background(), rec, rec, "local/qwen2.5", ch)

	body := rec.Body.String()
	if !strings.Contains(body, "boom") {
		t.Errorf("expected error message in body, got: %s", body)
	}
	if strings.Contains(body, "[DONE]") {
		t.Errorf("an errored stream must not also emit [DONE]: %s", body)
	}
}

func TestStreamSSEFromResponse_ToolCalls(t *testing.T) {
	resp := provider.ChatResponse{
		Content: "Let me check.",
		ToolCalls: []provider.ToolCall{
			// JSON-string-encoded, matching the real OpenAI wire format —
			// see the comment in TestWriteNonStream_ToolCalls.
			{ID: "call_01ABC", Type: "function", Function: provider.ToolCallFunction{
				Name: "get_weather", Arguments: json.RawMessage(`"{\"location\":\"SF\"}"`),
			}},
		},
	}

	rec := flushRecorder{httptest.NewRecorder()}
	fullText := StreamSSEFromResponse(rec, rec, "openai/gpt-4o", resp, "")
	if fullText != "Let me check." {
		t.Errorf("returned text = %q", fullText)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"finish_reason":"tool_calls"`) {
		t.Errorf("expected finish_reason tool_calls, got: %s", body)
	}
	if !strings.Contains(body, `"name":"get_weather"`) {
		t.Errorf("missing tool call function name in body: %s", body)
	}
	if !strings.Contains(body, `"arguments":"{\"location\":\"SF\"}"`) {
		t.Errorf("expected arguments embedded as a JSON string (not double-encoded), got: %s", body)
	}
	if !strings.Contains(body, "[DONE]") {
		t.Errorf("missing [DONE] terminator: %s", body)
	}
}

func TestStreamSSEFromResponse_TextOnly(t *testing.T) {
	resp := provider.ChatResponse{Content: "just a plain answer"}
	rec := flushRecorder{httptest.NewRecorder()}
	fullText := StreamSSEFromResponse(rec, rec, "local/qwen2.5", resp, "stop")
	if fullText != "just a plain answer" {
		t.Errorf("returned text = %q", fullText)
	}
	if !strings.Contains(rec.Body.String(), `"finish_reason":"stop"`) {
		t.Errorf("expected stop finish_reason: %s", rec.Body.String())
	}
}

func TestCollectStream_Success(t *testing.T) {
	ch := make(chan provider.StreamChunk, 3)
	ch <- provider.StreamChunk{Content: "Hello "}
	ch <- provider.StreamChunk{Content: "world"}
	ch <- provider.StreamChunk{Done: true, FinishReason: "stop"}
	close(ch)

	content, finishReason, errMsg := CollectStream(context.Background(), ch)
	if content != "Hello world" {
		t.Errorf("content = %q", content)
	}
	if finishReason != "stop" {
		t.Errorf("finishReason = %q", finishReason)
	}
	if errMsg != "" {
		t.Errorf("errMsg = %q, want empty", errMsg)
	}
}

func TestCollectStream_PropagatesError(t *testing.T) {
	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Content: "partial"}
	ch <- provider.StreamChunk{Error: "boom", Done: true}
	close(ch)

	content, _, errMsg := CollectStream(context.Background(), ch)
	if content != "partial" {
		t.Errorf("content = %q, want partial content collected before the error", content)
	}
	if errMsg != "boom" {
		t.Errorf("errMsg = %q, want boom", errMsg)
	}
}

func TestEstimateTokens(t *testing.T) {
	messages := []provider.Message{
		{Role: "system", Content: "You are a helpful assistant"},
		{Role: "user", Content: "hello there friend"},
	}
	got := EstimateTokens(messages)
	if got <= 0 {
		t.Errorf("EstimateTokens = %d, want > 0", got)
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := WriteError(rec, 400, `model must be "type/model-id"`); err != nil {
		t.Fatalf("WriteError: %v", err)
	}
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	errObj, ok := decoded["error"].(map[string]any)
	if !ok || errObj["message"] != `model must be "type/model-id"` {
		t.Errorf("error object = %+v", decoded["error"])
	}
}

func TestWriteModelList(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := WriteModelList(rec, []string{"local/qwen2.5", "openai/gpt-4o"}); err != nil {
		t.Fatalf("WriteModelList: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded["object"] != "list" {
		t.Errorf("object = %v, want list", decoded["object"])
	}
	data := decoded["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("data = %+v", data)
	}
	first := data[0].(map[string]any)
	if first["id"] != "local/qwen2.5" || first["object"] != "model" {
		t.Errorf("first entry = %+v", first)
	}
}
