package app

import (
	"context"
	"testing"
	"time"

	"memo/internal/livemode"
)

// fakeLiveSession is a minimal livemode.Session whose Events() channel the
// test fully controls, for exercising livePermissionRoutingSession's pump
// in isolation from any real provider client.
type fakeLiveSession struct {
	events chan livemode.SessionEvent
}

func (f *fakeLiveSession) Start(context.Context) error          { return nil }
func (f *fakeLiveSession) SendAudio([]byte) error               { return nil }
func (f *fakeLiveSession) InjectContext(string) error           { return nil }
func (f *fakeLiveSession) Events() <-chan livemode.SessionEvent { return f.events }
func (f *fakeLiveSession) Close() error                         { return nil }

func TestLivePermissionRoutingSession_ForwardsAllEventsUnchanged(t *testing.T) {
	a := &App{}
	inner := &fakeLiveSession{events: make(chan livemode.SessionEvent, 4)}
	inner.events <- livemode.SessionEvent{Type: livemode.EventAudioOut, Audio: []byte("pcm")}
	inner.events <- livemode.SessionEvent{Type: livemode.EventTranscript, Transcript: "hello"}
	close(inner.events)

	wrapped := a.wrapLiveModeSessionForPermissionRouting(inner)

	var got []livemode.SessionEvent
	for ev := range wrapped.Events() {
		got = append(got, ev)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events forwarded, got %d: %+v", len(got), got)
	}
	if got[0].Type != livemode.EventAudioOut || got[1].Type != livemode.EventTranscript {
		t.Errorf("events forwarded out of order or wrong type: %+v", got)
	}
	if got[1].Transcript != "hello" {
		t.Errorf("expected the transcript text to pass through unchanged, got %q", got[1].Transcript)
	}
}

// TestLivePermissionRoutingSession_RoutesTranscriptToPendingAnswer confirms
// the dual-consumption design: a transcript both resolves a pending
// voice_prompt permission question AND still reaches the outward Events()
// channel for normal Flutter display — see docs/plans/PLAN_live_mode_v2.md
// Phase 12.
func TestLivePermissionRoutingSession_RoutesTranscriptToPendingAnswer(t *testing.T) {
	a := &App{}
	inner := &fakeLiveSession{events: make(chan livemode.SessionEvent, 1)}
	wrapped := a.wrapLiveModeSessionForPermissionRouting(inner)

	type answer struct {
		text string
		ok   bool
	}
	answerCh := make(chan answer, 1)
	go func() {
		text, ok := a.awaitLivePermissionAnswer(context.Background())
		answerCh <- answer{text, ok}
	}()
	waitForPendingLivePermAnswer(t, a)

	inner.events <- livemode.SessionEvent{Type: livemode.EventTranscript, Transcript: "evet"}
	close(inner.events)

	select {
	case res := <-answerCh:
		if !res.ok || res.text != "evet" {
			t.Errorf("expected the routed transcript to resolve the pending answer, got text=%q ok=%v", res.text, res.ok)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("awaitLivePermissionAnswer never returned")
	}

	select {
	case ev := <-wrapped.Events():
		if ev.Type != livemode.EventTranscript || ev.Transcript != "evet" {
			t.Errorf("expected the transcript event to still reach the outward channel, got %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("transcript event never reached the outward channel")
	}
}

func TestLivePermissionRoutingSession_ClosesOutwardChannelWhenInnerCloses(t *testing.T) {
	a := &App{}
	inner := &fakeLiveSession{events: make(chan livemode.SessionEvent)}
	wrapped := a.wrapLiveModeSessionForPermissionRouting(inner)
	close(inner.events)

	select {
	case _, ok := <-wrapped.Events():
		if ok {
			t.Error("expected the outward channel to be closed, got an event instead")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("outward channel never closed after the inner session's channel closed")
	}
}
