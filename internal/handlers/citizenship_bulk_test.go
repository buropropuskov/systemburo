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

// Групповая архивация/восстановление гражданств на уровне сервиса (лёгкий тест
// без CleanDB/Seed - пакет handlers на грани CI-таймаута под -race, урок про
// #ci_handlers_test_timeout: не плодить лишние Seed-циклы). Реюзает единожды
// засеянную БД, создаёт гражданства с уникальными именами и чистит их за собой.
func TestCitizenshipService_BulkArchiveRestore(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewCitizenshipService(db)
	userID, _, cleanup := setupMWUser(t, db, true, false)
	defer cleanup()
	ctx := context.Background()

	idA, err := svc.Create(ctx, userID, models.CreateCitizenshipRequest{Name: uniq("bulkCitizenA")})
	require.NoError(t, err)
	idB, err := svc.Create(ctx, userID, models.CreateCitizenshipRequest{Name: uniq("bulkCitizenB")})
	require.NoError(t, err)
	defer func() { db.Delete(&models.Citizenship{}, idA); db.Delete(&models.Citizenship{}, idB) }()

	isActive := func(id int) bool {
		var c models.Citizenship
		require.NoError(t, db.First(&c, id).Error)
		return c.IsActive
	}

	// Полный успех: оба гражданства в архив.
	res, err := svc.BulkArchive(ctx, []int{idA, idB}, userID)
	require.NoError(t, err)
	assert.Equal(t, 2, res.SuccessCount)
	assert.Equal(t, 0, res.ErrorCount)
	assert.False(t, isActive(idA))
	assert.False(t, isActive(idB))

	// Дедуп: один и тот же id дважды -> один успех.
	res, err = svc.BulkRestore(ctx, []int{idA, idA}, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, res.SuccessCount)
	assert.True(t, isActive(idA))

	// Частичный успех: существующий + несуществующий -> 1 успех, 1 ошибка.
	res, err = svc.BulkArchive(ctx, []int{idB, 999999}, userID)
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
