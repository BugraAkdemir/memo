package taskloop

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func TestStoreCreateGetList(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	tl, err := store.Create("chat1", "Test list", []string{"item 1", "item 2", "item 3"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tl.ID == "" {
		t.Fatal("ID empty")
	}
	if len(tl.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(tl.Items))
	}
	for _, item := range tl.Items {
		if item.Status != "pending" {
			t.Fatalf("item %s status = %s, want pending", item.ID, item.Status)
		}
	}

	got, err := store.Get(tl.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != tl.ID {
		t.Fatal("Get returned wrong ID")
	}

	list := store.List()
	if len(list) != 1 {
		t.Fatalf("List: expected 1, got %d", len(list))
	}
}

func TestStoreItemStatusTransitions(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl, _ := store.Create("chat1", "Test", []string{"do stuff"})
	itemID := tl.Items[0].ID

	if err := store.SetItemRunning(tl.ID, itemID); err != nil {
		t.Fatalf("SetItemRunning: %v", err)
	}
	tl, _ = store.Get(tl.ID)
	if tl.Items[0].Status != "running" {
		t.Fatalf("status = %s, want running", tl.Items[0].Status)
	}

	if err := store.SetItemDone(tl.ID, itemID); err != nil {
		t.Fatalf("SetItemDone: %v", err)
	}
	tl, _ = store.Get(tl.ID)
	if tl.Items[0].Status != "done" {
		t.Fatalf("status = %s, want done", tl.Items[0].Status)
	}
}

func TestStoreInvalidID(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)

	if _, err := store.Get("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent")
	}
	if err := store.SetStatus("nonexistent", "running"); err == nil {
		t.Fatal("expected error for nonexistent setstatus")
	}
}

func TestStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl, _ := store.Create("chat1", "Test", []string{"item"})

	if err := store.Delete(tl.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list := store.List()
	if len(list) != 0 {
		t.Fatalf("after delete: %d items remain", len(list))
	}
}

func TestStoreIncrementRounds(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl, _ := store.Create("chat1", "Test", []string{"item"})
	itemID := tl.Items[0].ID

	for i := 1; i <= 5; i++ {
		if err := store.IncrementRounds(tl.ID, itemID); err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
		tl, _ = store.Get(tl.ID)
		if tl.Items[0].Rounds != i {
			t.Fatalf("round %d: rounds = %d", i, tl.Items[0].Rounds)
		}
	}
}

func TestEngineApprovedFirstTry(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl, _ := store.Create("chat1", "Test", []string{"create file x"})

	var mu sync.Mutex
	workerCalls := 0
	chiefCalls := 0
	bypassLog := make([]bool, 0)

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			mu.Lock()
			workerCalls++
			mu.Unlock()
			return "done: created file x", nil
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
			mu.Lock()
			chiefCalls++
			mu.Unlock()
			return true, "", nil
		},
		func(v bool) {
			mu.Lock()
			bypassLog = append(bypassLog, v)
			mu.Unlock()
		},
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := engine.Start(ctx, tl.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	wc := workerCalls
	cc := chiefCalls
	mu.Unlock()

	tl, _ = store.Get(tl.ID)
	if tl.Status != "done" {
		t.Fatalf("list status = %s, want done", tl.Status)
	}
	if tl.Items[0].Status != "done" {
		t.Fatalf("item status = %s, want done", tl.Items[0].Status)
	}
	if wc != 1 {
		t.Fatalf("worker called %d times, want 1", wc)
	}
	if cc != 1 {
		t.Fatalf("chief called %d times, want 1", cc)
	}

	mu.Lock()
	bl := append([]bool{}, bypassLog...)
	mu.Unlock()
	if len(bl) < 2 {
		t.Fatalf("bypass log: expected at least 2 entries, got %d", len(bl))
	}
	if !bl[0] {
		t.Fatal("bypass was not enabled")
	}
	if bl[len(bl)-1] {
		t.Fatal("bypass was not disabled")
	}
}

