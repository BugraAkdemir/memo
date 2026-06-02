package api

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestNewTextMessage(t *testing.T) {
	msg := NewTextMessage("user", "hello")
	if msg.Role != "user" {
		t.Errorf("Role = %q, want user", msg.Role)
	}
	if msg.Content != "hello" {
		t.Errorf("Content = %v, want hello", msg.Content)
	}
}

func TestNewMultimodalMessage(t *testing.T) {
	msg := NewMultimodalMessage("user", "desc", "imgdata1", "imgdata2")
	if msg.Role != "user" {
		t.Errorf("Role = %q, want user", msg.Role)
	}
	parts, ok := msg.Content.([]ContentPart)
	if !ok {
		t.Fatalf("Content type = %T, want []ContentPart", msg.Content)
	}
	if len(parts) != 3 {
		t.Fatalf("len(parts) = %d, want 3", len(parts))
	}
	if parts[0].Type != "text" || parts[0].Text != "desc" {
		t.Errorf("parts[0] = %+v, want text desc", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL.URL != "imgdata1" {
		t.Errorf("parts[1] = %+v, want image imgdata1", parts[1])
	}
}

func TestGetTextContentTextMessage(t *testing.T) {
	msg := NewTextMessage("user", "plain text")
	if got := msg.GetTextContent(); got != "plain text" {
		t.Errorf("GetTextContent() = %q, want %q", got, "plain text")
	}
}

func TestGetTextContentMultimodal(t *testing.T) {
	msg := NewMultimodalMessage("user", "text part", "img")
	if got := msg.GetTextContent(); got != "text part" {
		t.Errorf("GetTextContent() = %q, want %q", got, "text part")
	}
}

func TestGetTextContentInterfaceSlice(t *testing.T) {
	msg := Message{
		Role: "user",
		Content: []interface{}{
			map[string]interface{}{"type": "text", "text": "hello"},
		},
	}
	if got := msg.GetTextContent(); got != "hello" {
		t.Errorf("GetTextContent() = %q, want hello", got)
	}
}

func TestGetTextContentEmpty(t *testing.T) {
	var msg Message
	if got := msg.GetTextContent(); got != "" {
		t.Errorf("GetTextContent() = %q, want empty", got)
	}
}

func TestStreamChunkJSON(t *testing.T) {
	chunk := StreamChunk{Content: "hello", Done: true, FinishReason: "stop"}
	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatal(err)
	}
	var decoded StreamChunk
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Content != "hello" || !decoded.Done || decoded.FinishReason != "stop" {
		t.Errorf("round-trip failed: %+v", decoded)
	}
}

func TestChatCompletionRequestJSON(t *testing.T) {
	req := ChatCompletionRequest{
		Model:      "test-model",
		Messages:   []Message{NewTextMessage("user", "hi")},
		Stream:     true,
		ToolChoice: "none",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ChatCompletionRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Model != "test-model" {
		t.Errorf("Model = %q, want test-model", decoded.Model)
	}
	if len(decoded.Messages) != 1 {
		t.Errorf("len(Messages) = %d, want 1", len(decoded.Messages))
	}
	if decoded.ToolChoice != "none" {
		t.Errorf("ToolChoice = %q, want none", decoded.ToolChoice)
	}
}

func TestThinkingParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantReg  string
		wantThink string
	}{
		{"no tags", "hello world", "hello world", ""},
		{"full think", "A<think>hidden</think>B", "AB", "hidden"},
		{"think only", "<think>secret</think>", "", "secret"},
		{"multiple tags", "A<think>X</think>B<think>Y</think>C", "ABC", "XY"},
		{"unclosed tag", "A<think>open", "A", "open"},
		{"empty think", "<think></think>", "", ""},
		{"just text", "plain", "plain", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tp thinkingParser
			reg, think := tp.process(tt.input)
			if reg != tt.wantReg {
				t.Errorf("regular = %q, want %q", reg, tt.wantReg)
			}
			if think != tt.wantThink {
				t.Errorf("thinking = %q, want %q", think, tt.wantThink)
			}
		})
	}
}

func TestThinkingParserMultiCall(t *testing.T) {
	var tp thinkingParser
	reg, think := tp.process("A<think>")
	if reg != "A" || think != "" {
		t.Errorf("first call: reg=%q think=%q, want A / empty", reg, think)
	}
	reg, think = tp.process("hidden")
	if reg != "" || think != "hidden" {
		t.Errorf("second call: reg=%q think=%q, want empty / hidden", reg, think)
	}
	reg, think = tp.process("</think>B")
	if reg != "B" || think != "" {
		t.Errorf("third call: reg=%q think=%q, want B / empty", reg, think)
	}
}

