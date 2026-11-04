package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func durableArchiveDB(t *testing.T, base *gorm.DB) *gorm.DB {
	t.Helper()
	pool, err := base.DB()
	require.NoError(t, err)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}), &gorm.Config{Logger: base.Logger})
	require.NoError(t, err)
	return db
}

func durableArchiveWorker(t *testing.T, w archiveWorld, db *gorm.DB, quota services.ArchiveQuotaGuard, override ...services.AttachmentBlankService) *services.BlankExportService {
	t.Helper()
	writer, err := services.NewArchiveWriter(w.root)
	require.NoError(t, err)
	blanks := services.NewAttachmentBlankService(db)
	if len(override) > 0 {
		blanks = override[0]
	}
	return services.NewBlankExportService(db, blanks,
		services.NewArchivePathService(db, time.UTC), writer,
		services.NewSettingsService(db, &config.Config{PaginationMaxLimit: 100}), quota)
}

func durableArchiveJob(t *testing.T, db *gorm.DB, id int) models.ArchiveExportJob {
	t.Helper()
	var job models.ArchiveExportJob
	require.NoError(t, db.Where("application_id = ?", id).First(&job).Error)
	return job
}

func durableArchiveCleanupJob(t *testing.T, db *gorm.DB, ids ...int) {
	t.Helper()
	t.Cleanup(func() {
		require.NoError(t, db.Where("application_id IN ?", ids).Delete(&models.ArchiveExportJob{}).Error)
	})
}

