package livemode

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEchoSession_EchoesAudioBack(t *testing.T) {
	s := NewEchoSession()
	defer s.Close()

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.SendAudio([]byte("pcm-chunk-1")); err != nil {
		t.Fatalf("SendAudio: %v", err)
	}

	select {
	case ev := <-s.Events():
		if ev.Type != EventAudioOut {
			t.Errorf("expected EventAudioOut, got %s", ev.Type)
		}
		if string(ev.Audio) != "pcm-chunk-1" {
			t.Errorf("expected echoed audio to match input, got %q", ev.Audio)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for echoed event")
	}
}

func TestEchoSession_SendAudioAfterCloseFails(t *testing.T) {
	s := NewEchoSession()
	s.Close()

	if err := s.SendAudio([]byte("x")); err == nil {
		t.Error("expected an error sending audio to a closed session")
	}
}

func TestEchoSession_CloseIsIdempotent(t *testing.T) {
	s := NewEchoSession()
	s.Close()
	if err := s.Close(); err != nil {
		t.Errorf("expected a second Close to be a no-op, got: %v", err)
	}
}

func TestEchoSession_EventsChannelClosesAfterClose(t *testing.T) {
	s := NewEchoSession()
	s.Close()

	_, ok := <-s.Events()
	if ok {
		t.Error("expected Events() channel to be closed")
	}
}

func TestEchoSession_InjectContextIsANoOp(t *testing.T) {
	s := NewEchoSession()
	defer s.Close()
	if err := s.InjectContext("anything"); err != nil {
		t.Errorf("expected InjectContext to be a no-op, got: %v", err)
	}
}

// TestNewEchoSessionWithReason_EmitsErrorOnStart guards the stability-audit
// fix (H1): a plain NewEchoSession() (the original prove-the-transport-works
// case) must stay silent, but the fallback path — NewLiveModeSession
// reaching for an echo session because the real engine isn't configured —
// must tell the user why instead of just looping their own voice back with
// no explanation.
func TestNewEchoSessionWithReason_EmitsErrorOnStart(t *testing.T) {
	reason := errors.New("engine not configured")
	s := NewEchoSessionWithReason(reason)
	defer s.Close()

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	select {
	case ev := <-s.Events():
		if ev.Type != EventError {
			t.Fatalf("got event type %s, want %s", ev.Type, EventError)
		}
		if ev.Err != reason {
			t.Errorf("got Err=%v, want the exact reason passed to NewEchoSessionWithReason", ev.Err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the EventError Start() should emit")
	}
}

func TestNewEchoSession_PlainConstructorStaysSilentOnStart(t *testing.T) {
	s := NewEchoSession()
	defer s.Close()

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	select {
	case ev := <-s.Events():
		t.Fatalf("expected no event from a reason-less EchoSession, got %+v", ev)
	case <-time.After(100 * time.Millisecond):
		// Expected: nothing queued.
	}
}

// TestEchoSession_ErrorThenAudioBothDeliver guards that emitting the
// EventError on Start doesn't consume the buffer slot a subsequent echoed
// SendAudio needs — the session must still actually echo, not just report
// why it's an echo session.
func TestEchoSession_ErrorThenAudioBothDeliver(t *testing.T) {
	s := NewEchoSessionWithReason(errors.New("no engine"))
	defer s.Close()
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.SendAudio([]byte("pcm")); err != nil {
		t.Fatalf("SendAudio: %v", err)
	}

	var gotError, gotAudio bool
	for i := 0; i < 2; i++ {
		select {
		case ev := <-s.Events():
			switch ev.Type {
			case EventError:
				gotError = true
			case EventAudioOut:
				gotAudio = true
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for both events")
		}
	}
	if !gotError || !gotAudio {
		t.Errorf("gotError=%v gotAudio=%v, want both true", gotError, gotAudio)
	}
}
