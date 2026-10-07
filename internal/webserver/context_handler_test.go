package webserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"memo/internal/models"
)

func TestHandleContext_ReturnsTheReportAsJSON(t *testing.T) {
	s := &Server{fullBridge: &swarmStubBridge{}}
	rec := httptest.NewRecorder()
	s.handleContext(rec, httptest.NewRequest(http.MethodGet, "/api/context?chat_id=c1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var rep models.ContextReport
	if err := json.Unmarshal(rec.Body.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Window != 1000 || rep.Used != 250 || len(rep.Categories) != 1 || rep.Categories[0].Key != "messages" {
		t.Errorf("report = %+v", rep)
	}
	// The client decodes these as lists: they must be [] in the JSON, never null.
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	if string(raw["limits"]) != "[]" {
		t.Errorf("limits = %s, want []", raw["limits"])
	}
}

func TestHandleContext_IsReadOnly(t *testing.T) {
	s := &Server{fullBridge: &swarmStubBridge{}}
	rec := httptest.NewRecorder()
	s.handleContext(rec, httptest.NewRequest(http.MethodPost, "/api/context", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST got %d, want 405", rec.Code)
	}
}
