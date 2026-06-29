package handlers_test

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUniqueCar_History_UnionLegacyAndAudit проверяет, что GetHistory объединяет
// замороженные строки unique_cars_history (старый write-path) и новые записи
// audit_log[unique_car] в одну ленту (#870, срез 1.12d). Легаси-строка пишется
// напрямую в таблицу, новое изменение идёт через сервис (recorder -> audit_log) -
// оба видны, новейшее первым, с разрезолвленным username.
func TestUniqueCar_History_UnionLegacyAndAudit(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	owner := models.User{
		Username:       "uc_union_owner",
		Password:       "x",
		TypeID:         6,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	require.NoError(t, db.Create(&owner).Error)

	number := "У111НН77"
	mark := "Лада"
	formatOld := 1
	car := models.UniqueCar{
		Number:         &number,
		Mark:           &mark,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
		FormatID:       &formatOld,
		UserID:         &owner.ID,
	}
	require.NoError(t, db.Create(&car).Error)

	// Легаси-строка истории напрямую в замороженную unique_cars_history (час назад).
	legacyField := "number"
	legacyOld := "У000НН77"
	legacyNew := "У111НН77"
	legacy := models.UniqueCarHistory{
		UniqueCarID: car.ID,
		UserID:      &owner.ID,
		ActionType:  "data_changed",
		FieldName:   &legacyField,
		OldValue:    &legacyOld,
		NewValue:    &legacyNew,
		CreatedAt:   time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// Новое изменение через сервис -> recorder -> audit_log[unique_car].
	svc := services.NewUniqueCarService(db)
	formatNew := 2
	_, err := svc.Update(context.Background(), owner.Username, car.ID, services.NewUniqueCarRequest{
		Number:         number,
		Mark:           mark,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
		FormatID:       &formatNew,
		UserID:         &owner.ID,
	})
	require.NoError(t, err)

	// Новая запись в audit_log, легаси-таблица не выросла.
	var legacyCount int64
	require.NoError(t, db.Model(&models.UniqueCarHistory{}).
		Where("unique_car_id = ?", car.ID).Count(&legacyCount).Error)
	assert.Equal(t, int64(1), legacyCount, "unique_cars_history содержит только легаси-строку")

	var auditCount int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ?", models.AuditEntityUniqueCar, car.ID).
		Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount, "изменение format_id записано в audit_log")

	// Union-чтение возвращает обе записи, новейшая (audit, format_id) первой.
	items, err := svc.GetHistory(context.Background(), owner.Username, car.ID)
	require.NoError(t, err)
	require.Len(t, items, 2, "история объединяет легаси-строку и audit_log")

	assert.Equal(t, "data_changed", items[0].ActionType)
	assert.Equal(t, "format_id", deref(items[0].FieldName))
	assert.Equal(t, "1", deref(items[0].OldValue))
	assert.Equal(t, "2", deref(items[0].NewValue))
	require.NotNil(t, items[0].Username)
	assert.Equal(t, owner.Username, *items[0].Username)

	assert.Equal(t, "number", deref(items[1].FieldName))
	assert.Equal(t, legacyOld, deref(items[1].OldValue))
	assert.Equal(t, legacyNew, deref(items[1].NewValue))
	require.NotNil(t, items[1].Username)
	assert.Equal(t, owner.Username, *items[1].Username)
}
