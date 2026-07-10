package webserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleClientRegister(t *testing.T) {
	m := &mockBridge{registerClient: func() string { return "client-abc" }}
	s := newMockServer(m)

	req := httptest.NewRequest(http.MethodPost, "/api/clients/register", nil)
	w := httptest.NewRecorder()
	s.handleClientRegister(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp struct {
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ClientID != "client-abc" {
		t.Errorf("client_id = %q, want %q", resp.ClientID, "client-abc")
	}
}

func TestHandleClientRegister_GETRejected(t *testing.T) {
	s := newMockServer(&mockBridge{})
	req := httptest.NewRequest(http.MethodGet, "/api/clients/register", nil)
	w := httptest.NewRecorder()
	s.handleClientRegister(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleClientHeartbeat(t *testing.T) {
	var gotID string
	m := &mockBridge{heartbeatClient: func(id string) error { gotID = id; return nil }}
	s := newMockServer(m)

	req := httptest.NewRequest(http.MethodPost, "/api/clients/heartbeat", strings.NewReader(`{"client_id":"client-abc"}`))
	w := httptest.NewRecorder()
	s.handleClientHeartbeat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if gotID != "client-abc" {
		t.Errorf("heartbeat client_id = %q, want %q", gotID, "client-abc")
	}
}

func TestHandleClientHeartbeat_UnknownIDReturns404(t *testing.T) {
	m := &mockBridge{heartbeatClient: func(id string) error { return errors.New("unknown client") }}
	s := newMockServer(m)

	req := httptest.NewRequest(http.MethodPost, "/api/clients/heartbeat", strings.NewReader(`{"client_id":"stale"}`))
	w := httptest.NewRecorder()
	s.handleClientHeartbeat(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandleClientHeartbeat_BadJSON(t *testing.T) {
	s := newMockServer(&mockBridge{})
	req := httptest.NewRequest(http.MethodPost, "/api/clients/heartbeat", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	s.handleClientHeartbeat(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleClientUnregister(t *testing.T) {
	var gotID string
	m := &mockBridge{unregisterClient: func(id string) { gotID = id }}
	s := newMockServer(m)

	req := httptest.NewRequest(http.MethodPost, "/api/clients/unregister", strings.NewReader(`{"client_id":"client-abc"}`))
	w := httptest.NewRecorder()
	s.handleClientUnregister(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if gotID != "client-abc" {
		t.Errorf("unregister client_id = %q, want %q", gotID, "client-abc")
	}
}
