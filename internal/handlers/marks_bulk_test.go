package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Групповая архивация/восстановление марок на уровне сервиса (лёгкий тест без
// CleanDB/Seed - пакет handlers на грани CI-таймаута под -race, урок про
// #ci_handlers_test_timeout: не плодить лишние Seed-циклы). Реюзает единожды
// засеянную БолД, создаёт марки с уникальными именами и чистит их за собой.
func TestMarkService_BulkArchiveRestore(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewMarkService(db)
	userID, _, cleanup := setupMWUser(t, db, true, false)
	defer cleanup()
	ctx := context.Background()

	a, err := svc.Create(ctx, models.CreateMarkRequest{Name: uniqMarkName("bulkA")}, userID)
	require.NoError(t, err)
	b, err := svc.Create(ctx, models.CreateMarkRequest{Name: uniqMarkName("bulkB")}, userID)
	require.NoError(t, err)
	defer func() { db.Delete(&models.Mark{}, a.ID); db.Delete(&models.Mark{}, b.ID) }()

	// Полный успех: обе марки в архив.
	res, err := svc.BulkArchive(ctx, []int{a.ID, b.ID}, userID)
	require.NoError(t, err)
	assert.Equal(t, 2, res.SuccessCount)
	assert.Equal(t, 0, res.ErrorCount)
	gotA, _ := svc.GetByID(ctx, a.ID)
	assert.False(t, gotA.IsActive)

	// Дедуп: один и тот же id дважды -> один успех.
	res, err = svc.BulkRestore(ctx, []int{a.ID, a.ID}, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, res.SuccessCount)
	gotA, _ = svc.GetByID(ctx, a.ID)
	assert.True(t, gotA.IsActive)

	// Частичный успех: существующий + несуществующий -> 1 успех, 1 ошибка.
	res, err = svc.BulkArchive(ctx, []int{b.ID, 999999}, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, res.SuccessCount)
	assert.Equal(t, 1, res.ErrorCount)
	require.Len(t, res.Errors, 1)
	assert.Equal(t, 999999, res.Errors[0].ID)

	// Пустой список -> пустой результат без ошибок (гейт len==0 -> 400 живёт в handler).
	res, err = svc.BulkArchive(ctx, []int{}, userID)
	require.NoError(t, err)
	assert.Equal(t, 0, res.SuccessCount)
	assert.Equal(t, 0, res.ErrorCount)
}
