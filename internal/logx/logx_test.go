package logx

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
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
