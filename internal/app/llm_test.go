package app

import (
	"context"
	"testing"

	"memo/internal/api"
	"memo/internal/provider"
)

// TestClientSwapped is a regression test for BUG-L4: callLLMStream/callLLM
// capture a.client into a local var under clientMu at the start of a call,
// then hold it for the call's whole duration (which can be seconds for a
// stream). If the local model is stopped/restarted in the meantime, the
// call keeps talking to the now-dead client and fails with a confusing raw
// transport error ("connection refused") instead of an explanation.
// clientSwapped lets the caller detect that specific situation.
func TestClientSwapped(t *testing.T) {
	original := api.NewClient("http://127.0.0.1:8081", 30)
	a := &App{client: original}

	if a.clientSwapped(original) {
		t.Error("clientSwapped(original) = true, want false — a.client hasn't changed")
	}

	replacement := api.NewClient("http://127.0.0.1:8081", 30)
	a.client = replacement

	if !a.clientSwapped(original) {
		t.Error("clientSwapped(original) = false, want true — a.client was reassigned to a different *api.Client")
	}
	if a.clientSwapped(replacement) {
		t.Error("clientSwapped(replacement) = true, want false — replacement is a.client's current value")
	}
}

// TestProviderSwapped is providerSwapped's counterpart — a.providerRouter
// changed (the active external API provider was switched) mid-call.
func TestProviderSwapped(t *testing.T) {
	original := provider.NewRouter(nil)
	a := &App{providerRouter: original}

	if a.providerSwapped(original) {
		t.Error("providerSwapped(original) = true, want false — a.providerRouter hasn't changed")
	}

	replacement := provider.NewRouter(nil)
	a.providerRouter = replacement

	if !a.providerSwapped(original) {
		t.Error("providerSwapped(original) = false, want true — a.providerRouter was reassigned")
	}
	if a.providerSwapped(replacement) {
		t.Error("providerSwapped(replacement) = true, want false — replacement is a.providerRouter's current value")
	}
}

// naiveSendOrCancel reproduces the exact pre-fix body of trySend: a single
// select racing the send against ctx.Done(). This is what every streaming
// producer in llm.go/chat.go used before this fix.
func naiveSendOrCancel(ctx context.Context, outCh chan<- api.StreamChunk, chunk api.StreamChunk) {
	select {
	case outCh <- chunk:
	case <-ctx.Done():
	}
}

// TestNaiveSendOrCancel_DropsFinalChunkUnderCancelledContext demonstrates the
// GUI stop-button-stuck bug's root cause in isolation: when ctx is already
// Done at the exact moment outCh also has buffer room, Go's random
// tie-breaking between the two simultaneously-ready select cases means the
// naive pattern silently drops the value a meaningful fraction of the time.
// This is exactly what happened to the final Done:true chunk — the frontend
// never saw it, so its "sending" UI state (the stop-button icon) never
// reverted. 2000 trials makes a false pass (zero drops) astronomically
// unlikely (~0.5^2000) if the race is present, so this test reliably fails
// against the old naiveSendOrCancel-shaped code and documents why trySend
// (below) exists.
func TestNaiveSendOrCancel_DropsFinalChunkUnderCancelledContext(t *testing.T) {
	drops := 0
	const trials = 2000
	for i := 0; i < trials; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		outCh := make(chan api.StreamChunk, 1)
		naiveSendOrCancel(ctx, outCh, api.StreamChunk{Done: true, Content: "final"})
		select {
		case <-outCh:
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
// test for the fix: trySend must always deliver a chunk when outCh has
// buffer room, even if ctx is simultaneously (or already) cancelled. Same
// setup as TestNaiveSendOrCancel_DropsFinalChunkUnderCancelledContext above,
// but exercising the real, fixed trySend — it must show zero drops across
// every trial.
func TestTrySend_NeverDropsReadyChunkUnderCancelledContext(t *testing.T) {
	const trials = 2000
	for i := 0; i < trials; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		outCh := make(chan api.StreamChunk, 1)
		trySend(ctx, outCh, api.StreamChunk{Done: true, Content: "final"})
		select {
		case v := <-outCh:
			if !v.Done {
				t.Fatalf("trial %d: got wrong chunk %+v", i, v)
			}
		default:
			t.Fatalf("trial %d: trySend dropped the final chunk despite outCh having buffer room", i)
		}
	}
}

// TestRecvChunk_NeverDropsReadyChunkUnderCancelledContext is recvChunk's
// counterpart to TestTrySend_NeverDropsReadyChunkUnderCancelledContext: when
// ch already has a value ready AND ctx is already Done, recvChunk must
// return that value (ctxDone=false, ok=true) rather than reporting a
// cancellation and losing it — this is what agentLoop/providerLoop/localLoop
// in llm.go rely on to not report "⏹️ Cevap durduruldu" for a turn that had
// actually already finished successfully.
func TestRecvChunk_NeverDropsReadyChunkUnderCancelledContext(t *testing.T) {
	const trials = 2000
	for i := 0; i < trials; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ch := make(chan api.StreamChunk, 1)
		ch <- api.StreamChunk{Done: true, Content: "final"}
		chunk, ok, ctxDone := recvChunk(ctx, ch)
		if ctxDone {
			t.Fatalf("trial %d: recvChunk reported ctxDone despite a value being ready on ch", i)
		}
		if !ok || !chunk.Done {
			t.Fatalf("trial %d: recvChunk returned ok=%v chunk=%+v, want the ready final chunk", i, ok, chunk)
		}
	}
}
