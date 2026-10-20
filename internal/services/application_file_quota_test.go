package services_test

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
	"systemburo/internal/upload"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type quotaWorld struct {
	db    *gorm.DB
	quota *services.ApplicationFileQuota
	root  string
	now   atomic.Int64
}

func draftQuotaWorld(t *testing.T, count, size int64) *quotaWorld {
	t.Helper()
	_, db, closeApp := testutil.SetupTestApp(t)
	t.Cleanup(closeApp)
	testutil.CleanDB(t, db)
	w := &quotaWorld{db: db, root: t.TempDir()}
	w.now.Store(time.Date(2040, 1, 2, 12, 0, 0, 0, time.UTC).Unix())
	var err error
	w.quota, err = services.NewApplicationFileQuota(db, w.root, services.DraftQuotaLimits{MaxCount: count, MaxBytes: size}, func() time.Time { return time.Unix(w.now.Load(), 0).UTC() })
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(w.root, services.ApplicationFilesDir), 0700))
	return w
}

func quotaStatus(t *testing.T, err error, want int) {
	t.Helper()
	var value *apperr.Error
	require.ErrorAs(t, err, &value)
	require.Equal(t, want, value.Code)
}

func TestDraftQuota_BoundariesAndOwners(t *testing.T) {
	w := draftQuotaWorld(t, 50, 200*1024*1024)
	ctx := context.Background()
	rows := make([]models.ApplicationFile, 49)
	for i := range rows {
		rows[i] = models.ApplicationFile{UploadedBy: 10001, FileName: "synthetic.pdf", StoredName: "old", FileSize: 1}
	}
	require.NoError(t, w.db.Create(&rows).Error)
	r, err := w.quota.ReserveDrafts(ctx, 10001, 1, 1)
	require.NoError(t, err)
	_, err = w.quota.ReserveDrafts(ctx, 10001, 1, 1)
	quotaStatus(t, err, 409)
	other, err := w.quota.ReserveDrafts(ctx, 10002, 1, 200*1024*1024)
	require.NoError(t, err)
	_, err = w.quota.ReserveDrafts(ctx, 10002, 1, 1)
	quotaStatus(t, err, 409)
	usage, err := w.quota.Usage(ctx, 10001)
	require.NoError(t, err)
	require.EqualValues(t, 49, usage.SavedCount)
	require.EqualValues(t, 1, usage.ReservedCount)
	w.quota.FinishWriting(r.ID)
	require.NoError(t, w.quota.AbortReservation(ctx, 10001, r.ID))
	w.quota.FinishWriting(other.ID)
	require.NoError(t, w.quota.AbortReservation(ctx, 10002, other.ID))
	_, err = w.quota.ReserveDrafts(ctx, 10003, 51, 1)
	quotaStatus(t, err, 409)
	_, err = w.quota.ReserveDrafts(ctx, 10003, 1, 200*1024*1024+1)
	quotaStatus(t, err, 409)
}

