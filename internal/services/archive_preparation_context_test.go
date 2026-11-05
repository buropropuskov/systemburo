package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type archivePreparationSettings struct {
	SettingsService
	seen context.Context
}

func (s *archivePreparationSettings) GetArchiveSettings(ctx context.Context) (*models.ArchiveSettings, error) {
	s.seen = ctx
	return &models.ArchiveSettings{Enabled: false}, nil
}

func assertArchivePreparationDeadline(t *testing.T, ctx context.Context) {
	t.Helper()
	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	require.LessOrEqual(t, time.Until(deadline), 120*time.Second)
}

func TestArchiveQueuePreparationTimerEndsWithoutCancelingWorker(t *testing.T) {
	settings := &archivePreparationSettings{}
	svc := &BlankExportService{settings: settings, outbox: NewArchiveExportOutbox(nil), writer: &ArchiveWriter{}}
	parent := context.Background()
	processed, failed := svc.ProcessQueue(parent)
	require.Zero(t, processed)
	require.Zero(t, failed)
	require.NotNil(t, settings.seen)
	assertArchivePreparationDeadline(t, settings.seen)
	require.ErrorIs(t, settings.seen.Err(), context.Canceled)
	require.NoError(t, parent.Err())
	require.False(t, svc.processing.Load())
}

func TestArchiveFailedJobPersistsBackoffWithFreshParentContext(t *testing.T) {
	var jobContext, retryContext context.Context
	db := loginBudgetFaultDB(t, func(tx *gorm.DB) {
		jobContext = tx.Statement.Context
		assertArchivePreparationDeadline(t, jobContext)
		tx.AddError(context.DeadlineExceeded)
	})
	require.NoError(t, db.Callback().Update().Replace("gorm:update", func(tx *gorm.DB) {
		retryContext = tx.Statement.Context
		assertArchivePreparationDeadline(t, retryContext)
		require.ErrorIs(t, jobContext.Err(), context.Canceled, "job phase ends before retry SQL starts")
		require.NoError(t, retryContext.Err(), "retry must not inherit the ended job phase")
		tx.RowsAffected = 1
	}))
	svc := &BlankExportService{db: db, outbox: NewArchiveExportOutbox(db)}
	processed, failed, stop := svc.processArchiveJob(context.Background(), models.ArchiveExportJob{ApplicationID: 1, Revision: "synthetic"})
	require.Zero(t, processed)
	require.Equal(t, 1, failed)
	require.False(t, stop)
	require.NotNil(t, retryContext)
	require.NotEqual(t, jobContext, retryContext)
	require.ErrorIs(t, retryContext.Err(), context.Canceled, "settlement releases its own timer")
}

func TestArchiveSettlementPreservesShutdownAndReleasesTimerOnError(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	job, cancelJob := ResourcePreparationContext(parent)
	cancelJob()
	require.ErrorIs(t, job.Err(), context.Canceled)
	var settlement context.Context
	sentinel := errors.New("synthetic settlement failure")
	err := settleArchiveJob(parent, func(ctx context.Context) error {
		settlement = ctx
		require.NoError(t, ctx.Err())
		assertArchivePreparationDeadline(t, ctx)
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)
	require.ErrorIs(t, settlement.Err(), context.Canceled)
	cancelParent()
	called := false
	err = settleArchiveJob(parent, func(context.Context) error { called = true; return nil })
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, called, "shutdown must not start detached settlement")
}

func TestArchivePeriodSQLUsesBoundedPreparationAndKeepsStreamParent(t *testing.T) {
	db := loginBudgetFaultDB(t, func(tx *gorm.DB) { tx.AddError(errors.New("unexpected query")) })
	var sqlContext context.Context
	sentinel := errors.New("synthetic preparation query failure")
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register("archive-preparation-context", func(tx *gorm.DB) {
		sqlContext = tx.Statement.Context
		assertArchivePreparationDeadline(t, sqlContext)
		tx.AddError(sentinel) // Stop before the driver; this fixture has no database.
	}))
	svc := &ArchiveDownloadService{db: db}
	parent := context.Background()
	from := time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC)
	_, err := svc.periodEntries(parent, from, from.AddDate(0, 0, 1))
	require.Error(t, err)
	require.NotNil(t, sqlContext)
	require.ErrorIs(t, sqlContext.Err(), context.Canceled)
	require.NoError(t, parent.Err(), "SQL preparation must not cancel the later HTTP stream")
	_, inherited := parent.Deadline()
	require.False(t, inherited)
}
