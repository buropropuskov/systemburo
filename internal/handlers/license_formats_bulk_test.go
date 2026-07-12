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

// Групповая архивация/восстановление форматов номеров на уровне сервиса (лёгкий
// тест без CleanDB/Seed - пакет handlers на грани CI-таймаута под -race, урок
// #ci_handlers_test_timeout: не плодить лишние Seed-циклы). Реюзает единожды
// засеянную БД, создаёт форматы с уникальными именами и чистит их за собой.
func TestLicenseFormatService_BulkArchiveRestore(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewLicensePlateFormatService(db)
	userID, _, cleanup := setupMWUser(t, db, true, false)
	defer cleanup()
	ctx := context.Background()

	// cells может быть пустым; is_default не ставим, чтобы Delete не упирался в
	// "нельзя архивировать формат по умолчанию".
	aID, err := svc.Create(ctx, userID, models.CreateLicensePlateFormatRequest{Name: uniq("bulkfmtA")})
	require.NoError(t, err)
	bID, err := svc.Create(ctx, userID, models.CreateLicensePlateFormatRequest{Name: uniq("bulkfmtB")})
	require.NoError(t, err)
	defer func() { db.Delete(&models.LicensePlateFormat{}, aID); db.Delete(&models.LicensePlateFormat{}, bID) }()

	isActive := func(id int) bool {
		var f models.LicensePlateFormat
		require.NoError(t, db.First(&f, id).Error)
		return f.IsActive
	}

	// Полный успех: оба формата в архив.
	res, err := svc.BulkArchive(ctx, userID, []int{aID, bID})
	require.NoError(t, err)
	assert.Equal(t, 2, res.SuccessCount)
	assert.Equal(t, 0, res.ErrorCount)
	assert.False(t, isActive(aID))
	assert.False(t, isActive(bID))

	// Дедуп: один и тот же id дважды -> один успех.
	res, err = svc.BulkRestore(ctx, userID, []int{aID, aID})
	require.NoError(t, err)
	assert.Equal(t, 1, res.SuccessCount)
	assert.True(t, isActive(aID))

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
