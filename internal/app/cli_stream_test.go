// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/provider"
	"memo/internal/sessions"
)

// writeFakeClaudeScript writes a stand-in executable (not the real `claude`
// binary — these tests must run without it installed) that sleeps for
// delay then prints one valid stream-json "result" line, so
// provider.NewProvider(ProviderClaudeCodeCLI) with BaseURL pointed at it
// behaves like a real, slow CLI call for exactly long enough to test
// context-cancellation behavior.
func writeFakeClaudeScript(t *testing.T, delay string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-claude.sh")
	script := "#!/bin/sh\nsleep " + delay + "\necho '{\"type\":\"result\",\"session_id\":\"sess-1\",\"is_error\":false,\"result\":\"ok\"}'\n"
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake claude script: %v", err)
	}
	return path
}

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

// TestSendCLIMessageStream_SurvivesRequestContextCancellation is the
// regression test for SendCLIMessageStream's doc comment: the job must not
// die just because the HTTP request that started it disconnects (user
// switched chats, closed the app window). It passes an ALREADY-cancelled
// ctx and still expects the underlying (fake) CLI process to run to
// completion successfully.
func TestSendCLIMessageStream_SurvivesRequestContextCancellation(t *testing.T) {
	a, sm := newTestAppForCLI(t)
	a.lifecycleCtx, a.lifecycleCancel = context.WithCancel(context.Background())
	t.Cleanup(a.lifecycleCancel)

	chat := sm.NewChat()
	if err := sm.SetCLIProvider(chat, "claude-code-cli"); err != nil {
		t.Fatalf("SetCLIProvider: %v", err)
	}
	a.providerCfgMgr.Set(provider.ProviderConfig{
		Type: provider.ProviderClaudeCodeCLI, Name: "claude-code-cli",
		Model: "x", BaseURL: writeFakeClaudeScript(t, "0.2"), Enabled: true,
	})

	reqCtx, reqCancel := context.WithCancel(context.Background())
	reqCancel() // simulate the request already being gone before the call

	var gotDone bool
	var gotError string
	for chunk := range a.SendCLIMessageStream(reqCtx, chat, "hello") {
		if chunk.Done {
			gotDone = true
			gotError = chunk.Error
		}
	}
	if !gotDone {
		t.Fatal("expected the job to run to completion despite a pre-cancelled request ctx")
	}
	if gotError != "" {
		t.Errorf("unexpected error, job should have completed cleanly: %s", gotError)
	}
}

// TestSendCLIMessageStream_KilledByLifecycleCancel is
// TestSendCLIMessageStream_SurvivesRequestContextCancellation's
// counterpart: an actual backend shutdown (a.lifecycleCancel) must still
// stop the job. Uses a longer-sleeping fake script and cancels
// lifecycleCtx shortly after starting, asserting the stream ends (with an
// error, since the process was killed mid-flight) well before the script's
// own sleep would have elapsed on its own.
func TestSendCLIMessageStream_KilledByLifecycleCancel(t *testing.T) {
	a, sm := newTestAppForCLI(t)
	a.lifecycleCtx, a.lifecycleCancel = context.WithCancel(context.Background())

	chat := sm.NewChat()
	if err := sm.SetCLIProvider(chat, "claude-code-cli"); err != nil {
		t.Fatalf("SetCLIProvider: %v", err)
	}
	a.providerCfgMgr.Set(provider.ProviderConfig{
		Type: provider.ProviderClaudeCodeCLI, Name: "claude-code-cli",
		Model: "x", BaseURL: writeFakeClaudeScript(t, "5"), Enabled: true,
	})

	ch := a.SendCLIMessageStream(context.Background(), chat, "hello")

	go func() {
		time.Sleep(200 * time.Millisecond)
		a.lifecycleCancel()
	}()

	start := time.Now()
	var gotDoneWithError bool
	for chunk := range ch {
		if chunk.Done && chunk.Error != "" {
			gotDoneWithError = true
		}
	}
	elapsed := time.Since(start)

	if !gotDoneWithError {
		t.Fatal("expected a terminal error chunk once lifecycleCancel killed the subprocess")
	}
	if elapsed > 4*time.Second {
		t.Errorf("stream took %v to end — lifecycleCancel should have killed the subprocess almost immediately, not let its 5s sleep run out", elapsed)
	}
	if a.IsCLIJobRunning(chat) {
		t.Error("job should be cleared from the registry once the stream ends")
	}
}

