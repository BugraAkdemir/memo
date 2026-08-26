package livemode

import (
	"context"
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
