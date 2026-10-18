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

// sseServer replays the given raw SSE lines (each already "data: ..." form).
func sseServer(t *testing.T, lines ...string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, l := range lines {
			io.WriteString(w, l+"\n\n")
		}
	}))
}

// drain reads a stream to its end and returns the concatenated text, the
// concatenated thinking and the terminal chunk.
func drain(t *testing.T, ch <-chan StreamChunk) (text, thinking string, last StreamChunk) {
	t.Helper()
	var tb, kb strings.Builder
	for c := range ch {
		tb.WriteString(c.Content)
		kb.WriteString(c.Thinking)
		last = c
	}
	return tb.String(), kb.String(), last
}

// A streamed OpenAI tool call arrives as fragments: the first carries id and
// name, the rest only pieces of the arguments string. The terminal chunk must
// hold the whole call, with the arguments in the same string-literal shape a
// non-streaming response decodes into (that is what the pipeline replays).
func TestOpenAIStream_AssemblesToolCallFragments(t *testing.T) {
	srv := sseServer(t,
		`data: {"choices":[{"index":0,"delta":{"role":"assistant","content":"Bakıyorum. "},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"read_file","arguments":""}}]},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":"}}]},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"a.go\"}"}}]},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_b","type":"function","function":{"name":"list_directory","arguments":"{\"path\":\".\"}"}}]},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
	)
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	ch, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "x")}})
	if err != nil {
		t.Fatal(err)
	}
	text, _, last := drain(t, ch)
	if text != "Bakıyorum. " {
		t.Errorf("text = %q", text)
	}
	if len(last.ToolCalls) != 2 {
		t.Fatalf("tool calls = %+v, want 2", last.ToolCalls)
	}
	first := last.ToolCalls[0]
	if first.ID != "call_a" || first.Function.Name != "read_file" {
		t.Errorf("first call = %+v", first)
	}
	var argStr string
	if err := json.Unmarshal(first.Function.Arguments, &argStr); err != nil {
		t.Fatalf("arguments are not a JSON string literal: %s", first.Function.Arguments)
	}
	if argStr != `{"path":"a.go"}` {
		t.Errorf("arguments = %q", argStr)
	}
	if last.ToolCalls[1].ID != "call_b" || last.ToolCalls[1].Function.Name != "list_directory" {
		t.Errorf("second call = %+v", last.ToolCalls[1])
	}
}

// Some gateways number every call 0. A new non-empty id must start a new call
// instead of being glued onto the previous one.
func TestOpenAIStream_ReusedIndexWithNewIDIsANewCall(t *testing.T) {
	srv := sseServer(t,
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"read_file","arguments":"{\"path\":\"a\"}"}}]},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"c2","function":{"name":"read_file","arguments":"{\"path\":\"b\"}"}}]},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
	)
	defer srv.Close()
	ch, err := newTestOpenAIProvider(t, srv).ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "x")}})
	if err != nil {
		t.Fatal(err)
	}
	_, _, last := drain(t, ch)
	if len(last.ToolCalls) != 2 || last.ToolCalls[0].ID != "c1" || last.ToolCalls[1].ID != "c2" {
		t.Fatalf("tool calls = %+v, want c1 and c2", last.ToolCalls)
	}
}

// A tool call with no arguments must still replay as valid JSON ("{}").
func TestOpenAIStream_EmptyArgumentsBecomeEmptyObject(t *testing.T) {
	srv := sseServer(t,
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"read_env"}}]},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
	)
	defer srv.Close()
	ch, err := newTestOpenAIProvider(t, srv).ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "x")}})
	if err != nil {
		t.Fatal(err)
	}
	_, _, last := drain(t, ch)
	var s string
	if len(last.ToolCalls) != 1 || json.Unmarshal(last.ToolCalls[0].Function.Arguments, &s) != nil || s != "{}" {
		t.Fatalf("tool calls = %+v", last.ToolCalls)
	}
}

// Reasoning models stream their visible thinking in reasoning_content (or
// "reasoning"); it must reach the consumer as Thinking, never as the answer.
func TestOpenAIStream_ReasoningBecomesThinking(t *testing.T) {
	srv := sseServer(t,
		`data: {"choices":[{"index":0,"delta":{"reasoning_content":"önce "},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{"reasoning":"düşün"},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{"content":"Cevap"},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`data: [DONE]`,
	)
	defer srv.Close()
	ch, err := newTestOpenAIProvider(t, srv).ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "x")}})
	if err != nil {
		t.Fatal(err)
	}
	text, thinking, _ := drain(t, ch)
	if text != "Cevap" || thinking != "önce düşün" {
		t.Errorf("text=%q thinking=%q", text, thinking)
	}
}