func TestEngineChiefRejectsThenApproves(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl, _ := store.Create("chat1", "Test", []string{"create file x"})

	var mu sync.Mutex
	workerCalls := 0
	feedback := ""
	callCount := 0

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			mu.Lock()
			workerCalls++
			callCount++
			cn := callCount
			mu.Unlock()
			if cn == 1 {
				return "partial output", nil
			}
			return "complete output with all details", nil
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
			if workerOutput == "partial output" {
				return false, "missing details section", nil
			}
			return true, "", nil
		},
		func(v bool) {},
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = feedback
	_ = engine.Start(ctx, tl.ID)

	time.Sleep(500 * time.Millisecond)

	tl, _ = store.Get(tl.ID)
	if tl.Status != "done" {
		t.Fatalf("list status = %s, want done", tl.Status)
	}
	if tl.Items[0].Status != "done" {
		t.Fatalf("item status = %s, want done", tl.Items[0].Status)
	}
}

func TestEngineStuckAfterMaxRounds(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl, _ := store.Create("chat1", "Test", []string{"create file x"})

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			return "incomplete output", nil
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
			return false, "still missing things", nil
		},
		func(v bool) {},
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = engine.Start(ctx, tl.ID)

	time.Sleep(1 * time.Second)

	tl, _ = store.Get(tl.ID)
	// Every item stuck and none done -> the list is "failed" (v4.4.0). A list
	// that finishes with at least one item done is "done".
	if tl.Status != "failed" {
		t.Fatalf("list status = %s, want failed (only item is stuck)", tl.Status)
	}
	if tl.Items[0].Status != "stuck" {
		t.Fatalf("item status = %s, want stuck after %d rounds", tl.Items[0].Status, maxRoundsPerItem)
	}
	if tl.Items[0].Rounds != maxRoundsPerItem {
		t.Fatalf("rounds = %d, want %d", tl.Items[0].Rounds, maxRoundsPerItem)
	}
}

func TestEngineContextCancel(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl, _ := store.Create("chat1", "Test", []string{"item1", "item2", "item3"})

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(10 * time.Second):
				return "output", nil
			}
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
			return true, "", nil
		},
		func(v bool) {},
		nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	_ = engine.Start(ctx, tl.ID)

	time.Sleep(100 * time.Millisecond)
	cancel()

	time.Sleep(500 * time.Millisecond)

	tl, _ = store.Get(tl.ID)
	if tl.Status != "paused" {
		t.Fatalf("status = %s, want paused after cancel", tl.Status)
	}
	if tl.Items[0].Status != "pending" {
		t.Fatalf("interrupted item status = %s, want pending so a resumed run retries it instead of skipping it forever", tl.Items[0].Status)
	}
}

// TestEngineContextCancelLastItem guards against the list being marked "done"
// when the run loop's cancellation branch fires on the final item — a
// cancelled item must never let the loop fall through to the end-of-list
// SetStatus(listID, "done").
func TestEngineContextCancelLastItem(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl, _ := store.Create("chat1", "Test", []string{"only item"})

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(10 * time.Second):
				return "output", nil
			}
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
			return true, "", nil
		},
		func(v bool) {},
		nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	_ = engine.Start(ctx, tl.ID)

	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(500 * time.Millisecond)

	tl, _ = store.Get(tl.ID)
	if tl.Status != "paused" {
		t.Fatalf("status = %s, want paused (not done) after cancelling the last item", tl.Status)
	}
	if tl.Items[0].Status != "pending" {
		t.Fatalf("interrupted last item status = %s, want pending", tl.Items[0].Status)
	}
}

