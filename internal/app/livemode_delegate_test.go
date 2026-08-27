package app

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"memo/internal/agent"
	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/sessions"
)

func newTestAppForLiveModeDelegate(t *testing.T) *App {
	t.Helper()
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("sessions.NewManager: %v", err)
	}
	return &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: false},
			Llama:  config.LlamaConfig{CtxSize: 4096},
		},
		identity: identity.New("Test", "Memo", "casual", "", false),
		sessions: sm,
	}
}

func TestStartFinishLiveJob_PerChatExclusivity(t *testing.T) {
	a := &App{}
	chatA, chatB := "chat-a", "chat-b"

	if !a.startLiveJob(chatA, func() {}) {
		t.Fatal("first startLiveJob(chatA) should succeed")
	}
	if a.startLiveJob(chatA, func() {}) {
		t.Error("second concurrent startLiveJob(chatA) should be rejected")
	}
	if !a.startLiveJob(chatB, func() {}) {
		t.Error("startLiveJob(chatB) must not be blocked by chatA's in-flight job")
	}

	a.finishLiveJob(chatA)
	if !a.startLiveJob(chatA, func() {}) {
		t.Error("startLiveJob(chatA) should succeed again after finishLiveJob")
	}
}

func TestGetOrCreateLiveModeChat_CreatesOnceAndReuses(t *testing.T) {
	a := newTestAppForLiveModeDelegate(t)

	id1, err := a.getOrCreateLiveModeChat()
	if err != nil {
		t.Fatalf("getOrCreateLiveModeChat: %v", err)
	}
	if id1 == "" {
		t.Fatal("expected a non-empty chat id")
	}

	id2, err := a.getOrCreateLiveModeChat()
	if err != nil {
		t.Fatalf("getOrCreateLiveModeChat (second call): %v", err)
	}
	if id1 != id2 {
		t.Errorf("expected the same chat id reused, got %q then %q", id1, id2)
	}
}

func TestGetOrCreateLiveModeChat_NoSessionsInitialized(t *testing.T) {
	a := &App{}
	if _, err := a.getOrCreateLiveModeChat(); err == nil {
		t.Fatal("expected an error when sessions aren't initialized")
	}
}

// TestGetOrCreateLiveModeChat_RootsAtHome is the regression test for
// hands-free "list the files on my Desktop" / "make a note on my Desktop"
// failing: the Live Mode background chat used to have no ProjectPath, so
// the delegated (or standalone) agent ran from the backend's own cwd and
// couldn't reach the user's Desktop without a mid-conversation
// change_directory. It now defaults to the user's home directory.
func TestGetOrCreateLiveModeChat_RootsAtHome(t *testing.T) {
	a := newTestAppForLiveModeDelegate(t)
	id, err := a.getOrCreateLiveModeChat()
	if err != nil {
		t.Fatalf("getOrCreateLiveModeChat: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir on this platform: %v", err)
	}
	if got := a.getSessionManager().GetProjectPath(id); got != home {
		t.Errorf("Live Mode chat ProjectPath = %q, want home %q", got, home)
	}
}

// TestSendLiveDelegatedMessageStream_DoesNotBlockOnGlobalStreamMu is the
// Live Mode counterpart to cli_stream_test.go's
// TestSendCLIMessageStream_DoesNotBlockOtherChats — holds a.streamMu
// locked (simulating an in-flight interactive chat stream) and asserts a
// delegated task still completes instead of hanging behind it, proving
// SendLiveDelegatedMessageStream really does route through
// sendMessageStreamCore directly rather than sendMessageStreamInnerTo.
func TestSendLiveDelegatedMessageStream_DoesNotBlockOnGlobalStreamMu(t *testing.T) {
	a := newTestAppForLiveModeDelegate(t)

	a.streamMu.Lock()
	defer a.streamMu.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		ch := a.SendLiveDelegatedMessageStream(context.Background(), "fix the bug")
		for range ch {
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SendLiveDelegatedMessageStream blocked behind a.streamMu — it must use its own, independent locking")
	}
}

// TestSendLiveDelegatedMessageStream_RejectsConcurrentDelegation confirms
// a second delegation call while one is already in flight (on the same
// dedicated background chat — there is only ever one) gets the
// "already running" error immediately rather than queuing or racing.
func TestSendLiveDelegatedMessageStream_RejectsConcurrentDelegation(t *testing.T) {
	a := newTestAppForLiveModeDelegate(t)

	chatID, err := a.getOrCreateLiveModeChat()
	if err != nil {
		t.Fatalf("getOrCreateLiveModeChat: %v", err)
	}
	// Simulate a task already running on the one Live Mode chat.
	if !a.startLiveJob(chatID, func() {}) {
		t.Fatal("failed to seed an in-flight job")
	}
	defer a.finishLiveJob(chatID)

	ch := a.SendLiveDelegatedMessageStream(context.Background(), "another task")
	chunk, ok := <-ch
	if !ok || chunk.Error == "" || !chunk.Done {
		t.Fatalf("expected an immediate Done error chunk, got ok=%v chunk=%+v", ok, chunk)
	}
}

// TestSendLiveDelegatedMessageStream_SessionCancelStopsTheJob confirms the
// job is tied to the passed-in sessionCtx (the live realtime session's own
// context), not a.lifecycleCtx — cancelling it must end the stream rather
// than leaving it running past the session's own lifetime.
func TestSendLiveDelegatedMessageStream_SessionCancelStopsTheJob(t *testing.T) {
	a := newTestAppForLiveModeDelegate(t)
	sessionCtx, cancel := context.WithCancel(context.Background())

	ch := a.SendLiveDelegatedMessageStream(sessionCtx, "a task")
	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			// Draining further chunks (e.g. a terminal error/Done chunk from
			// the cancelled context) is fine; what matters is the channel
			// closes rather than hanging forever.
			for range ch {
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("channel never produced a value after sessionCtx was cancelled")
	}

	chatID, _ := a.getOrCreateLiveModeChat()
	if a.startLiveJob(chatID, func() {}) {
		a.finishLiveJob(chatID) // clean up what we just registered
	} else {
		t.Error("expected the job to have finished (and been unregistered) after sessionCtx cancellation")
	}
}

func TestDrainLiveDelegatedReply_AutoApproveMatchesSelfChatBehavior(t *testing.T) {
	a := &App{}
	ev := agent.AgentEvent{Type: agent.EventPermissionRequest, RequestID: "req-1", ToolName: "run_command"}
	raw, _ := json.Marshal(ev)

	ch := make(chan api.StreamChunk, 4)
	ch <- api.StreamChunk{Content: string(raw), FinishReason: "agent_event"}
	ch <- api.StreamChunk{Content: "done"}
	close(ch)

	// nil buildQuestion/sendQuestion/awaitAnswer would panic if actually
	// called — reaching "done" proves the autoApprove early-return path
	// (inherited from drainSelfChatReply) was taken.
	got := a.drainLiveDelegatedReply(ch, true, nil, nil, nil)
	if got != "done" {
		t.Errorf("drainLiveDelegatedReply() = %q, want %q", got, "done")
	}
}

func TestDrainLiveDelegatedReply_AccumulatesTextAndStopsOnError(t *testing.T) {
	a := &App{}
	ch := make(chan api.StreamChunk, 4)
	ch <- api.StreamChunk{Content: "partial"}
	ch <- api.StreamChunk{Error: "boom", Done: true}
	close(ch)

	got := a.drainLiveDelegatedReply(ch, false, nil, nil, nil)
	if got != "boom" {
		t.Errorf("drainLiveDelegatedReply() = %q, want %q", got, "boom")
	}
}
