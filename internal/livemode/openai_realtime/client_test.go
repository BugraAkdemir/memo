package openai_realtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type fakeRealtimeServer struct {
	srv           *httptest.Server
	gotAuth       chan string
	gotModelQuery chan string
	gotUpdate     chan sessionUpdateEvent
	gotAudioIn    chan string
}

func newFakeRealtimeServer(t *testing.T) *fakeRealtimeServer {
	t.Helper()
	f := &fakeRealtimeServer{
		gotAuth:       make(chan string, 1),
		gotModelQuery: make(chan string, 1),
		gotUpdate:     make(chan sessionUpdateEvent, 1),
		gotAudioIn:    make(chan string, 8),
	}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.gotAuth <- r.Header.Get("Authorization")
		f.gotModelQuery <- r.URL.Query().Get("model")

		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()

		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var first sessionUpdateEvent
		if err := json.Unmarshal(data, &first); err != nil || first.Type != "session.update" {
			t.Errorf("expected the first client message to be session.update, got %q", data)
			return
		}
		f.gotUpdate <- first

		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var ev inputAudioBufferAppendEvent
			if err := json.Unmarshal(data, &ev); err != nil {
				continue
			}
			if ev.Type == "input_audio_buffer.append" {
				select {
				case f.gotAudioIn <- ev.Audio:
				default:
				}
			}
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeRealtimeServer) wsURL() string {
	return "ws" + strings.TrimPrefix(f.srv.URL, "http")
}

func TestClient_SendsAuthHeaderAndModelQueryParam(t *testing.T) {
	f := newFakeRealtimeServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case auth := <-f.gotAuth:
		if auth != "Bearer oa-key" {
			t.Errorf("expected Bearer oa-key, got %q", auth)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the Authorization header")
	}
	select {
	case model := <-f.gotModelQuery:
		if model != "gpt-realtime-2.1" {
			t.Errorf("expected model query param gpt-realtime-2.1, got %q", model)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the model query param")
	}
}

func TestClient_SendsSessionUpdateOnStart(t *testing.T) {
	f := newFakeRealtimeServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "You are Memo's live voice.")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case update := <-f.gotUpdate:
		if update.Session.Model != "gpt-realtime-2.1" {
			t.Errorf("unexpected model in session.update: %q", update.Session.Model)
		}
		if update.Session.Instructions != "You are Memo's live voice." {
			t.Errorf("unexpected instructions: %q", update.Session.Instructions)
		}
		if len(update.Session.OutputModalities) != 1 || update.Session.OutputModalities[0] != "audio" {
			t.Errorf("expected output_modalities=[audio], got %v", update.Session.OutputModalities)
		}
		if update.Session.Audio == nil || update.Session.Audio.Input == nil || update.Session.Audio.Input.Format.Rate != inputSampleRateHz {
			t.Errorf("expected audio.input.format.rate=%d, got %+v", inputSampleRateHz, update.Session.Audio)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session.update on the server side")
	}
}

func TestClient_SendAudioForwardsBase64EncodedPCM(t *testing.T) {
	f := newFakeRealtimeServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()
	<-f.gotUpdate // wait past the session.update handshake

	if err := c.SendAudio([]byte("raw-pcm-bytes")); err != nil {
		t.Fatalf("SendAudio: %v", err)
	}

	select {
	case b64 := <-f.gotAudioIn:
		decoded, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if string(decoded) != "raw-pcm-bytes" {
			t.Errorf("expected round-tripped PCM bytes, got %q", decoded)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for audio chunk on the server side")
	}
}

func TestClient_EmitsAudioOutFromResponseOutputAudioDelta(t *testing.T) {
	audioB64 := base64.StdEncoding.EncodeToString([]byte("reply-pcm-bytes"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume session.update
			return
		}
		payload, _ := json.Marshal(serverEvent{Type: serverEventAudioDelta, Delta: audioB64})
		c.Write(ctx, websocket.MessageText, payload)
		<-ctx.Done()
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case ev := <-c.Events():
		if ev.Type != "audio_out" {
			t.Fatalf("expected an audio_out event, got %s (err=%v)", ev.Type, ev.Err)
		}
		if string(ev.Audio) != "reply-pcm-bytes" {
			t.Errorf("expected decoded reply audio, got %q", ev.Audio)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the audio_out event")
	}
}

func TestClient_IgnoresUnrelatedServerEventTypes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil {
			return
		}
		// session.created / input_audio_buffer.speech_started style events
		// must not produce an audio_out (or any) event.
		payload, _ := json.Marshal(serverEvent{Type: "session.created"})
		c.Write(ctx, websocket.MessageText, payload)
		<-ctx.Done()
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case ev := <-c.Events():
		t.Fatalf("expected no event for an unrelated server event type, got %+v", ev)
	case <-time.After(300 * time.Millisecond):
		// expected: nothing arrives
	}
}