func TestDraftQuota_ConcurrentReservations(t *testing.T) {
	w := draftQuotaWorld(t, 1, 100)
	var successes atomic.Int64
	var wg sync.WaitGroup
	var mu sync.Mutex
	var unexpected []error
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := w.quota.ReserveDrafts(context.Background(), 10001, 1, 1)
			if err == nil {
				successes.Add(1)
				return
			}
			var typed *apperr.Error
			if !errors.As(err, &typed) || typed.Code != 409 {
				mu.Lock()
				unexpected = append(unexpected, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	require.Empty(t, unexpected)
	require.EqualValues(t, 1, successes.Load())
}

func TestDraftReservation_PrepareCommitAndReplay(t *testing.T) {
	w := draftQuotaWorld(t, 2, 12)
	ctx := context.Background()
	r, err := w.quota.ReserveDrafts(ctx, 10001, 2, 10)
	require.NoError(t, err)
	first, err := w.quota.PrepareReservedFile(ctx, 10001, r.ID, 6, ".pdf")
	require.NoError(t, err)
	second, err := w.quota.PrepareReservedFile(ctx, 10001, r.ID, 6, ".pdf")
	require.NoError(t, err)
	saved := []upload.SavedFile{{StoredName: first, FileName: "one.pdf", Size: 6, DetectedMime: "application/pdf", Encrypted: true}, {StoredName: second, FileName: "two.pdf", Size: 6, DetectedMime: "application/pdf", Encrypted: true}}
	_, err = w.quota.CommitDrafts(ctx, 10001, r.ID, saved)
	quotaStatus(t, err, 409)
	w.quota.FinishWriting(r.ID)
	items, err := w.quota.CommitDrafts(ctx, 10001, r.ID, saved)
	require.NoError(t, err)
	require.Len(t, items, 2)
	usage, err := w.quota.Usage(ctx, 10001)
	require.NoError(t, err)
	require.EqualValues(t, 2, usage.SavedCount)
	require.EqualValues(t, 12, usage.SavedBytes)
	require.Zero(t, usage.ReservedCount)
	_, err = w.quota.CommitDrafts(ctx, 10001, r.ID, saved)
	quotaStatus(t, err, 404)
	var files []models.ApplicationFile
	require.NoError(t, w.db.Where("uploaded_by = ?", 10001).Find(&files).Error)
	require.Len(t, files, 2)
	require.True(t, files[0].Encrypted)
}

func TestDraftReservation_GrowthAndInvalidInputs(t *testing.T) {
	w := draftQuotaWorld(t, 2, 10)
	ctx := context.Background()
	r, err := w.quota.ReserveDrafts(ctx, 10001, 2, 8)
	require.NoError(t, err)
	_, err = w.quota.PrepareReservedFile(ctx, 10002, r.ID, 1, ".pdf")
	quotaStatus(t, err, 404)
	_, err = w.quota.PrepareReservedFile(ctx, 10001, r.ID, 1, "/bad")
	quotaStatus(t, err, 400)
	_, err = w.quota.PrepareReservedFile(ctx, 10001, r.ID, 6, ".pdf")
	require.NoError(t, err)
	_, err = w.quota.PrepareReservedFile(ctx, 10001, r.ID, 5, ".pdf")
	quotaStatus(t, err, 409)
	var row models.ApplicationFileReservation
	require.NoError(t, w.db.First(&row, "id = ?", r.ID).Error)
	require.Len(t, row.StoredNames, 1)
	require.EqualValues(t, 8, row.ReservedBytes)
	_, err = w.quota.ReserveDrafts(ctx, 0, 1, 1)
	quotaStatus(t, err, 400)
	_, err = w.quota.ReserveDrafts(ctx, 10001, -1, 1)
	quotaStatus(t, err, 400)
	_, err = w.quota.ReserveDrafts(ctx, 10003, 1, math.MaxInt64)
	quotaStatus(t, err, 409)
	_, err = w.quota.PrepareReservedFile(ctx, 10001, r.ID, math.MaxInt64, ".pdf")
	quotaStatus(t, err, 400)
}

func TestDraftReservation_ExpiryKeepsLiveWriter(t *testing.T) {
	w := draftQuotaWorld(t, 2, 10)
	ctx := context.Background()
	r, err := w.quota.ReserveDrafts(ctx, 10001, 1, 5)
	require.NoError(t, err)
	name, err := w.quota.PrepareReservedFile(ctx, 10001, r.ID, 5, ".pdf")
	require.NoError(t, err)
	file := filepath.Join(w.root, services.ApplicationFilesDir, name)
	require.NoError(t, os.WriteFile(file, []byte("dummy"), 0600))
	w.now.Add(5 * 60)
	_, err = w.quota.PrepareReservedFile(ctx, 10001, r.ID, 1, ".pdf")
	quotaStatus(t, err, 409)
	n, err := w.quota.RecoverReservations(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
	require.FileExists(t, file)
	w.quota.FinishWriting(r.ID)
	_, err = w.quota.CommitDrafts(ctx, 10001, r.ID, []upload.SavedFile{{StoredName: name, Size: 5}})
	quotaStatus(t, err, 409)
	n, err = w.quota.RecoverReservations(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.NoFileExists(t, file)
}

func TestDraftReservation_CleanupFailureAndRestart(t *testing.T) {
	w := draftQuotaWorld(t, 1, 10)
	ctx := context.Background()
	r, err := w.quota.ReserveDrafts(ctx, 10001, 1, 5)
	require.NoError(t, err)
	name, err := w.quota.PrepareReservedFile(ctx, 10001, r.ID, 5, ".pdf")
	require.NoError(t, err)
	file := filepath.Join(w.root, services.ApplicationFilesDir, name)
	require.NoError(t, os.Mkdir(file, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(file, "held"), []byte("x"), 0600))
	w.quota.FinishWriting(r.ID)
	require.Error(t, w.quota.AbortReservation(ctx, 10001, r.ID))
	usage, err := w.quota.Usage(ctx, 10001)
	require.NoError(t, err)
	require.EqualValues(t, 1, usage.ReservedCount)
	_, err = w.quota.ReserveDrafts(ctx, 10001, 1, 1)
	quotaStatus(t, err, 409)
	require.NoError(t, os.Remove(filepath.Join(file, "held")))
	restarted, err := services.NewApplicationFileQuota(w.db, w.root, services.DraftQuotaLimits{MaxCount: 1, MaxBytes: 10}, func() time.Time { return time.Unix(w.now.Load(), 0) })
	require.NoError(t, err)
	n, err := restarted.RecoverReservations(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	usage, err = restarted.Usage(ctx, 10001)
	require.NoError(t, err)
	require.Zero(t, usage.ReservedCount)
}

func TestDraftReservation_IncompleteCommitAndDatabaseFailure(t *testing.T) {
	w := draftQuotaWorld(t, 2, 10)
	ctx := context.Background()
	r, err := w.quota.ReserveDrafts(ctx, 10001, 2, 8)
	require.NoError(t, err)
	name, err := w.quota.PrepareReservedFile(ctx, 10001, r.ID, 4, ".pdf")
	require.NoError(t, err)
	w.quota.FinishWriting(r.ID)
	_, err = w.quota.CommitDrafts(ctx, 10001, r.ID, []upload.SavedFile{{StoredName: name, Size: 4}})
	quotaStatus(t, err, 400)
	usage, err := w.quota.Usage(ctx, 10001)
	require.NoError(t, err)
	require.Zero(t, usage.SavedCount)
	require.EqualValues(t, 2, usage.ReservedCount)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = w.quota.ReserveDrafts(canceled, 10002, 1, 1)
	require.Error(t, err)
}

func TestDraftQuota_AttachedFilesExcludedAndOldOverLimitPreserved(t *testing.T) {
	w := draftQuotaWorld(t, 2, 10)
	seed := testutil.SeedTestData(t, w.db)
	user := models.User{Username: "quota_synthetic", Password: "x", TypeID: 1, OrganizationID: &seed.OrgID, IsActive: true}
	require.NoError(t, w.db.Create(&user).Error)
	app := models.Application{OrganizationID: seed.OrgID, SenderUserID: user.ID}
	require.NoError(t, w.db.Create(&app).Error)
	files := []models.ApplicationFile{{UploadedBy: user.ID, FileSize: 11, StoredName: "draft.pdf"}, {UploadedBy: user.ID, FileSize: 1000, StoredName: "attached.pdf", ApplicationID: &app.ID}}
	require.NoError(t, w.db.Create(&files).Error)
	usage, err := w.quota.Usage(context.Background(), user.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, usage.SavedCount)
	require.EqualValues(t, 11, usage.SavedBytes)
	_, err = w.quota.ReserveDrafts(context.Background(), user.ID, 1, 1)
	quotaStatus(t, err, 409)
	var remaining int64
	require.NoError(t, w.db.Model(&models.ApplicationFile{}).Where("uploaded_by = ?", user.ID).Count(&remaining).Error)
	require.EqualValues(t, 2, remaining)
}

func TestDraftReservation_CommitFailureRetainsReservation(t *testing.T) {
	w := draftQuotaWorld(t, 1, 10)
	ctx := context.Background()
	r, err := w.quota.ReserveDrafts(ctx, 10001, 1, 5)
	require.NoError(t, err)
	name, err := w.quota.PrepareReservedFile(ctx, 10001, r.ID, 5, ".pdf")
	require.NoError(t, err)
	w.quota.FinishWriting(r.ID)
	saved := []upload.SavedFile{{StoredName: name, FileName: "synthetic.pdf", DetectedMime: "application/pdf", Size: 5, Encrypted: true}}
	quotaStatus(t, w.quota.AbortReservation(ctx, 10002, r.ID), 404)
	require.NoError(t, w.db.Callback().Create().Before("gorm:create").Register("quota_insert_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "application_files" {
			tx.AddError(errors.New("synthetic insert failure"))
		}
	}))
	t.Cleanup(func() { w.db.Callback().Create().Remove("quota_insert_failure") })
	_, err = w.quota.CommitDrafts(ctx, 10001, r.ID, saved)
	require.ErrorContains(t, err, "synthetic insert failure")
	usage, err := w.quota.Usage(ctx, 10001)
	require.NoError(t, err)
	require.Zero(t, usage.SavedCount)
	require.EqualValues(t, 1, usage.ReservedCount)
	require.NoError(t, w.db.Callback().Create().Remove("quota_insert_failure"))
	items, err := w.quota.CommitDrafts(ctx, 10001, r.ID, saved)
	require.NoError(t, err)
	require.Len(t, items, 1)
}

func TestDraftReservation_RestartDoesNotTakeFreshForeignReservation(t *testing.T) {
	w := draftQuotaWorld(t, 1, 10)
	ctx := context.Background()
	_, err := w.quota.ReserveDrafts(ctx, 10001, 1, 5)
	require.NoError(t, err)
	restarted, err := services.NewApplicationFileQuota(w.db, w.root, services.DraftQuotaLimits{MaxCount: 1, MaxBytes: 10}, func() time.Time { return time.Unix(w.now.Load(), 0) })
	require.NoError(t, err)
	n, err := restarted.RecoverReservations(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
	w.now.Add(301)
	n, err = restarted.RecoverReservations(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}
