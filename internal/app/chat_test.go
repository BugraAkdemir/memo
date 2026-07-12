package app

import (
	"context"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/sessions"
)

// TestSendMessageStreamTo_UnknownChatID_ReturnsError verifies SendMessageStreamTo
// rejects a chat ID that doesn't exist instead of silently operating on
// whatever chat happens to be globally active — the whole point of taking an
// explicit chatID (docs/plans/PLAN_chatid_refactor.md Faz 3).
func TestSendMessageStreamTo_UnknownChatID_ReturnsError(t *testing.T) {
	id := identity.New("Test", "Memo", "casual", "", false)
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: false},
			Llama:  config.LlamaConfig{CtxSize: 4096},
		},
		identity: id,
		sessions: sm,
	}

	ch := a.SendMessageStreamTo(context.Background(), "does-not-exist", "hello")
	chunk, ok := <-ch
	if !ok {
		t.Fatal("expected an error chunk, channel closed with nothing")
	}
	if chunk.Error == "" || !chunk.Done {
		t.Fatalf("expected a Done error chunk for an unknown chat ID, got %+v", chunk)
	}
	if _, stillOpen := <-ch; stillOpen {
		t.Fatal("expected channel to be closed after the single error chunk")
	}
}

// TestSendMessageStreamTo_TargetsGivenChatID_NotGloballyActiveChat is the
// regression test for BUG_REPORT.md's TD-1 / PLAN_chatid_refactor.md Faz 3:
// the task loop used to SwitchChat(chatID) before every call so
// SendMessageStream would act on the right chat — clobbering whatever chat
// the user had open in the GUI for the call's duration. SendMessageStreamTo
// must write the user's message into chatA's own history without touching
// which chat is globally active, even while chatB is the one currently
// active.
func TestSendMessageStreamTo_TargetsGivenChatID_NotGloballyActiveChat(t *testing.T) {
	id := identity.New("Test", "Memo", "casual", "", false)
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	chatA := sm.NewAgentChat(t.TempDir())
	chatB := sm.NewChat()
	if err := sm.SwitchChat(chatB); err != nil {
		t.Fatalf("SwitchChat(chatB) error = %v", err)
	}

	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: false},
			Llama:  config.LlamaConfig{CtxSize: 4096},
		},
		identity: id,
		sessions: sm,
	}

	ch := a.SendMessageStreamTo(context.Background(), chatA, "task loop message")

	// Drain fully (no working client/provider is configured, so this will
	// resolve to an error chunk quickly) so streamMu is released and the
	// background goroutine doesn't leak past the test.
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SendMessageStreamTo did not finish streaming in time")
	}

	if got := sm.GetActiveID(); got != chatB {
		t.Fatalf("GetActiveID() = %q, want %q — SendMessageStreamTo must not change the globally active chat", got, chatB)
	}

	historyA := sm.GetHistoryForAPIForSession(chatA, 100)
	var foundInA bool
	for _, m := range historyA {
		if m["content"] == "task loop message" {
			foundInA = true
		}
	}
	if !foundInA {
		t.Fatal("chat A's history does not contain the message sent via SendMessageStreamTo(chatA, ...)")
	}

	historyB := sm.GetHistoryForAPIForSession(chatB, 100)
	for _, m := range historyB {
		if m["content"] == "task loop message" {
			t.Fatal("message sent via SendMessageStreamTo(chatA, ...) leaked into chat B's history")
		}
	}
}
