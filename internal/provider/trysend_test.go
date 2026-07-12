package provider

import (
	"context"
	"testing"
)

// naiveSendOrCancel reproduces the exact pre-fix body every processSSE
// implementation in this package used (openai.go, gemini.go, claude.go): a
// single select racing the send against ctx.Done().
func naiveSendOrCancel(ctx context.Context, ch chan<- StreamChunk, chunk StreamChunk) {
	select {
	case ch <- chunk:
	case <-ctx.Done():
	}
}

// TestNaiveSendOrCancel_DropsFinalChunkUnderCancelledContext demonstrates the
// bug class trySend (below) fixes: when ctx is already Done at the exact
// moment ch also has buffer room, Go's random tie-breaking between the two
// simultaneously-ready select cases silently drops the value a meaningful
// fraction of the time — including a final Done:true chunk, which is how
// downstream code (internal/app/llm.go's providerLoop, ultimately the
// Flutter client) learns a stream actually finished. 2000 trials makes a
// false pass (zero drops) astronomically unlikely if the race is real.
func TestNaiveSendOrCancel_DropsFinalChunkUnderCancelledContext(t *testing.T) {
	drops := 0
	const trials = 2000
	for i := 0; i < trials; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ch := make(chan StreamChunk, 1)
		naiveSendOrCancel(ctx, ch, StreamChunk{Done: true, Content: "final"})
		select {
		case <-ch:
		default:
			drops++
		}
	}
	if drops == 0 {
		t.Fatalf("expected the naive select-race pattern to drop the final chunk at least once in %d trials, got 0 — the race this test documents may no longer be reproducible", trials)
	}
	t.Logf("naive pattern dropped the final chunk in %d/%d trials", drops, trials)
}

// TestTrySend_NeverDropsReadyChunkUnderCancelledContext is the regression
// test for the fix applied to openai.go/gemini.go/claude.go's processSSE:
// trySend must always deliver a chunk when ch has buffer room, even if ctx
// is simultaneously (or already) cancelled.
func TestTrySend_NeverDropsReadyChunkUnderCancelledContext(t *testing.T) {
	const trials = 2000
	for i := 0; i < trials; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ch := make(chan StreamChunk, 1)
		trySend(ctx, ch, StreamChunk{Done: true, Content: "final"})
		select {
		case v := <-ch:
			if !v.Done {
				t.Fatalf("trial %d: got wrong chunk %+v", i, v)
			}
		default:
			t.Fatalf("trial %d: trySend dropped the final chunk despite ch having buffer room", i)
		}
	}
}
