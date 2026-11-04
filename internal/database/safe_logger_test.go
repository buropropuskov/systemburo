package database

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"systemburo/internal/httpx"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm/logger"
)

func TestSafeLoggerNeverEmitsSQLOrDatabaseDetails(t *testing.T) {
	var out bytes.Buffer
	log := NewSafeLogger(slog.New(slog.NewJSONHandler(&out, nil)), logger.Info)
	ctx := context.Background()
	log.Info(ctx, "PRIVATE-QUERY", "PRIVATE-PARAMETER")
	log.Warn(ctx, "PRIVATE-QUERY", "PRIVATE-PARAMETER")
	log.Error(ctx, "PRIVATE-QUERY", "PRIVATE-PARAMETER")
	dbError := &pgconn.PgError{Code: "23505", Message: "PRIVATE-MESSAGE", Detail: "PRIVATE-DETAIL"}
	log.Trace(ctx, time.Now(), func() (string, int64) {
		t.Fatal("SQL interpolation callback must not be evaluated")
		return "SELECT 'PRIVATE-SQL'", 1
	}, dbError)
	if strings.Contains(out.String(), "PRIVATE-") {
		t.Fatal("database details appeared in logs")
	}
	for _, safe := range []string{"database operation", "duration", "*pgconn.PgError/other"} {
		if !strings.Contains(out.String(), safe) {
			t.Fatalf("safe diagnostic %q missing", safe)
		}
	}
}

func TestSafeLoggerLevelsAndImmutableLogMode(t *testing.T) {
	for _, tc := range []struct {
		level logger.LogLevel
		count int
	}{{logger.Silent, 0}, {logger.Error, 2}, {logger.Warn, 3}, {logger.Info, 5}} {
		var out bytes.Buffer
		log := NewSafeLogger(slog.New(slog.NewJSONHandler(&out, nil)), tc.level)
		ctx := context.Background()
		log.Info(ctx, "ignored")
		log.Warn(ctx, "ignored")
		log.Error(ctx, "ignored")
		trace := func() (string, int64) {
			t.Fatal("SQL callback must remain unused")
			return "", 0
		}
		log.Trace(ctx, time.Now(), trace, nil)
		log.Trace(ctx, time.Now(), trace, &pgconn.PgError{Message: "PRIVATE-ERROR"})
		if got := strings.Count(out.String(), "\n"); got != tc.count {
			t.Fatalf("GORM level %d emitted %d records, want %d", tc.level, got, tc.count)
		}
	}

	var out bytes.Buffer
	base := NewSafeLogger(slog.New(slog.NewJSONHandler(&out, nil)), logger.Silent)
	enabled := base.LogMode(logger.Info)
	base.Info(context.Background(), "ignored")
	enabled.Info(context.Background(), "ignored")
	if strings.Count(out.String(), "\n") != 1 {
		t.Fatal("LogMode mutated the original logger")
	}
}

func TestSafeLoggerRetainsSlowOperationWarning(t *testing.T) {
	var out bytes.Buffer
	log := NewSafeLogger(slog.New(slog.NewJSONHandler(&out, nil)), logger.Warn)
	log.Trace(context.Background(), time.Now().Add(-time.Second), func() (string, int64) {
		t.Fatal("SQL callback must remain unused")
		return "", 0
	}, nil)
	if !strings.Contains(out.String(), "WARN") || !strings.Contains(out.String(), "duration") {
		t.Fatal("slow database operation warning missing")
	}
}

func TestSafeLoggerCarriesRequestCorrelation(t *testing.T) {
	var out bytes.Buffer
	log := NewSafeLogger(slog.New(slog.NewJSONHandler(&out, nil)), logger.Info)
	ctx := httpx.WithRequestID(context.Background(), "258cc566-d739-4af0-bcf4-6ed8b1771090")
	log.Trace(ctx, time.Now(), nil, nil)
	if !strings.Contains(out.String(), "258cc566-d739-4af0-bcf4-6ed8b1771090") {
		t.Fatal("request correlation missing from database diagnostic")
	}
}
