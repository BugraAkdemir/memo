package webserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"memo/internal/models"
)

func TestHandleMascotActivity(t *testing.T) {
	bridge := &swarmStubBridge{
		getActivityStatus: func() models.ActivityStatus {
			return models.ActivityStatus{State: models.ActivityWriting, ToolName: "edit_file"}
		},
	}
	s := &Server{fullBridge: bridge}

	req := httptest.NewRequest(http.MethodGet, "/api/mascot/activity", nil)
	w := httptest.NewRecorder()
	s.handleMascotActivity(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", w.Code, w.Body.String())
	}
	var got models.ActivityStatus
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.State != models.ActivityWriting || got.ToolName != "edit_file" {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestHandleMascotActivity_RejectsNonGet(t *testing.T) {
	s := &Server{fullBridge: &swarmStubBridge{}}
	req := httptest.NewRequest(http.MethodPost, "/api/mascot/activity", nil)
	w := httptest.NewRecorder()
	s.handleMascotActivity(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", w.Code)
	}
}
