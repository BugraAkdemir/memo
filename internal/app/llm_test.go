package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"memo/internal/api"
	"memo/internal/config"
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

// TestPreemptBackgroundLLM_CancelsRegisteredCall is a regression test for
// BUG_REPORT TD-2: a background call (auto fact extraction) must be
// cancellable so a real chat message never queues behind it on the local
// model's single inference slot.
func TestPreemptBackgroundLLM_CancelsRegisteredCall(t *testing.T) {
	a := &App{}

	bgCtx, done := a.beginBackgroundLLMCall(context.Background())
	defer done()

	select {
	case <-bgCtx.Done():
		t.Fatal("bgCtx already cancelled before preemptBackgroundLLM was called")
	default:
	}

	a.preemptBackgroundLLM()

	select {
	case <-bgCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("preemptBackgroundLLM did not cancel the registered background context")
	}
}

// TestPreemptBackgroundLLM_NoopWhenNothingRegistered ensures the real chat
// paths can call preemptBackgroundLLM unconditionally, even when no
// background call is in flight.
func TestPreemptBackgroundLLM_NoopWhenNothingRegistered(t *testing.T) {
	a := &App{}
	a.preemptBackgroundLLM() // must not panic
}

// TestBeginBackgroundLLMCall_StaleCleanupDoesNotClobberNewerCall covers the
// exact race beginBackgroundLLMCall's cleanup guards against: an older
// background call's deferred cleanup running after a newer one has already
// registered must not cancel or clear the newer call's slot.
func TestBeginBackgroundLLMCall_StaleCleanupDoesNotClobberNewerCall(t *testing.T) {
	a := &App{}

	_, done1 := a.beginBackgroundLLMCall(context.Background())
	bgCtx2, done2 := a.beginBackgroundLLMCall(context.Background())
	defer done2()

	// Simulate call 1 finishing after call 2 has already taken the slot.
	done1()

	select {
	case <-bgCtx2.Done():
		t.Fatal("stale done() from an older background call cancelled the newer, still-active one")
	default:
	}

	a.bgLLMMu.Lock()
	registered := a.bgLLMCtx
	a.bgLLMMu.Unlock()
	if registered != bgCtx2 {
		t.Fatal("stale done() from an older background call cleared the newer, still-registered one")
	}

	// The newer call is still preemptible.
	a.preemptBackgroundLLM()
	select {
	case <-bgCtx2.Done():
	case <-time.After(time.Second):
		t.Fatal("preemptBackgroundLLM did not cancel the still-registered newer call")
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

// TestCallLLMStream_ExternalProvider_CtxDoneSendsTerminalChunk is a
// regression test: when a genuine ctxDone fires mid-stream (300s generation
// budget elapsed, or the client disconnected — not the "value was ready
// anyway" case recvChunk already protects above), the external-provider loop
// in callLLMStream used to record "⏹️ Cevap durduruldu." to the session and
// return WITHOUT ever sending a terminal chunk to outCh, unlike every other
// return path in this file. outCh's close was the only signal the SSE
// pipeline forwarded, so the live client never learned a response had been
// cut off — it just silently stopped, indistinguishable from a hung
// connection. Same gap existed in callAgentStream and the local-model loop.
func TestCallLLMStream_ExternalProvider_CtxDoneSendsTerminalChunk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		// Simulate a generation that's still running when the client gives
		// up (timeout or disconnect) — never sends [DONE] or a finish_reason,
		// just holds the connection open until the request context ends.
		<-r.Context().Done()
	}))
	defer srv.Close()

	router := provider.NewRouter([]provider.ProviderConfig{{
		Type:    provider.ProviderCustom,
		Name:    "test",
		BaseURL: srv.URL,
		Model:   "test-model",
		Enabled: true,
	}})

	a := &App{providerRouter: router, activeProviderName: "test", cfg: &config.AppConfig{}}

	ctx, cancel := context.WithCancel(context.Background())
	outCh := a.callLLMStream(ctx, []api.Message{api.NewTextMessage("user", "hi")}, "hi", "", "", "")

	// Let the partial chunk arrive, then cut the stream — exercises
	// recvChunk's real ctxDone branch (nothing else is ready on the channel).
	time.Sleep(100 * time.Millisecond)
	cancel()

	gotTerminal := false
	timeout := time.After(2 * time.Second)
loop:
	for {
		select {
		case chunk, ok := <-outCh:
			if !ok {
				break loop
			}
			if chunk.Done {
				gotTerminal = true
				if chunk.Error == "" {
					t.Errorf("terminal chunk after ctx cancellation has no Error set: %+v", chunk)
				}
			}
		case <-timeout:
			t.Fatal("timed out waiting for outCh to close")
		}
	}

	if !gotTerminal {
		t.Fatal("outCh closed without ever sending a terminal (Done) chunk after ctx was cancelled mid-stream — the frontend never learns the turn ended, leaving its UI state stuck with no explanation")
	}
}

// TestDrainAgentStream_EmptyChannelSendsTerminalChunk is the regression test
// for BUG_REPORT.md's SF-5: callAgentStream's tail (now extracted as
// drainAgentStream) used to reach `if fullReply.Len() > 0 {...} else {
// recordStreamError(...) }` when streamCh closed without ever sending a
// Done chunk and with no content received — and that else branch only
// persisted the error to session history, never to outCh. A client waiting
// on the stream saw it just close with nothing, the same stuck-UI symptom
// as the ctx-cancellation chunk-race bug this file already fixed once
// (BUG-H1/H2) — except with no explanation at all, not even a late one.
func TestDrainAgentStream_EmptyChannelSendsTerminalChunk(t *testing.T) {
	a := &App{}
	streamCh := make(chan provider.StreamChunk)
	close(streamCh) // closes immediately with nothing sent — the exact shape this bug needed

	outCh := make(chan api.StreamChunk, 4)
	a.drainAgentStream(context.Background(), streamCh, outCh, time.Now(), "hi", "", &usageMeta{}, nil)
	close(outCh)

	var chunks []api.StreamChunk
	for c := range outCh {
		chunks = append(chunks, c)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected exactly one terminal chunk, got %d: %+v", len(chunks), chunks)
	}
	if !chunks[0].Done {
		t.Errorf("expected the terminal chunk to have Done=true, got %+v", chunks[0])
	}
	if chunks[0].Error == "" {
		t.Errorf("expected the terminal chunk to carry an error explaining the empty response, got %+v", chunks[0])
	}
}
