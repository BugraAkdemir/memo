package livemode

import (
	"context"
	"fmt"
	"sync"
)

// EchoSession is a stub Session that plays back exactly the audio it
// receives — originally used only to prove the WS transport (Flutter mic
// capture -> backend -> Flutter playback) works end-to-end before any real
// engine client existed (docs/plans/PLAN_live_mode_v2.md's Phase 6).
// NewLiveModeSession (internal/app/livemode_session.go) now also falls back
// to it whenever a real engine session can't be built, so it doubles as
// that failure mode's degraded-but-not-broken state — see reason below.
type EchoSession struct {
	events chan SessionEvent
	reason error

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

// NewEchoSessionWithReason is NewEchoSession, plus one EventError emitted
// as soon as Start runs — for the fallback case (unconfigured/misconfigured
// native engine, or an internal setup failure) rather than the original
// prove-the-transport-works case. A silent echo session used to be
// indistinguishable from a real one to the user: they'd hear their own
// voice looped back with no explanation why the model never actually
// responded. reason is delivered exactly once, through the same
// EventError/frame.error path a real engine's own errors already use
// (internal/webserver/handlers_livemode_session.go ->
// live_realtime_session_provider.dart's _setError -> errorMessageProvider),
// so it surfaces as the same kind of visible toast instead of nothing.
func NewEchoSessionWithReason(reason error) *EchoSession {
	s := NewEchoSession()
	s.reason = reason
	return s
}

func (s *EchoSession) Start(ctx context.Context) error {
	if s.reason != nil {
		select {
		case s.events <- SessionEvent{Type: EventError, Err: s.reason}:
		default:
		}
	}
	return nil
}

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

// InjectContext is a no-op — EchoSession has no real conversation to inject
// text into.
func (s *EchoSession) InjectContext(text string) error { return nil }

func (s *EchoSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.closed)
		close(s.events)
	})
	return nil
}
