package whatsapp

import (
	"errors"
	"sync"
	"testing"

	waEvent "go.mau.fi/whatsmeow/types/events"
)

// TestHandleEventConcurrentAccess is a regression test for BUG-C2: handleEvent
// used to write c.qrCodes/c.lastError/c.started/c.reconnecting with no lock,
// while QRCodes()/LastError()/IsReconnecting()/IsConnected() read them under
// startMu — a classic data race. Fixed in 854e04d by taking startMu around
// every handleEvent write. Run with -race to catch any regression.
func TestHandleEventConcurrentAccess(t *testing.T) {
	c := NewClient(Config{})

	events := []any{
		&waEvent.QR{Codes: []string{"code-1", "code-2"}},
		&waEvent.PairSuccess{},
		&waEvent.PairError{Error: errors.New("boom")},
		&waEvent.Connected{},
		&waEvent.Disconnected{},
	}

	var wg sync.WaitGroup
	const iterations = 200

	for _, evt := range events {
		wg.Go(func() {
			for range iterations {
				c.handleEvent(evt)
			}
		})
	}

	for range 4 {
		wg.Go(func() {
			for range iterations {
				_ = c.QRCodes()
				_ = c.LastError()
				_ = c.IsReconnecting()
				_ = c.IsConnected()
				_ = c.IsLoggedIn()
			}
		})
	}

	wg.Wait()
}

// TestStopChRecreatedAfterStop is a regression test for BUG-QH3: stopCh was
// created once in NewClient and closed one-shot (via stopOnce) by Stop(),
// but Start() never recreated either. After any Stop()+Start() cycle,
// autoReconnect's `case <-c.stopCh:` saw an already-closed channel from the
// *very first* Stop() ever called and returned immediately on every
// subsequent disconnect — permanently disabling auto-reconnect until the
// process restarts.
//
// Start() itself can't be driven directly here (it opens a real whatsmeow
// session and dials the network), so this exercises the exact fix — resetting
// stopCh/stopOnce under startMu — the same way Start() now does internally,
// and verifies a second Stop() can close the *new* channel.
func TestStopChRecreatedAfterStop(t *testing.T) {
	c := NewClient(Config{})

	c.Stop()
	select {
	case <-c.stopCh:
	default:
		t.Fatal("stopCh should be closed after the first Stop()")
	}

	c.startMu.Lock()
	c.stopCh = make(chan struct{})
	c.stopOnce = sync.Once{}
	c.startMu.Unlock()

	select {
	case <-c.stopCh:
		t.Fatal("recreated stopCh must not already be closed")
	default:
	}

	// This is exactly what silently failed before the fix: stopOnce, once
	// fired, never runs its function again, so the second Stop() would have
	// been a no-op against the old channel — but against the *new* one it
	// must actually close it.
	c.Stop()
	select {
	case <-c.stopCh:
	default:
		t.Fatal("second Stop() should have closed the recreated stopCh")
	}
}

// TestDisconnectedGuardsReconnectLoop covers the handleEvent guard added
// when autoReconnect became an indefinite retry: a Disconnected event on a
// started client claims `reconnecting` under startMu, and a second one
// while it's still claimed must not clear it (or start a competing loop
// that would then run forever). Start() can't be driven here (real
// whatsmeow session) so waClient stays nil — the spawned autoReconnect
// goroutine sees alive==false and exits; what's under test is the guard.
func TestDisconnectedGuardsReconnectLoop(t *testing.T) {
	c := NewClient(Config{})
	c.startMu.Lock()
	c.started = true
	c.startMu.Unlock()
	t.Cleanup(func() {
		c.startMu.Lock()
		c.started = false
		close(c.stopCh) // let any spawned autoReconnect goroutine exit now
		c.startMu.Unlock()
	})

	c.handleEvent(&waEvent.Disconnected{})
	if !c.IsReconnecting() {
		t.Fatal("first Disconnected on a started client should claim reconnecting")
	}

	c.handleEvent(&waEvent.Disconnected{})
	if !c.IsReconnecting() {
		t.Error("a second Disconnected must not clear the reconnecting flag")
	}
}

// TestAttemptReconnect_StaleGenerationDoesNotClobberNewerState guards Y3:
// Stop() does not wait for an in-flight autoReconnect goroutine to exit —
// it just closes stopCh and relies on the goroutine noticing on its own
// next check. If a Stop()+Start() cycle (toggling WhatsApp off and back on)
// completes while an old autoReconnect attempt is past its stopCh check
// (e.g. mid wa.Connect(), an unlocked network call, or in the
// field-write right after), it used to overwrite c.reconnecting/
// c.lastError unconditionally on the way out — silently clobbering
// whatever the new generation's own Start()/autoReconnect had already set.
// attemptReconnect(myGen) must recognize its generation was superseded
// (c.startGen bumped by a newer Start()) and return without touching
// either field.
func TestAttemptReconnect_StaleGenerationDoesNotClobberNewerState(t *testing.T) {
	c := NewClient(Config{})
	c.startMu.Lock()
	staleGen := c.startGen // generation this (simulated) goroutine was spawned under
	// Simulate a Stop()+Start() cycle completing concurrently: a newer
	// generation is now active, and it (or its own autoReconnect) has
	// already recorded real state that must survive.
	c.startGen++
	c.started = true
	c.reconnecting = true
	c.lastError = "a real error from the new generation"
	c.startMu.Unlock()

	outcome := c.attemptReconnect(staleGen)
	if outcome != reconnectDone {
		t.Fatalf("attemptReconnect() = %v, want reconnectDone for a superseded generation", outcome)
	}

	c.startMu.Lock()
	defer c.startMu.Unlock()
	if !c.reconnecting {
		t.Error("stale attemptReconnect must not clear reconnecting — it does not own the current generation's state")
	}
	if c.lastError != "a real error from the new generation" {
		t.Errorf("lastError = %q, want the newer generation's error left untouched", c.lastError)
	}
}

// TestAttemptReconnect_SameGenerationStoppedClient_ClearsReconnecting is the
// non-superseded counterpart: when the generation still matches but the
// client has genuinely been stopped (c.started == false), attemptReconnect
// must still clear reconnecting — this is the ordinary "give up, we were
// stopped" path, unaffected by the Y3 generation guard.
func TestAttemptReconnect_SameGenerationStoppedClient_ClearsReconnecting(t *testing.T) {
	c := NewClient(Config{})
	c.startMu.Lock()
	myGen := c.startGen
	c.reconnecting = true
	c.started = false
	c.startMu.Unlock()

	outcome := c.attemptReconnect(myGen)
	if outcome != reconnectDone {
		t.Fatalf("attemptReconnect() = %v, want reconnectDone for a stopped client", outcome)
	}
	if c.IsReconnecting() {
		t.Error("expected reconnecting to be cleared for a genuinely stopped client")
	}
}
