package database

import (
	"context"
	"log/slog"
	"time"

	"systemburo/internal/httpx"
	"systemburo/internal/logmask"

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
		attrs = append(attrs, logmask.ReplaceAttr(nil, slog.Any("error", err)))
	}
	l.log.LogAttrs(ctx, level, "database operation", attrs...)
}
