package webserver

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var errSynthesizeFail = errors.New("synthesis failed")

func TestHandleTTSSynthesize(t *testing.T) {
	wantAudio := []byte("RIFF....WAVEfmt fake wav bytes")
	stub := &swarmStubBridge{synthesize: func(text string) ([]byte, error) {
		if text != "merhaba" {
			t.Errorf("got text %q, want %q", text, "merhaba")
		}
		return wantAudio, nil
	}}
	s := New(stub)
	s.fullBridge = stub

	body := strings.NewReader(`{"text":"merhaba"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tts/synthesize", body)
	w := httptest.NewRecorder()
	s.handleTTSSynthesize(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "audio/wav" {
		t.Errorf("got Content-Type %q, want %q", ct, "audio/wav")
	}
	if !bytes.Equal(w.Body.Bytes(), wantAudio) {
		t.Errorf("got body %q, want %q", w.Body.Bytes(), wantAudio)
	}
}

func TestHandleTTSSynthesize_Error(t *testing.T) {
	stub := &swarmStubBridge{synthesize: func(text string) ([]byte, error) {
		return nil, errSynthesizeFail
	}}
	s := New(stub)
	s.fullBridge = stub

	body := strings.NewReader(`{"text":"merhaba"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tts/synthesize", body)
	w := httptest.NewRecorder()
	s.handleTTSSynthesize(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandleTTSSynthesize_EmptyTextRejected(t *testing.T) {
	stub := &swarmStubBridge{}
	s := New(stub)
	s.fullBridge = stub

	body := strings.NewReader(`{"text":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tts/synthesize", body)
	w := httptest.NewRecorder()
	s.handleTTSSynthesize(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// TestHandleTTSSynthesize_NilFullBridgeRejected covers the base AppBridge
// (non-Flutter) client case — mirrors handleMemoryImportText's same guard.
func TestHandleTTSSynthesize_NilFullBridgeRejected(t *testing.T) {
	m := &mockBridge{}
	s := newMockServer(m)

	body := strings.NewReader(`{"text":"merhaba"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tts/synthesize", body)
	w := httptest.NewRecorder()
	s.handleTTSSynthesize(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("got status %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleTTSSynthesize_GetRejected(t *testing.T) {
	stub := &swarmStubBridge{}
	s := New(stub)
	s.fullBridge = stub

	req := httptest.NewRequest(http.MethodGet, "/api/tts/synthesize", nil)
	w := httptest.NewRecorder()
	s.handleTTSSynthesize(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("got status %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}