// TestEngineCancelIsTerminal: Cancel() must leave the list at "cancelled", not
// "paused". Regression for a live bug where CancelTaskList set "cancelled"
// after Stop(), and the run goroutine's own ctx.Done cleanup then raced it back
// to "paused" — so "cancel" behaved exactly like "pause" (list stayed
// resumable, card kept showing "Resume").
func TestEngineCancelIsTerminal(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl, _ := store.Create("chat1", "Test", []string{"item1", "item2", "item3"})

	var events []string
	var evMu sync.Mutex
	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(10 * time.Second):
				return "output", nil
			}
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
			return true, "", nil
		},
		func(v bool) {},
		func(name, data string) {
			evMu.Lock()
			events = append(events, name)
			evMu.Unlock()
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = engine.Start(ctx, tl.ID)

	time.Sleep(100 * time.Millisecond)
	engine.Cancel(tl.ID)
	time.Sleep(500 * time.Millisecond)

	tl, _ = store.Get(tl.ID)
	if tl.Status != taskListCancelled {
		t.Fatalf("status = %s, want %q after Cancel()", tl.Status, taskListCancelled)
	}

	evMu.Lock()
	defer evMu.Unlock()
	var sawCancelled bool
	for _, e := range events {
		if e == "taskloop:cancelled" {
			sawCancelled = true
		}
		if e == "taskloop:paused" {
			t.Fatalf("emitted taskloop:paused during a Cancel() — the run cleanup clobbered the terminal state; events=%v", events)
		}
	}
	if !sawCancelled {
		t.Fatalf("never emitted taskloop:cancelled; events=%v", events)
	}
}

func TestEngineCEOErrorFallback(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl, _ := store.Create("chat1", "Test", []string{"do task"})

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			return "worker output", nil
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
			return false, "", fmt.Errorf("network error")
		},
		func(v bool) {},
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = engine.Start(ctx, tl.ID)

	time.Sleep(1 * time.Second)

	tl, _ = store.Get(tl.ID)
	if tl.Items[0].Status != "stuck" {
		t.Fatalf("item status = %s, want stuck after CEO error", tl.Items[0].Status)
	}
}

func TestExtractAndParseReview(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		approved bool
		feedback string
		err      bool
	}{
		{"plain json approved", `{"approved": true, "feedback": ""}`, true, "", false},
		{"plain json rejected", `{"approved": false, "feedback": "missing tests"}`, false, "missing tests", false},
		{"markdown code block", "```json\n{\"approved\": true, \"feedback\": \"\"}\n```", true, "", false},
		{"code block no lang", "```\n{\"approved\": false, \"feedback\": \"redo\"}\n```", false, "redo", false},
		{"with surrounding text", "Analysis:\n{\"approved\": true, \"feedback\": \"ok\"}\nDone.", true, "ok", false},
		{"feedback containing a literal brace", `{"approved": false, "feedback": "Kapanış } eksik, fonksiyonu tamamla"}`, false, "Kapanış } eksik, fonksiyonu tamamla", false},
		{"invalid json", "not json at all", false, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			approved, feedback, err := ExtractAndParseReview(tt.raw)
			if tt.err && err == nil {
				t.Fatal("expected error")
			}
			if !tt.err && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.err {
				if approved != tt.approved {
					t.Fatalf("approved = %v, want %v", approved, tt.approved)
				}
				if feedback != tt.feedback {
					t.Fatalf("feedback = %q, want %q", feedback, tt.feedback)
				}
			}
		})
	}
}

