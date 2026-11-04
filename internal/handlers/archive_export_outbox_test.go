package handlers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestArchiveExportOutbox_DurableRevisionAndSuppression(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	ctx := context.Background()
	store := services.NewArchiveExportOutbox(db)
	require.NoError(t, store.Enqueue(ctx, nil, "empty"))
	require.NoError(t, store.Enqueue(ctx, []int{12, 11, 12, 0, -1}, "first"))
	jobs, err := services.NewArchiveExportOutbox(db).ListDue(ctx, 200)
	require.NoError(t, err)
	require.Len(t, jobs, 2, "новый store читает durable задания, ID дедуплицированы")
	require.Equal(t, 11, jobs[0].ApplicationID)
	old := jobs[0]
	require.NoError(t, store.Enqueue(ctx, []int{11}, "changed"))
	require.NoError(t, store.Retry(ctx, old))
	require.NoError(t, store.Ack(ctx, old))
	var latest models.ArchiveExportJob
	require.NoError(t, db.First(&latest, "application_id = ?", 11).Error)
	require.NotEqual(t, old.Revision, latest.Revision)
	require.Zero(t, latest.Attempts)
	require.Equal(t, "changed", latest.Reason)
	// DELETE/INSERT не должен возвращать старому ACK право удалить новую запись.
	require.NoError(t, store.Ack(ctx, latest))
	require.NoError(t, store.Enqueue(ctx, []int{11}, "recreated"))
	require.NoError(t, store.Ack(ctx, latest))
	require.NoError(t, db.First(&latest, "application_id = ?", 11).Error)
	require.NoError(t, store.Suppress(ctx, 11))
	require.NoError(t, store.Enqueue(ctx, []int{11}, "must_not_revive"))
	require.NoError(t, store.Ack(ctx, latest))
	require.NoError(t, store.Retry(ctx, latest))
	require.NoError(t, db.First(&latest, "application_id = ?", 11).Error)
	require.Equal(t, models.ArchiveExportJobSuppressed, latest.State)
	terminalRevision := latest.Revision
	require.NoError(t, store.Suppress(ctx, 11))
	require.NoError(t, db.First(&latest, "application_id = ?", 11).Error)
	require.NotEqual(t, terminalRevision, latest.Revision)
	suppressed, err := store.IsSuppressed(ctx, 11)
	require.NoError(t, err)
	require.True(t, suppressed)
	jobs, err = store.ListDue(ctx, 200)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, 12, jobs[0].ApplicationID)
}

func TestArchiveExportOutbox_TransactionAndSelection(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	ctx := context.Background()
	store := services.NewArchiveExportOutbox(db)
	rollback := errors.New("synthetic rollback")
	err := db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, store.EnqueueTx(ctx, tx, []int{21, 22}, "rollback"))
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	jobs, err := store.ListDue(ctx, 200)
	require.NoError(t, err)
	require.Empty(t, jobs)
	require.NoError(t, store.Suppress(ctx, 22))
	count, err := store.EnqueueSelectionTx(ctx, db,
		"SELECT n AS application_id FROM generate_series(21, 23) n UNION ALL SELECT 21", nil, "selection")
	require.NoError(t, err)
	require.EqualValues(t, 2, count, "suppressed не считается поставленным; повторы схлопываются")
	jobs, err = store.ListDue(ctx, 200)
	require.NoError(t, err)
	require.Len(t, jobs, 2)
	require.NoError(t, store.Retry(ctx, jobs[0]))
	var retried models.ArchiveExportJob
	require.NoError(t, db.First(&retried, "application_id = ?", jobs[0].ApplicationID).Error)
	require.Equal(t, 1, retried.Attempts)
	require.True(t, retried.NextAttemptAt.After(retried.QueuedAt))
	jobs, err = store.ListDue(ctx, 200)
	require.NoError(t, err)
	require.Len(t, jobs, 1, "отложенный retry пока не выбирается")
}

func TestArchiveExportOutbox_RecheckPagesResumeAndRollback(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	ctx := context.Background()
	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Реестр без applications проверяет вторую половину UNION и её upper bound.
	require.NoError(t, db.Exec(`INSERT INTO blank_exports (application_id, attachment_id, bucket_date, queued_at)
		SELECT n, 0, ?, ? FROM generate_series(1, 2005) n`, cutoff, cutoff).Error)
	require.NoError(t, db.Exec("INSERT INTO applications (id, sending_datetime) VALUES (2006, ?)", cutoff).Error)
	store := services.NewArchiveExportOutbox(db)
	require.NoError(t, store.BeginRecheck(ctx, cutoff))
	// Новый ID после фиксации upper bound будет покрыт следующим проходом.
	require.NoError(t, db.Exec("INSERT INTO blank_exports (application_id, attachment_id, bucket_date, queued_at) VALUES (3000, 0, ?, ?)", cutoff, cutoff).Error)
	count, err := store.AdvanceRecheck(ctx, 5000)
	require.NoError(t, err)
	require.Equal(t, 2000, count, "размер страницы ограничен даже при большом limit")
	var state models.ArchiveExportScanState
	require.NoError(t, db.First(&state, 1).Error)
	require.Equal(t, 2000, state.AfterID)
	require.Equal(t, 2006, state.UpperID)
	require.True(t, state.Active)
	due, err := store.ListDue(ctx, 5000)
	require.NoError(t, err)
	require.Len(t, due, 200, "воркер читает ограниченную пачку")
	restarted := services.NewArchiveExportOutbox(db)
	require.NoError(t, restarted.BeginRecheck(ctx, cutoff.AddDate(1, 0, 0)))
	// Ошибка постановки не должна продвинуть курсор. SAVEPOINT отменяет
	// временный CHECK целиком после диагностического вызова.
	err = db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, tx.Exec("ALTER TABLE archive_export_jobs ADD CONSTRAINT synthetic_recheck_failure CHECK (application_id <= 2000)").Error)
		failedStore := services.NewArchiveExportOutbox(tx)
		_, advanceErr := failedStore.AdvanceRecheck(ctx, 2000)
		require.Error(t, advanceErr)
		return rollbackRecheckTest
	})
	require.ErrorIs(t, err, rollbackRecheckTest)
	require.NoError(t, db.First(&state, 1).Error)
	require.Equal(t, 2000, state.AfterID)
	count, err = restarted.AdvanceRecheck(ctx, 2000)
	require.NoError(t, err)
	require.Equal(t, 6, count)
	require.NoError(t, db.First(&state, 1).Error)
	require.False(t, state.Active)
	require.Equal(t, 2006, state.AfterID)
	var total int64
	require.NoError(t, db.Model(&models.ArchiveExportJob{}).Count(&total).Error)
	require.EqualValues(t, 2006, total)
}

var rollbackRecheckTest = errors.New("synthetic scan rollback")
