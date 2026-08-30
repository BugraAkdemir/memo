package app

import (
	"os"
	"path/filepath"
	"testing"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/sessions"
	"memo/internal/taskloop"
)

func newTestAppForTaskMd(t *testing.T) (*App, *sessions.Manager) {
	t.Helper()
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("sessions.NewManager: %v", err)
	}
	store, err := taskloop.NewStore(filepath.Join(t.TempDir(), "tasklists"))
	if err != nil {
		t.Fatalf("taskloop.NewStore: %v", err)
	}
	a := &App{
		cfg:           &config.AppConfig{},
		identity:      identity.New("Test", "Memo", "casual", "", false),
		sessions:      sm,
		taskloopStore: store,
	}
	return a, sm
}

func TestCreateTaskListFromTaskMd_SeedsItemsAndMeta(t *testing.T) {
	a, sm := newTestAppForTaskMd(t)

	dir := t.TempDir()
	chat := sm.NewAgentChat(dir)

	taskMd := filepath.Join(dir, "Task.md")
	if err := os.WriteFile(taskMd, []byte("# bildirim: her-şey\n\n- [ ] first\n- [x] already\n- [ ] third\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tl, err := a.CreateTaskListFromTaskMd(chat, "", taskMd)
	if err != nil {
		t.Fatalf("CreateTaskListFromTaskMd: %v", err)
	}
	if tl.NotifyLevel != taskloop.NotifyEverything {
		t.Fatalf("NotifyLevel = %q, want %q", tl.NotifyLevel, taskloop.NotifyEverything)
	}
	if tl.TaskMdPath != taskMd {
		t.Fatalf("TaskMdPath = %q, want %q", tl.TaskMdPath, taskMd)
	}
	if len(tl.Items) != 3 {
		t.Fatalf("Items = %d, want 3", len(tl.Items))
	}
	if tl.Items[0].Status != "pending" || tl.Items[1].Status != "done" || tl.Items[2].Status != "pending" {
		t.Fatalf("item statuses = %q/%q/%q, want pending/done/pending",
			tl.Items[0].Status, tl.Items[1].Status, tl.Items[2].Status)
	}
	if tl.Title != "Task.md" {
		t.Fatalf("Title = %q, want the file base name", tl.Title)
	}
}

func TestCreateTaskListFromTaskMd_ModeHeaderSelectsPlanner(t *testing.T) {
	a, sm := newTestAppForTaskMd(t)
	dir := t.TempDir()
	chat := sm.NewAgentChat(dir)
	taskMd := filepath.Join(dir, "Task.md")
	if err := os.WriteFile(taskMd, []byte("# mod: planlayıcı\n\n- [ ] one\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tl, err := a.CreateTaskListFromTaskMd(chat, "", taskMd)
	if err != nil {
		t.Fatalf("CreateTaskListFromTaskMd: %v", err)
	}
	if tl.Mode != taskloop.ModePlanner {
		t.Fatalf("Mode = %q, want %q", tl.Mode, taskloop.ModePlanner)
	}
}

func TestCreateTaskListFromTaskMd_NoCheckboxesErrors(t *testing.T) {
	a, sm := newTestAppForTaskMd(t)
	dir := t.TempDir()
	chat := sm.NewAgentChat(dir)

	taskMd := filepath.Join(dir, "Task.md")
	if err := os.WriteFile(taskMd, []byte("just prose, nothing actionable\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateTaskListFromTaskMd(chat, "", taskMd); err == nil {
		t.Fatal("expected an error for a Task.md with no checkbox items")
	}
}

func TestCreateTaskListFromTaskMd_RejectsNonAgentChat(t *testing.T) {
	a, sm := newTestAppForTaskMd(t)
	plain := sm.NewChat() // no project path -> not an agent chat

	dir := t.TempDir()
	taskMd := filepath.Join(dir, "Task.md")
	if err := os.WriteFile(taskMd, []byte("- [ ] x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateTaskListFromTaskMd(plain, "", taskMd); err == nil {
		t.Fatal("expected an error binding a task list to a non-agent chat")
	}
}
