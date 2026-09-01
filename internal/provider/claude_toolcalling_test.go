package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestClaudeProvider_ChatCompletion_SendsToolsAndParsesToolUse is the
// regression test for the bug found while investigating a real Self-Driving
// run: Memo's "Anthropic Claude" provider type never sent req.Tools and
// never parsed a tool_use content block back out, so agent mode silently
// degraded to plain chat on this provider — exactly the "the tools arrived
// as text, not real function calls" failure mode later confirmed live
// against a different (third-party) provider with the same underlying gap.
func TestClaudeProvider_ChatCompletion_SendsToolsAndParsesToolUse(t *testing.T) {
	var gotBody claudeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("could not decode outgoing request: %v", err)
		}
		json.NewEncoder(w).Encode(claudeResponse{
			Model:      "claude-3-5-sonnet-20241022",
			StopReason: "tool_use",
			Content: []claudeBlock{
				{Type: "text", Text: "Klasörü listeliyorum."},
				{Type: "tool_use", ID: "toolu_01abc", Name: "list_directory", Input: json.RawMessage(`{"path":"."}`)},
			},
		})
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-3-5-sonnet-20241022")
	resp, err := p.ChatCompletion(context.Background(), ChatRequest{
		Messages: []Message{TextMessage("user", "list files")},
		Tools: []ToolDefinition{{
			Type: "function",
			Function: ToolFunction{
				Name:        "list_directory",
				Description: "List a directory",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
			},
		}},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	// Outgoing: tools translated to Anthropic's flat shape.
	if len(gotBody.Tools) != 1 {
		t.Fatalf("outgoing Tools = %+v, want exactly 1", gotBody.Tools)
	}
	if gotBody.Tools[0].Name != "list_directory" {
		t.Errorf("outgoing tool name = %q, want list_directory", gotBody.Tools[0].Name)
	}
	if string(gotBody.Tools[0].InputSchema) != `{"type":"object","properties":{"path":{"type":"string"}}}` {
		t.Errorf("outgoing tool input_schema = %s, want the parameters passed straight through", gotBody.Tools[0].InputSchema)
	}

	// Incoming: tool_use block parsed into ToolCalls, text block still
	// concatenated into Content.
	if resp.Content != "Klasörü listeliyorum." {
		t.Errorf("resp.Content = %q, want the text block", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("resp.ToolCalls = %+v, want exactly 1", resp.ToolCalls)
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "toolu_01abc" || tc.Function.Name != "list_directory" {
		t.Errorf("resp.ToolCalls[0] = %+v, want id=toolu_01abc name=list_directory", tc)
	}
	if string(tc.Function.Arguments) != `{"path":"."}` {
		t.Errorf("resp.ToolCalls[0].Function.Arguments = %s, want the tool_use input verbatim", tc.Function.Arguments)
	}
}

// TestClaudeProvider_ChatCompletion_RoundTripsToolResult is the second half
// of the loop: pipeline.go appends the assistant's ToolCalls message, then
// one Message{Role:"tool",...} per executed call, and sends that whole
// history back on the next turn. buildClaudeRequest must translate that into
// Anthropic's shape: the assistant turn's tool_use block(s) echoed back, and
// the tool result(s) merged into exactly one following user turn with
// tool_result blocks — never a "tool"-role message, which Anthropic rejects
// outright (only "user"/"assistant" are valid roles).
func TestClaudeProvider_ChatCompletion_RoundTripsToolResult(t *testing.T) {
	var gotBody claudeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		json.NewEncoder(w).Encode(claudeResponse{
			Model:   "claude-3-5-sonnet-20241022",
			Content: []claudeBlock{{Type: "text", Text: "Tamamdır."}},
		})
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-3-5-sonnet-20241022")
	_, err := p.ChatCompletion(context.Background(), ChatRequest{
		Messages: []Message{
			TextMessage("user", "list files"),
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "toolu_01abc", Type: "function", Function: ToolCallFunction{Name: "list_directory", Arguments: json.RawMessage(`{"path":"."}`)}},
				},
			},
			{Role: "tool", ToolCallID: "toolu_01abc", Content: "a.txt\nb.txt"},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	if len(gotBody.Messages) != 3 {
		t.Fatalf("outgoing Messages = %+v, want 3 (user text, assistant tool_use, user tool_result)", gotBody.Messages)
	}

	// Find the assistant turn and confirm it carries a tool_use block, and
	// the very next turn is role "user" with a tool_result block — never a
	// bare "tool" role.
	var sawToolUse, sawToolResult bool
	for i, m := range gotBody.Messages {
		for _, b := range m.Content {
			if b.Type == "tool_use" {
				sawToolUse = true
				if m.Role != "assistant" {
					t.Errorf("tool_use block found on role %q, want assistant", m.Role)
				}
				if b.ID != "toolu_01abc" || b.Name != "list_directory" {
					t.Errorf("echoed tool_use = %+v, want id=toolu_01abc name=list_directory", b)
				}
			}
			if b.Type == "tool_result" {
				sawToolResult = true
				if m.Role != "user" {
					t.Errorf("tool_result block found on role %q, want user (Anthropic rejects role=tool)", m.Role)
				}
				if b.ToolUseID != "toolu_01abc" {
					t.Errorf("tool_result.tool_use_id = %q, want toolu_01abc", b.ToolUseID)
				}
				if b.Content != "a.txt\nb.txt" {
					t.Errorf("tool_result.content = %q, want the tool's raw output", b.Content)
				}
			}
		}
		if m.Role == "tool" {
			t.Errorf("Messages[%d] has role %q — Anthropic only accepts user/assistant", i, m.Role)
		}
	}
	if !sawToolUse {
		t.Error("no tool_use block found in the outgoing request — the assistant's tool call was dropped")
	}
	if !sawToolResult {
		t.Error("no tool_result block found in the outgoing request — the tool's result was dropped")
	}
}

// TestClaudeProvider_ChatCompletion_MergesParallelToolResults: two tool
// calls in one assistant turn must produce ONE following user message with
// two tool_result blocks, not two separate user turns (which would violate
// Anthropic's strict role alternation).
func TestClaudeProvider_ChatCompletion_MergesParallelToolResults(t *testing.T) {
	var gotBody claudeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		json.NewEncoder(w).Encode(claudeResponse{Content: []claudeBlock{{Type: "text", Text: "ok"}}})
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-3-5-sonnet-20241022")
	_, err := p.ChatCompletion(context.Background(), ChatRequest{
		Messages: []Message{
			TextMessage("user", "check both files"),
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call_1", Type: "function", Function: ToolCallFunction{Name: "read_file", Arguments: json.RawMessage(`{"path":"a.txt"}`)}},
					{ID: "call_2", Type: "function", Function: ToolCallFunction{Name: "read_file", Arguments: json.RawMessage(`{"path":"b.txt"}`)}},
				},
			},
			{Role: "tool", ToolCallID: "call_1", Content: "content of a"},
			{Role: "tool", ToolCallID: "call_2", Content: "content of b"},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	// Exactly one user message should follow the assistant turn, carrying
	// both tool_result blocks.
	var resultMsg *claudeMsg
	for i := range gotBody.Messages {
		m := &gotBody.Messages[i]
		hasToolUse := false
		for _, b := range m.Content {
			if b.Type == "tool_use" {
				hasToolUse = true
			}
		}
		if hasToolUse {
			if i+1 >= len(gotBody.Messages) {
				t.Fatal("assistant tool_use turn has no following message")
			}
			resultMsg = &gotBody.Messages[i+1]
		}
	}
	if resultMsg == nil {
		t.Fatal("could not find the assistant tool_use turn")
	}
	if resultMsg.Role != "user" {
		t.Errorf("merged results message role = %q, want user", resultMsg.Role)
	}
	if len(resultMsg.Content) != 2 {
		t.Fatalf("merged results message has %d blocks, want 2 (one per tool call)", len(resultMsg.Content))
	}
	ids := map[string]bool{}
	for _, b := range resultMsg.Content {
		if b.Type != "tool_result" {
			t.Errorf("unexpected block type %q in the merged results message", b.Type)
		}
		ids[b.ToolUseID] = true
	}
	if !ids["call_1"] || !ids["call_2"] {
		t.Errorf("merged tool_use_ids = %v, want both call_1 and call_2", ids)
	}
}

// TestClaudeProvider_ChatCompletion_NoToolsOmitsToolsField: a plain chat
// call (no tools) must not send an empty tools array — Anthropic treats an
// empty (vs. omitted) tools field as a signal worth avoiding.
func TestClaudeProvider_ChatCompletion_NoToolsOmitsToolsField(t *testing.T) {
	var raw map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &raw)
		json.NewEncoder(w).Encode(claudeResponse{Content: []claudeBlock{{Type: "text", Text: "hi"}}})
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-3-5-sonnet-20241022")
	_, err := p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if _, present := raw["tools"]; present {
		t.Errorf("outgoing request has a \"tools\" key with no tools configured, want it omitted entirely: %v", raw["tools"])
	}
}
