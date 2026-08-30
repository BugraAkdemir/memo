package webserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memo/internal/config"
)

type planExecStubBridge struct {
	swarmStubBridge
	settings config.TaskLoopConfig
	approved string
	planMd   string
}

func (b *planExecStubBridge) GetTaskLoopSettings() config.TaskLoopConfig { return b.settings }
func (b *planExecStubBridge) UpdateTaskLoopSettings(c config.TaskLoopConfig) error {
	b.settings = c
	return nil
}
func (b *planExecStubBridge) ApproveTaskPlan(listID string) error { b.approved = listID; return nil }
func (b *planExecStubBridge) GetTaskPlanMd(listID string) (string, error) {
	if b.planMd == "" {
		return "", http.ErrNoLocation
	}
	return b.planMd, nil
}

func TestHandleTaskLoopSettings_GetAndPut(t *testing.T) {
	b := &planExecStubBridge{settings: config.TaskLoopConfig{StepGranularity: "hybrid", MaxParallelSteps: 3}}
	s := &Server{fullBridge: b}

	rr := httptest.NewRecorder()
	s.handleTaskLoopSettings(rr, httptest.NewRequest(http.MethodGet, "/api/taskloop/settings", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET code = %d", rr.Code)
	}
	var got config.TaskLoopConfig
	json.Unmarshal(rr.Body.Bytes(), &got)
	if got.StepGranularity != "hybrid" || got.MaxParallelSteps != 3 {
		t.Fatalf("GET body = %+v", got)
	}

	body := `{"step_granularity":"literal","auto_approve_plan":true,"coder_model":"local","max_parallel_steps":2}`
	rr = httptest.NewRecorder()
	s.handleTaskLoopSettings(rr, httptest.NewRequest(http.MethodPut, "/api/taskloop/settings", strings.NewReader(body)))
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT code = %d: %s", rr.Code, rr.Body)
	}
	if b.settings.StepGranularity != "literal" || !b.settings.AutoApprovePlan || b.settings.CoderModel != "local" {
		t.Fatalf("PUT did not persist: %+v", b.settings)
	}
}

func TestHandleTaskListByID_ApprovePlan(t *testing.T) {
	b := &planExecStubBridge{planMd: "# Plan\n"}
	s := &Server{fullBridge: b}

	rr := httptest.NewRecorder()
	s.handleTaskListByID(rr, httptest.NewRequest(http.MethodGet, "/api/tasklists/abc/plan", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "# Plan") {
		t.Fatalf("GET plan: code=%d body=%s", rr.Code, rr.Body)
	}

	rr = httptest.NewRecorder()
	s.handleTaskListByID(rr, httptest.NewRequest(http.MethodPost, "/api/tasklists/abc/approve-plan", nil))
	if rr.Code != http.StatusOK || b.approved != "abc" {
		t.Fatalf("approve-plan: code=%d approved=%q", rr.Code, b.approved)
	}
}