func TestArchiveExportWorkerDurable(t *testing.T) {
	w := setupArchiveWorld(t)
	ctx := context.Background()
	uaID := w.newExportType(t, "Durable export", false, true)
	t.Run("coalesces persists across service restart and ACK empties queue", func(t *testing.T) {
		store := services.NewArchiveExportOutbox(w.db)
		jobs, err := store.ListDue(ctx, 200)
		require.NoError(t, err)
		require.Empty(t, jobs)
		first, _ := w.newExportApp(t, "durable-restart-first", uaID, "")
		second, _ := w.newExportApp(t, "durable-restart-second", uaID, "")
		durableArchiveCleanupJob(t, w.db, first, second)
		require.NoError(t, store.Enqueue(ctx, []int{first, second}, services.BlankExportReasonSubmit))
		require.NoError(t, store.Enqueue(ctx, []int{first}, services.BlankExportReasonUpdate))
		jobs, err = store.ListDue(ctx, 200)
		require.NoError(t, err)
		require.Len(t, jobs, 2)
		reasons := map[int]string{}
		for _, job := range jobs {
			reasons[job.ApplicationID] = job.Reason
		}
		require.Equal(t, services.BlankExportReasonUpdate, reasons[first])
		require.Equal(t, services.BlankExportReasonSubmit, reasons[second])
		select {
		case <-store.Wake():
		default:
			t.Fatal("persisted enqueue did not wake the worker")
		}
		restarted := durableArchiveWorker(t, w, w.db, nil)
		processed, failed := restarted.ProcessQueue(ctx)
		require.Equal(t, 2, processed)
		require.Zero(t, failed)
		jobs, err = store.ListDue(ctx, 200)
		require.NoError(t, err)
		require.Empty(t, jobs)
		processed, failed = restarted.ProcessQueue(ctx)
		require.Zero(t, processed)
		require.Zero(t, failed)
	})
	t.Run("disabled and canceled calls retain jobs until resume", func(t *testing.T) {
		id, _ := w.newExportApp(t, "durable-disabled", uaID, "")
		durableArchiveCleanupJob(t, w.db, id)
		store := services.NewArchiveExportOutbox(w.db)
		require.NoError(t, store.Enqueue(ctx, []int{id}, services.BlankExportReasonSubmit))
		worker := durableArchiveWorker(t, w, w.db, nil)
		disabled := false
		w.putSettings(t, models.UpdateArchiveSettingsRequest{Enabled: &disabled})
		defer w.putSettings(t, models.UpdateArchiveSettingsRequest{Enabled: ptrArchiveBool(true)})
		processed, failed := worker.ProcessQueue(ctx)
		require.Zero(t, processed)
		require.Zero(t, failed)
		require.Zero(t, durableArchiveJob(t, w.db, id).Attempts)
		enabled := true
		w.putSettings(t, models.UpdateArchiveSettingsRequest{Enabled: &enabled})
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		worker.ProcessQueue(canceled)
		require.Zero(t, durableArchiveJob(t, w.db, id).Attempts)
		restarted := durableArchiveWorker(t, w, w.db, nil)
		processed, failed = restarted.ProcessQueue(ctx)
		require.Equal(t, 1, processed)
		require.Zero(t, failed)
	})
	t.Run("new revision during export survives old ACK", func(t *testing.T) {
		id, _ := w.newExportApp(t, "durable-revision", uaID, "")
		durableArchiveCleanupJob(t, w.db, id)
		db := durableArchiveDB(t, w.db)
		store := services.NewArchiveExportOutbox(db)
		require.NoError(t, store.Enqueue(ctx, []int{id}, services.BlankExportReasonSubmit))
		old := durableArchiveJob(t, db, id)
		var changed atomic.Bool
		require.NoError(t, db.Callback().Create().Before("gorm:create").Register("durable:enqueue_during_export", func(tx *gorm.DB) {
			if tx.Statement.Table == "blank_exports" && changed.CompareAndSwap(false, true) {
				tx.AddError(store.Enqueue(ctx, []int{id}, services.BlankExportReasonUpdate))
			}
		}))
		worker := durableArchiveWorker(t, w, db, nil)
		processed, failed := worker.ProcessQueue(ctx)
		require.Equal(t, 1, processed)
		require.Zero(t, failed)
		current := durableArchiveJob(t, db, id)
		require.NotEqual(t, old.Revision, current.Revision)
		require.Equal(t, services.BlankExportReasonUpdate, current.Reason)
		require.NoError(t, db.Callback().Create().Remove("durable:enqueue_during_export"))
		processed, failed = worker.ProcessQueue(ctx)
		require.Equal(t, 1, processed)
		require.Zero(t, failed)
	})
	t.Run("cancel during first export does not ACK batch", func(t *testing.T) {
		first, _ := w.newExportApp(t, "durable-cancel-first", uaID, "")
		second, _ := w.newExportApp(t, "durable-cancel-second", uaID, "")
		durableArchiveCleanupJob(t, w.db, first, second)
		db := durableArchiveDB(t, w.db)
		store := services.NewArchiveExportOutbox(db)
		require.NoError(t, store.Enqueue(ctx, []int{first, second}, services.BlankExportReasonSubmit))
		canceled, cancel := context.WithCancel(ctx)
		defer cancel()
		var interrupted atomic.Bool
		require.NoError(t, db.Callback().Create().Before("gorm:create").Register("durable:cancel", func(tx *gorm.DB) {
			if tx.Statement.Table == "blank_exports" && interrupted.CompareAndSwap(false, true) {
				cancel()
			}
		}))
		worker := durableArchiveWorker(t, w, db, nil)
		processed, failed := worker.ProcessQueue(canceled)
		require.Zero(t, processed)
		require.Zero(t, failed)
		require.Zero(t, durableArchiveJob(t, db, first).Attempts)
		require.Zero(t, durableArchiveJob(t, db, second).Attempts)
		require.NoError(t, db.Callback().Create().Remove("durable:cancel"))
		processed, failed = worker.ProcessQueue(ctx)
		require.Equal(t, 2, processed)
		require.Zero(t, failed)
	})
	for _, target := range []string{"skipped", "no_template", "generator_no_template", "snapshot"} {
		t.Run("registry fault retains "+target, func(t *testing.T) {
			typeID := w.newExportType(t, "Durable fault "+target, target == "generator_no_template", target == "no_template" || target == "generator_no_template")
			id, attID := w.newExportApp(t, "durable-fault-"+target, typeID, "")
			durableArchiveCleanupJob(t, w.db, id)
			db := durableArchiveDB(t, w.db)
			store := services.NewArchiveExportOutbox(db)
			require.NoError(t, store.Enqueue(ctx, []int{id}, services.BlankExportReasonSubmit))
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("durable:registry_fault", func(tx *gorm.DB) {
				row, ok := tx.Statement.Dest.(*models.BlankExport)
				if ok && row.ApplicationID == id && ((target == "snapshot" && row.AttachmentID == 0) || (target != "snapshot" && row.AttachmentID == attID)) {
					tx.AddError(errors.New("synthetic archive registry failure"))
				}
			}))
			worker := durableArchiveWorker(t, w, db, nil)
			if target == "generator_no_template" {
				worker = durableArchiveWorker(t, w, db, nil, durableMissingTemplate{services.NewAttachmentBlankService(db)})
			}
			processed, failed := worker.ProcessQueue(ctx)
			require.Zero(t, processed)
			require.Equal(t, 1, failed)
			require.Equal(t, 1, durableArchiveJob(t, db, id).Attempts)
			jobs, err := store.ListDue(ctx, 200)
			require.NoError(t, err)
			require.Empty(t, jobs, "failed job must wait for its retry")
			require.NoError(t, db.Callback().Create().Remove("durable:registry_fault"))
		})
	}
	t.Run("missing app is orphaned before ACK and is not swept again", func(t *testing.T) {
		const id = 900000001
		durableArchiveCleanupJob(t, w.db, id)
		due := time.Now().Add(-time.Minute)
		row := models.BlankExport{ApplicationID: id, AttachmentID: id, BucketDate: due,
			Status: models.BlankExportFailed, NextAttemptAt: &due, QueuedAt: due,
			RelDir: "preserved", FileName: "missing.xlsx", ContentHash: "preserved"}
		require.NoError(t, w.db.Create(&row).Error)
		store := services.NewArchiveExportOutbox(w.db)
		require.NoError(t, store.Enqueue(ctx, []int{id}, services.BlankExportReasonUpdate))
		worker := durableArchiveWorker(t, w, w.db, nil)
		processed, failed := worker.ProcessQueue(ctx)
		require.Equal(t, 1, processed)
		require.Zero(t, failed)
		var nullRetry int64
		require.NoError(t, w.db.Model(&models.BlankExport{}).Where("id = ? AND next_attempt_at IS NULL", row.ID).Count(&nullRetry).Error)
		require.EqualValues(t, 1, nullRetry, "проверяем фактический SQL NULL, а не прежнее значение указателя")
		var persisted models.BlankExport
		require.NoError(t, w.db.First(&persisted, row.ID).Error)
		row = persisted
		require.Equal(t, models.BlankExportOrphan, row.Status)
		require.Nil(t, row.NextAttemptAt)
		require.Equal(t, "preserved", row.RelDir)
		require.Equal(t, "preserved", row.ContentHash)
		processed, failed = worker.Sweep(ctx)
		require.Zero(t, processed)
		require.Zero(t, failed)
	})
}

