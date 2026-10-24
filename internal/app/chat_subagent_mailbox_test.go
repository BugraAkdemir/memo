package app

import (
	"context"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/sessions"
	"memo/internal/subagents"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestChatSubAgentInterruptedMainTurnNeverAcknowledgesResult(t *testing.T) {
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	chat := sm.NewChat()
	a := &App{cfg: &config.AppConfig{}, sessions: sm, identity: identity.New("Test", "Memo", "casual", "", true)}
	ctx, cancel := context.WithCancel(context.WithValue(withCodeMode(context.Background()), subAgentCompletionKey{}, uint64(8)))
	cancel()
	a.persistInterruptedTurn(ctx, time.Now(), 0, "partial review", "result", chat, &usageMeta{})
	if a.completionWasSaved(chat, 8) {
		t.Fatal("stopped review acknowledged child result")
	}
	success := context.WithValue(withCodeMode(context.Background()), subAgentCompletionKey{}, uint64(9))
	a.finishStream(success, time.Now(), 0, "stop", "accepted", "result", chat, &usageMeta{})
	if !a.completionWasSaved(chat, 9) {
		t.Fatal("successful saved review not acknowledged")
	}
}

// A failed save of the interruption marker leaves an orphaned inflight claim.
// Once storage recovers the mailbox must park it for explicit resume, without
// replaying an unsaved parent turn or requiring a process restart.
func TestChatSubAgentMailboxRecoversOrphanedClaimAfterStorageFailure(t *testing.T) {
	dir := t.TempDir()
	m, err := subagents.Open(dir, 1, func(context.Context, subagents.Job, func(subagents.Event)) (subagents.RunResult, error) {
		return subagents.RunResult{Output: "saved child work"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())
	job, err := m.Create(subagents.CreateRequest{ChatID: "chat", Prompt: "inspect"})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(3 * time.Second)
	var completion subagents.Completion
	for completion.ID == 0 {
		if c, ok := m.PeekCompletion("chat"); ok {
			completion = c
			break
		}
		select {
		case <-deadline:
			t.Fatal("child did not complete")
		case <-time.After(time.Millisecond):
		}
	}
	if err = m.ClaimCompletion("chat", completion.ID); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(dir, "state.json")
	backup := filepath.Join(dir, "saved-state.json")
	if err = os.Rename(state, backup); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	if err = m.MarkDeliveryInterrupted("chat", completion.ID, "review failed"); err == nil {
		t.Fatal("expected failed interruption save")
	}
	if err = os.Remove(state); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(backup, state); err != nil {
		t.Fatal(err)
	}
	a := &App{chatSubAgents: m, subAgentDelivering: map[uint64]bool{completion.ID: true}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); a.pumpChatSubAgents(ctx) }()
	defer func() { cancel(); <-done; a.subAgentDispatchWG.Wait() }()
	// A real live dispatcher must not be mistaken for an orphan.
	time.Sleep(200 * time.Millisecond)
	live, _ := m.Get("chat", job.ID)
	if live.ResultDelivery != "inflight" {
		t.Fatalf("interrupted an active delivery: %+v", live)
	}
	a.subAgentDeliveryMu.Lock()
	delete(a.subAgentDelivering, completion.ID)
	a.subAgentDeliveryMu.Unlock()
	deadline = time.After(time.Second)
	for {
		j, err := m.Get("chat", job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if j.ResultDelivery == "interrupted" {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("orphaned claim stuck after storage recovered: %+v", j)
		case <-time.After(10 * time.Millisecond):
		}
	}
}
