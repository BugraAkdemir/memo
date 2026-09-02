package app

import (
	"context"
	"strings"
	"testing"

	"memo/internal/agent"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/taskloop"
)

// TestEmitToolActivity_ErrorLineDoesNotReadAsSuccess is the regression for a
// bug found live (2026-09-01): a Self-Driving task's coder turn called
// create_task_md with a relative path in a chat that has no project folder;
// the call genuinely errored ("relative path but this chat has no project
// folder"), but the task card's activity line still read "⚠ Task.md
// oluşturdu" ("Created Task.md") — toolVerbs' completed-action phrasing,
// reused unmodified, with only the ⚠ glyph (easy to miss) as the sole signal
// anything had gone wrong. A user watching the card would reasonably believe
// the file was created when it was not.
func TestEmitToolActivity_ErrorLineDoesNotReadAsSuccess(t *testing.T) {
	store, err := taskloop.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	tl, err := store.Create("chat-1", "T", []string{"do it"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var events []string
	engine := taskloop.NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) { return "", nil },
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(bool) {},
		func(name, data string) { events = append(events, name+"|"+data) },
	)

	a := &App{
		cfg:            &config.AppConfig{Memory: config.MemoryConfig{MemoryEnabled: false}},
		identity:       identity.New("Test", "Memo", "casual", "", false),
		taskloopEngine: engine,
	}

	a.emitToolActivity(tl.ID, "", agent.AgentEvent{
		Type:     agent.EventToolError,
		ToolName: "create_task_md",
		Error:    "Task.md oluşturulamadı: göreli yol verildi ama bu sohbetin bir proje klasörü yok",
	})

	var found string
	for _, e := range events {
		if strings.HasPrefix(e, "taskloop:activity|") {
			found = e
		}
	}
	if found == "" {
		t.Fatalf("no taskloop:activity event fired; events = %v", events)
	}
	// The old behavior: "⚠ Task.md oluşturdu" — a bare success phrase with a
	// glyph prefix. The line must now say the call failed in words, and
	// carry the real reason, not the create-succeeded verb standing alone.
	// App.t() picks TR or EN by locale (this branch defaults to English —
	// AGENTS.md's l10n note), so check for either rather than hardcoding one.
	if !strings.Contains(found, a.t("Başarısız", "Failed")) {
		t.Errorf("activity line = %q, want it to say the call failed (%q)", found, a.t("Başarısız", "Failed"))
	}
	if !strings.Contains(found, "proje klasörü yok") {
		t.Errorf("activity line = %q, want the real error reason included", found)
	}
}
