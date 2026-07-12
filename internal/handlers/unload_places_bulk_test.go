package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Групповая архивация/восстановление мест разгрузки на уровне сервиса (лёгкий
// тест без CleanDB/Seed - пакет handlers на грани CI-таймаута под -race, урок
// #ci_handlers_test_timeout: не плодить лишние Seed-циклы). Реюзает единожды
// засеянную БД, создаёт места с уникальными именами и чистит их за собой.
func TestUnloadPlaceService_BulkArchiveRestore(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewUnloadPlaceService(db)
	userID, _, cleanup := setupMWUser(t, db, true, false)
	defer cleanup()
	ctx := context.Background()

	aID, err := svc.Create(ctx, userID, services.CreateUnloadPlaceRequest{Name: uniq("bulkPlaceA")})
	require.NoError(t, err)
	bID, err := svc.Create(ctx, userID, services.CreateUnloadPlaceRequest{Name: uniq("bulkPlaceB")})
	require.NoError(t, err)
	defer func() {
		db.Exec("DELETE FROM unload_places WHERE id IN (?, ?)", aID, bID)
	}()

	// Полный успех: оба места в архив.
	res, err := svc.BulkArchive(ctx, userID, []int{aID, bID})
	require.NoError(t, err)
	assert.Equal(t, 2, res.SuccessCount)
	assert.Equal(t, 0, res.ErrorCount)
	gotA, err := svc.GetByID(ctx, aID)
	require.NoError(t, err)
	assert.False(t, gotA.IsActive)

	// Дедуп: один и тот же id дважды -> один успех.
	res, err = svc.BulkRestore(ctx, userID, []int{aID, aID})
	require.NoError(t, err)
	assert.Equal(t, 1, res.SuccessCount)
	gotA, err = svc.GetByID(ctx, aID)
	require.NoError(t, err)
	assert.True(t, gotA.IsActive)

	// Частичный успех: существующий + несуществующий -> 1 успех, 1 ошибка.
	res, err = svc.BulkArchive(ctx, userID, []int{bID, 999999})
	require.NoError(t, err)
	assert.Equal(t, 1, res.SuccessCount)
	assert.Equal(t, 1, res.ErrorCount)
	require.Len(t, res.Errors, 1)
	assert.Equal(t, 999999, res.Errors[0].ID)

	// Пустой список -> пустой результат без ошибок (гейт len==0 -> 400 живёт в handler).
	res, err = svc.BulkArchive(ctx, userID, []int{})
	require.NoError(t, err)
	assert.Equal(t, 0, res.SuccessCount)
	assert.Equal(t, 0, res.ErrorCount)
}
