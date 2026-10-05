package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestOpenAIStream_EmptyFinishReasonIsNotAFinish is S6 (BUG_REPORT
// 2026-09-28): a server that sends `"finish_reason": ""` on ordinary content
// chunks (instead of null) was read as "content finished, wait for usage".
// The usage watchdog then armed on the first such chunk, fired mid-answer,
// closed the body, and — with no real finish pending — the turn ended as an
// error while the model was still talking.
func TestOpenAIStream_EmptyFinishReasonIsNotAFinish(t *testing.T) {
	prev := streamUsageGrace
	streamUsageGrace = 100 * time.Millisecond
	defer func() { streamUsageGrace = prev }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		for _, word := range []string{"one ", "two ", "three"} {
			io.WriteString(w, `data: {"choices":[{"index":0,"delta":{"content":"`+word+`"},"finish_reason":""}]}`+"\n\n")
			fl.Flush()
			time.Sleep(150 * time.Millisecond) // longer than the grace period
		}
		io.WriteString(w, `data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`+"\n\n")
		io.WriteString(w, `data: {"choices":[],"usage":{"prompt_tokens":3,"completion_tokens":3,"total_tokens":6}}`+"\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	ch, err := p.ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "count")}})
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	var last StreamChunk
	for c := range ch {
		got.WriteString(c.Content)
		last = c
	}
	if last.Error != "" {
		t.Fatalf("stream ended with error %q after %q", last.Error, got.String())
	}
	if got.String() != "one two three" {
		t.Errorf("content = %q, want the whole answer", got.String())
	}
	if last.FinishReason != "stop" || last.Usage == nil {
		t.Errorf("terminal chunk = %+v, want finish stop with usage", last)
	}
}
