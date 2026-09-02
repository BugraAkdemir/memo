package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/sessions"
	"memo/internal/taskloop"
)

func newSelfDrivingTaskApp(t *testing.T) (*App, *sessions.Manager) {
	t.Helper()
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("sessions.NewManager: %v", err)
	}
	store, err := taskloop.NewStore(filepath.Join(t.TempDir(), "tasklists"))
	if err != nil {
		t.Fatalf("taskloop.NewStore: %v", err)
	}
	eng := taskloop.NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) { return "ok", nil },
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
	return a, sm
}

func writeTaskMd(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "Task.md")
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStartSelfDrivingTaskFromChat_NoChatInContext(t *testing.T) {
	a, _ := newSelfDrivingTaskApp(t)
	dir := t.TempDir()
	md := writeTaskMd(t, dir, "- [ ] one\n")

	// Plain context — withCurrentChatID was never called.
	if _, err := a.StartSelfDrivingTaskFromChat(context.Background(), md, ""); err == nil {
		t.Fatal("expected an error when no chat id is on the context")
	}
}

func TestStartSelfDrivingTaskFromChat_RejectsNonAgentChat(t *testing.T) {
	a, sm := newSelfDrivingTaskApp(t)
	plain := sm.NewChat() // no project path -> not an agent chat
	dir := t.TempDir()
	md := writeTaskMd(t, dir, "- [ ] one\n")

	ctx := withCurrentChatID(context.Background(), plain)
	if _, err := a.StartSelfDrivingTaskFromChat(ctx, md, ""); err == nil {
		t.Fatal("expected an error starting a task from a non-agent chat")
	}
}

func TestStartSelfDrivingTaskFromChat_StartsOnAgentChat(t *testing.T) {
	a, sm := newSelfDrivingTaskApp(t)
	dir := t.TempDir()
	chat := sm.NewAgentChat(dir)
	md := writeTaskMd(t, dir, "# bildirim: önemli\n\n- [ ] first\n- [x] already\n- [ ] third\n")

	ctx := withCurrentChatID(context.Background(), chat)
	out, err := a.StartSelfDrivingTaskFromChat(ctx, md, "My Site")
	if err != nil {
		t.Fatalf("StartSelfDrivingTaskFromChat: %v", err)
	}
	if !strings.Contains(out, "My Site") {
		t.Fatalf("reply %q does not mention the task title", out)
	}

	lists := a.taskloopStore.List()
	if len(lists) != 1 {
		t.Fatalf("want 1 task list persisted, got %d", len(lists))
	}
	if lists[0].ItemCount != 3 {
		t.Fatalf("want 3 items, got %d", lists[0].ItemCount)
	}
	tl, err := a.taskloopStore.Get(lists[0].ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tl.ChatID != chat {
		t.Fatalf("task list bound to chat %q, want %q", tl.ChatID, chat)
	}
	if tl.TaskMdPath != md {
		t.Fatalf("TaskMdPath = %q, want %q", tl.TaskMdPath, md)
	}
}
