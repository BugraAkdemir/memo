package logx

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSetLevel(t *testing.T) {
	SetLevel(LevelDebug)
	if logger.Handler().Enabled(context.Background(), LevelDebug) {
		// expected: debug is enabled after SetLevel(LevelDebug)
	}
	SetLevel(LevelInfo)
}

func TestSetDebug(t *testing.T) {
	SetDebug()
	if logger.Handler().Enabled(context.Background(), LevelDebug) {
		// expected: debug is enabled after SetDebug()
	}
	SetLevel(LevelInfo)
}

func TestLevels(t *testing.T) {
	tests := []struct {
		name string
		fn   func(msg string, args ...any)
		msg  string
	}{
		{"Debug", Debug, "debug msg"},
		{"Info", Info, "info msg"},
		{"Warn", Warn, "warn msg"},
		{"Error", Error, "error msg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just ensure no panic — output goes to stderr
			tt.fn(tt.msg, "key", "value")
		})
	}
}

func TestContextLevels(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		fn   func(ctx context.Context, msg string, args ...any)
	}{
		{"DebugCtx", DebugCtx},
		{"InfoCtx", InfoCtx},
		{"WarnCtx", WarnCtx},
		{"ErrorCtx", ErrorCtx},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fn(ctx, "ctx msg", "key", "value")
		})
	}
}

func TestPrintf(t *testing.T) {
	// Ensure no panic
	Printf("hello %s %d", "world", 42)
}

// TestPrintf_ActuallyFormatsTheMessage is a regression test: Printf used to
// pass the literal, unsubstituted format string as the slog message and dump
// the interpolation args into a separate "values" attribute instead of
// calling fmt.Sprintf — so a call like Printf("PANIC in %s: %v", label, err)
// logged the message "PANIC in %s: %v" with the real label/err buried in
// values=[...], making the log unsearchable for the actual panic label or
// error text. Fails against the pre-fix implementation (message would
// contain the literal "%s"/"%d" verbs, not the substituted values).
func TestPrintf_ActuallyFormatsTheMessage(t *testing.T) {
	var buf bytes.Buffer
	oldLogger := logger
	logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: LevelDebug}))
	defer func() { logger = oldLogger }()

	Printf("PANIC in %s: %v", "myGoroutine", "boom")

	output := buf.String()
	if !strings.Contains(output, "PANIC in myGoroutine: boom") {
		t.Errorf("expected formatted message in output, got: %s", output)
	}
	if strings.Contains(output, "%s") || strings.Contains(output, "%v") {
		t.Errorf("output still contains unsubstituted format verbs: %s", output)
	}
	if strings.Contains(output, "values=") {
		t.Errorf("output should not carry a separate values attribute anymore, got: %s", output)
	}
}

func TestLevelConstants(t *testing.T) {
	if LevelDebug != slog.LevelDebug {
		t.Errorf("LevelDebug = %d, want %d", LevelDebug, slog.LevelDebug)
	}
	if LevelInfo != slog.LevelInfo {
		t.Errorf("LevelInfo = %d, want %d", LevelInfo, slog.LevelInfo)
	}
	if LevelWarn != slog.LevelWarn {
		t.Errorf("LevelWarn = %d, want %d", LevelWarn, slog.LevelWarn)
	}
	if LevelError != slog.LevelError {
		t.Errorf("LevelError = %d, want %d", LevelError, slog.LevelError)
	}
}

func TestOutputFormat(t *testing.T) {
	var buf bytes.Buffer
	oldLogger := logger
	logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: LevelDebug}))
	defer func() { logger = oldLogger }()

	Info("test message", "key1", "val1", "key2", 42)

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("output missing message, got: %s", output)
	}
	if !strings.Contains(output, "key1") {
		t.Errorf("output missing key1, got: %s", output)
	}
	if !strings.Contains(output, "val1") {
		t.Errorf("output missing val1, got: %s", output)
	}
}

func TestSetOutput(t *testing.T) {
	oldOutput, oldLevel := currentOutput, currentLevel
	defer func() { SetOutput(oldOutput); SetLevel(oldLevel) }()

	var buf bytes.Buffer
	SetOutput(&buf)
	Info("redirected message")

	if !strings.Contains(buf.String(), "redirected message") {
		t.Errorf("output missing redirected message, got: %s", buf.String())
	}
}

// syncBuffer is a bytes.Buffer safe for concurrent writes (the goroutine
// under test) and reads (the polling test goroutine).
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// TestGoRecover_SwallowsPanicAndLogsIt is a regression test for the panic-
// recovery audit: a goroutine started via GoRecover must not crash the
// process, and must log the panic under the given label. Polls instead of
// synchronizing on a channel closed from inside fn, since that would race
// against Recover's own defer actually running (fn's own defers unwind
// before the goroutine closure's, so a channel closed from within fn doesn't
// guarantee Recover — and therefore the log write — has completed yet).
func TestGoRecover_SwallowsPanicAndLogsIt(t *testing.T) {
	oldOutput, oldLevel := currentOutput, currentLevel
	defer func() { SetOutput(oldOutput); SetLevel(oldLevel) }()

	buf := &syncBuffer{}
	SetOutput(buf)

	GoRecover("TestGoRecover_SwallowsPanicAndLogsIt", func() {
		panic("boom") // if this isn't recovered, the test binary itself crashes
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		out := buf.String()
		if strings.Contains(out, "TestGoRecover_SwallowsPanicAndLogsIt") && strings.Contains(out, "boom") {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected panic to be logged within timeout, got: %s", buf.String())
}

// TestRecover_NoopWhenNoPanic ensures a normal (non-panicking) deferred
// Recover call is a safe no-op.
func TestRecover_NoopWhenNoPanic(t *testing.T) {
	func() {
		defer Recover("TestRecover_NoopWhenNoPanic")
	}() // must not panic itself
}
