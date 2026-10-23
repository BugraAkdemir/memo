package subagents

import (
	"context"
	"encoding/json"
	"memo/internal/provider"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func eventually(t *testing.T, f func() bool) {
	t.Helper()
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		if f() {
			return
		}
		time.Sleep(time.Millisecond * 5)
	}
	t.Fatal("condition not reached")
}
func TestQueueCompletionAndFollowup(t *testing.T) {
	gate := make(chan struct{}, 8)
	var running, max atomic.Int32
	m, err := Open(t.TempDir(), 1, func(ctx context.Context, j Job, emit func(Event)) (RunResult, error) {
		n := running.Add(1)
		defer running.Add(-1)
		for old := max.Load(); n > old && !max.CompareAndSwap(old, n); old = max.Load() {
		}
		emit(Event{Type: "output", Content: j.Prompt})
		select {
		case <-ctx.Done():
			return RunResult{}, ctx.Err()
		case <-gate:
		}
		return RunResult{Output: j.Prompt, Messages: append(j.Messages, provider.TextMessage("assistant", j.Prompt))}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())
	a, err := m.Create(CreateRequest{ChatID: "chat", Prompt: "one"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Create(CreateRequest{ChatID: "chat", Prompt: "two"})
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return running.Load() == 1 })
	if j, _ := m.Get("chat", b.ID); j.State != Queued {
		t.Fatal(j.State)
	}
	gate <- struct{}{}
	eventually(t, func() bool { j, _ := m.Get("chat", a.ID); return j.State == Awaiting })
	first, ok := m.PeekCompletion("chat")
	if !ok || first.JobID != a.ID {
		t.Fatalf("%+v", first)
	}
	gate <- struct{}{}
	eventually(t, func() bool { j, _ := m.Get("chat", b.ID); return j.State == Awaiting })
	same, _ := m.PeekCompletion("chat")
	if same.ID != first.ID {
		t.Fatal("unacknowledged completion displaced")
	}
	if err := m.AckCompletion("other", first.ID); err == nil {
		t.Fatal("cross-chat acknowledgment accepted")
	}
	if err := m.AckCompletion("chat", first.ID); err != nil {
		t.Fatal(err)
	}
	next, _ := m.PeekCompletion("chat")
	if next.ID <= first.ID || next.JobID != b.ID {
		t.Fatal(next)
	}
	j, err := m.Followup("chat", a.ID, "fix")
	if err != nil {
		t.Fatal(err)
	}
	if len(j.Messages) != 3 || j.Round != 2 {
		t.Fatalf("lost context: %+v", j)
	}
	gate <- struct{}{}
	eventually(t, func() bool { j, _ := m.Get("chat", a.ID); return j.State == Awaiting })
	if max.Load() != 1 {
		t.Fatal(max.Load())
	}
}
func TestRecoveryAndCancellation(t *testing.T) {
	dir := t.TempDir()
	m, err := Open(dir, 1, func(ctx context.Context, j Job, emit func(Event)) (RunResult, error) {
		<-ctx.Done()
		return RunResult{}, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	a, _ := m.Create(CreateRequest{ChatID: "chat", Prompt: "one"})
	b, _ := m.Create(CreateRequest{ChatID: "chat", Prompt: "two"})
	if err := m.Cancel("other", a.ID); err == nil {
		t.Fatal("cross-chat cancel accepted")
	}
	if err := m.Cancel("chat", b.ID); err != nil {
		t.Fatal(err)
	}
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	recovered, err := Open(dir, 1, func(context.Context, Job, func(Event)) (RunResult, error) { return RunResult{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Shutdown(context.Background())
	j, _ := recovered.Get("chat", a.ID)
	if j.State != Interrupted {
		t.Fatal(j.State)
	}
	j, _ = recovered.Get("chat", b.ID)
	if j.State != Cancelled {
		t.Fatal(j.State)
	}
	c, ok := recovered.PeekCompletion("chat")
	if !ok || c.ID == 0 {
		t.Fatal("missing durable completion")
	}
}

func TestCrashRecoveryDoesNotReplayRunningAndRecoversQueue(t *testing.T) {
	dir := t.TempDir()
	runner := func(context.Context, Job, func(Event)) (RunResult, error) { return RunResult{Output: "ok"}, nil }
	m, err := Open(dir, 1, runner)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := m.Create(CreateRequest{ChatID: "chat", Prompt: "already started"})
	b, _ := m.Create(CreateRequest{ChatID: "chat", Prompt: "queued"})
	// Simulate durable state at the exact point a process loses power. No
	// goroutine was started, so this test cannot race the old writer.
	m.mu.Lock()
	m.data.Jobs[a.ID].State = Running
	m.data.Queue = []string{b.ID}
	err = m.saveLocked()
	m.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := Open(dir, 1, runner)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Shutdown(context.Background())
	j, _ := recovered.Get("chat", a.ID)
	if j.State != Interrupted {
		t.Fatal(j.State)
	}
	if err = recovered.Start(); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { j, _ := recovered.Get("chat", b.ID); return j.State == Awaiting })
	j, _ = recovered.Get("chat", a.ID)
	if j.State != Interrupted {
		t.Fatal("replayed interrupted task")
	}
	if len(recovered.PendingChats()) != 1 {
		t.Fatal(recovered.PendingChats())
	}
	info, err := os.Stat(filepath.Join(dir, "state.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal(info, err)
	}
}
func TestLimitUpdateAndPanicRecovery(t *testing.T) {
	release := make(chan struct{})
	var running atomic.Int32
	m, err := Open(t.TempDir(), 1, func(ctx context.Context, j Job, emit func(Event)) (RunResult, error) {
		running.Add(1)
		defer running.Add(-1)
		select {
		case <-ctx.Done():
			return RunResult{}, ctx.Err()
		case <-release:
		}
		panic("test panic")
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err = m.Create(CreateRequest{ChatID: "chat", Prompt: "task"}); err != nil {
			t.Fatal(err)
		}
	}
	eventually(t, func() bool { return running.Load() == 1 })
	if err = m.SetLimit(2); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return running.Load() == 2 })
	close(release)
	eventually(t, func() bool {
		jobs := m.List("chat")
		for _, j := range jobs {
			if j.State != Failed {
				return false
			}
		}
		return true
	})
	if err = m.SetLimit(0); err == nil {
		t.Fatal("zero concurrency accepted")
	}
}
func TestSharedWritersSerialize(t *testing.T) {
	release := make(chan struct{}, 3)
	var active, max atomic.Int32
	m, err := Open(t.TempDir(), 3, func(ctx context.Context, j Job, emit func(Event)) (RunResult, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := max.Load(); n > old && !max.CompareAndSwap(old, n); old = max.Load() {
		}
		select {
		case <-ctx.Done():
			return RunResult{}, ctx.Err()
		case <-release:
		}
		return RunResult{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())
	repo := t.TempDir()
	for i := 0; i < 3; i++ {
		if _, err = m.Create(CreateRequest{ChatID: "chat", Prompt: "write", Write: true, ProjectDir: repo}); err != nil {
			t.Fatal(err)
		}
	}
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return active.Load() == 1 })
	for i := 0; i < 3; i++ {
		release <- struct{}{}
	}
	eventually(t, func() bool {
		for _, j := range m.List("chat") {
			if j.State != Awaiting {
				return false
			}
		}
		return true
	})
	if max.Load() != 1 {
		t.Fatal("shared writers ran concurrently", max.Load())
	}
}
func TestCompletionClaimRecoveryAndRetry(t *testing.T) {
	dir := t.TempDir()
	runner := func(context.Context, Job, func(Event)) (RunResult, error) { return RunResult{Output: "result"}, nil }
	m, err := Open(dir, 1, runner)
	if err != nil {
		t.Fatal(err)
	}
	j, _ := m.Create(CreateRequest{ChatID: "chat", Prompt: "task"})
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { jj, _ := m.Get("chat", j.ID); return jj.State == Awaiting })
	c, _ := m.PeekCompletion("chat")
	if c.Delivery != "pending" {
		t.Fatal(c)
	}
	if err = m.ClaimCompletion("other", c.ID); err == nil {
		t.Fatal("cross chat claim accepted")
	}
	if err = m.ClaimCompletion("chat", c.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.ClaimCompletion("chat", c.ID); err == nil {
		t.Fatal("double delivery accepted")
	}
	if err = m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	recovered, err := Open(dir, 1, runner)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Shutdown(context.Background())
	c, _ = recovered.PeekCompletion("chat")
	if c.Delivery != "interrupted" {
		t.Fatal(c)
	}
	if err = recovered.RetryCompletion("chat", j.ID); err != nil {
		t.Fatal(err)
	}
	c, _ = recovered.PeekCompletion("chat")
	if c.Delivery != "pending" {
		t.Fatal(c)
	}
	if err = recovered.AckCompletion("chat", c.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := recovered.PeekCompletion("chat"); ok {
		t.Fatal("acknowledged event retained")
	}
}
func TestSimultaneousCompletionsHaveUniqueDurableFIFO(t *testing.T) {
	dir := t.TempDir()
	release := make(chan struct{})
	var active atomic.Int32
	m, err := Open(dir, 3, func(ctx context.Context, j Job, emit func(Event)) (RunResult, error) {
		active.Add(1)
		select {
		case <-ctx.Done():
			return RunResult{}, ctx.Err()
		case <-release:
		}
		return RunResult{Output: j.ID}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err = m.Create(CreateRequest{ChatID: "chat", Prompt: "task"}); err != nil {
			t.Fatal(err)
		}
	}
	eventually(t, func() bool { return active.Load() == 3 })
	close(release)
	eventually(t, func() bool {
		for _, j := range m.List("chat") {
			if j.State != Awaiting {
				return false
			}
		}
		return true
	})
	if err = m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	recovered, err := Open(dir, 3, func(context.Context, Job, func(Event)) (RunResult, error) {
		t.Error("completed child rerun")
		return RunResult{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Shutdown(context.Background())
	seen := map[string]bool{}
	var previous uint64
	for i := 0; i < 3; i++ {
		c, ok := recovered.PeekCompletion("chat")
		if !ok || c.ID <= previous || seen[c.JobID] || c.Result != c.JobID {
			t.Fatalf("invalid completion: %+v", c)
		}
		if err = recovered.AckCompletion("chat", c.ID+1); err == nil {
			t.Fatal("out-of-order ack accepted")
		}
		seen[c.JobID] = true
		previous = c.ID
		if err = recovered.AckCompletion("chat", c.ID); err != nil {
			t.Fatal(err)
		}
	}
}
func TestInterruptedDeliveryCanRetryWithoutChildReplay(t *testing.T) {
	var calls atomic.Int32
	m, err := Open(t.TempDir(), 1, func(context.Context, Job, func(Event)) (RunResult, error) {
		calls.Add(1)
		return RunResult{Output: "child done"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())
	j, _ := m.Create(CreateRequest{ChatID: "chat", Prompt: "task"})
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { jj, _ := m.Get("chat", j.ID); return jj.State == Awaiting })
	c, _ := m.PeekCompletion("chat")
	if err = m.ClaimCompletion("chat", c.ID); err != nil {
		t.Fatal(err)
	}
	jj, _ := m.Get("chat", j.ID)
	if jj.ResultDelivery != "inflight" {
		t.Fatal(jj.ResultDelivery)
	}
	if err = m.MarkDeliveryInterrupted("other", c.ID, "failed"); err == nil {
		t.Fatal("cross-chat interruption accepted")
	}
	if err = m.MarkDeliveryInterrupted("chat", c.ID, "main cancelled"); err != nil {
		t.Fatal(err)
	}
	jj, _ = m.Get("chat", j.ID)
	if jj.ResultDelivery != "interrupted" || jj.DeliveryError != "main cancelled" {
		t.Fatal(jj)
	}
	if err = m.RetryCompletion("chat", j.ID); err != nil {
		t.Fatal(err)
	}
	jj, _ = m.Get("chat", j.ID)
	if jj.ResultDelivery != "pending" || jj.DeliveryError != "" {
		t.Fatal(jj)
	}
	if err = m.CloseJob("chat", j.ID); err != nil {
		t.Fatal(err)
	}
	if len(m.Completions("chat")) != 1 {
		t.Fatal("close dropped unacked result")
	}
	if err = m.AckCompletion("chat", c.ID); err != nil {
		t.Fatal(err)
	}
	jj, _ = m.Get("chat", j.ID)
	if jj.ResultDelivery != "" {
		t.Fatal(jj.ResultDelivery)
	}
	if calls.Load() != 1 {
		t.Fatal("child was replayed", calls.Load())
	}
}
func TestStreamingDeltasCoalesceAndFinalStateIsDurable(t *testing.T) {
	dir := t.TempDir()
	emitted, release := make(chan struct{}), make(chan struct{})
	m, err := Open(dir, 1, func(ctx context.Context, j Job, emit func(Event)) (RunResult, error) {
		for i := 0; i < 100; i++ {
			emit(Event{Type: "output", Content: "x"})
		}
		emit(Event{Type: "tool_result", ToolName: "read_file", Result: "read"})
		emit(Event{Type: "thinking", Content: "a"})
		emit(Event{Type: "thinking", Content: "b"})
		emit(Event{Type: "output", Content: "tail"})
		close(emitted)
		select {
		case <-ctx.Done():
			return RunResult{}, ctx.Err()
		case <-release:
		}
		return RunResult{Output: "complete"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	j, err := m.Create(CreateRequest{ChatID: "chat", Prompt: "task"})
	if err != nil {
		t.Fatal(err)
	}
	<-emitted
	jj, _ := m.Get("chat", j.ID)
	if len(jj.Events) != 4 || len(jj.Events[0].Content) != 100 || jj.Events[2].Content != "ab" {
		t.Fatal("deltas not coalesced with tool order retained", jj.Events)
	}
	// Even if the provider goes silent after a delta, the periodic checkpoint
	// persists that tail before the task finishes.
	eventually(t, func() bool {
		b, err := os.ReadFile(filepath.Join(dir, "state.json"))
		if err != nil {
			return false
		}
		var state diskState
		if json.Unmarshal(b, &state) != nil {
			return false
		}
		saved := state.Jobs[j.ID]
		return saved != nil && len(saved.Events) == 4 && saved.Events[3].Content == "tail"
	})
	close(release)
	eventually(t, func() bool { jj, _ := m.Get("chat", j.ID); return jj.State == Awaiting })
	if err = m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	recovered, err := Open(dir, 1, func(context.Context, Job, func(Event)) (RunResult, error) { return RunResult{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Shutdown(context.Background())
	jj, _ = recovered.Get("chat", j.ID)
	if len(jj.Events) != 4 || jj.Events[3].Content != "tail" || jj.Result != "complete" {
		t.Fatal("final delta was not persisted", jj)
	}
}
func TestNotifierFollowsDurableCompletionOrder(t *testing.T) {
	dir := t.TempDir()
	m, err := Open(dir, 3, func(context.Context, Job, func(Event)) (RunResult, error) { return RunResult{Output: "done"}, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())
	notifications := make(chan Completion, 4)
	m.SetCompletionNotifier(func(c Completion) {
		b, err := os.ReadFile(filepath.Join(dir, "state.json"))
		if err != nil {
			t.Error(err)
			return
		}
		var state diskState
		if err = json.Unmarshal(b, &state); err != nil {
			t.Error(err)
			return
		}
		found := false
		for _, saved := range state.Completions {
			if saved.ID == c.ID && saved.Result == c.Result {
				found = true
			}
		}
		if !found {
			t.Error("notified before durable save")
		}
		notifications <- c
	})
	for i := 0; i < 3; i++ {
		if _, err = m.Create(CreateRequest{ChatID: "chat", Prompt: "task"}); err != nil {
			t.Fatal(err)
		}
	}
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	var previous uint64
	for i := 0; i < 3; i++ {
		select {
		case c := <-notifications:
			if c.ID <= previous {
				t.Fatal("notification order violated")
			}
			previous = c.ID
		case <-time.After(5 * time.Second):
			t.Fatal("missing completion notification")
		}
	}
	first, _ := m.PeekCompletion("chat")
	if err = m.ClaimCompletion("chat", first.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.MarkDeliveryInterrupted("chat", first.ID, "main cancelled"); err != nil {
		t.Fatal(err)
	}
	if err = m.RetryCompletion("chat", first.JobID); err != nil {
		t.Fatal(err)
	}
	select {
	case c := <-notifications:
		if c.ID != first.ID || c.Delivery != "pending" {
			t.Fatal(c)
		}
	case <-time.After(time.Second):
		t.Fatal("retry not notified")
	}
}
func TestNotifierPanicKeepsResultForExplicitRetry(t *testing.T) {
	m, err := Open(t.TempDir(), 1, func(context.Context, Job, func(Event)) (RunResult, error) { return RunResult{Output: "result"}, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())
	m.SetCompletionNotifier(func(Completion) { panic("callback failed") })
	j, err := m.Create(CreateRequest{ChatID: "chat", Prompt: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { jj, _ := m.Get("chat", j.ID); return jj.ResultDelivery == "interrupted" })
	c, _ := m.PeekCompletion("chat")
	if c.Result != "result" || c.DeliveryError == "" {
		t.Fatal("callback failure lost completed result", c)
	}
}

func TestCorruptRecoveryRecordIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	original := []byte(`{"jobs":`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Open(dir, 3, func(context.Context, Job, func(Event)) (RunResult, error) { return RunResult{}, nil })
	if err == nil {
		t.Fatal("corrupt recovery state was accepted")
	}
	saved, rerr := os.ReadFile(path)
	if rerr != nil || string(saved) != string(original) {
		t.Fatal("failed recovery destroyed original evidence")
	}
}
func TestFailedCreatePersistenceNeverRunsOrPublishesTask(t *testing.T) {
	dir := t.TempDir()
	var calls atomic.Int32
	m, err := Open(dir, 3, func(context.Context, Job, func(Event)) (RunResult, error) { calls.Add(1); return RunResult{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())
	// Fail persistence independently of OS access modes or test user privileges.
	if err = os.Remove(filepath.Join(dir, "state.json")); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(dir, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Create(CreateRequest{ChatID: "chat", Prompt: "never execute without saved recovery state"}); err == nil {
		t.Fatal("task accepted after failed persistence")
	}
	if calls.Load() != 0 || len(m.List("chat")) != 0 {
		t.Fatal("unsaved task ran or escaped into API view")
	}
	if err = m.ClaimCompletion("chat", 1); err == nil {
		t.Fatal("nonexistent completion claimed")
	}
}

func TestCloseJobPreservesReadableHistoryAfterRestart(t *testing.T) {
	dir := t.TempDir()
	runner := func(ctx context.Context, j Job, emit func(Event)) (RunResult, error) {
		emit(Event{Type: "tool_result", ToolName: "read_file", Result: "tool evidence"})
		return RunResult{Output: "finished output", Messages: append(j.Messages, provider.TextMessage("assistant", "finished output"))}, nil
	}
	m, err := Open(dir, 1, runner)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	j, err := m.Create(CreateRequest{ChatID: "chat", Prompt: "original task"})
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { got, _ := m.Get("chat", j.ID); return got.State == Awaiting })
	if err = m.CloseJob("chat", j.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dir, 1, runner)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Shutdown(context.Background())
	got, err := reopened.Get("chat", j.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != Closed || got.Prompt != "original task" || got.Result != "finished output" || len(got.Messages) != 2 || len(got.Events) == 0 {
		t.Fatalf("closed history was lost: %+v", got)
	}
	if _, err = reopened.Followup("chat", j.ID, "restart"); err == nil {
		t.Fatal("closed history became runnable")
	}
}
