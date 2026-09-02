package taskloop

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestEngine_ChatBusyWaitsAndRetriesSameItem is the regression for the bug
// that made every Self-Driving list fail two seconds after starting: the agent
// turn that called start_self_driving_task is still streaming in the list's
// own chat when the loop reaches item 1, so the host's per-chat stream lock
// rejected the worker turn ("⏳ Lütfen önceki cevap tamamlanana kadar
// bekleyin"). That error matched no provider bucket, so the item went straight
// to stuck — and so did every following item, in one burst.
//
// A busy chat must instead cost a short wait and a re-run of the SAME item,
// without burning a review round or a provider switch.
func TestEngine_ChatBusyWaitsAndRetriesSameItem(t *testing.T) {
	oldWait := chatBusyWait
	chatBusyWait = 10 * time.Millisecond
	defer func() { chatBusyWait = oldWait }()

	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("chat1", "T", []string{"do it"})

	var mu sync.Mutex
	calls := 0
	healCalls := 0

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			mu.Lock()
			calls++
			n := calls
			mu.Unlock()
			if n <= 2 {
				return "", fmt.Errorf("%w: ⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", ErrChatBusy)
			}
			return "done for real", nil
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(v bool) {},
		nil,
		WithSelfHeal(func(ctx context.Context, listID string, err error) bool {
			mu.Lock()
			healCalls++
			mu.Unlock()
			return true
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = engine.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListDone, 3*time.Second)

	got, _ := store.Get(tl.ID)
	if got.Items[0].Status != "done" {
		t.Fatalf("item status = %q, want done (a busy chat must not strand the item)", got.Items[0].Status)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 3 {
		t.Fatalf("worker calls = %d, want 3 (two busy waits, then the real turn)", calls)
	}
	if healCalls != 0 {
		t.Fatalf("selfHeal called %d time(s) — a busy chat is not a provider fault", healCalls)
	}
	// Rounds are the CEO-review budget; a turn that never reached a model
	// must not consume one.
	if got.Items[0].Rounds > 1 {
		t.Fatalf("item Rounds = %d, want <= 1 — busy retries must not burn review rounds", got.Items[0].Rounds)
	}
}

// TestEngine_ChatBusyGivesUpAfterBudget: if the chat never frees up, the item
// still ends stuck rather than looping forever — but only after the wait
// budget, and with a note that names the real cause.
func TestEngine_ChatBusyGivesUpAfterBudget(t *testing.T) {
	oldWait := chatBusyWait
	chatBusyWait = 5 * time.Millisecond
	defer func() { chatBusyWait = oldWait }()

	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("chat1", "T", []string{"do it"})

	var mu sync.Mutex
	calls := 0

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			mu.Lock()
			calls++
			mu.Unlock()
			return "", fmt.Errorf("%w: busy", ErrChatBusy)
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(v bool) {},
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = engine.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListFailed, 3*time.Second)

	mu.Lock()
	defer mu.Unlock()
	if want := maxChatBusyWaits + 1; calls != want {
		t.Fatalf("worker calls = %d, want %d (budget of %d waits, then give up)", calls, want, maxChatBusyWaits)
	}
}

func TestIsChatBusyErr(t *testing.T) {
	if !IsChatBusyErr(fmt.Errorf("işçi hatası: %w", ErrChatBusy)) {
		t.Fatal("IsChatBusyErr must see through wrapping")
	}
	if IsChatBusyErr(errors.New("429 rate limit")) {
		t.Fatal("a rate-limit error must not read as chat-busy")
	}
	// A busy chat must not be mistaken for a provider fault by any of the
	// classifiers — that mislabelling is what parked lists in the wrong state.
	busy := fmt.Errorf("%w: ⏳ Lütfen önceki cevap tamamlanana kadar bekleyin.", ErrChatBusy)
	if IsRateLimitErr(busy) || IsAuthErr(busy) || IsTransientErr(busy) {
		t.Fatal("chat-busy must classify as none of rate-limit/auth/transient")
	}
}
