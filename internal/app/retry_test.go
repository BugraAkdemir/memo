package app

import (
	"context"
	"testing"
	"time"
)

func TestRetryWithBackoff_SucceedsAfterFailures(t *testing.T) {
	calls := 0
	ok := retryWithBackoff(context.Background(), time.Millisecond, 5*time.Millisecond, func() bool {
		calls++
		return calls >= 3
	})
	if !ok {
		t.Fatal("expected retryWithBackoff to report success")
	}
	if calls != 3 {
		t.Errorf("expected fn to be called exactly 3 times, got %d", calls)
	}
}

func TestRetryWithBackoff_FirstTrySuccessNoDelay(t *testing.T) {
	start := time.Now()
	calls := 0
	ok := retryWithBackoff(context.Background(), time.Hour, time.Hour, func() bool {
		calls++
		return true
	})
	if !ok || calls != 1 {
		t.Fatalf("expected one call and success, got calls=%d ok=%v", calls, ok)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Error("a first-attempt success must not sleep the initial delay")
	}
}

func TestRetryWithBackoff_StopsOnCtxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	done := make(chan bool, 1)
	go func() {
		done <- retryWithBackoff(ctx, 10*time.Millisecond, time.Second, func() bool {
			calls++
			return false // never succeeds
		})
	}()
	time.Sleep(35 * time.Millisecond)
	cancel()

	select {
	case ok := <-done:
		if ok {
			t.Error("expected false when ctx is cancelled before fn ever succeeds")
		}
	case <-time.After(time.Second):
		t.Fatal("retryWithBackoff did not return after ctx cancel")
	}
	if calls < 2 {
		t.Errorf("expected several attempts before cancel, got %d", calls)
	}
}
