package handlers_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"systemburo/internal/database"
	"systemburo/internal/logmask"
	"systemburo/internal/models"
	"systemburo/internal/services"
)

type quotaDiagnosticSettings struct {
	services.SettingsService
	value models.ArchiveSettings
}

func (s *quotaDiagnosticSettings) GetArchiveSettings(context.Context) (*models.ArchiveSettings, error) {
	v := s.value
	return &v, nil
}

func TestFileArchiveQuota_OptionalFlagsDiagnostics(t *testing.T) {
	w := setupQuotaWorld(t)
	var out bytes.Buffer
	safe := database.NewSafeLogger(slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{ReplaceAttr: logmask.ReplaceAttr})), logger.Info)
	db := w.db.Session(&gorm.Session{Logger: safe})
	settings := &quotaDiagnosticSettings{value: models.ArchiveSettings{Enabled: true}}
	makeQuota := func() *services.BlankExportQuotaService {
		return services.NewBlankExportQuotaService(db, settings, nil, nil, services.NewAuditRecorder(db), w.archive, w.archive, "")
	}
	q := makeQuota()
	ctx := context.Background()
	first, err := q.EnforceThresholds(ctx)
	require.NoError(t, err)
	require.False(t, first.HardTripped)
	require.False(t, first.SoftTripped)
	require.NotContains(t, out.String(), `"level":"ERROR"`, "initial absence of both flags is normal")
	var count int64
	require.NoError(t, db.Model(&models.SystemSetting{}).Where("key IN ?", []string{archiveWarnFlagSettingKey, archiveBlockFlagSettingKey}).Count(&count).Error)
	require.Zero(t, count, "normal startup must not create flags just to silence logs")
	settings.value.MinFreeBytes = quotaImpossibleFreeBytes
	blocked, err := q.EnforceThresholds(ctx)
	require.NoError(t, err)
	require.True(t, blocked.HardTripped)
	require.NotContains(t, out.String(), `"level":"ERROR"`, "first creation of the marker is also normal")
	var flag models.SystemSetting
	require.NoError(t, db.Where("key = ?", archiveBlockFlagSettingKey).First(&flag).Error)
	require.Equal(t, "true", flag.Value)
	settings.value.MinFreeBytes = 0
	_, err = q.EnforceThresholds(ctx)
	require.NoError(t, err)
	require.NoError(t, db.Where("key = ?", archiveBlockFlagSettingKey).First(&flag).Error)
	require.Equal(t, "false", flag.Value)
	_, err = makeQuota().EnforceThresholds(ctx)
	require.NoError(t, err)
	require.NotContains(t, out.String(), `"level":"ERROR"`)
	// Fail result handling after SQL ran, so GORM emits a database trace.
	// A callback before query construction has no SQL operation to trace.
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("quota:diagnostic_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "system_settings" {
			tx.AddError(errors.New("PRIVATE-INJECTED-DB-FAILURE"))
		}
	}))
	defer db.Callback().Query().Remove("quota:diagnostic_failure")
	out.Reset()
	_, _ = makeQuota().EnforceThresholds(ctx)
	require.Contains(t, out.String(), `"level":"ERROR"`)
	require.NotContains(t, out.String(), "PRIVATE-INJECTED")
}
