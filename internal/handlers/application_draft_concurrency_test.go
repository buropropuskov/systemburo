package handlers_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestApplicationDraft_AttachWinsConcurrentDelete(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "draftattachrace", "pass123", 1, td.OrgID, td.CompanyID)
	var owner models.User
	require.NoError(t, db.Where("username = ?", "draftattachrace").First(&owner).Error)
	application := submitWithFiles(t, e, db, token, "race", nil)
	require.Equal(t, 200, application.Code, application.Body.String())
	appID := testutil.ParseResponse[services.CompleteApplicationResponse](t, application).ApplicationID
	svc := services.NewApplicationFileService(db, t.TempDir(), nil)
	require.NoError(t, os.MkdirAll(svc.Dir(), 0700))
	row := models.ApplicationFile{UploadedBy: owner.ID, FileName: "race.pdf", StoredName: "race.pdf", FileSize: 7}
	require.NoError(t, db.Create(&row).Error)
	disk := filepath.Join(svc.Dir(), row.StoredName)
	require.NoError(t, os.WriteFile(disk, []byte("payload"), 0600))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx := db.WithContext(ctx).Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	require.NoError(t, svc.Attach(tx, owner.ID, appID, []int{row.ID}, 30, 104857600))
	done := make(chan error, 1)
	go func() { done <- svc.DeleteDraft(ctx, owner.ID, row.ID) }()
	// Wait for the competing operation to reach the owner lock, not a guessed sleep.
	require.Eventually(t, func() bool {
		var waiting int64
		err := db.WithContext(ctx).Raw("SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND NOT granted").Scan(&waiting).Error
		return err == nil && waiting > 0
	}, 2*time.Second, 10*time.Millisecond)
	require.NoError(t, tx.Commit().Error)
	var typed *apperr.Error
	err := <-done
	require.True(t, errors.As(err, &typed))
	require.Equal(t, 404, typed.Code)
	require.NoError(t, db.First(&row, row.ID).Error)
	require.Equal(t, appID, *row.ApplicationID)
	require.FileExists(t, disk)
}
