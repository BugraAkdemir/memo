// Package logx provides structured logging via log/slog with level support
// and a drop-in Printf-compatible API for gradual migration.
package logx

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Level represents log severity.
type Level = slog.Level

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

var (
	currentOutput io.Writer = os.Stderr
	currentLevel  Level     = LevelInfo
	logger        = slog.New(slog.NewTextHandler(currentOutput, &slog.HandlerOptions{Level: currentLevel}))
)

func rebuild() {
	logger = slog.New(slog.NewTextHandler(currentOutput, &slog.HandlerOptions{Level: currentLevel}))
}

// SetLevel changes the minimum log level at runtime.
func SetLevel(l Level) {
	currentLevel = l
	rebuild()
}

// SetOutput redirects all subsequent log output to w. Used by the terminal
// REPL to keep backend logs out of the interactive session — they'd
// otherwise interleave with the prompt on the same stdout/stderr terminal.
func SetOutput(w io.Writer) {
	currentOutput = w
	rebuild()
}

// SetDebug enables debug-level output.
func SetDebug() { SetLevel(LevelDebug) }

func Debug(msg string, args ...any)       { logger.Debug(msg, args...) }
func Info(msg string, args ...any)        { logger.Info(msg, args...) }
func Warn(msg string, args ...any)        { logger.Warn(msg, args...) }
func Error(msg string, args ...any)       { logger.Error(msg, args...) }
func DebugCtx(ctx context.Context, msg string, args ...any) { logger.DebugContext(ctx, msg, args...) }
func InfoCtx(ctx context.Context, msg string, args ...any)  { logger.InfoContext(ctx, msg, args...) }
func WarnCtx(ctx context.Context, msg string, args ...any)  { logger.WarnContext(ctx, msg, args...) }
func ErrorCtx(ctx context.Context, msg string, args ...any) { logger.ErrorContext(ctx, msg, args...) }

// Printf exists as a migration helper — replaces log.Printf calls with Info-level slog.
// Use it as a drop-in while migrating: s/log\.Printf/logx.Printf/
func Printf(format string, v ...interface{}) {
	logger.Info(format, "values", v)
}
