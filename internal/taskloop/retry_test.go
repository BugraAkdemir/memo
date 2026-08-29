package taskloop

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryScheduler_ArmFires(t *testing.T) {
	var got atomic.Int32
	s := NewRetryScheduler(15*time.Millisecond, func(string) { got.Add(1) })
	s.Arm("L1")
	time.Sleep(60 * time.Millisecond)
	if got.Load() != 1 {
		t.Fatalf("resume fired %d times, want 1", got.Load())
	}
	if s.Pending("L1") {
		t.Fatal("timer still pending after it fired")
	}
}

func TestRetryScheduler_CancelBeforeFire(t *testing.T) {
	var got atomic.Int32
	s := NewRetryScheduler(30*time.Millisecond, func(string) { got.Add(1) })
	s.Arm("L1")
	s.Cancel("L1")
	time.Sleep(60 * time.Millisecond)
	if got.Load() != 0 {
		t.Fatalf("resume fired %d times after Cancel, want 0", got.Load())
	}
}

func TestRetryScheduler_ReArmRestartsTimer(t *testing.T) {
	var got atomic.Int32
	s := NewRetryScheduler(40*time.Millisecond, func(string) { got.Add(1) })
	s.Arm("L1")
	time.Sleep(25 * time.Millisecond)
	s.Arm("L1") // restart before the first fires
	time.Sleep(25 * time.Millisecond)
	if got.Load() != 0 {
		t.Fatalf("resume fired early: %d", got.Load())
	}
	time.Sleep(40 * time.Millisecond)
	if got.Load() != 1 {
		t.Fatalf("resume fired %d times, want 1", got.Load())
	}
}

func TestRetryScheduler_NilSafe(t *testing.T) {
	var s *RetryScheduler
	s.Arm("x")
	s.Cancel("x")
	if s.Pending("x") {
		t.Fatal("nil scheduler reported pending")
	}
}

func TestEngine_RateLimitParksThenResumes(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("chat1", "T", []string{"the item"})

	var mu sync.Mutex
	workerCalls := 0
	var sawWaitingLimit atomic.Bool

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			mu.Lock()
			workerCalls++
			n := workerCalls
			mu.Unlock()
			if n == 1 {
				return "", errors.New("status 429: Too Many Requests, try again in 1 seconds")
			}
			return "done now", nil
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(v bool) {},
		func(name, data string) {
			if name == "taskloop:waiting_limit" {
				sawWaitingLimit.Store(true)
			}
		},
		WithRetryScheduler(20*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = engine.Start(ctx, tl.ID)

	// It should pass through waiting-limit...
	deadline := time.After(2 * time.Second)
	parked := false
	for !parked {
		got, _ := store.Get(tl.ID)
		if got.Status == taskListWaitingLimit {
			parked = true
			if got.Items[0].Status != "pending" {
				t.Fatalf("rate-limited item = %q, want pending", got.Items[0].Status)
			}
		}
		select {
		case <-deadline:
			t.Fatalf("never entered waiting-limit; status=%s", got.Status)
		case <-time.After(10 * time.Millisecond):
		}
	}
	if !sawWaitingLimit.Load() {
		t.Error("no taskloop:waiting_limit event emitted")
	}

	// ...then the retry timer resumes it to completion.
	waitForStatus(t, store, tl.ID, taskListDone, 3*time.Second)
	got, _ := store.Get(tl.ID)
	if got.Items[0].Status != "done" {
		t.Fatalf("item = %q, want done after resume", got.Items[0].Status)
	}
	mu.Lock()
	defer mu.Unlock()
	if workerCalls < 2 {
		t.Fatalf("workerCalls = %d, want >= 2", workerCalls)
	}
}
