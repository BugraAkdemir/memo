package webserver

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
	"memo/internal/livemode"
	"memo/internal/livemode/google"
	"memo/internal/livemode/openai_realtime"
)

// TestHandleLiveModeSession_EchoesBinaryAudio proves the Phase 6 transport
// end-to-end: a client connects, sends a binary (PCM) frame, and gets the
// exact same bytes back — the EchoSession's whole purpose, standing in for
// a real engine client until Phase 7/8. No fullBridge method this handler
// actually calls needs anything beyond zero values, so the swarmStubBridge
// test double (already used by other test files in this package to satisfy
// the full FullBridge interface) is enough.
func TestHandleLiveModeSession_EchoesBinaryAudio(t *testing.T) {
	srv := &Server{fullBridge: &swarmStubBridge{}}
	httpSrv := httptest.NewServer(http.HandlerFunc(srv.handleLiveModeSession))
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.CloseNow()

	if err := c.Write(ctx, websocket.MessageBinary, []byte("pcm-chunk")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	msgType, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if msgType != websocket.MessageBinary {
		t.Errorf("expected a binary message back, got %v", msgType)
	}
	if string(data) != "pcm-chunk" {
		t.Errorf("expected echoed audio to match, got %q", data)
	}

	c.Close(websocket.StatusNormalClosure, "")
}

// TestHandleLiveModeSession_UsesWhateverBridgeSessionReturns proves the
// handler's whole job end-to-end: whatever livemode.Session
// FullBridge.NewLiveModeSession hands back (a real google.Client here,
// standing in for App.NewLiveModeSession's own dispatch — see
// internal/app/livemode_session_test.go for that dispatch logic itself) is
// what the Flutter-side WS client actually talks to. A fake Gemini Live
// server's pushed audio must arrive back at the Flutter-side client
// unchanged, proving the whole chain (Flutter WS <-> Go bridge <-> Google
// WS) works, not just the local echo path TestHandleLiveModeSession_
// EchoesBinaryAudio already covers.
func TestHandleLiveModeSession_UsesWhateverBridgeSessionReturns(t *testing.T) {
	replyB64 := base64.StdEncoding.EncodeToString([]byte("gemini-reply-pcm"))
	fakeGoogle := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume setup
			return
		}
		payload, _ := json.Marshal(map[string]any{
			"serverContent": map[string]any{
				"modelTurn": map[string]any{
					"parts": []map[string]any{
						{"inlineData": map[string]any{"mimeType": "audio/pcm;rate=24000", "data": replyB64}},
					},
				},
			},
		})
		c.Write(ctx, websocket.MessageText, payload)
		<-ctx.Done()
	}))
	defer fakeGoogle.Close()

	originalBase := google.SessionBaseURL
	google.SessionBaseURL = "ws" + strings.TrimPrefix(fakeGoogle.URL, "http")
	defer func() { google.SessionBaseURL = originalBase }()

	srv := &Server{fullBridge: &swarmStubBridge{
		newLiveModeSession: func(ctx context.Context) livemode.Session {
			return google.NewClient("g-key", "models/gemini-3.1-flash-live-preview", "", nil, nil)
		},
	}}
	httpSrv := httptest.NewServer(http.HandlerFunc(srv.handleLiveModeSession))
	defer httpSrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpSrv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.CloseNow()

	msgType, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if msgType != websocket.MessageBinary {
		t.Fatalf("expected a binary audio message, got %v", msgType)
	}
	if string(data) != "gemini-reply-pcm" {
		t.Errorf("expected the fake Gemini server's audio to reach the Flutter-side client unchanged, got %q", data)
	}

	c.Close(websocket.StatusNormalClosure, "")
}

// TestHandleLiveModeSession_UsesWhateverBridgeSessionReturns_OpenAI mirrors
// the Google test above against OpenAI Realtime's own message shape
// (response.output_audio.delta).
func TestHandleLiveModeSession_UsesWhateverBridgeSessionReturns_OpenAI(t *testing.T) {
	replyB64 := base64.StdEncoding.EncodeToString([]byte("openai-reply-pcm"))
	fakeOpenAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume session.update
			return
		}
		payload, _ := json.Marshal(map[string]any{
			"type":  "response.output_audio.delta",
			"delta": replyB64,
		})
		c.Write(ctx, websocket.MessageText, payload)
		<-ctx.Done()
	}))
	defer fakeOpenAI.Close()

	originalBase := openai_realtime.SessionBaseURL
	openai_realtime.SessionBaseURL = "ws" + strings.TrimPrefix(fakeOpenAI.URL, "http")
	defer func() { openai_realtime.SessionBaseURL = originalBase }()

	srv := &Server{fullBridge: &swarmStubBridge{
		newLiveModeSession: func(ctx context.Context) livemode.Session {
			return openai_realtime.NewClient("oa-key", "gpt-realtime-2.1", "", nil, nil)
		},
	}}
	httpSrv := httptest.NewServer(http.HandlerFunc(srv.handleLiveModeSession))
	defer httpSrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpSrv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.CloseNow()

	msgType, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if msgType != websocket.MessageBinary {
		t.Fatalf("expected a binary audio message, got %v", msgType)
	}
	if string(data) != "openai-reply-pcm" {
		t.Errorf("expected the fake OpenAI server's audio to reach the Flutter-side client unchanged, got %q", data)
	}

	c.Close(websocket.StatusNormalClosure, "")
}

func TestHandleLiveModeSession_NotAvailableWithoutBridge(t *testing.T) {
	srv := &Server{}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/livemode/session", nil)
	srv.handleLiveModeSession(rr, req)
	if rr.Code != 501 {
		t.Errorf("expected 501 Not Implemented without a bridge, got %d", rr.Code)
	}
}
