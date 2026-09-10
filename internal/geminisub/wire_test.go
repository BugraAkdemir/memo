// SPDX-License-Identifier: AGPL-3.0-or-later

package geminisub

import (
	"encoding/json"
	"testing"

	"memo/internal/provider"
)

func TestBuildGenContentRequest_SystemAndRoles(t *testing.T) {
	req := provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "system", Content: "be terse"},
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "hello"},
		},
	}
	got := buildGenContentRequest(req)

	if got.SystemInstruction == nil || got.SystemInstruction.Parts[0].Text != "be terse" {
		t.Fatalf("systemInstruction not set: %+v", got.SystemInstruction)
	}
	if len(got.Contents) != 2 {
		t.Fatalf("contents len = %d, want 2", len(got.Contents))
	}
	if got.Contents[0].Role != "user" || got.Contents[0].Parts[0].Text != "hi" {
		t.Errorf("contents[0] = %+v", got.Contents[0])
	}
	if got.Contents[1].Role != "model" || got.Contents[1].Parts[0].Text != "hello" {
		t.Errorf("assistant role not mapped to model: %+v", got.Contents[1])
	}
}

func TestBuildGenContentRequest_ToolRoundTrip(t *testing.T) {
	req := provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "run it"},
			{Role: "assistant", ToolCalls: []provider.ToolCall{{
				ID:       "call_1",
				Type:     "function",
				Function: provider.ToolCallFunction{Name: "do_thing", Arguments: json.RawMessage(`{"x":1}`)},
			}}},
			{Role: "tool", ToolCallID: "call_1", Content: "it worked"},
		},
		Tools: []provider.ToolDefinition{{
			Type:     "function",
			Function: provider.ToolFunction{Name: "do_thing", Description: "does the thing", Parameters: json.RawMessage(`{"type":"object"}`)},
		}},
	}
	got := buildGenContentRequest(req)

	// assistant turn echoes a functionCall part
	var sawCall, sawResp bool
	for _, c := range got.Contents {
		for _, p := range c.Parts {
			if p.FunctionCall != nil && p.FunctionCall.Name == "do_thing" && c.Role == "model" {
				sawCall = true
			}
			if p.FunctionResponse != nil && c.Role == "user" {
				sawResp = true
				if p.FunctionResponse.Name != "do_thing" {
					t.Errorf("functionResponse name = %q, want do_thing (recovered from call id)", p.FunctionResponse.Name)
				}
				if string(p.FunctionResponse.Response) != `{"result":"it worked"}` {
					t.Errorf("functionResponse payload = %s", p.FunctionResponse.Response)
				}
			}
		}
	}
	if !sawCall || !sawResp {
		t.Fatalf("missing functionCall(%v)/functionResponse(%v) in %+v", sawCall, sawResp, got.Contents)
	}

	if len(got.Tools) != 1 || len(got.Tools[0].FunctionDeclarations) != 1 {
		t.Fatalf("tools not wrapped into a single functionDeclarations entry: %+v", got.Tools)
	}
	if got.Tools[0].FunctionDeclarations[0].Name != "do_thing" {
		t.Errorf("declared tool name = %q", got.Tools[0].FunctionDeclarations[0].Name)
	}
}

func TestParseGenerateResponse(t *testing.T) {
	body := []byte(`{"response":{"candidates":[{"content":{"parts":[
		{"text":"hello "},
		{"text":"world"},
		{"functionCall":{"id":"c1","name":"f","args":{"a":1}}}
	]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":7,"totalTokenCount":12}}}`)

	resp, err := parseGenerateResponse(body, "gemini-2.5-pro")
	if err != nil {
		t.Fatalf("parseGenerateResponse: %v", err)
	}
	if resp.Content != "hello world" {
		t.Errorf("content = %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Function.Name != "f" {
		t.Fatalf("tool calls = %+v", resp.ToolCalls)
	}
	if string(resp.ToolCalls[0].Function.Arguments) != `{"a":1}` {
		t.Errorf("tool args = %s", resp.ToolCalls[0].Function.Arguments)
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 12 {
		t.Errorf("usage = %+v", resp.Usage)
	}
}

func TestParseGenerateResponse_NoCandidates(t *testing.T) {
	resp, err := parseGenerateResponse([]byte(`{"response":{"candidates":[]}}`), "m")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Content != "" || len(resp.ToolCalls) != 0 {
		t.Errorf("expected empty response, got %+v", resp)
	}
}

func TestParseSSELine(t *testing.T) {
	ev, ok := parseSSELine(`{"response":{"candidates":[{"content":{"parts":[{"text":"chunk"}]},"finishReason":""}]}}`)
	if !ok || ev.Text != "chunk" {
		t.Fatalf("parseSSELine = %+v ok=%v", ev, ok)
	}

	ev, ok = parseSSELine(`{"response":{"candidates":[{"content":{"parts":[]},"finishReason":"MAX_TOKENS"}]}}`)
	if !ok || ev.FinishReason != "MAX_TOKENS" {
		t.Fatalf("finishReason not surfaced: %+v ok=%v", ev, ok)
	}

	if _, ok := parseSSELine(`not json`); ok {
		t.Error("parseSSELine ok=true for garbage")
	}
	if _, ok := parseSSELine(`{"response":{"candidates":[]}}`); ok {
		t.Error("parseSSELine ok=true for zero candidates")
	}
}
