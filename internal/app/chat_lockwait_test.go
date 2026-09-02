package app

import (
	"context"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/sessions"
)

func newLockWaitTestApp(t *testing.T) (*App, *sessions.Manager) {
	t.Helper()
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return &App{
		cfg:      &config.AppConfig{Memory: config.MemoryConfig{MemoryEnabled: false}, Llama: config.LlamaConfig{CtxSize: 4096}},
		identity: identity.New("Test", "Memo", "casual", "", false),
		sessions: sm,
	}, sm
}

// TestSendMessageStreamTo_ChatLockWait_QueuesInsteadOfBusy is the regression
// for the bug that killed every Self-Driving run: start_self_driving_task
// returns while its own agent turn is still streaming in the task's chat, so
// the loop's first worker turn hit the per-chat lock and was rejected with the
// busy notice — which the engine read as a worker failure and marked the item
// (and then every remaining item) stuck.
//
// With withChatLockWait the worker turn queues behind the in-flight turn.
func TestSendMessageStreamTo_ChatLockWait_QueuesInsteadOfBusy(t *testing.T) {
	a, sm := newLockWaitTestApp(t)
	chatA := sm.NewChat()

	release, ok := a.lockChatStream(chatA)
	if !ok {
		t.Fatal("could not take chat A's stream lock")
	}
	// The launching turn finishes shortly after the worker turn starts.
	go func() {
		time.Sleep(300 * time.Millisecond)
		release()
	}()

	ctx := withChatLockWait(context.Background())
	ch := a.SendMessageStreamTo(ctx, chatA, "task loop worker turn")

	deadline := time.After(10 * time.Second)
	for {
		select {
		case chunk, more := <-ch:
			if !more {
				t.Fatal("stream closed without any chunk")
			}
			// It will still fail fast on "no configured model" — that's fine.
			// What must never come back is the busy notice.
			if chunk.Error == a.busyNotice() {
				t.Fatal("a task worker turn was rejected as busy instead of queueing behind the in-flight turn")
			}
			if chunk.Done {
				return
			}
		case <-deadline:
			t.Fatal("stream did not finish in time")
		}
	}
}

// TestSendMessageStreamTo_NoWaitFlag_StillFailsFast keeps the interactive
// contract intact: without the flag a second turn in the same chat is still
// rejected immediately, so a human is never left staring at a frozen input.
func TestSendMessageStreamTo_NoWaitFlag_StillFailsFast(t *testing.T) {
	a, sm := newLockWaitTestApp(t)
	chatA := sm.NewChat()

	release, ok := a.lockChatStream(chatA)
	if !ok {
		t.Fatal("could not take chat A's stream lock")
	}
	defer release()

	start := time.Now()
	got := <-a.SendMessageStreamTo(context.Background(), chatA, "second interactive turn")
	if got.Error != a.busyNotice() {
		t.Fatalf("Error = %q, want the busy notice", got.Error)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("interactive busy reply took %s — it must fail fast, not queue", elapsed)
	}
}

// TestLockChatStreamWait_HonoursContextCancel: a cancelled task (task_cancel /
// shutdown) must not leave a worker turn parked on the lock for minutes.
func TestLockChatStreamWait_HonoursContextCancel(t *testing.T) {
	a, sm := newLockWaitTestApp(t)
	chatA := sm.NewChat()

	release, ok := a.lockChatStream(chatA)
	if !ok {
		t.Fatal("could not take chat A's stream lock")
	}
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	if _, ok := a.lockChatStreamWait(ctx, chatA); ok {
		t.Fatal("lockChatStreamWait acquired a lock that was never released")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("lockChatStreamWait ignored ctx cancellation for %s", elapsed)
	}
}
