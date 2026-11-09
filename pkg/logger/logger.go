package logger

import (
	"log/slog"
	"os"
	"sync/atomic"
)

// current holds the configured logger. Access is atomic so Setup and logging
// can run concurrently (for example, tests that reconfigure logging while
// background jobs are still running).
var current atomic.Pointer[slog.Logger]

// Setup initializes the global logger based on the environment
func Setup(env string) {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	l := slog.New(handler)
	current.Store(l)
	slog.SetDefault(l)
}

func get() *slog.Logger {
	if l := current.Load(); l != nil {
		return l
	}
	return slog.Default()
}

// Info logs an info message
func Info(msg string, args ...any) {
	get().Info(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	get().Error(msg, args...)
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	get().Debug(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	get().Warn(msg, args...)
}
