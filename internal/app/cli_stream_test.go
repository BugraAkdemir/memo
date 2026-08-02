// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/provider"
	"memo/internal/sessions"
)

func newTestAppForCLI(t *testing.T) (*App, *sessions.Manager) {
	t.Helper()
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
		identity:       id,
		sessions:       sm,
		providerCfgMgr: provider.NewConfigManager(t.TempDir()+"/providers.json", nil),
	}
	return a, sm
}

func TestSendCLIMessageStream_UnknownChatReturnsError(t *testing.T) {
	a, _ := newTestAppForCLI(t)
	ch := a.SendCLIMessageStream(context.Background(), "does-not-exist", "hello")
	chunk, ok := <-ch
	if !ok || chunk.Error == "" || !chunk.Done {
		t.Fatalf("expected a Done error chunk, got ok=%v chunk=%+v", ok, chunk)
	}
}

func TestSendCLIMessageStream_NoCLIProviderSetReturnsError(t *testing.T) {
	a, sm := newTestAppForCLI(t)
	chat := sm.NewChat()

	ch := a.SendCLIMessageStream(context.Background(), chat, "hello")
	chunk, ok := <-ch
	if !ok || chunk.Error == "" || !chunk.Done {
		t.Fatalf("expected a Done error chunk, got ok=%v chunk=%+v", ok, chunk)
	}
}

func TestSendCLIMessageStream_ProviderNotConfiguredReturnsError(t *testing.T) {
	a, sm := newTestAppForCLI(t)
	chat := sm.NewChat()
	if err := sm.SetCLIProvider(chat, "claude-code-cli"); err != nil {
		t.Fatalf("SetCLIProvider: %v", err)
	}

	ch := a.SendCLIMessageStream(context.Background(), chat, "hello")
	chunk, ok := <-ch
	if !ok || chunk.Error == "" {
		t.Fatalf("expected an error chunk (no claude-code-cli provider configured), got ok=%v chunk=%+v", ok, chunk)
	}
}

// TestSendCLIMessageStream_DoesNotBlockOtherChats is the regression test for
// yapacam.md §2.5's core requirement: a CLI stream must use its own per-chat
// exclusivity, never the app-wide streamMu every other stream serializes
// through. It holds a.streamMu locked (simulating another chat's in-flight
// normal stream) for the whole call and asserts SendCLIMessageStream still
// completes instead of hanging behind it.
func TestSendCLIMessageStream_DoesNotBlockOtherChats(t *testing.T) {
	a, sm := newTestAppForCLI(t)
	chat := sm.NewChat()
	if err := sm.SetCLIProvider(chat, "claude-code-cli"); err != nil {
		t.Fatalf("SetCLIProvider: %v", err)
	}

	a.streamMu.Lock()
	defer a.streamMu.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		ch := a.SendCLIMessageStream(context.Background(), chat, "hello")
		for range ch {
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SendCLIMessageStream blocked behind a.streamMu — it must use its own, independent locking")
	}
}

func TestStartFinishCLIJob_PerChatExclusivity(t *testing.T) {
	a := &App{}
	chatA, chatB := "chat-a", "chat-b"

	if !a.startCLIJob(chatA, func() {}) {
		t.Fatal("first startCLIJob(chatA) should succeed")
	}
	if a.startCLIJob(chatA, func() {}) {
		t.Error("second concurrent startCLIJob(chatA) should be rejected")
	}
	if !a.startCLIJob(chatB, func() {}) {
		t.Error("startCLIJob(chatB) must not be blocked by chatA's in-flight job")
	}
	if !a.IsCLIJobRunning(chatA) || !a.IsCLIJobRunning(chatB) {
		t.Error("both chats should report running")
	}

	a.finishCLIJob(chatA)
	if a.IsCLIJobRunning(chatA) {
		t.Error("chatA should no longer be running after finishCLIJob")
	}
	if !a.IsCLIJobRunning(chatB) {
		t.Error("finishCLIJob(chatA) must not affect chatB")
	}

	if !a.startCLIJob(chatA, func() {}) {
		t.Error("startCLIJob(chatA) should succeed again after finishCLIJob")
	}
}

func TestCancelCLIJob(t *testing.T) {
	a := &App{}
	cancelled := false
	a.startCLIJob("chat-x", func() { cancelled = true })

	if !a.CancelCLIJob("chat-x") {
		t.Fatal("CancelCLIJob should report success for a running job")
	}
	if !cancelled {
		t.Error("CancelCLIJob did not invoke the registered cancel func")
	}

	// CancelCLIJob only signals — it's the stream goroutine's own
	// ctx-cancelled path that calls finishCLIJob (see SendCLIMessageStream),
	// simulated here directly.
	a.finishCLIJob("chat-x")
	if a.CancelCLIJob("chat-x") {
		t.Error("CancelCLIJob after finishCLIJob should report false, nothing running")
	}
	if a.CancelCLIJob("no-such-chat") {
		t.Error("CancelCLIJob on a chat with no job should report false")
	}
}