func TestEngineBypassCounterConcurrent(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl1, _ := store.Create("chat1", "List 1", []string{"item1", "item2", "item3"})
	tl2, _ := store.Create("chat2", "List 2", []string{"itemA", "itemB", "itemC"})

	var mu sync.Mutex
	bypassState := false
	bypassChanges := 0

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(5 * time.Second):
				return "output", nil
			}
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
			return true, "", nil
		},
		func(v bool) {
			mu.Lock()
			bypassState = v
			bypassChanges++
			t.Logf("bypass set to %v (change #%d)", v, bypassChanges)
			mu.Unlock()
		},
		nil,
	)

	ctx := context.Background()
	t.Log("starting list 1")
	_ = engine.Start(ctx, tl1.ID)
	time.Sleep(100 * time.Millisecond)
	t.Log("starting list 2")
	_ = engine.Start(ctx, tl2.ID)

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	state := bypassState
	changes := bypassChanges
	mu.Unlock()
	t.Logf("after both starts: bypass=%v changes=%d", state, changes)

	if !state {
		t.Fatal("bypass should be true with 2 active lists")
	}

	t.Log("stopping list 1")
	engine.Stop(tl1.ID)
	time.Sleep(time.Second)

	mu.Lock()
	state = bypassState
	mu.Unlock()
	t.Logf("after stop 1: bypass=%v", state)

	if !state {
		t.Fatal("bypass should still be true with 1 remaining list")
	}

	t.Log("stopping list 2")
	engine.Stop(tl2.ID)
	time.Sleep(time.Second)

	mu.Lock()
	state = bypassState
	mu.Unlock()
	t.Logf("after stop 2: bypass=%v", state)

	if state {
		t.Fatal("bypass should be false after all lists stopped")
	}

	mu.Lock()
	changes = bypassChanges
	mu.Unlock()
	if changes < 2 {
		t.Fatalf("bypass changes = %d, want at least 2", changes)
	}
}

func TestWorkerErrorHandling(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl, _ := store.Create("chat1", "Test", []string{"do task"})

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			return "", fmt.Errorf("worker crash")
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
			return true, "", nil
		},
		func(v bool) {},
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = engine.Start(ctx, tl.ID)

	time.Sleep(500 * time.Millisecond)

	tl, _ = store.Get(tl.ID)
	if tl.Items[0].Status != "stuck" {
		t.Fatalf("item status = %s, want stuck after worker error", tl.Items[0].Status)
	}
	if tl.Items[0].Note == "" {
		t.Fatal("note should contain error info")
	}
}

func TestEngineDuplicateStart(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl, _ := store.Create("chat1", "Test", []string{"item"})

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			time.Sleep(500 * time.Millisecond)
			return "output", nil
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
			return true, "", nil
		},
		func(v bool) {},
		nil,
	)

	ctx := context.Background()
	if err := engine.Start(ctx, tl.ID); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if err := engine.Start(ctx, tl.ID); err == nil {
		t.Fatal("expected error for duplicate Start")
	}

	time.Sleep(100 * time.Millisecond)
	engine.Stop(tl.ID)
}

func TestEmptyWorkerOutput(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl, _ := store.Create("chat1", "Test", []string{"do task"})

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			return "", nil
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
			return true, "", nil
		},
		func(v bool) {},
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = engine.Start(ctx, tl.ID)

	time.Sleep(500 * time.Millisecond)

	tl, _ = store.Get(tl.ID)
	if tl.Items[0].Status != "stuck" {
		t.Fatalf("item status = %s, want stuck for empty output", tl.Items[0].Status)
	}
}

func TestStorePersistence(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl, _ := store.Create("chat1", "Test", []string{"item"})

	if _, err := os.Stat(dir + "/" + tl.ID + ".json"); err != nil {
		t.Fatalf("file not on disk: %v", err)
	}

	store2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore reload: %v", err)
	}
	got, err := store2.Get(tl.ID)
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}
	if got.ID != tl.ID {
		t.Fatal("wrong ID after reload")
	}
}

func TestListSkipsDoneAndStuckItems(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(dir)
	tl, _ := store.Create("chat1", "Test", []string{"item1", "item2", "item3"})

	store.SetItemDone(tl.ID, tl.Items[0].ID)
	store.SetItemStuck(tl.ID, tl.Items[1].ID, "test")

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			return "output", nil
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
			return true, "", nil
		},
		func(v bool) {},
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = engine.Start(ctx, tl.ID)

	time.Sleep(500 * time.Millisecond)

	tl, _ = store.Get(tl.ID)
	if tl.Items[0].Status != "done" {
		t.Fatal("item1 should stay done")
	}
	if tl.Items[1].Status != "stuck" {
		t.Fatal("item2 should stay stuck")
	}
	if tl.Items[2].Status != "done" {
		t.Fatal("item3 should be done (only pending item processed)")
	}
}

