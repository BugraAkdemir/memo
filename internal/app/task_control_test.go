package app

import (
	"context"
	"path/filepath"
	"testing"

	"memo/internal/config"
	"memo/internal/taskloop"
)

func newTaskControlApp(t *testing.T) *App {
	t.Helper()
	store, err := taskloop.NewStore(filepath.Join(t.TempDir(), "tl"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	eng := taskloop.NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) { return "ok", nil },
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(bool) {},
		func(string, string) {},
	)
	return &App{
		cfg:            &config.AppConfig{},
		taskloopStore:  store,
		taskloopEngine: eng,
	}
}

func TestHandleTaskControl_UnfocusedPassThrough(t *testing.T) {
	a := newTaskControlApp(t)
	reply, handled := a.handleTaskControl(context.Background(), taskSurfaceTelegram, "merhaba nasılsın")
	if handled {
		t.Fatalf("plain chat with no focused task should pass through, got reply=%q", reply)
	}
	// A bare "dur" is also pass-through when nothing is focused.
	if _, h := a.handleTaskControl(context.Background(), taskSurfaceTelegram, "dur"); h {
		t.Fatal("'dur' with no focus should pass through")
	}
}

func TestHandleTaskControl_TaskListAlwaysHandled(t *testing.T) {
	a := newTaskControlApp(t)
	reply, handled := a.handleTaskControl(context.Background(), taskSurfaceWhatsApp, "task_list")
	if !handled || reply == "" {
		t.Fatalf("task_list should always be handled with a reply; handled=%v reply=%q", handled, reply)
	}
}

func TestHandleTaskControl_FocusThenExit(t *testing.T) {
	a := newTaskControlApp(t)
	tl, _ := a.taskloopStore.Create("chat1", "My List", []string{"a", "b"})

	reply, handled := a.handleTaskControl(context.Background(), taskSurfaceTelegram, "task_change "+tl.ID)
	if !handled {
		t.Fatal("task_change not handled")
	}
	if a.taskFocus.get(taskSurfaceTelegram) != tl.ID {
		t.Fatalf("focus = %q, want %q (reply was %q)", a.taskFocus.get(taskSurfaceTelegram), tl.ID, reply)
	}

	// Now a plain message is handled (would be injected) rather than passed through.
	if _, h := a.handleTaskControl(context.Background(), taskSurfaceTelegram, "durum"); !h {
		t.Fatal("'durum' while focused should be handled")
	}

	// task_exit clears focus.
	if _, h := a.handleTaskControl(context.Background(), taskSurfaceTelegram, "task_exit"); !h {
		t.Fatal("task_exit not handled")
	}
	if a.taskFocus.get(taskSurfaceTelegram) != "" {
		t.Fatal("focus not cleared by task_exit")
	}
	// WhatsApp focus is independent of Telegram focus.
	if a.taskFocus.get(taskSurfaceWhatsApp) != "" {
		t.Fatal("WhatsApp focus should be unaffected")
	}
}

func TestHandleTaskControl_BadRefRejected(t *testing.T) {
	a := newTaskControlApp(t)
	reply, handled := a.handleTaskControl(context.Background(), taskSurfaceTelegram, "task_change nope-not-real")
	if !handled || reply == "" {
		t.Fatal("task_change with a bad ref should be handled with an error message")
	}
	if a.taskFocus.get(taskSurfaceTelegram) != "" {
		t.Fatal("bad ref must not set focus")
	}
}
