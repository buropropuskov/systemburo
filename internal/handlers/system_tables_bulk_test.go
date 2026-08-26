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

// Групповая архивация/восстановление системных таблиц на уровне сервиса
// (лёгкий тест без CleanDB/Seed - пакет handlers на грани CI-таймаута под
// -race, урок про #ci_handlers_test_timeout: не плодить лишние Seed-циклы).
// Создаёт таблицы с уникальными именами и чистит их за собой.
func TestSystemTableService_BulkArchiveRestore(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewSystemTableService(db, "", 0, nil)
	ctx := context.Background()

	aName := uniq("bulk_st_a")
	bName := uniq("bulk_st_b")
	aID, err := svc.Create(ctx, models.CreateSystemTableRequest{
		Name: aName, DisplayName: "Bulk Table A", TableType: models.TableTypeCars,
	})
	require.NoError(t, err)
	bID, err := svc.Create(ctx, models.CreateSystemTableRequest{
		Name: bName, DisplayName: "Bulk Table B", TableType: models.TableTypeCars,
	})
	require.NoError(t, err)
	defer func() {
		db.Delete(&models.SystemTable{}, aID)
		db.Delete(&models.SystemTable{}, bID)
	}()

	// Полный успех: обе таблицы в архив.
	res, err := svc.BulkArchive(ctx, []int{aID, bID})
	require.NoError(t, err)
	assert.Equal(t, 2, res.SuccessCount)
	assert.Equal(t, 0, res.ErrorCount)

	// GetByID фильтрует только активные - после архивации 404, что и ожидаем.
	_, err = svc.GetByID(ctx, aID)
	assert.Error(t, err)

	// Дедуп: один и тот же id дважды -> один успех. Также проверяет, что имя
	// архивной таблицы резолвится верно (findTableName не фильтрует is_active) -
	// восстановление archived-таблицы через GetByID сломало бы имя в Errors.
	res, err = svc.BulkRestore(ctx, []int{aID, aID})
	require.NoError(t, err)
	assert.Equal(t, 1, res.SuccessCount)
	gotA, err := svc.GetByID(ctx, aID)
	require.NoError(t, err)
	assert.True(t, gotA.Table.IsActive)

	// Частичный успех: существующий (архивируем b) + несуществующий -> 1 успех, 1 ошибка.
	res, err = svc.BulkArchive(ctx, []int{bID, 999999})
	require.NoError(t, err)
	assert.Equal(t, 1, res.SuccessCount)
	assert.Equal(t, 1, res.ErrorCount)
	require.Len(t, res.Errors, 1)
	assert.Equal(t, 999999, res.Errors[0].ID)

	// Повторный restore уже активной таблицы -> ошибка "не в архиве", попадает в Errors.
	res, err = svc.BulkRestore(ctx, []int{aID})
	require.NoError(t, err)
	assert.Equal(t, 0, res.SuccessCount)
	assert.Equal(t, 1, res.ErrorCount)
	assert.Equal(t, "Bulk Table A", res.Errors[0].Name)

	// Пустой список -> пустой результат без ошибок (гейт len==0 -> 400 живёт в handler).
	res, err = svc.BulkArchive(ctx, []int{})
	require.NoError(t, err)
	assert.Equal(t, 0, res.SuccessCount)
	assert.Equal(t, 0, res.ErrorCount)
}
