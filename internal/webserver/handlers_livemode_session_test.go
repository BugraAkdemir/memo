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
	"memo/internal/config"
	"memo/internal/livemode"
	"memo/internal/livemode/google"
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

// TestHandleLiveModeSession_DispatchesToGoogleClientWhenConfigured proves
// Phase 7's dispatch end-to-end: a Flutter-side WS client connects to
// handleLiveModeSession, whose bridge reports ActiveEngine="google_live"
// with a saved API key/model — newLiveModeSession must construct a real
// google.Client (not EchoSession) for it, which then talks to a fake
// Gemini Live server standing in for the real one. Audio pushed by that
// fake server must arrive back at the Flutter-side client unchanged,
// proving the whole chain (Flutter WS <-> Go bridge <-> Google WS) works,
// not just the bridge's local echo path already covered above.
func TestHandleLiveModeSession_DispatchesToGoogleClientWhenConfigured(t *testing.T) {
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
		getLiveModeConfig: func() config.LiveModeConfig {
			return config.LiveModeConfig{ActiveEngine: "google_live"}
		},
		getLiveModeEngines: func() []livemode.EngineConfig {
			return []livemode.EngineConfig{{
				Type:    livemode.EngineGoogleLive,
				APIKey:  "g-key",
				Model:   "models/gemini-3.1-flash-live-preview",
				Enabled: true,
			}}
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

func TestHandleLiveModeSession_NotAvailableWithoutBridge(t *testing.T) {
	srv := &Server{}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/livemode/session", nil)
	srv.handleLiveModeSession(rr, req)
	if rr.Code != 501 {
		t.Errorf("expected 501 Not Implemented without a bridge, got %d", rr.Code)
	}
}
