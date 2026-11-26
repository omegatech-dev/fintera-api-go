package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func captureLogs(t *testing.T, fn func()) []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	previous := current.Load()
	current.Store(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { current.Store(previous) })

	fn()

	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("invalid JSON log line %q: %v", line, err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func TestGormLoggerSlowSQLIncludesDurationAndQuery(t *testing.T) {
	l := NewGormLogger(logger.Info, 200*time.Millisecond)

	entries := captureLogs(t, func() {
		l.Trace(context.Background(), time.Now().Add(-500*time.Millisecond), func() (string, int64) {
			return "SELECT * FROM users WHERE id = $1", 1
		}, nil)
	})

	if len(entries) != 1 {
		t.Fatalf("expected one log entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry["level"] != "WARN" {
		t.Fatalf("level = %v, want WARN", entry["level"])
	}
	msg, _ := entry["msg"].(string)
	if !strings.HasPrefix(msg, "Slow SQL ") || !strings.Contains(msg, "ms: SELECT * FROM users WHERE id = $1") {
		t.Fatalf("message should carry duration and query, got %q", msg)
	}
	if ms, ok := entry["durationMs"].(float64); !ok || ms < 500 {
		t.Fatalf("durationMs = %v, want >= 500", entry["durationMs"])
	}
	if _, ok := entry["elapsed"]; ok {
		t.Fatal("raw elapsed attribute should be replaced by durationMs")
	}
}

func TestGormLoggerParamsFilterKeepsBindValuesOut(t *testing.T) {
	l := NewGormLogger(logger.Info, 200*time.Millisecond)

	sql, vars := l.ParamsFilter(context.Background(), "SELECT * FROM users WHERE email = $1", "josej@example.com")
	if sql != "SELECT * FROM users WHERE email = $1" {
		t.Fatalf("sql = %q, want parameterized statement", sql)
	}
	if vars != nil {
		t.Fatalf("vars = %v, want nil so bind values are not interpolated", vars)
	}
}

func TestGormLoggerErrorIncludesQuery(t *testing.T) {
	l := NewGormLogger(logger.Error, 200*time.Millisecond)

	entries := captureLogs(t, func() {
		l.Trace(context.Background(), time.Now(), func() (string, int64) {
			return "UPDATE contracts SET status = $1", 0
		}, errors.New("boom"))
	})

	if len(entries) != 1 || entries[0]["level"] != "ERROR" {
		t.Fatalf("expected one ERROR entry, got %v", entries)
	}
	if msg, _ := entries[0]["msg"].(string); !strings.HasPrefix(msg, "SQL Error ") || !strings.Contains(msg, "UPDATE contracts") {
		t.Fatalf("message should carry the query, got %q", msg)
	}
	if entries[0]["error"] != "boom" {
		t.Fatalf("error attribute = %v, want boom", entries[0]["error"])
	}
}

func TestGormLoggerClientCanceledIsNotAnError(t *testing.T) {
	l := NewGormLogger(logger.Error, 200*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	entries := captureLogs(t, func() {
		l.Trace(ctx, time.Now(), func() (string, int64) {
			return `SELECT * FROM "projects" WHERE guid = $1`, 0
		}, fmt.Errorf("query failed: %w", context.Canceled))
	})

	if len(entries) != 1 {
		t.Fatalf("expected one log entry, got %v", entries)
	}
	if entries[0]["level"] != "DEBUG" {
		t.Fatalf("an aborted client must not log at %v", entries[0]["level"])
	}
	if msg, _ := entries[0]["msg"].(string); msg != "SQL client canceled" {
		t.Fatalf("msg = %q, want no SQL Error label for a canceled request", msg)
	}
}

func TestGormLoggerRoutineSQLIsDebug(t *testing.T) {
	l := NewGormLogger(logger.Info, time.Hour)

	entries := captureLogs(t, func() {
		l.Trace(context.Background(), time.Now(), func() (string, int64) {
			return "SELECT 1", 1
		}, nil)
	})

	if len(entries) != 1 || entries[0]["level"] != "DEBUG" {
		t.Fatalf("routine SQL should log at DEBUG, got %v", entries)
	}
}

func TestSanitizeSQL(t *testing.T) {
	long := strings.Repeat("x", maxLoggedSQLLength+10)
	got := sanitizeSQL(long)
	if len(got) != maxLoggedSQLLength+3 || !strings.HasSuffix(got, "...") {
		t.Fatalf("sanitizeSQL length = %d, want %d with ellipsis", len(got), maxLoggedSQLLength+3)
	}
	if sanitizeSQL("SELECT 1") != "SELECT 1" {
		t.Fatal("short SQL should be unchanged")
	}
	if got := sanitizeSQL(`WHERE email = $1$ AND id = $2$`); got != `WHERE email = $1 AND id = $2` {
		t.Fatalf("placeholders not restored: %q", got)
	}
}

type loggingProbeUser struct {
	ID    uint
	Email string
}

func TestGormPipelineDoesNotLogBindValues(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=127.0.0.1 port=1 user=probe password=probe dbname=probe sslmode=disable",
	}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger:               NewGormLogger(logger.Info, time.Nanosecond),
	})
	if err != nil {
		t.Fatal(err)
	}

	var users []loggingProbeUser
	entries := captureLogs(t, func() {
		db.Where("email = ? AND id = ?", "josej@example.com", 7).Find(&users)
	})

	if len(entries) == 0 {
		t.Fatal("expected a SQL log entry")
	}
	sql, _ := entries[0]["sql"].(string)
	if strings.Contains(sql, "josej@example.com") || strings.Contains(sql, "$1$") {
		t.Fatalf("bind values leaked or placeholders malformed: %q", sql)
	}
	if !strings.Contains(sql, "$1") || !strings.Contains(sql, "$2") {
		t.Fatalf("expected parameterized SQL, got %q", sql)
	}
}
