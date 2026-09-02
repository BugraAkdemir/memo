package app

import (
	"context"
	"strings"
	"testing"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/sessions"
	"memo/internal/taskloop"
)

// TestEmitChatBusyProbe_BusyChatEmitsWaitingActivity is the regression for a
// UX gap found while reviewing this branch, not from a live bug report: a
// worker turn that has to queue behind another turn in the same chat
// (chat_locks.go's per-chat lock, up to chatLockWaitMax = 5 minutes) did so
// silently — the task card's SilentSec climbed with no tool line and no
// "model generating" text (that label only covers silence *while running*),
// reading exactly like a stall for as long as the queue lasted.
func TestEmitChatBusyProbe_BusyChatEmitsWaitingActivity(t *testing.T) {
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	chatID := sm.NewChat()

	store, err := taskloop.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	tl, err := store.Create(chatID, "T", []string{"do it"})
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
		sessions:       sm,
		taskloopEngine: engine,
	}

	release, ok := a.lockChatStream(chatID)
	if !ok {
		t.Fatal("could not take the chat's stream lock")
	}
	defer release()

	a.emitChatBusyProbe(tl.ID, chatID)

	var found string
	for _, e := range events {
		if strings.HasPrefix(e, "taskloop:activity|") {
			found = e
		}
	}
	if found == "" {
		t.Fatalf("no taskloop:activity event fired for a busy chat; events = %v", events)
	}
	if !strings.Contains(found, "waiting") {
		t.Errorf("activity event = %q, want kind \"waiting\"", found)
	}
}

// TestEmitChatBusyProbe_FreeChatEmitsNothing: a free chat must not produce a
// misleading "waiting" line — the probe's own acquire+release must not leak
// the lock either (checked by re-acquiring it after the call).
func TestEmitChatBusyProbe_FreeChatEmitsNothing(t *testing.T) {
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	chatID := sm.NewChat()

	store, err := taskloop.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	tl, err := store.Create(chatID, "T", []string{"do it"})
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
		sessions:       sm,
		taskloopEngine: engine,
	}

	a.emitChatBusyProbe(tl.ID, chatID)

	for _, e := range events {
		if strings.Contains(e, "waiting") {
			t.Fatalf("a free chat produced a \"waiting\" line: %q", e)
		}
	}

	// The probe must have released what it acquired.
	release, ok := a.lockChatStream(chatID)
	if !ok {
		t.Fatal("emitChatBusyProbe leaked the chat lock on a free chat")
	}
	release()
}
