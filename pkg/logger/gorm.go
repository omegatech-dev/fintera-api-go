package logger

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type GormLogger struct {
	LogLevel      logger.LogLevel
	SlowThreshold time.Duration
}

func NewGormLogger(logLevel logger.LogLevel, slowThreshold time.Duration) *GormLogger {
	return &GormLogger{
		LogLevel:      logLevel,
		SlowThreshold: slowThreshold,
	}
}

func (l *GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		Info(fmt.Sprintf(msg, data...))
	}
}

func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		Warn(fmt.Sprintf(msg, data...))
	}
}

func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		Error(fmt.Sprintf(msg, data...))
	}
}

// isRecordNotFound reports whether a query failed only because no row matched.
// Those lookups are normal control flow (cache misses, optional relations) and
// must not be reported as SQL errors.
func isRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// isClientCanceled reports whether a query stopped because the caller hung up:
// an aborted browser request or a proxy that gave up upstream. Like a cache miss
// it is normal control flow, and no client is left to observe a failure, so it
// must not be reported as an SQL error either.
func isClientCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}

// ParamsFilter stops GORM from interpolating bind values into the statement it
// logs. Values such as emails or names must never reach the log; the logged SQL
// keeps its $n placeholders instead.
func (l *GormLogger) ParamsFilter(_ context.Context, sql string, _ ...interface{}) (string, []interface{}) {
	return sql, nil
}

const maxLoggedSQLLength = 1000

var gormPlaceholder = regexp.MustCompile(`\$(\d+)\$`)

// sanitizeSQL restores the $n placeholders that GORM wraps while explaining a
// statement and bounds the length of what is logged.
func sanitizeSQL(sql string) string {
	sql = gormPlaceholder.ReplaceAllString(sql, "$$$1")
	if len(sql) <= maxLoggedSQLLength {
		return sql
	}
	return sql[:maxLoggedSQLLength] + "..."
}

// sqlMessage puts the duration and the query into the message itself, so the
// line is readable even when only the message text is displayed.
func sqlMessage(prefix string, elapsed time.Duration, sql string) string {
	return fmt.Sprintf("%s %.1fms: %s", prefix, float64(elapsed)/float64(time.Millisecond), sql)
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()
	sql = sanitizeSQL(sql)

	fields := []any{
		slog.String("sql", sql),
		slog.Int64("rows", rows),
		slog.Float64("durationMs", float64(elapsed)/float64(time.Millisecond)),
	}

	if err != nil {
		if isClientCanceled(err) {
			Debug("SQL client canceled", fields...)
			return
		}
		if isRecordNotFound(err) {
			Debug("SQL record not found", fields...)
			return
		}
		if l.LogLevel >= logger.Error {
			fields = append(fields, slog.String("error", err.Error()))
			Error(sqlMessage("SQL Error", elapsed, sql), fields...)
		}
		return
	}

	if l.SlowThreshold != 0 && elapsed > l.SlowThreshold && l.LogLevel >= logger.Warn {
		Warn(sqlMessage("Slow SQL", elapsed, sql), fields...)
		return
	}

	if l.LogLevel >= logger.Info {
		Debug(sqlMessage("SQL", elapsed, sql), fields...)
	}
}
