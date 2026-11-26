package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// current holds the configured logger. Access is atomic so Setup and logging
// can run concurrently (for example, tests that reconfigure logging while
// background jobs are still running).
var current atomic.Pointer[slog.Logger]

// logFile is the LOG_FILE sink opened by the most recent handler setup.
// logFileMu guards it so Setup, newHandler, and Close can run from different
// goroutines.
var (
	logFile   *os.File
	logFileMu sync.Mutex
)

// Setup initializes the global logger based on the environment. Deployed
// environments always emit structured JSON so the collector receives a true
// severity field; local text logs are opt-in with LOG_FORMAT=text, and LOG_FILE
// keeps a second, greppable copy of every record. Re-running Setup replaces the
// previously opened LOG_FILE sink.
func Setup(env string) {
	l := slog.New(newHandler(env))
	current.Store(l)
	slog.SetDefault(l)
}

func newHandler(_ string) slog.Handler {
	opts := &slog.HandlerOptions{
		Level: logLevel(),
	}

	out := logOutput()
	if os.Getenv("LOG_FORMAT") == "text" {
		return slog.NewTextHandler(out, opts)
	}
	return slog.NewJSONHandler(out, opts)
}

// logOutput returns where records are written. Records always reach stdout; when
// LOG_FILE names a usable path they are duplicated into that file, so a local
// run can be grepped after the terminal has scrolled past it. An unusable path
// warns on stderr and degrades to stdout only.
func logOutput() io.Writer {
	path := strings.TrimSpace(os.Getenv("LOG_FILE"))
	if path == "" {
		return os.Stdout
	}

	logFileMu.Lock()
	defer logFileMu.Unlock()

	closeFile()

	// LOG_FILE is operator-supplied startup configuration at the same trust level
	// as DATABASE_URL or STORAGE_PATH: it never carries request data, so the
	// file-inclusion and traversal rules do not apply. The path is still cleaned,
	// and the file is created owner-read/write only, because captured records can
	// contain user IDs and route paths.
	// #nosec G304 G703
	opened, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: LOG_FILE %q is not usable (%v); logging to stdout only\n", path, err)
		return os.Stdout
	}
	logFile = opened

	// stdout stays first: if the file is closed later, records keep reaching it.
	return io.MultiWriter(os.Stdout, opened)
}

// Close releases the LOG_FILE sink. Logging keeps working afterwards, writing to
// stdout only.
func Close() {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	closeFile()
}

// closeFile closes the tracked sink. Callers must hold logFileMu.
func closeFile() {
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}

// logLevel reads LOG_LEVEL (debug, info, warn, error). Unset or invalid values
// keep the info default; routine SQL and per-run job details log at debug.
func logLevel() slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.TrimSpace(os.Getenv("LOG_LEVEL")))); err != nil {
		return slog.LevelInfo
	}
	return level
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
