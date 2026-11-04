package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/entityarchive"
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestArchiveExportRetention_SuppressionFailureDoesNotRemoveFiles(t *testing.T) {
	w := setupArchiveWorld(t)
	typ := w.newExportType(t, "Сохранность архива", false, true)
	app, _ := w.newExportApp(t, "20600101/910", typ, "")
	result := w.reexport(t, app)
	snapshot := w.abs(path.Join(result.RelDir, archiveSnapshotFileName))
	require.FileExists(t, snapshot)
	constraint := "archive_export_test_reject_suppress"
	require.NoError(t, w.db.Exec("ALTER TABLE archive_export_jobs ADD CONSTRAINT "+constraint+" CHECK (state <> 'suppressed')").Error)
	defer w.db.Exec("ALTER TABLE archive_export_jobs DROP CONSTRAINT IF EXISTS " + constraint)
	_, err := entityarchive.AnonymizeApplication(context.Background(), w.db, services.NewAuditRecorder(w.db), app,
		entityarchive.DestructionOptions{Apply: true, Basis: entityarchive.BasisOperator, Files: entityarchive.FilePaths{ArchivePath: w.root}})
	require.Error(t, err)
	require.FileExists(t, snapshot, "без durable запрета удаление не начинается")
	var row models.BlankExport
	require.NoError(t, w.db.Where("application_id = ? AND attachment_id = 0", app).First(&row).Error)
	require.Equal(t, models.BlankExportOK, row.Status)
}

func TestArchiveExportRetention_FailedDestructionKeepsSuppression(t *testing.T) {
	w := setupArchiveWorld(t)
	typ := w.newExportType(t, "Повтор удаления", false, true)
	app, _ := w.newExportApp(t, "20600101/911", typ, "")
	result := w.reexport(t, app)
	badRoot := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(badRoot, []byte("fixture"), 0o600))
	_, err := entityarchive.AnonymizeApplication(context.Background(), w.db, services.NewAuditRecorder(w.db), app,
		entityarchive.DestructionOptions{Apply: true, Basis: entityarchive.BasisOperator, Files: entityarchive.FilePaths{ArchivePath: badRoot}})
	require.Error(t, err, "намеренно недоступный каталог не считается удалённым")
	store := services.NewArchiveExportOutbox(w.db)
	blocked, err := store.IsSuppressed(context.Background(), app)
	require.NoError(t, err)
	require.True(t, blocked)
	require.NoError(t, store.Enqueue(context.Background(), []int{app}, services.BlankExportReasonUpdate))
	_, err = w.newWorkerExport(t).ExportApplication(context.Background(), app, services.BlankExportReasonUpdate)
	require.ErrorIs(t, err, services.ErrArchiveDestructionPending)
	require.FileExists(t, w.abs(path.Join(result.RelDir, archiveSnapshotFileName)))
	var count int64
	require.NoError(t, w.db.Model(&models.DestructionRecord{}).Where("entity_type = ? AND entity_id = ?", entityarchive.TypeApplication, app).Count(&count).Error)
	require.Zero(t, count, "запрет не выдаётся за завершённое уничтожение")
}

