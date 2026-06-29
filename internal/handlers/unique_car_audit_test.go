package handlers_test

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUniqueCar_History_BackfillLegacyIntoAudit проверяет финал #870 (срез F.4):
// GetHistory переведён на audit_log-only, а замороженные строки unique_cars_history
// поднимаются в audit_log разовым BackfillAuditFromLegacy (плоские field_name/old/new/
// comment сворачиваются в details jsonb формы carAuditDetails). Новое изменение идёт
// через сервис (recorder -> audit_log), legacy - через backfill; оба видны, новейшее
// первым, с разрезолвленным username.
func TestUniqueCar_History_BackfillLegacyIntoAudit(t *testing.T) {
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

	// До backfill читатель видит только audit_log -> legacy-строка ещё невидима.
	items, err := svc.GetHistory(context.Background(), owner.Username, car.ID)
	require.NoError(t, err)
	require.Len(t, items, 1, "до backfill видно только audit_log (format_id)")

	// Снять гард-флаг (Seed выставил его на пустой таблице) и перенести legacy-строку.
	require.NoError(t, db.Where("key = ?", "audit_backfilled:"+models.AuditEntityUniqueCar).
		Delete(&models.SystemSetting{}).Error)
	require.NoError(t, database.BackfillAuditFromLegacy(db))

	// Легаси-строка скопирована в audit_log, замороженная таблица цела (бэкап).
	var legacyCount, auditCount int64
	require.NoError(t, db.Model(&models.UniqueCarHistory{}).
		Where("unique_car_id = ?", car.ID).Count(&legacyCount).Error)
	assert.Equal(t, int64(1), legacyCount, "unique_cars_history не тронута backfill'ом - read-only бэкап")
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ?", models.AuditEntityUniqueCar, car.ID).
		Count(&auditCount).Error)
	assert.Equal(t, int64(2), auditCount, "format_id (cutover) + перенесённая number (backfill)")

	// Чтение возвращает обе записи, новейшая (audit, format_id) первой.
	items, err = svc.GetHistory(context.Background(), owner.Username, car.ID)
	require.NoError(t, err)
	require.Len(t, items, 2, "история отдаёт и audit_log, и перенесённую legacy-строку")

	assert.Equal(t, "data_changed", items[0].ActionType)
	assert.Equal(t, "format_id", deref(items[0].FieldName))
	assert.Equal(t, "1", deref(items[0].OldValue))
	assert.Equal(t, "2", deref(items[0].NewValue))
	require.NotNil(t, items[0].Username)
	assert.Equal(t, owner.Username, *items[0].Username)

	assert.Equal(t, "data_changed", items[1].ActionType)
	assert.Equal(t, "number", deref(items[1].FieldName))
	assert.Equal(t, legacyOld, deref(items[1].OldValue))
	assert.Equal(t, legacyNew, deref(items[1].NewValue))
	require.NotNil(t, items[1].Username)
	assert.Equal(t, owner.Username, *items[1].Username)

	// Идемпотентность: повторный backfill не дублирует (гард-флаг снова стоит).
	require.NoError(t, database.BackfillAuditFromLegacy(db))
	items, err = svc.GetHistory(context.Background(), owner.Username, car.ID)
	require.NoError(t, err)
	assert.Len(t, items, 2, "повторный backfill не создаёт дублей")
}
