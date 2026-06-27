package orchestra

import (
	"context"
	"errors"
	"testing"
)

func TestRetryTaskSuccess(t *testing.T) {
	c := NewConductor(DefaultConfig(), nil, nil)
	callCount := 0
	err := c.retryTask(context.Background(), "test", func() error {
		callCount++
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestRetryTaskAllFail(t *testing.T) {
	c := NewConductor(DefaultConfig(), nil, nil)
	callCount := 0
	err := c.retryTask(context.Background(), "test", func() error {
		callCount++
		return errors.New("persistent error")
	})
	if err == nil {
		t.Fatal("expected error after retries")
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls (1 + 2 retries), got %d", callCount)
	}
}

func TestCallWithRetryRateLimit(t *testing.T) {
	callCount := 0
	err := callWithRetry(context.Background(), "test", func() error {
		callCount++
		return errors.New("rate limit exceeded: try again in 1s")
	})
	if err == nil {
		t.Fatal("expected error after retries")
	}
	if callCount != 4 {
		t.Errorf("expected 4 calls (1 + 3 retries), got %d", callCount)
	}
}

func TestCallWithRetryNonRateLimit(t *testing.T) {
	callCount := 0
	err := callWithRetry(context.Background(), "test", func() error {
		callCount++
		return errors.New("non-retryable error")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call (no retry), got %d", callCount)
	}
}

func TestCallWithRetryContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := callWithRetry(ctx, "test", func() error {
		return errors.New("rate limit: try again in 1s")
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestIsRateLimitError(t *testing.T) {
	tests := []struct {
		err  string
		want bool
	}{
		{"rate_limit_exceeded", true},
		{"Rate limit hit", true},
		{"Too Many Requests", true},
		{"429 too many requests", true},
		{"503 service unavailable", true},
		{"slowdown detected", true},
		{"connection timeout", false},
		{"internal server error 500", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.err, func(t *testing.T) {
			got := isRateLimitError(errors.New(tt.err))
			if got != tt.want {
				t.Errorf("isRateLimitError(%q) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsRateLimitErrorNil(t *testing.T) {
	if isRateLimitError(nil) {
		t.Error("isRateLimitError(nil) should be false")
	}
}
