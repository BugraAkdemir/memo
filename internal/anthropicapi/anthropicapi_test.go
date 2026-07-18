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

func TestToChatRequest_Tools(t *testing.T) {
	body := []byte(`{
		"model": "openai/gpt-4o",
		"messages": [{"role": "user", "content": "what's the weather in SF?"}],
		"tools": [{"name": "get_weather", "description": "Get the weather", "input_schema": {"type":"object","properties":{"location":{"type":"string"}},"required":["location"]}}]
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

func TestToChatRequest_AssistantToolUseBlock(t *testing.T) {
	body := []byte(`{
		"model": "openai/gpt-4o",
		"messages": [
			{"role": "user", "content": "what's the weather in SF?"},
			{"role": "assistant", "content": [
				{"type": "text", "text": "Let me check."},
				{"type": "tool_use", "id": "toolu_01ABC", "name": "get_weather", "input": {"location": "SF"}}
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
	if tc.ID != "toolu_01ABC" || tc.Function.Name != "get_weather" {
		t.Errorf("tool call = %+v", tc)
	}
	// Arguments must be JSON-string-encoded (OpenAI's wire format for
	// function.arguments), not a raw embedded object — unmarshal into a
	// string first, then parse that string's content.
	var argsText string
	if err := json.Unmarshal(tc.Function.Arguments, &argsText); err != nil {
		t.Fatalf("Arguments is not a JSON string: %v (raw: %s)", err, tc.Function.Arguments)
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(argsText), &input); err != nil {
		t.Fatalf("Arguments string content not valid JSON: %v", err)
	}
	if input["location"] != "SF" {
		t.Errorf("input = %+v", input)
	}
}

func TestToChatRequest_UserToolResultBlock(t *testing.T) {
	body := []byte(`{
		"model": "openai/gpt-4o",
		"messages": [
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "toolu_01ABC", "content": "Sunny, 65F"}
			]}
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
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != "toolu_01ABC" || toolMsg.Content != "Sunny, 65F" {
		t.Errorf("tool result message = %+v", toolMsg)
	}
}

func TestToChatRequest_UserToolResultPlusText(t *testing.T) {
	body := []byte(`{
		"model": "openai/gpt-4o",
		"messages": [
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "toolu_01ABC", "content": "Sunny, 65F"},
				{"type": "text", "text": "thanks, what about tomorrow?"}
			]}
		]
	}`)
	req, err := ParseRequest(body)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	chatReq := ToChatRequest(req)
	if len(chatReq.Messages) != 2 {
		t.Fatalf("Messages = %+v, want a tool message + a trailing user text message", chatReq.Messages)
	}
	if chatReq.Messages[0].Role != "tool" || chatReq.Messages[0].ToolCallID != "toolu_01ABC" {
		t.Errorf("tool message = %+v", chatReq.Messages[0])
	}
	if chatReq.Messages[1].Role != "user" || chatReq.Messages[1].Content != "thanks, what about tomorrow?" {
		t.Errorf("trailing user message = %+v", chatReq.Messages[1])
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

func TestWriteNonStream_ToolCalls(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := provider.ChatResponse{
		Content: "Let me check.",
		ToolCalls: []provider.ToolCall{
			// Arguments is JSON-string-encoded (`"{\"location\":\"SF\"}"`),
			// matching the real OpenAI wire format for function.arguments —
			// not a raw embedded object. This is the shape a real backend
			// actually produces; toAnthropicContentBlocks must unwrap it back
			// into a real object for Anthropic's tool_use.input.
			{ID: "toolu_01ABC", Type: "function", Function: provider.ToolCallFunction{
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
	if decoded["stop_reason"] != "tool_use" {
		t.Errorf("stop_reason = %v, want tool_use", decoded["stop_reason"])
	}
	content := decoded["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("content = %+v, want [text, tool_use]", content)
	}
	textBlock := content[0].(map[string]any)
	if textBlock["type"] != "text" || textBlock["text"] != "Let me check." {
		t.Errorf("text block = %+v", textBlock)
	}
	toolBlock := content[1].(map[string]any)
	if toolBlock["type"] != "tool_use" || toolBlock["id"] != "toolu_01ABC" || toolBlock["name"] != "get_weather" {
		t.Errorf("tool_use block = %+v", toolBlock)
	}
	input := toolBlock["input"].(map[string]any)
	if input["location"] != "SF" {
		t.Errorf("input = %+v", input)
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

func TestStreamSSEFromResponse_ToolUse(t *testing.T) {
	resp := provider.ChatResponse{
		Content: "Let me check.",
		ToolCalls: []provider.ToolCall{
			// JSON-string-encoded, matching the real OpenAI wire format —
			// see the comment in TestWriteNonStream_ToolCalls.
			{ID: "toolu_01ABC", Type: "function", Function: provider.ToolCallFunction{
				Name: "get_weather", Arguments: json.RawMessage(`"{\"location\":\"SF\"}"`),
			}},
		},
		Usage: &provider.Usage{PromptTokens: 20, CompletionTokens: 5},
	}

	rec := flushRecorder{httptest.NewRecorder()}
	fullText := StreamSSEFromResponse(rec, rec, "openai/gpt-4o", 20, resp, "tool_calls")
	if fullText != "Let me check." {
		t.Errorf("returned text = %q", fullText)
	}

	body := rec.Body.String()
	var events []string
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		if name, ok := strings.CutPrefix(scanner.Text(), "event: "); ok {
			events = append(events, name)
		}
	}
	want := []string{
		"message_start",
		"content_block_start", "content_block_delta", "content_block_stop", // text block
		"content_block_start", "content_block_delta", "content_block_stop", // tool_use block
		"message_delta", "message_stop",
	}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i, e := range want {
		if events[i] != e {
			t.Errorf("event[%d] = %q, want %q", i, events[i], e)
		}
	}

	if !strings.Contains(body, `"type":"tool_use"`) {
		t.Errorf("missing tool_use content_block_start in body: %s", body)
	}
	if !strings.Contains(body, `"partial_json":"{\"location\":\"SF\"}"`) {
		t.Errorf("missing tool arguments as input_json_delta in body: %s", body)
	}
	if !strings.Contains(body, `"stop_reason":"tool_use"`) {
		t.Errorf("expected stop_reason tool_use in message_delta, got: %s", body)
	}
}

func TestStreamSSEFromResponse_TextOnly(t *testing.T) {
	resp := provider.ChatResponse{Content: "just a plain answer"}
	rec := flushRecorder{httptest.NewRecorder()}
	fullText := StreamSSEFromResponse(rec, rec, "local/qwen2.5", 5, resp, "stop")
	if fullText != "just a plain answer" {
		t.Errorf("returned text = %q", fullText)
	}
	if !strings.Contains(rec.Body.String(), `"stop_reason":"end_turn"`) {
		t.Errorf("expected end_turn stop_reason for a text-only response: %s", rec.Body.String())
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
	if err := WriteError(rec, 400, "model must be \"type/model-id\""); err != nil {
		t.Fatalf("WriteError: %v", err)
	}
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded["type"] != "error" {
		t.Errorf("type = %v, want error", decoded["type"])
	}
	errObj, ok := decoded["error"].(map[string]any)
	if !ok || errObj["message"] != `model must be "type/model-id"` {
		t.Errorf("error object = %+v", decoded["error"])
	}
}

// TestToolArgumentsRoundTrip is the regression test for the argument-
// encoding bug found via a live smoke test: Anthropic's tool_use.input is a
// real JSON object, but OpenAI's function.arguments is a JSON *string*
// containing that object's text — conflating the two either double-encodes
// or leaves the object literally unparsed (Claude Code would receive an
// "input" field holding the string "{\"location\":\"SF\"}" instead of an
// object). anthropicInputToOpenAIArguments/openAIArgumentsToJSONText must be
// exact inverses.
func TestToolArgumentsRoundTrip(t *testing.T) {
	anthropicInput := json.RawMessage(`{"location":"SF","unit":"celsius"}`)

	openAIArgs := anthropicInputToOpenAIArguments(anthropicInput)
	// On the wire, this must be a JSON *string* (quoted), not a raw object.
	var asString string
	if err := json.Unmarshal(openAIArgs, &asString); err != nil {
		t.Fatalf("anthropicInputToOpenAIArguments did not produce a JSON string: %v (got: %s)", err, openAIArgs)
	}

	back := openAIArgumentsToJSONText(openAIArgs)
	var obj map[string]any
	if err := json.Unmarshal([]byte(back), &obj); err != nil {
		t.Fatalf("openAIArgumentsToJSONText output not valid JSON: %v (got: %s)", err, back)
	}
	if obj["location"] != "SF" || obj["unit"] != "celsius" {
		t.Errorf("round-tripped object = %+v, want the original fields preserved", obj)
	}
}
