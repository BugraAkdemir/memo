package webserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memo/internal/taskloop"
)

func TestHandleTasksRunning(t *testing.T) {
	bridge := &swarmStubBridge{
		listRunningTasks: func() []taskloop.RunningTaskInfo {
			return []taskloop.RunningTaskInfo{{ID: "L1", Title: "One", Phase: "executing", DoneCount: 1, ItemCount: 3}}
		},
	}
	s := &Server{fullBridge: bridge}

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/running", nil)
	w := httptest.NewRecorder()
	s.handleTasksRunning(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", w.Code, w.Body.String())
	}
	var got []taskloop.RunningTaskInfo
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].ID != "L1" || got[0].DoneCount != 1 {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestHandleTasksRunning_RejectsNonGet(t *testing.T) {
	s := &Server{fullBridge: &swarmStubBridge{}}
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/running", nil)
	w := httptest.NewRecorder()
	s.handleTasksRunning(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", w.Code)
	}
}

func TestHandleTaskControlByID_Inject(t *testing.T) {
	var gotList, gotText string
	bridge := &swarmStubBridge{
		injectTask: func(listID, text string) (string, error) {
			gotList, gotText = listID, text
			return "done: applied", nil
		},
	}
	s := &Server{fullBridge: bridge}

	req := httptest.NewRequest(http.MethodPost, "/api/tasks/L9/inject", strings.NewReader(`{"text":"change item 3"}`))
	w := httptest.NewRecorder()
	s.handleTaskControlByID(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", w.Code, w.Body.String())
	}
	if gotList != "L9" || gotText != "change item 3" {
		t.Fatalf("bridge got list=%q text=%q", gotList, gotText)
	}
	var body map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["reply"] != "done: applied" {
		t.Fatalf("reply = %q", body["reply"])
	}
}

func TestHandleTaskControlByID_InjectRequiresText(t *testing.T) {
	s := &Server{fullBridge: &swarmStubBridge{}}
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/L9/inject", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	s.handleTaskControlByID(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestHandleTaskControlByID_BadPath(t *testing.T) {
	s := &Server{fullBridge: &swarmStubBridge{}}
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/L9", nil)
	w := httptest.NewRecorder()
	s.handleTaskControlByID(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a path with no action", w.Code)
	}
}

func TestHandleTaskControlByID_UnknownAction(t *testing.T) {
	s := &Server{fullBridge: &swarmStubBridge{}}
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/L9/frobnicate", nil)
	w := httptest.NewRecorder()
	s.handleTaskControlByID(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