// TestSendCLIMessageStream_PersistsUserMessage is the regression test for a
// real reported bug: the user's own message was never saved to session
// history (only the assistant reply was), so leaving a CLI chat and
// reopening it — which re-fetches from disk — showed the reply with no user
// message above it, looking like sent messages were silently deleted.
func TestSendCLIMessageStream_PersistsUserMessage(t *testing.T) {
	a, sm := newTestAppForCLI(t)
	a.lifecycleCtx, a.lifecycleCancel = context.WithCancel(context.Background())
	t.Cleanup(a.lifecycleCancel)

	chat := sm.NewChat()
	if err := sm.SetCLIProvider(chat, "claude-code-cli"); err != nil {
		t.Fatalf("SetCLIProvider: %v", err)
	}
	a.providerCfgMgr.Set(provider.ProviderConfig{
		Type: provider.ProviderClaudeCodeCLI, Name: "claude-code-cli",
		Model: "x", BaseURL: writeFakeClaudeScript(t, "0"), Enabled: true,
	})

	for range a.SendCLIMessageStream(context.Background(), chat, "merhaba") {
	}

	msgs := sm.GetActiveMessagesForSession(chat)
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2 (user + assistant): %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Content != "merhaba" {
		t.Errorf("msgs[0] = %+v, want role=user content=merhaba", msgs[0])
	}
	if msgs[1].Role != "assistant" {
		t.Errorf("msgs[1].Role = %q, want assistant", msgs[1].Role)
	}
}

// TestSendCLIMessageStream_UserMessagePersistedEvenOnCLIError covers the
// case where the CLI process itself fails — the user's message must still
// be saved (it was actually sent), even though the reply is an error.
func TestSendCLIMessageStream_UserMessagePersistedEvenOnCLIError(t *testing.T) {
	a, sm := newTestAppForCLI(t)
	a.lifecycleCtx, a.lifecycleCancel = context.WithCancel(context.Background())
	t.Cleanup(a.lifecycleCancel)

	chat := sm.NewChat()
	if err := sm.SetCLIProvider(chat, "claude-code-cli"); err != nil {
		t.Fatalf("SetCLIProvider: %v", err)
	}
	scriptPath := writeFakeClaudeScript(t, "0")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
		t.Fatalf("overwrite script: %v", err)
	}
	a.providerCfgMgr.Set(provider.ProviderConfig{
		Type: provider.ProviderClaudeCodeCLI, Name: "claude-code-cli",
		Model: "x", BaseURL: scriptPath, Enabled: true,
	})

	for range a.SendCLIMessageStream(context.Background(), chat, "merhaba") {
	}

	msgs := sm.GetActiveMessagesForSession(chat)
	if len(msgs) < 1 || msgs[0].Role != "user" || msgs[0].Content != "merhaba" {
		t.Fatalf("user message not persisted: %+v", msgs)
	}
}

// TestSendCLIMessageStream_PassesChatCLIModelToProvider is the regression
// test for the top-bar model picker's whole point: a chat's CLIModel
// override (sessions.Manager.SetCLIModel) must actually reach the CLI
// subprocess as --model, not just sit in the session unused.
func TestSendCLIMessageStream_PassesChatCLIModelToProvider(t *testing.T) {
	a, sm := newTestAppForCLI(t)
	a.lifecycleCtx, a.lifecycleCancel = context.WithCancel(context.Background())
	t.Cleanup(a.lifecycleCancel)

	chat := sm.NewChat()
	if err := sm.SetCLIProvider(chat, "claude-code-cli"); err != nil {
		t.Fatalf("SetCLIProvider: %v", err)
	}
	if err := sm.SetCLIModel(chat, "opus"); err != nil {
		t.Fatalf("SetCLIModel: %v", err)
	}

	argsFile := filepath.Join(t.TempDir(), "args.txt")
	scriptPath := writeFakeClaudeScript(t, "0")
	script := "#!/bin/sh\necho \"$@\" > " + argsFile + "\n" +
		"echo '{\"type\":\"result\",\"session_id\":\"sess-1\",\"is_error\":false,\"result\":\"ok\"}'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("overwrite script: %v", err)
	}
	a.providerCfgMgr.Set(provider.ProviderConfig{
		Type: provider.ProviderClaudeCodeCLI, Name: "claude-code-cli",
		Model: "x", BaseURL: scriptPath, Enabled: true,
	})

	for range a.SendCLIMessageStream(context.Background(), chat, "merhaba") {
	}

	got, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("reading captured args: %v", err)
	}
	if !strings.Contains(string(got), "--model opus") {
		t.Errorf("subprocess args = %q, missing --model opus", got)
	}
}
