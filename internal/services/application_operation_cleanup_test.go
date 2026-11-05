package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type cleanupOperationFixture struct {
	OwnerUserID     int
	EffectiveUserID int
	Scope           string
	Key             string
	Fingerprint     string
	Result          json.RawMessage `gorm:"type:jsonb"`
	CreatedAt       time.Time
}

func TestCleanupApplicationOperationsBoundedWindowAndDeterministicOrder(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	now := time.Date(2030, 1, 2, 10, 0, 0, 0, time.UTC)
	cutoff := now.Add(-services.ApplicationOperationRetention)
	rows := make([]cleanupOperationFixture, 0, 505)
	for i := 0; i < 501; i++ {
		rows = append(rows, cleanupOperationFixture{OwnerUserID: 1, EffectiveUserID: 1, Scope: "submit",
			Key: fmt.Sprintf("expired-%04d", i), Fingerprint: "synthetic", Result: json.RawMessage(`{"id":1}`), CreatedAt: cutoff.Add(-time.Second)})
	}
	// Exact cutoff, the replay-window boundary and pending/new commands survive.
	for i, created := range []time.Time{cutoff, now.Add(-24 * time.Hour), now.Add(-time.Minute), now.Add(time.Minute)} {
		rows = append(rows, cleanupOperationFixture{OwnerUserID: 1, EffectiveUserID: 1, Scope: "submit",
			Key: fmt.Sprintf("retained-%d", i), Fingerprint: "synthetic", CreatedAt: created})
	}
	require.NoError(t, db.Table("application_operations").Create(&rows).Error)
	deleted, err := services.CleanupApplicationOperations(context.Background(), db, now)
	require.NoError(t, err)
	require.EqualValues(t, 500, deleted)
	var expired int64
	require.NoError(t, db.Table("application_operations").Where("created_at < ?", cutoff).Count(&expired).Error)
	require.EqualValues(t, 1, expired)
	require.NoError(t, db.Table("application_operations").Where("key = ?", "expired-0500").Count(&expired).Error)
	require.EqualValues(t, 1, expired, "the ordered composite key picks a stable first batch")
	deleted, err = services.CleanupApplicationOperations(context.Background(), db, now)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)
	deleted, err = services.CleanupApplicationOperations(context.Background(), db, now)
	require.NoError(t, err)
	require.Zero(t, deleted)
	var remaining int64
	require.NoError(t, db.Table("application_operations").Count(&remaining).Error)
	require.EqualValues(t, 4, remaining)
}

func TestCleanupApplicationOperationsSkipsLocksAndMatchesWholeCompositeKey(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	now := time.Date(2030, 1, 2, 10, 0, 0, 0, time.UTC)
	old := now.Add(-services.ApplicationOperationRetention - time.Second)
	rows := []cleanupOperationFixture{
		{OwnerUserID: 1, EffectiveUserID: 1, Scope: "submit", Key: "shared", Fingerprint: "synthetic", CreatedAt: old},
		{OwnerUserID: 2, EffectiveUserID: 1, Scope: "submit", Key: "shared", Fingerprint: "synthetic", CreatedAt: old},
		{OwnerUserID: 1, EffectiveUserID: 2, Scope: "submit", Key: "shared", Fingerprint: "synthetic", CreatedAt: now},
		{OwnerUserID: 1, EffectiveUserID: 1, Scope: "supplement:1", Key: "shared", Fingerprint: "synthetic", CreatedAt: now},
	}
	require.NoError(t, db.Table("application_operations").Create(&rows).Error)
	locked := db.Begin()
	require.NoError(t, locked.Error)
	defer locked.Rollback()
	require.NoError(t, locked.Exec(`SELECT 1 FROM application_operations WHERE owner_user_id=1 AND effective_user_id=1 AND scope='submit' AND key='shared' FOR UPDATE`).Error)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	deleted, err := services.CleanupApplicationOperations(ctx, db, now)
	require.NoError(t, err, "an existing row lock must be skipped, not waited on")
	require.EqualValues(t, 1, deleted)
	var remaining int64
	require.NoError(t, db.Table("application_operations").Count(&remaining).Error)
	require.EqualValues(t, 3, remaining)
	require.NoError(t, locked.Rollback().Error)
	deleted, err = services.CleanupApplicationOperations(context.Background(), db, now)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)
	require.NoError(t, db.Table("application_operations").Count(&remaining).Error)
	require.EqualValues(t, 2, remaining)
}

func TestCleanupApplicationOperationsCancellationAndSafeError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	deleted, err := services.CleanupApplicationOperations(ctx, nil, time.Date(2030, 1, 2, 10, 0, 0, 0, time.UTC))
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, deleted)

	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	const marker = "synthetic-payload-that-must-not-be-logged"
	callback := "cleanup-synthetic-error"
	var cancelDuringSQL context.CancelFunc
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register(callback, func(tx *gorm.DB) {
		if cancelDuringSQL != nil {
			cancelDuringSQL()
			return
		}
		tx.AddError(errors.New(marker))
	}))
	defer db.Callback().Raw().Remove(callback)
	deleted, err = services.CleanupApplicationOperations(context.Background(), db, time.Date(2030, 1, 2, 10, 0, 0, 0, time.UTC))
	require.Error(t, err)
	require.NotContains(t, err.Error(), marker)
	require.Zero(t, deleted)
	activeContext, activeCancel := context.WithCancel(context.Background())
	defer activeCancel()
	cancelDuringSQL = activeCancel
	deleted, err = services.CleanupApplicationOperations(activeContext, db, time.Date(2030, 1, 2, 10, 0, 0, 0, time.UTC))
	require.ErrorIs(t, err, context.Canceled, "cancellation at SQL dispatch must remain a context error")
	require.Zero(t, deleted)
}
