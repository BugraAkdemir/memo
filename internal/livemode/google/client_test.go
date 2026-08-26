package google

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

// fakeLiveServer accepts one WS connection acting like the real Gemini
// Live endpoint: expects a setup message first, records it, then lets the
// test drive further server->client sends via the returned channels.
type fakeLiveServer struct {
	srv        *httptest.Server
	gotSetup   chan setupMessage
	gotAudioIn chan audioChunk
}

func newFakeLiveServer(t *testing.T) *fakeLiveServer {
	t.Helper()
	f := &fakeLiveServer{
		gotSetup:   make(chan setupMessage, 1),
		gotAudioIn: make(chan audioChunk, 8),
	}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		var first clientMessage
		if err := json.Unmarshal(data, &first); err != nil || first.Setup == nil {
			t.Errorf("expected the first client message to be a setup message, got %q", data)
			return
		}
		f.gotSetup <- *first.Setup

		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var msg clientMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			if msg.RealtimeInput != nil && msg.RealtimeInput.Audio != nil {
				select {
				case f.gotAudioIn <- *msg.RealtimeInput.Audio:
				default:
				}
			}
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeLiveServer) wsURL() string {
	return "ws" + strings.TrimPrefix(f.srv.URL, "http")
}

func TestClient_SendsSetupMessageOnStart(t *testing.T) {
	f := newFakeLiveServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	c := NewClient("g-key", "models/gemini-3.1-flash-live-preview", "You are Memo's live voice.")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case setup := <-f.gotSetup:
		if setup.Model != "models/gemini-3.1-flash-live-preview" {
			t.Errorf("unexpected model in setup: %q", setup.Model)
		}
		if len(setup.ResponseModalities) != 1 || setup.ResponseModalities[0] != "AUDIO" {
			t.Errorf("expected responseModalities=[AUDIO], got %v", setup.ResponseModalities)
		}
		if setup.SystemInstruction == nil || setup.SystemInstruction.Parts[0].Text != "You are Memo's live voice." {
			t.Errorf("expected systemInstruction to carry the given text, got %+v", setup.SystemInstruction)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for setup message on the server side")
	}
}

func TestClient_SendAudioForwardsBase64EncodedPCM(t *testing.T) {
	f := newFakeLiveServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	c := NewClient("g-key", "models/gemini-3.1-flash-live-preview", "")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()
	<-f.gotSetup // wait past the setup handshake before sending audio

	if err := c.SendAudio([]byte("raw-pcm-bytes")); err != nil {
		t.Fatalf("SendAudio: %v", err)
	}

	select {
	case chunk := <-f.gotAudioIn:
		decoded, err := base64.StdEncoding.DecodeString(chunk.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if string(decoded) != "raw-pcm-bytes" {
			t.Errorf("expected round-tripped PCM bytes, got %q", decoded)
		}
		if chunk.MimeType != "audio/pcm;rate=16000" {
			t.Errorf("expected audio/pcm;rate=16000, got %q", chunk.MimeType)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for audio chunk on the server side")
	}
}

func TestClient_EmitsAudioOutFromServerContent(t *testing.T) {
	f := newFakeLiveServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	// Override the fake server to also push a serverContent audio frame
	// right after reading setup — simplest way is a second, dedicated
	// server for this one test rather than complicating fakeLiveServer's
	// shared helper with a send-back mode every other test doesn't need.
	audioB64 := base64.StdEncoding.EncodeToString([]byte("reply-pcm-bytes"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume setup
			return
		}
		serverMsg := serverMessage{ServerContent: &serverContent{
			ModelTurn: &modelTurn{Parts: []serverPart{{InlineData: &inlineData{
				MimeType: "audio/pcm;rate=24000",
				Data:     audioB64,
			}}}},
		}}
		payload, _ := json.Marshal(serverMsg)
		c.Write(ctx, websocket.MessageText, payload)
		<-ctx.Done()
	}))
	defer srv.Close()
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")

	c := NewClient("g-key", "models/gemini-3.1-flash-live-preview", "")
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
