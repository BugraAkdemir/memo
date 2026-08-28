package app

import (
	"context"
	"encoding/json"
	"os"
	"strings"
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
		ch := a.SendLiveDelegatedMessageStream(context.Background(), "fix the bug", 0)
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

	ch := a.SendLiveDelegatedMessageStream(context.Background(), "another task", 0)
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

	ch := a.SendLiveDelegatedMessageStream(sessionCtx, "a task", 0)
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

// TestDrainWithDeadline_ForwardsEverythingWhenInnerFinishesFirst is the
// happy path: the delegated turn completes before the deadline, so every
// chunk is forwarded and timedOut is false.
func TestDrainWithDeadline_ForwardsEverythingWhenInnerFinishesFirst(t *testing.T) {
	inner := make(chan api.StreamChunk, 3)
	inner <- api.StreamChunk{Content: "a"}
	inner <- api.StreamChunk{Content: "b"}
	close(inner)

	out := make(chan api.StreamChunk, 8)
	if drainWithDeadline(context.Background(), inner, out, time.Second) {
		t.Fatal("timedOut=true though inner closed well before the deadline")
	}
	close(out)

	var got string
	for c := range out {
		got += c.Content
	}
	if got != "ab" {
		t.Errorf("forwarded %q, want %q", got, "ab")
	}
}

// TestDrainWithDeadline_ReportsTimeoutWhenInnerOverruns is the regression
// test for the Live Mode "asked it to search the web, 30-40s of silence"
// symptom: an agent turn that outlasts the deadline must hand control back
// promptly with timedOut=true rather than blocking the realtime turn until
// the agent eventually finishes.
func TestDrainWithDeadline_ReportsTimeoutWhenInnerOverruns(t *testing.T) {
	inner := make(chan api.StreamChunk) // never sends, never closes
	out := make(chan api.StreamChunk, 8)

	start := time.Now()
	if !drainWithDeadline(context.Background(), inner, out, 40*time.Millisecond) {
		t.Fatal("expected timedOut=true when inner overran the deadline")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("returned after %s — expected it to fire at the ~40ms deadline", elapsed)
	}
}

// TestDrainWithDeadline_CtxCancelEndsItWithoutTimeout confirms a cancelled
// session context ends the drain as a non-timeout (the caller then just
// closes its stream) — it is not misreported as an overrun.
func TestDrainWithDeadline_CtxCancelEndsItWithoutTimeout(t *testing.T) {
	inner := make(chan api.StreamChunk)
	out := make(chan api.StreamChunk, 8)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan bool, 1)
	go func() { done <- drainWithDeadline(ctx, inner, out, 5*time.Second) }()

	cancel()
	close(inner) // the real pipeline closes its stream when its ctx is cancelled

	select {
	case timedOut := <-done:
		if timedOut {
			t.Error("ctx cancellation must end drainWithDeadline with timedOut=false")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("drainWithDeadline did not return after ctx cancellation")
	}
}

func TestResolveDelegateTimeout_NoMarkerLeavesReplyToNormalHandling(t *testing.T) {
	a := &App{identity: identity.New("Test", "Memo", "casual", "", false)}
	if _, ok := a.resolveDelegateTimeout("a perfectly normal reply"); ok {
		t.Error("resolveDelegateTimeout claimed a timeout on an unmarked reply")
	}
}

func TestResolveDelegateTimeout_MarkerWithNoPartialTextGivesHonestOverrunNote(t *testing.T) {
	a := &App{identity: identity.New("Test", "Memo", "casual", "", false)}
	out, ok := a.resolveDelegateTimeout(liveDelegateTimeoutMarker)
	if !ok {
		t.Fatal("expected resolveDelegateTimeout to handle a marked reply")
	}
	if strings.Contains(out, liveDelegateTimeoutMarker) {
		t.Error("the raw marker leaked into the text handed to the live model")
	}
	if !strings.Contains(strings.ToUpper(out), "TIMED OUT") && !strings.Contains(strings.ToUpper(out), "ZAMAN AŞIMINA") {
		t.Errorf("expected an explicit timeout instruction, got %q", out)
	}
}

func TestResolveDelegateTimeout_MarkerWithPartialTextRelaysItAsIncomplete(t *testing.T) {
	a := &App{identity: identity.New("Test", "Memo", "casual", "", false)}
	out, ok := a.resolveDelegateTimeout("here are the first three results" + liveDelegateTimeoutMarker)
	if !ok {
		t.Fatal("expected resolveDelegateTimeout to handle a marked reply")
	}
	if strings.Contains(out, liveDelegateTimeoutMarker) {
		t.Error("the raw marker leaked into the text handed to the live model")
	}
	if !strings.Contains(out, "here are the first three results") {
		t.Errorf("partial text was dropped: %q", out)
	}
	if !strings.Contains(strings.ToUpper(out), "PARTIAL") && !strings.Contains(strings.ToUpper(out), "KISMİ") {
		t.Errorf("expected a 'may be incomplete' framing around the partial text, got %q", out)
	}
}
