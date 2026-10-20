package services_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestApplicationDraft_DeleteFailureRetainsQuotaAndRetry(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "draftcleanup", "pass123", 1, td.OrgID, td.CompanyID)
	require.NotEmpty(t, token)
	var owner models.User
	require.NoError(t, db.Where("username = ?", "draftcleanup").First(&owner).Error)
	root := t.TempDir()
	svc := services.NewApplicationFileService(db, root, nil, services.DraftQuotaLimits{MaxCount: 1, MaxBytes: 100})
	name := "legacy.pdf"
	target := filepath.Join(svc.Dir(), name)
	require.NoError(t, os.MkdirAll(target, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(target, "busy"), []byte("x"), 0600))
	row := models.ApplicationFile{UploadedBy: owner.ID, FileName: name, StoredName: name, FileSize: 40}
	require.NoError(t, db.Create(&row).Error)
	ctx := context.Background()
	require.NoError(t, svc.DeleteDraft(ctx, owner.ID, row.ID))
	usage, err := svc.Quota().Usage(ctx, owner.ID)
	require.NoError(t, err)
	require.EqualValues(t, 0, usage.SavedCount)
	require.EqualValues(t, 1, usage.CleanupCount)
	require.EqualValues(t, 40, usage.ReservedBytes)
	_, err = svc.Quota().ReserveDrafts(ctx, owner.ID, 1, 1)
	quotaStatus(t, err, 409)
	require.NoError(t, os.Remove(filepath.Join(target, "busy")))
	require.NoError(t, os.Remove(target))
	removed, err := svc.Quota().RecoverReservations(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, removed)
	usage, err = svc.Quota().Usage(ctx, owner.ID)
	require.NoError(t, err)
	require.Zero(t, usage.ReservedCount)
}

func TestApplicationDraft_ListOwnAndForeignDeleteDenied(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	_ = testutil.RegisterAndLogin(t, e, "draftlist", "pass123", 1, td.OrgID, td.CompanyID)
	var owner models.User
	require.NoError(t, db.Where("username = ?", "draftlist").First(&owner).Error)
	svc := services.NewApplicationFileService(db, t.TempDir(), nil)
	now := time.Date(2040, 1, 2, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		row := models.ApplicationFile{UploadedBy: owner.ID, FileName: "own.pdf", StoredName: "own.pdf", FileSize: 10, CreatedAt: now.Add(time.Duration(i) * time.Second)}
		require.NoError(t, db.Create(&row).Error)
	}
	page, meta, err := svc.Quota().ListDrafts(context.Background(), owner.ID, 2, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, meta.Total)
	require.Len(t, page.Items, 1)
	require.EqualValues(t, 3, page.Usage.SavedCount)
	_, _, err = svc.Quota().ListDrafts(context.Background(), owner.ID, 0, 2)
	require.Error(t, err)
	require.Error(t, svc.DeleteDraft(context.Background(), owner.ID+10000, page.Items[0].ID))
}