func TestProcessSSEStreamBasic(t *testing.T) {
	input := "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n\n"
	body := io.NopCloser(strings.NewReader(input))
	ch := make(chan StreamChunk, 10)
	go processSSEStream(context.Background(), body, ch)

	var chunks []StreamChunk
	for c := range ch {
		chunks = append(chunks, c)
	}
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	if chunks[0].Content != "hello" {
		t.Errorf("chunk[0].Content = %q, want hello", chunks[0].Content)
	}
	if !chunks[1].Done {
		t.Error("chunk[1] should be Done")
	}
	if chunks[1].FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want stop", chunks[1].FinishReason)
	}
}

func TestProcessSSEStreamFinishReason(t *testing.T) {
	input := "data: {\"choices\":[{\"delta\":{\"content\":\"world\"},\"finish_reason\":\"length\"}]}\n\n"
	body := io.NopCloser(strings.NewReader(input))
	ch := make(chan StreamChunk, 10)
	go processSSEStream(context.Background(), body, ch)

	var chunks []StreamChunk
	for c := range ch {
		chunks = append(chunks, c)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].Content != "world" {
		t.Errorf("Content = %q, want world", chunks[0].Content)
	}
	if !chunks[0].Done {
		t.Error("should be Done")
	}
	if chunks[0].FinishReason != "length" {
		t.Errorf("FinishReason = %q, want length", chunks[0].FinishReason)
	}
}

func TestProcessSSEStreamErrorJSON(t *testing.T) {
	input := "data: {invalid json}\n\n"
	body := io.NopCloser(strings.NewReader(input))
	ch := make(chan StreamChunk, 10)
	go processSSEStream(context.Background(), body, ch)

	chunk := <-ch
	if !chunk.Done {
		t.Error("should be Done on parse error")
	}
	if chunk.Error == "" {
		t.Error("expected error message")
	}
}

func TestProcessSSEStreamSkipNonData(t *testing.T) {
	input := "ping\nevent: message\ndata: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"
	body := io.NopCloser(strings.NewReader(input))
	ch := make(chan StreamChunk, 10)
	go processSSEStream(context.Background(), body, ch)

	chunk := <-ch
	if chunk.Content != "ok" {
		t.Errorf("Content = %q, want ok", chunk.Content)
	}
}

func TestProcessSSEStreamContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r, w := io.Pipe()
	ch := make(chan StreamChunk, 10)

	go processSSEStream(ctx, r, ch)
	w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"))
	cancel()

	chunk := <-ch
	if chunk.Content != "partial" {
		t.Errorf("Content = %q, want partial", chunk.Content)
	}
	<-ch // context cancel closes the body, scanner stops, channel closes
}

func TestSetModel(t *testing.T) {
	c := NewClient("http://localhost:8081/v1", 30)
	if c.model != "local-model" {
		t.Errorf("default model = %q, want local-model", c.model)
	}
	c.SetModel("gpt-4")
	if c.model != "gpt-4" {
		t.Errorf("model after SetModel = %q, want gpt-4", c.model)
	}
	c.SetModel("")
	if c.model != "gpt-4" {
		t.Errorf("model should not change on empty SetModel")
	}
}

func TestNewClientWithKey(t *testing.T) {
	c := NewClientWithKey("http://test:8080/v1", "sk-test", 60)
	if c.baseURL != "http://test:8080/v1" {
		t.Errorf("baseURL = %q, want http://test:8080/v1", c.baseURL)
	}
	if c.apiKey != "sk-test" {
		t.Errorf("apiKey = %q, want sk-test", c.apiKey)
	}
}

func TestNewClientTrimsSlash(t *testing.T) {
	c := NewClient("http://test:8080/v1/", 30)
	if c.baseURL != "http://test:8080/v1" {
		t.Errorf("baseURL = %q, want http://test:8080/v1", c.baseURL)
	}
}

func TestNewClientZeroTimeout(t *testing.T) {
	c := NewClient("http://test:8080/v1", 0)
	if c.httpClient.Timeout != 0 {
		t.Errorf("Timeout = %v, want 0", c.httpClient.Timeout)
	}
}
