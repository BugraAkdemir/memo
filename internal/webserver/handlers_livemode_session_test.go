package webserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
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

func TestHandleLiveModeSession_NotAvailableWithoutBridge(t *testing.T) {
	srv := &Server{}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/livemode/session", nil)
	srv.handleLiveModeSession(rr, req)
	if rr.Code != 501 {
		t.Errorf("expected 501 Not Implemented without a bridge, got %d", rr.Code)
	}
}
