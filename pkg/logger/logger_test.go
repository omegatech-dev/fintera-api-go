package logger

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewHandlerEmitsJSONByDefault(t *testing.T) {
	t.Setenv("LOG_FORMAT", "")
	t.Setenv("LOG_FILE", "")

	handler := newHandler("production")

	if _, ok := handler.(*slog.JSONHandler); !ok {
		t.Fatalf("expected a JSON handler so severity is machine-readable, got %T", handler)
	}
}

func TestNewHandlerOptInText(t *testing.T) {
	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("LOG_FILE", "")

	handler := newHandler("development")

	if _, ok := handler.(*slog.TextHandler); !ok {
		t.Fatalf("expected a text handler when LOG_FORMAT=text, got %T", handler)
	}
}

// TestLogFileDuplicatesRecords covers the LOG_FILE sink used by `make dev`: the
// file must receive the same structured records stdout does, so a local run can
// be grepped after the terminal scrolled past it.
func TestLogFileDuplicatesRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.log")
	t.Setenv("LOG_FILE", path)
	t.Setenv("LOG_FORMAT", "")

	Setup("development")
	t.Cleanup(Close)

	Error("SQL Error 1.0ms: SELECT 1", slog.String("error", "boom"))
	Info("Incoming request", slog.Int("status", 500))

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading LOG_FILE: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 records in the log file, got %d: %q", len(lines), string(data))
	}

	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("log file must hold JSON records: %v", err)
	}
	if first["level"] != "ERROR" || first["msg"] != "SQL Error 1.0ms: SELECT 1" {
		t.Errorf("unexpected first record: %v", first)
	}
	if first["error"] != "boom" {
		t.Errorf("attributes must reach the log file, got %v", first)
	}
}

// TestLogFileAppendsAcrossSetup checks that reconfiguring keeps history instead
// of truncating the file, which matters for hot-reloading dev servers.
func TestLogFileAppendsAcrossSetup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.log")
	t.Setenv("LOG_FILE", path)

	Setup("development")
	Info("before restart")
	Close()

	Setup("development")
	Info("after restart")
	Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading LOG_FILE: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "before restart") || !strings.Contains(got, "after restart") {
		t.Errorf("LOG_FILE must append across Setup calls, got %q", got)
	}
}

// TestLogFileUnusablePathFallsBack keeps a bad LOG_FILE from taking the app down:
// stdout logging must continue.
func TestLogFileUnusablePathFallsBack(t *testing.T) {
	t.Setenv("LOG_FILE", filepath.Join(t.TempDir(), "missing-dir", "trace.log"))

	Setup("development")
	t.Cleanup(Close)

	// Must not panic and must still hand back a working handler.
	Info("still logging to stdout")

	if h := newHandler("development"); h == nil {
		t.Fatal("expected a usable handler when LOG_FILE cannot be opened")
	}
}

func TestLogLevelFromEnvironment(t *testing.T) {
	tests := map[string]slog.Level{
		"":        slog.LevelInfo,
		"debug":   slog.LevelDebug,
		"WARN":    slog.LevelWarn,
		" error ": slog.LevelError,
		"verbose": slog.LevelInfo,
	}

	for value, want := range tests {
		t.Setenv("LOG_LEVEL", value)
		if got := logLevel(); got != want {
			t.Errorf("LOG_LEVEL=%q: level = %v, want %v", value, got, want)
		}
	}
}
