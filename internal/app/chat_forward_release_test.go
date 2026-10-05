package app

import (
	"context"
	"sync"
	"testing"

	"memo/internal/api"
)

// The per-chat stream lock must already be free when a client reads Done.
// Before this, release ran only after forwardStream returned, so a client
// that posted its next message straight after Done could be refused with
// busyNotice — the intermittent CI failure of
// TestAgent_PlanSubMode_TextConfirmFlow under -race.
func TestForwardStreamReleasing_LockFreeWhenDoneArrives(t *testing.T) {
	a := &App{}
	release, ok := a.lockChatStream("chat-1")
	if !ok {
		t.Fatal("setup: could not take the chat lock")
	}

	inner := make(chan api.StreamChunk, 2)
	inner <- api.StreamChunk{Content: "hi"}
	inner <- api.StreamChunk{Done: true}
	close(inner)

	// Unbuffered, so each send completes only when the test reads it — the
	// strictest version of "the client sees Done".
	out := make(chan api.StreamChunk)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(out)
		defer release()
		forwardStreamReleasing(context.Background(), inner, out, release)
	}()

	for chunk := range out {
		if !chunk.Done {
			continue
		}
		next, ok := a.lockChatStream("chat-1")
		if !ok {
			t.Fatal("the chat was still locked when Done reached the client; a follow-up message would get busyNotice")
		}
		next()
	}
	wg.Wait()

	// The deferred second release must be a no-op, not a double unlock.
	if again, ok := a.lockChatStream("chat-1"); !ok {
		t.Fatal("lock not free after the stream ended")
	} else {
		again()
	}
}
