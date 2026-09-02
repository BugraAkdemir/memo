package app

import (
	"context"
	"strings"
	"testing"

	"memo/internal/config"
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
