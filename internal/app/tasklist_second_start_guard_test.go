package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/sessions"
	"memo/internal/taskloop"
)

// TestStartTaskList_RejectsSecondListOnSameChat is the regression test for a
// P1 found in a 2026-09-23 reliability audit: every consumer of "which task
// is this chat's" assumes a chat has at most one bound running task —
// chatBoundTaskID (taskstatus_tool.go, backs get_task_status/pause_task/
// resume_task) picks the first RunningTasks() match for a chatID with no
// tie-break, and the in-chat TaskActivityBlock (frontend) keys its live
// state by chatId alone, not (chatId, listId). Before this fix, StartTaskList
// had no guard against a second list binding to the same chat — both lists'
// progress could blend into one card, and Pause/Resume could hit the wrong
// list. Uses a run worker that blocks until released so the first list stays
// "running" for the whole window this test needs, deterministically — no
// reliance on the fake worker happening to still be mid-item when the second
// start is attempted.
func TestStartTaskList_RejectsSecondListOnSameChat(t *testing.T) {
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("sessions.NewManager: %v", err)
	}
	store, err := taskloop.NewStore(filepath.Join(t.TempDir(), "tasklists"))
	if err != nil {
		t.Fatalf("taskloop.NewStore: %v", err)
	}

	release := make(chan struct{})
	started := make(chan struct{}, 1)
	eng := taskloop.NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			select {
			case started <- struct{}{}:
			default:
			}
			select {
			case <-release:
			case <-ctx.Done():
			}
			return "ok", nil
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(bool) {},
		func(string, string) {},
	)
	a := &App{
		cfg:            &config.AppConfig{},
		identity:       identity.New("Test", "Memo", "casual", "", false),
		sessions:       sm,
		taskloopStore:  store,
		taskloopEngine: eng,
	}
	t.Cleanup(func() {
		close(release)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		eng.Shutdown(ctx)
	})

	dir := t.TempDir()
	chat := sm.NewAgentChat(dir)

	listA, err := a.CreateTaskList(chat, "List A", []string{"item one"})
	if err != nil {
		t.Fatalf("CreateTaskList A: %v", err)
	}
	if err := a.StartTaskList(context.Background(), listA.ID); err != nil {
		t.Fatalf("StartTaskList A: %v", err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("list A's worker never started")
	}

	listB, err := a.CreateTaskList(chat, "List B", []string{"item two"})
	if err != nil {
		t.Fatalf("CreateTaskList B: %v", err)
	}
	err = a.StartTaskList(context.Background(), listB.ID)
	if err == nil {
		t.Fatal("expected StartTaskList to refuse a second list on the same chat while the first is running")
	}
	if !strings.Contains(err.Error(), "List A") {
		t.Errorf("error should name the already-running list, got: %v", err)
	}

	for _, r := range a.taskloopEngine.RunningTasks() {
		if r.ID == listB.ID {
			t.Errorf("list B should not be in RunningTasks(), found: %+v", r)
		}
	}
}

// TestStartTaskList_AllowsRestartingTheSameListItIsAlreadyRunning guards
// against the new check being too strict: re-starting the exact same
// listID (e.g. an idempotent resume path) must not be refused as "a second
// list on this chat" — the guard only excludes a DIFFERENT list bound to
// the same chat (r.ID != listID).
func TestStartTaskList_AllowsRestartingTheSameListItIsAlreadyRunning(t *testing.T) {
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("sessions.NewManager: %v", err)
	}
	store, err := taskloop.NewStore(filepath.Join(t.TempDir(), "tasklists"))
	if err != nil {
		t.Fatalf("taskloop.NewStore: %v", err)
	}

	release := make(chan struct{})
	started := make(chan struct{}, 1)
	eng := taskloop.NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			select {
			case started <- struct{}{}:
			default:
			}
			select {
			case <-release:
			case <-ctx.Done():
			}
			return "ok", nil
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(bool) {},
		func(string, string) {},
	)
	a := &App{
		cfg:            &config.AppConfig{},
		identity:       identity.New("Test", "Memo", "casual", "", false),
		sessions:       sm,
		taskloopStore:  store,
		taskloopEngine: eng,
	}
	t.Cleanup(func() {
		close(release)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		eng.Shutdown(ctx)
	})

	dir := t.TempDir()
	chat := sm.NewAgentChat(dir)

	list, err := a.CreateTaskList(chat, "Only List", []string{"item one"})
	if err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}
	if err := a.StartTaskList(context.Background(), list.ID); err != nil {
		t.Fatalf("StartTaskList (first): %v", err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("list's worker never started")
	}

	// The engine's own "already running" check (a separate, pre-existing
	// guard in taskloop.Engine.Start) may itself reject this — this test
	// only cares that our new same-chat guard specifically does not.
	if err := a.StartTaskList(context.Background(), list.ID); err != nil &&
		strings.Contains(err.Error(), "zaten çalışan") {
		t.Errorf("same-list restart must not be rejected by the new same-chat guard, got: %v", err)
	}
}
