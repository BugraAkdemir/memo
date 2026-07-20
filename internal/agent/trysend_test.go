package agent

import (
	"context"
	"testing"

	"memo/internal/provider"
)

// naiveSendOrCancel is the pre-BUG-H1 body of pipeline.trySend: a single
// select racing the send against ctx.Done().
func naiveSendOrCancel(ctx context.Context, ch chan<- provider.StreamChunk, chunk provider.StreamChunk) {
	select {
	case ch <- chunk:
	case <-ctx.Done():
	}
}

// TestNaiveSendOrCancel_DropsFinalChunkUnderCancelledContext documents the
// race BUG-H1 fixed: when ctx is already Done and ch has buffer room, Go
// picks between the two ready cases at random and drops the chunk a large
// fraction of the time.
func TestNaiveSendOrCancel_DropsFinalChunkUnderCancelledContext(t *testing.T) {
	drops := 0
	const trials = 2000
	for i := 0; i < trials; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ch := make(chan provider.StreamChunk, 1)
		naiveSendOrCancel(ctx, ch, provider.StreamChunk{Done: true, Content: "final"})
		select {
		case <-ch:
		default:
			drops++
		}
	}
	if drops == 0 {
		t.Fatalf("expected naive select-race to drop the final chunk at least once in %d trials", trials)
	}
	t.Logf("naive pattern dropped the final chunk in %d/%d trials", drops, trials)
}

// TestTrySend_NeverDropsReadyChunkUnderCancelledContext is the BUG-H1
// regression: trySend must always deliver when ch has buffer room, even if
// ctx is already cancelled.
func TestTrySend_NeverDropsReadyChunkUnderCancelledContext(t *testing.T) {
	const trials = 2000
	for i := 0; i < trials; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ch := make(chan provider.StreamChunk, 1)
		trySend(ctx, ch, provider.StreamChunk{Done: true, Content: "final"})
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
