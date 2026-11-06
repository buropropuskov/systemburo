package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"time"

	"systemburo/internal/httpx"
	"systemburo/internal/logmask"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type safeLogger struct {
	log   *slog.Logger
	level logger.LogLevel
}

// NewSafeLogger emits structured GORM diagnostics without SQL, arguments or
// database error text. Its immutable LogMode copies are safe to share.
func NewSafeLogger(log *slog.Logger, level logger.LogLevel) logger.Interface {
	if log == nil {
		log = slog.Default()
	}
	return &safeLogger{log: log, level: level}
}

func (l *safeLogger) LogMode(level logger.LogLevel) logger.Interface {
	copy := *l
	copy.level = level
	return &copy
}

func (l *safeLogger) Info(ctx context.Context, _ string, _ ...interface{}) {
	if l.level >= logger.Info {
		l.log.InfoContext(ctx, "database info")
	}
}

func (l *safeLogger) Warn(ctx context.Context, _ string, _ ...interface{}) {
	if l.level >= logger.Warn {
		l.log.WarnContext(ctx, "database warning")
	}
}

func (l *safeLogger) Error(ctx context.Context, _ string, _ ...interface{}) {
	if l.level >= logger.Error {
		l.log.ErrorContext(ctx, "database error")
	}
}

func (l *safeLogger) Trace(ctx context.Context, begin time.Time, _ func() (string, int64), err error) {
	if l.level == logger.Silent {
		return
	}
	level := slog.LevelInfo
	elapsed := time.Since(begin)
	if err != nil {
		if l.level < logger.Error {
			return
		}
		level = slog.LevelError
	} else if elapsed > 200*time.Millisecond && l.level >= logger.Warn {
		level = slog.LevelWarn
	} else if l.level < logger.Info {
		return
	}
	attrs := []slog.Attr{slog.Duration("duration", elapsed)}
	if id := httpx.RequestID(ctx); id != "" {
		attrs = append(attrs, slog.String("request_id", id))
	}
	if err != nil {
		class := logmask.ErrorClass(err)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			class = "not_found"
		}
		attrs = append(attrs, slog.String("error_type", fmt.Sprintf("%T", err)), slog.String("error_class", class))
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr != nil && validSQLState(pgErr.Code) {
			attrs = append(attrs, slog.String("error_code", pgErr.Code))
		}
		if caller := databaseCaller(); caller != "" {
			attrs = append(attrs, slog.String("db_caller", caller))
		}
	}
	l.log.LogAttrs(ctx, level, "database operation", attrs...)
}

func validSQLState(code string) bool {
	if len(code) != 5 {
		return false
	}
	for _, ch := range code {
		if !(ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9') {
			return false
		}
	}
	return true
}

// Only compiled function names and line numbers are emitted, never local paths.
func databaseCaller() string {
	var pcs [32]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.HasPrefix(frame.Function, "systemburo/") &&
			!strings.HasPrefix(frame.Function, "systemburo/internal/database.(*safeLogger).") {
			return fmt.Sprintf("%s:%d", strings.TrimPrefix(frame.Function, "systemburo/"), frame.Line)
		}
		if !more {
			return ""
		}
	}
}
