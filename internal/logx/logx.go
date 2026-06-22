// Package logx provides structured logging via log/slog with level support
// and a drop-in Printf-compatible API for gradual migration.
package logx

import (
	"context"
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

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: LevelInfo}))

// SetLevel changes the minimum log level at runtime.
func SetLevel(l Level) {
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l}))
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
