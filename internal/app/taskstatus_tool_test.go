package app

import (
	"context"
	"strings"
	"testing"

	"memo/internal/config"
	"memo/internal/taskloop"
)

// TestTaskStatusForChat_NoRunningTask is the BUG-PLAN10 guard: with nothing
// running the tool must say so plainly, so the model reports "no task" instead
// of inventing a failure story from an empty Task.md.
func TestTaskStatusForChat_NoRunningTask(t *testing.T) {
	a, _ := newSelfDrivingTaskApp(t)

	got := a.TaskStatusForChat(context.Background())
	if !strings.Contains(strings.ToLower(got), "no self-driving task running") &&
		!strings.Contains(got, "çalışan bir otonom görev yok") {
		t.Fatalf("expected a clear 'nothing running' message, got %q", got)
	}
}

// TestTaskStatusForChat_NoEngine covers the not-initialised path.
func TestTaskStatusForChat_NoEngine(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	got := a.TaskStatusForChat(context.Background())
	if !strings.Contains(strings.ToLower(got), "not initialised") &&
		!strings.Contains(got, "başlatılmamış") {
		t.Fatalf("expected a 'not initialised' message, got %q", got)
	}
}

// TestFormatRunningTask_ReportsRealProgressNotFabrication is the positive-path
// BUG-PLAN10 regression that was missing: the two tests above only cover
// "nothing is running" — neither ever exercised the actual text the tool
// hands the model for a task that IS running, which is exactly the case the
// live bug happened in (7/13 steps genuinely done, model invented "the loop
// did nothing" instead). This locks the real numbers into the formatted
// string so a future regression here can't silently start showing 0/0 or
// dropping the step/item counts again.
func TestFormatRunningTask_ReportsRealProgressNotFabrication(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	r := taskloop.RunningTaskInfo{
		ID:            "L1",
		Title:         "blog sitesi",
		ChatID:        "c1",
		Phase:         "executing",
		DoneCount:     2,
		ItemCount:     6,
		CurrentItem:   "S8: style.css",
		ElapsedSec:    184,
		SubAgents:     []string{"coder"},
		Mode:          taskloop.ModePlanner,
		PlanSteps:     13,
		PlanStepsDone: 7,
	}
	got := formatRunningTask(a, r)

	for _, want := range []string{"blog sitesi", "executing", "7/13", "2/6", "S8: style.css", "coder", "184"} {
		if !strings.Contains(got, want) {
			t.Errorf("formatRunningTask output missing %q; got:\n%s", want, got)
		}
	}
	// The exact fabricated claims from the live bug report must never be
	// producible by this function — it only ever writes numbers it was
	// given, but assert it anyway as a tripwire against a future rewrite
	// reintroducing a hardcoded "nothing happened" fallback string.
	for _, mustNot := range []string{"hiçbir madde", "no provider configured", "not been created"} {
		if strings.Contains(got, mustNot) {
			t.Errorf("formatRunningTask output contains fabricated-looking text %q; got:\n%s", mustNot, got)
		}
	}
}

// TestFormatRunningTask_WorkerMode_OmitsStepCount covers the sibling branch:
// worker mode (no plan/steps concept) must not print a "step N/M" segment at
// all — PlanSteps is 0 there, and the "r.Mode == ModePlanner" guard is the
// only thing preventing a misleading "step 0/0".
func TestFormatRunningTask_WorkerMode_OmitsStepCount(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	r := taskloop.RunningTaskInfo{
		ID:        "L2",
		Title:     "simple list",
		Phase:     "executing",
		DoneCount: 1,
		ItemCount: 3,
		Mode:      "worker",
	}
	got := formatRunningTask(a, r)
	if strings.Contains(got, "step") || strings.Contains(got, "adım") {
		t.Errorf("worker-mode output should have no step-count segment at all; got:\n%s", got)
	}
	if !strings.Contains(got, "1/3") {
		t.Errorf("expected the item count 1/3, got:\n%s", got)
	}
}
