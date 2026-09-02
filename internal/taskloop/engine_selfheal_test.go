package taskloop

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func waitForStatus(t *testing.T, store *Store, id string, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		got, _ := store.Get(id)
		if got.Status == want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("list status = %q, want %q", got.Status, want)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestEngine_SelfHealRetriesItem(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("chat1", "T", []string{"do it"})

	var mu sync.Mutex
	workerCalls, healCalls := 0, 0

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			mu.Lock()
			workerCalls++
			n := workerCalls
			mu.Unlock()
			if n == 1 {
				return "", errors.New("401 unauthorized")
			}
			return "worked on retry", nil
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(v bool) {},
		nil,
		WithSelfHeal(func(ctx context.Context, listID string, err error) bool {
			mu.Lock()
			healCalls++
			mu.Unlock()
			return true // pretend we switched provider
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = engine.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListDone, 3*time.Second)

	got, _ := store.Get(tl.ID)
	if got.Items[0].Status != "done" {
		t.Fatalf("item status = %q, want done", got.Items[0].Status)
	}
	mu.Lock()
	defer mu.Unlock()
	if healCalls != 1 {
		t.Fatalf("healCalls = %d, want 1", healCalls)
	}
	if workerCalls < 2 {
		t.Fatalf("workerCalls = %d, want >= 2 (retry after heal)", workerCalls)
	}
}

func TestEngine_SelfHealDeclinedMarksStuck(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("chat1", "T", []string{"do it"})

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			return "", errors.New("provider rate limited")
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(v bool) {},
		nil,
		WithSelfHeal(func(ctx context.Context, listID string, err error) bool {
			return false // rate-limit: don't heal
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = engine.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListFailed, 3*time.Second)

	got, _ := store.Get(tl.ID)
	if got.Items[0].Status != "stuck" {
		t.Fatalf("item status = %q, want stuck", got.Items[0].Status)
	}
}
