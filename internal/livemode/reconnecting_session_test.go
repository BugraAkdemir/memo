package livemode

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeSession is a controllable Session stub for exercising
// ReconnectingSession without a real websocket/provider.
type fakeSession struct {
	events   chan SessionEvent
	startErr error

	mu        sync.Mutex
	sentAudio [][]byte
	closed    bool
}

func newFakeSession() *fakeSession {
	return &fakeSession{events: make(chan SessionEvent, 4)}
}

func (f *fakeSession) Start(ctx context.Context) error { return f.startErr }

func (f *fakeSession) SendAudio(pcm []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sentAudio = append(f.sentAudio, pcm)
	return nil
}

func (f *fakeSession) InjectContext(text string) error { return nil }
func (f *fakeSession) Events() <-chan SessionEvent     { return f.events }

func (f *fakeSession) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeSession) audioCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sentAudio)
}

func withShortBackoff(t *testing.T) {
	t.Helper()
	origInitial, origMax := reconnectInitialDelay, reconnectMaxDelay
	reconnectInitialDelay = 5 * time.Millisecond
	reconnectMaxDelay = 20 * time.Millisecond
	t.Cleanup(func() {
		reconnectInitialDelay, reconnectMaxDelay = origInitial, origMax
	})
}

func drainOne(t *testing.T, ch <-chan SessionEvent, timeout time.Duration) SessionEvent {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(timeout):
		t.Fatal("timed out waiting for an event")
		return SessionEvent{}
	}
}

func TestReconnectingSession_ForwardsEventsFromInner(t *testing.T) {
	withShortBackoff(t)
	inner := newFakeSession()
	var starts int32
	r := NewReconnectingSession(func() Session {
		atomic.AddInt32(&starts, 1)
		return inner
	})
	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer r.Close()

	inner.events <- SessionEvent{Type: EventTranscript, Transcript: "hello"}
	ev := drainOne(t, r.Events(), 2*time.Second)
	if ev.Type != EventTranscript || ev.Transcript != "hello" {
		t.Errorf("got %+v, want a forwarded transcript event", ev)
	}
	if got := atomic.LoadInt32(&starts); got != 1 {
		t.Errorf("factory called %d times, want 1 (no reconnect should have happened)", got)
	}
}

// TestReconnectingSession_ReconnectsAfterUnexpectedDrop is the regression
// test for the P1 finding that neither Live Mode engine had any reconnect
// logic: closing the first inner session's Events() channel without
// cancelling the outer context (exactly what a real network drop looks
// like from readLoop's perspective) must trigger a redial via factory, not
// end the whole session.
func TestReconnectingSession_ReconnectsAfterUnexpectedDrop(t *testing.T) {
	withShortBackoff(t)
	first := newFakeSession()
	second := newFakeSession()
	var starts int32
	r := NewReconnectingSession(func() Session {
		n := atomic.AddInt32(&starts, 1)
		if n == 1 {
			return first
		}
		return second
	})
	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer r.Close()

	first.events <- SessionEvent{Type: EventTranscript, Transcript: "before drop"}
	drainOne(t, r.Events(), 2*time.Second)

	// Simulate an unexpected drop: close the channel without cancelling ctx.
	close(first.events)

	// The supervisor should redial via factory and forward events from the
	// new (second) session.
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&starts) < 2 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&starts); got != 2 {
		t.Fatalf("factory called %d times, want 2 (reconnect never happened)", got)
	}

	second.events <- SessionEvent{Type: EventTranscript, Transcript: "after reconnect"}
	ev := drainOne(t, r.Events(), 2*time.Second)
	if ev.Transcript != "after reconnect" {
		t.Errorf("got %+v, want the post-reconnect event forwarded", ev)
	}
}

// TestReconnectingSession_RetriesDialFailureWithBackoff confirms a failed
// reconnect attempt (e.g. the network is still down) doesn't give up —
// it keeps retrying with backoff until one succeeds.
func TestReconnectingSession_RetriesDialFailureWithBackoff(t *testing.T) {
	withShortBackoff(t)
	first := newFakeSession()
	var starts int32
	r := NewReconnectingSession(func() Session {
		n := atomic.AddInt32(&starts, 1)
		switch n {
		case 1:
			return first
		case 2, 3:
			return &fakeSession{events: make(chan SessionEvent), startErr: errors.New("dial failed")}
		default:
			s := newFakeSession()
			return s
		}
	})
	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer r.Close()

	close(first.events) // unexpected drop

	deadline := time.Now().Add(3 * time.Second)
	for atomic.LoadInt32(&starts) < 4 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&starts); got != 4 {
		t.Fatalf("factory called %d times, want 4 (1 initial + 2 failed reconnects + 1 successful)", got)
	}
}

// TestReconnectingSession_CloseStopsForGood confirms Close() ends the
// session permanently — no further reconnect attempts, and Events()
// closes.
func TestReconnectingSession_CloseStopsForGood(t *testing.T) {
	withShortBackoff(t)
	inner := newFakeSession()
	var starts int32
	r := NewReconnectingSession(func() Session {
		atomic.AddInt32(&starts, 1)
		return inner
	})
	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Close() cancels ctx; the inner's events channel is still open here
	// (fakeSession.Close doesn't close it), but the supervisor should exit
	// via ctx.Done() rather than waiting on it forever.
	select {
	case _, ok := <-r.Events():
		if ok {
			t.Error("expected Events() to be closed (or empty) after Close()")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Events() never closed after Close()")
	}

	inner.mu.Lock()
	closed := inner.closed
	inner.mu.Unlock()
	if !closed {
		t.Error("inner session was not Close()d")
	}

	startsAfterClose := atomic.LoadInt32(&starts)
	time.Sleep(50 * time.Millisecond) // long enough for a stray reconnect to fire, if one were going to
	if got := atomic.LoadInt32(&starts); got != startsAfterClose {
		t.Errorf("factory called again after Close() (%d -> %d) — reconnect should not fire post-close", startsAfterClose, got)
	}
}

// TestReconnectingSession_SendAudioRoutesToCurrentInner confirms
// SendAudio/InjectContext follow the reconnect — a stale reference to the
// old inner session must not swallow audio silently after a drop.
func TestReconnectingSession_SendAudioRoutesToCurrentInner(t *testing.T) {
	withShortBackoff(t)
	first := newFakeSession()
	second := newFakeSession()
	var starts int32
	r := NewReconnectingSession(func() Session {
		n := atomic.AddInt32(&starts, 1)
		if n == 1 {
			return first
		}
		return second
	})
	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer r.Close()

	if err := r.SendAudio([]byte("chunk1")); err != nil {
		t.Fatalf("SendAudio: %v", err)
	}
	if first.audioCount() != 1 {
		t.Fatalf("first.audioCount() = %d, want 1", first.audioCount())
	}

	close(first.events)
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&starts) < 2 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}

	if err := r.SendAudio([]byte("chunk2")); err != nil {
		t.Fatalf("SendAudio after reconnect: %v", err)
	}
	if second.audioCount() != 1 {
		t.Errorf("second.audioCount() = %d, want 1 (post-reconnect audio should route to the new session)", second.audioCount())
	}
}
