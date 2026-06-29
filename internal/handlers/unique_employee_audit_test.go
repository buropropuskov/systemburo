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

// TestUniqueEmployee_History_BackfillLegacyIntoAudit проверяет финал #870 (срез F.4):
// GetHistory переведён на audit_log-only, а замороженные строки unique_employees_history
// поднимаются в audit_log разовым BackfillAuditFromLegacy (плоские field_name/old/new/
// comment сворачиваются в details jsonb формы carAuditDetails). Новое изменение идёт
// через сервис (recorder -> audit_log), legacy - через backfill; оба видны, новейшее
// первым, с разрезолвленным username.
func TestUniqueEmployee_History_BackfillLegacyIntoAudit(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	owner := models.User{
		Username:       "ue_union_owner",
		Password:       "x",
		TypeID:         6,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	require.NoError(t, db.Create(&owner).Error)

	lastOld := "Иванов"
	first := "Иван"
	posOld := "Грузчик"
	emp := models.UniqueEmployee{
		LastName:       &lastOld,
		FirstName:      &first,
		Position:       &posOld,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
		UserID:         &owner.ID,
	}
	require.NoError(t, db.Create(&emp).Error)

	// Легаси-строка истории напрямую в замороженную unique_employees_history (час назад).
	legacyField := "middle_name"
	legacyOld := "Петрович"
	legacyNew := "Сергеевич"
	legacy := models.UniqueEmployeeHistory{
		UniqueEmployeeID: emp.ID,
		UserID:           &owner.ID,
		ActionType:       "data_changed",
		FieldName:        &legacyField,
		OldValue:         &legacyOld,
		NewValue:         &legacyNew,
		CreatedAt:        time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// Новое изменение через сервис -> recorder -> audit_log[unique_employee].
	svc := services.NewUniqueEmployeeService(db)
	newPos := "Старший грузчик"
	_, err := svc.Update(context.Background(), owner.Username, emp.ID, services.NewUniqueEmployeeRequest{
		LastName:       &lastOld,
		FirstName:      &first,
		Position:       &newPos,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
		UserID:         &owner.ID,
	})
	require.NoError(t, err)

	// До backfill читатель видит только audit_log -> legacy-строка ещё невидима.
	items, err := svc.GetHistory(context.Background(), owner.Username, emp.ID)
	require.NoError(t, err)
	require.Len(t, items, 1, "до backfill видно только audit_log (position)")

	// Снять гард-флаг (Seed выставил его на пустой таблице) и перенести legacy-строку.
	require.NoError(t, db.Where("key = ?", "audit_backfilled:"+models.AuditEntityUniqueEmployee).
		Delete(&models.SystemSetting{}).Error)
	require.NoError(t, database.BackfillAuditFromLegacy(db))

	// Легаси-строка скопирована в audit_log, замороженная таблица цела (бэкап).
	var legacyCount, auditCount int64
	require.NoError(t, db.Model(&models.UniqueEmployeeHistory{}).
		Where("unique_employee_id = ?", emp.ID).Count(&legacyCount).Error)
	assert.Equal(t, int64(1), legacyCount, "unique_employees_history не тронута backfill'ом - read-only бэкап")
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ?", models.AuditEntityUniqueEmployee, emp.ID).
		Count(&auditCount).Error)
	assert.Equal(t, int64(2), auditCount, "position (cutover) + перенесённая middle_name (backfill)")

	// Чтение возвращает обе записи, новейшая (audit, position) первой.
	items, err = svc.GetHistory(context.Background(), owner.Username, emp.ID)
	require.NoError(t, err)
	require.Len(t, items, 2, "история отдаёт и audit_log, и перенесённую legacy-строку")

	assert.Equal(t, "data_changed", items[0].ActionType)
	assert.Equal(t, "position", deref(items[0].FieldName))
	assert.Equal(t, "Грузчик", deref(items[0].OldValue))
	assert.Equal(t, "Старший грузчик", deref(items[0].NewValue))
	require.NotNil(t, items[0].Username)
	assert.Equal(t, owner.Username, *items[0].Username)

	assert.Equal(t, "data_changed", items[1].ActionType)
	assert.Equal(t, "middle_name", deref(items[1].FieldName))
	assert.Equal(t, legacyOld, deref(items[1].OldValue))
	assert.Equal(t, legacyNew, deref(items[1].NewValue))
	require.NotNil(t, items[1].Username)
	assert.Equal(t, owner.Username, *items[1].Username)

	// Идемпотентность: повторный backfill не дублирует (гард-флаг снова стоит).
	require.NoError(t, database.BackfillAuditFromLegacy(db))
	items, err = svc.GetHistory(context.Background(), owner.Username, emp.ID)
	require.NoError(t, err)
	assert.Len(t, items, 2, "повторный backfill не создаёт дублей")
}