func ptrArchiveBool(value bool) *bool { return &value }

type durableMissingTemplate struct {
	services.AttachmentBlankService
}

func (durableMissingTemplate) GenerateBlank(context.Context, int, int, services.BlankOptions) (io.Reader, string, error) {
	return nil, "", echo.NewHTTPError(http.StatusNotFound, "Шаблон недоступен")
}

type durableStopQuota struct{}

func (durableStopQuota) EnforceThresholds(context.Context) (*services.QuotaEnforcementResult, error) {
	return &services.QuotaEnforcementResult{HardTripped: true}, nil
}

func TestArchiveExportWorkerDurable_ContinuesScanAfterRestart(t *testing.T) {
	w := setupArchiveWorld(t)
	ctx := context.Background()
	now := time.Now()
	apps := make([]models.Application, 2001)
	for i := range apps {
		number := fmt.Sprintf("durable-scan-%d", i)
		apps[i] = models.Application{ApplicationNumber: &number, OrganizationID: w.orgID,
			SenderUserID: w.senderID, SendingDatetime: &now}
	}
	require.NoError(t, w.db.CreateInBatches(apps, 500).Error)
	store := services.NewArchiveExportOutbox(w.db)
	require.NoError(t, store.BeginRecheck(ctx, now.Add(-time.Hour)))
	worker := durableArchiveWorker(t, w, w.db, durableStopQuota{})
	worker.ProcessQueue(ctx)
	var count int64
	require.NoError(t, w.db.Model(&models.ArchiveExportJob{}).Count(&count).Error)
	require.EqualValues(t, 2000, count)
	restarted := durableArchiveWorker(t, w, w.db, durableStopQuota{})
	restarted.ProcessQueue(ctx)
	require.NoError(t, w.db.Model(&models.ArchiveExportJob{}).Count(&count).Error)
	require.EqualValues(t, 2001, count)
	require.NoError(t, w.db.Model(&models.BlankExport{}).Count(&count).Error)
	require.Zero(t, count, "hard quota must prevent file/registry creation while scanning progresses")
}
