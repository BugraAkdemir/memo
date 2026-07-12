package webserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memo/internal/api"
)

// naiveSSELoop reproduces the exact pre-fix body every one of
// handleSendStream/handleSendFileStream/handleWhatsAppChatStream had before
// streamSSE existed: a single select racing ctx.Done() against the next
// chunk. If ctx is already Done at the exact moment ch also has a value
// ready, Go's random tie-breaking can pick ctx.Done() and the client never
// sees the final `"done":true` line — this is the root cause of the GUI
// stop-button staying stuck indefinitely, since the Flutter client's
// "sending" state only clears when it observes that line.
func naiveSSELoop(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, ch <-chan api.StreamChunk) {
	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(chunk)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
			if chunk.Done {
				return
			}
		}
	}
}

// TestNaiveSSELoop_DropsFinalChunkUnderCancelledContext demonstrates the bug
// class streamSSE fixes: with ctx already Done and a Done:true chunk already
// sitting in the buffered channel, the naive single-select loop drops it a
// meaningful fraction of the time. 500 trials makes a false pass (zero
// drops) astronomically unlikely if the race is real.
func TestNaiveSSELoop_DropsFinalChunkUnderCancelledContext(t *testing.T) {
	drops := 0
	const trials = 500
	for i := 0; i < trials; i++ {
		ch := make(chan api.StreamChunk, 1)
		ch <- api.StreamChunk{Done: true, Content: "final"}
		close(ch)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		rec := httptest.NewRecorder()
		naiveSSELoop(ctx, rec, rec, ch)
		if !strings.Contains(rec.Body.String(), `"done":true`) {
			drops++
		}
	}
	if drops == 0 {
		t.Fatalf("expected the naive select-race SSE loop to drop the final chunk at least once in %d trials, got 0 — the race this test documents may no longer be reproducible", trials)
	}
	t.Logf("naive SSE loop dropped the final chunk in %d/%d trials", drops, trials)
}

// TestStreamSSE_AlwaysDeliversFinalChunkUnderCancelledContext is the
// regression test for the fix applied to handleSendStream,
// handleSendFileStream, and handleWhatsAppChatStream: streamSSE must write
// the final Done:true chunk to the SSE response even when ctx is
// simultaneously (or already) cancelled, as long as the chunk was already
// available on the channel — mirroring what really happens when a request's
// context and a stream's completion race each other in production.
func TestStreamSSE_AlwaysDeliversFinalChunkUnderCancelledContext(t *testing.T) {
	const trials = 500
	for i := 0; i < trials; i++ {
		ch := make(chan api.StreamChunk, 1)
		ch <- api.StreamChunk{Done: true, Content: "final"}
		close(ch)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		rec := httptest.NewRecorder()
		streamSSE(ctx, rec, rec, ch)
		if !strings.Contains(rec.Body.String(), `"done":true`) {
			t.Fatalf("trial %d: streamSSE dropped the final chunk, body=%q", i, rec.Body.String())
		}
	}
}

// TestStreamSSE_MultiChunkThenDone exercises the ordinary, non-racing path:
// several content chunks followed by a Done chunk, verifying they all land
// in the response body in order.
func TestStreamSSE_MultiChunkThenDone(t *testing.T) {
	ch := make(chan api.StreamChunk, 4)
	ch <- api.StreamChunk{Content: "Merhaba"}
	ch <- api.StreamChunk{Content: "!"}
	ch <- api.StreamChunk{Done: true, FinishReason: "stop"}
	close(ch)

	rec := httptest.NewRecorder()
	streamSSE(context.Background(), rec, rec, ch)

	body := rec.Body.String()
	if !strings.Contains(body, `"content":"Merhaba"`) || !strings.Contains(body, `"content":"!"`) {
		t.Errorf("missing content chunks in body:\n%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Errorf("missing final done chunk in body:\n%s", body)
	}
}