func TestArchiveExportRetention_PhysicalPurgeRetainsTerminalBarrier(t *testing.T) {
	w := setupArchiveWorld(t)
	typ := w.newExportType(t, "Терминальное состояние", false, true)
	app, _ := w.newExportApp(t, "20600101/913", typ, "")
	result := w.reexport(t, app)
	snapshot := w.abs(path.Join(result.RelDir, archiveSnapshotFileName))
	_, err := entityarchive.PurgeApplication(context.Background(), w.db, services.NewAuditRecorder(w.db), app,
		entityarchive.DestructionOptions{Apply: true, Basis: entityarchive.BasisOperator, Files: entityarchive.FilePaths{ArchivePath: w.root}})
	require.NoError(t, err)
	require.NoFileExists(t, snapshot)
	var count int64
	require.NoError(t, w.db.Model(&models.Application{}).Where("id = ?", app).Count(&count).Error)
	require.Zero(t, count)
	var job models.ArchiveExportJob
	require.NoError(t, w.db.First(&job, "application_id = ?", app).Error)
	require.Equal(t, models.ArchiveExportJobSuppressed, job.State)
	require.NoError(t, services.NewArchiveExportOutbox(w.db).Enqueue(context.Background(), []int{app}, services.BlankExportReasonUpdate))
	require.NoError(t, w.db.First(&job, "application_id = ?", app).Error)
	require.Equal(t, models.ArchiveExportJobSuppressed, job.State)
	_, err = w.newWorkerExport(t).ExportApplication(context.Background(), app, services.BlankExportReasonUpdate)
	require.Error(t, err)
	require.NoFileExists(t, snapshot)
}

func TestArchiveExportRetention_InFlightSnapshotCannotReviveDestroyedFiles(t *testing.T) {
	w := setupArchiveWorld(t)
	typ := w.newExportType(t, "Координация архива", false, true)
	app, att := w.newExportApp(t, "20600101/912", typ, "")
	name := "Архивный участник"
	require.NoError(t, w.db.Create(&models.Employee{AttachmentID: &att, LastName: &name}).Error)
	svc := w.newWorkerExport(t)
	read, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	var captured atomic.Bool
	const callback = "test:hold_archive_snapshot_after_people"
	require.NoError(t, w.db.Callback().Query().After("gorm:after_query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "employees" && captured.CompareAndSwap(false, true) {
			close(read)
			<-release
		}
	}))
	defer w.db.Callback().Query().Remove(callback)
	exported, purged := make(chan error, 1), make(chan error, 1)
	exportDone, purgeDone := make(chan struct{}), make(chan struct{})
	purgeStarted := false
	defer func() {
		unblock()
		select {
		case <-exportDone:
		case <-time.After(10 * time.Second):
			t.Error("export cleanup timed out")
		}
		if purgeStarted {
			select {
			case <-purgeDone:
			case <-time.After(10 * time.Second):
				t.Error("retention cleanup timed out")
			}
		}
	}()
	go func() {
		defer close(exportDone)
		_, err := svc.ExportApplication(context.Background(), app, services.BlankExportReasonUpdate)
		exported <- err
	}()
	select {
	case <-read:
	case <-time.After(5 * time.Second):
		t.Fatal("snapshot did not read participants")
	}
	purgeStarted = true
	go func() {
		defer close(purgeDone)
		_, err := entityarchive.AnonymizeApplication(context.Background(), w.db, services.NewAuditRecorder(w.db), app,
			entityarchive.DestructionOptions{Apply: true, Basis: entityarchive.BasisOperator, Files: entityarchive.FilePaths{ArchivePath: w.root}})
		purged <- err
	}()
	// Сохраняем экспорт с уже прочитанными данными, пока запущен retention.
	select {
	case err := <-purged:
		t.Fatalf("retention passed active exporter: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	unblock()
	require.NoError(t, <-exported)
	require.NoError(t, <-purged)
	var rows []models.BlankExport
	require.NoError(t, w.db.Where("application_id = ?", app).Find(&rows).Error)
	require.NotEmpty(t, rows)
	for _, row := range rows {
		require.Equal(t, models.BlankExportPurged, row.Status)
		require.Empty(t, row.FileName)
	}
	_, err := svc.ExportApplication(context.Background(), app, services.BlankExportReasonUpdate)
	require.True(t, errors.Is(err, services.ErrArchivePurged) || errors.Is(err, services.ErrArchiveDestructionPending), fmt.Sprint(err))
	var remaining []string
	err = filepath.WalkDir(w.root, func(p string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			remaining = append(remaining, p)
		}
		return nil
	})
	require.NoError(t, err)
	require.Empty(t, remaining, "экспорт после retention не должен оставлять ни одного файла")
}
