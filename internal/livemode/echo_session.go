package livemode

import (
	"context"
	"fmt"
	"sync"
)

// EchoSession is a stub Session that plays back exactly the audio it
// receives — used only to prove the WS transport (Flutter mic capture ->
// backend -> Flutter playback) works end-to-end before any real engine
// client exists. See docs/plans/PLAN_live_mode_v2.md's Phase 6.
type EchoSession struct {
	events chan SessionEvent

	closeOnce sync.Once
	closed    chan struct{}
}

// NewEchoSession creates a ready-to-use EchoSession. Events has a small
// buffer so a burst of SendAudio calls doesn't block the caller while the
// WS bridge's own read/write pump goroutines are scheduled.
func NewEchoSession() *EchoSession {
	return &EchoSession{
		events: make(chan SessionEvent, 16),
		closed: make(chan struct{}),
	}
}

func (s *EchoSession) Start(ctx context.Context) error { return nil }

func (s *EchoSession) SendAudio(pcm []byte) error {
	select {
	case <-s.closed:
		return fmt.Errorf("livemode: session closed")
	default:
	}
	select {
	case s.events <- SessionEvent{Type: EventAudioOut, Audio: pcm}:
		return nil
	case <-s.closed:
		return fmt.Errorf("livemode: session closed")
	}
}

func (s *EchoSession) Events() <-chan SessionEvent { return s.events }

func (s *EchoSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.closed)
		close(s.events)
	})
	return nil
}