// Claude: a thinking block with its signature plus a tool_use block must come
// out as a tool call AND as Native blocks that replay the thinking verbatim.
func TestClaudeStream_AssemblesToolUseAndSignedThinking(t *testing.T) {
	srv := sseServer(t,
		`data: {"type":"message_start","message":{"usage":{"input_tokens":10}}}`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"hmm "}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"ok"}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"SIG=="}}`,
		`data: {"type":"content_block_stop","index":0}`,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Dosyayı okuyorum."}}`,
		`data: {"type":"content_block_stop","index":1}`,
		`data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"toolu_1","name":"read_file","input":{}}}`,
		`data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"path\":"}}`,
		`data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\"a.go\"}"}}`,
		`data: {"type":"content_block_stop","index":2}`,
		`data: {"type":"message_delta","usage":{"output_tokens":7}}`,
		`data: {"type":"message_stop"}`,
	)
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-test")
	ch, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "x")}})
	if err != nil {
		t.Fatal(err)
	}
	text, thinking, last := drain(t, ch)
	if text != "Dosyayı okuyorum." || thinking != "hmm ok" {
		t.Errorf("text=%q thinking=%q", text, thinking)
	}
	if len(last.ToolCalls) != 1 || last.ToolCalls[0].ID != "toolu_1" || string(last.ToolCalls[0].Function.Arguments) != `{"path":"a.go"}` {
		t.Fatalf("tool calls = %+v", last.ToolCalls)
	}
	native, ok := last.Native.([]claudeBlock)
	if !ok || len(native) != 3 {
		t.Fatalf("native = %#v, want 3 blocks", last.Native)
	}
	raw, _ := json.Marshal(native[0])
	if !strings.Contains(string(raw), `"signature":"SIG=="`) || !strings.Contains(string(raw), `"thinking":"hmm ok"`) {
		t.Errorf("thinking block is not replayed with its signature: %s", raw)
	}
	if last.Usage == nil || last.Usage.CompletionTokens != 7 {
		t.Errorf("usage = %+v", last.Usage)
	}
}

// A Claude turn that did not think needs no Native replay.
func TestClaudeStream_NoThinkingMeansNoNative(t *testing.T) {
	srv := sseServer(t,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"t","name":"read_env","input":{}}}`,
		`data: {"type":"content_block_stop","index":0}`,
		`data: {"type":"message_stop"}`,
	)
	defer srv.Close()
	ch, err := newTestClaudeProvider(t, srv, "claude-test").ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "x")}})
	if err != nil {
		t.Fatal(err)
	}
	_, _, last := drain(t, ch)
	if last.Native != nil || len(last.ToolCalls) != 1 || string(last.ToolCalls[0].Function.Arguments) != `{}` {
		t.Fatalf("last = %+v", last)
	}
}

func TestGeminiStream_CollectsFunctionCalls(t *testing.T) {
	srv := sseServer(t,
		`data: {"candidates":[{"content":{"parts":[{"text":"Bakıyorum. "}]}}]}`,
		`data: {"candidates":[{"content":{"parts":[{"functionCall":{"id":"g1","name":"read_file","args":{"path":"a.go"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7}}`,
	)
	defer srv.Close()
	ch, err := newTestGeminiProvider(t, srv, "gemini-test").ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "x")}})
	if err != nil {
		t.Fatal(err)
	}
	text, _, last := drain(t, ch)
	if text != "Bakıyorum. " {
		t.Errorf("text = %q", text)
	}
	if !last.Done || len(last.ToolCalls) != 1 || last.ToolCalls[0].Function.Name != "read_file" || string(last.ToolCalls[0].Function.Arguments) != `{"path":"a.go"}` {
		t.Fatalf("last = %+v", last)
	}
	if last.Usage == nil || last.Usage.TotalTokens != 7 {
		t.Errorf("usage = %+v", last.Usage)
	}
}
