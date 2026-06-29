package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSystemTable_History_BackfillLegacyIntoAudit проверяет финал #870 (срез F.3):
// читатель истории таблицы переведён на audit_log-only, а до-cutover строки замороженной
// system_table_histories поднимаются в audit_log разовым BackfillAuditFromLegacy.
// details уже jsonb - переносится verbatim.
func TestSystemTable_History_BackfillLegacyIntoAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Создание через API -> запись created уходит уже в audit_log.
	rec := testutil.POST(t, e, "/system-tables", `{"name":"audit_test_table","display_name":"Audit Test","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	id := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Легаси-строка напрямую в замороженную таблицу с более ранним временем (как до cutover).
	legacy := models.SystemTableHistory{
		SystemTableID: id,
		ActionType:    models.SystemTableActionUpdated,
		Details:       json.RawMessage(`{"display_name":"Old Name"}`),
		CreatedAt:     time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// До backfill читатель видит только audit_log -> legacy-строка ещё невидима.
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, testutil.ParseSlice(t, rec), 1, "до backfill видно только audit_log")

	// CleanDB-Seed уже выставил гард-флаг (backfill прогонялся на пустой таблице) -
	// снимаем, чтобы перенести только что вставленную legacy-строку.
	require.NoError(t, db.Where("key = ?", "audit_backfilled:"+models.AuditEntitySystemTable).
		Delete(&models.SystemSetting{}).Error)
	require.NoError(t, database.BackfillAuditFromLegacy(db))

	// Легаси-строка физически скопирована в audit_log, а старая таблица цела (бэкап).
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntitySystemTable, id, models.SystemTableActionUpdated).
		Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount, "legacy updated перенесён в audit_log")
	require.NoError(t, db.Table("system_table_histories").Where("system_table_id = ?", id).Count(&legacyCount).Error)
	assert.Equal(t, int64(1), legacyCount, "старая таблица не тронута backfill'ом - read-only бэкап")

	// История отдаёт обе записи из audit_log, новые сверху, details verbatim.
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2, "после backfill видны и created, и перенесённая legacy-строка")
	assert.Equal(t, "created", hist[0]["action_type"], "новее сверху - created из audit_log")
	assert.Equal(t, "updated", hist[1]["action_type"], "legacy updated час назад - ниже")
	assert.Equal(t, "Old Name", hist[1]["details"].(map[string]interface{})["display_name"], "details перенесены verbatim")
	assert.NotEmpty(t, hist[0]["user_name"], "user_name должен заполняться")

	// Идемпотентность: повторный backfill не дублирует (гард-флаг снова стоит).
	require.NoError(t, database.BackfillAuditFromLegacy(db))
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, testutil.ParseSlice(t, rec), 2, "повторный backfill не создаёт дублей")
}

// TestTrash_History_BackfillLegacyIntoAudit проверяет финал #870 (срез F.3):
// читатель истории корзины переведён на audit_log-only, а до-cutover строки замороженной
// system_table_trash_histories поднимаются в audit_log разовым BackfillAuditFromLegacy.
// Плоский affected_count + items сворачиваются в details в форме recorder'а.
func TestTrash_History_BackfillLegacyIntoAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "trashaudit1", "pass123", 1, td.OrgID, td.CompanyID)

	dn := "Корзина Аудит"
	tbl := models.SystemTable{Name: "trash_audit_t1", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	h := testutil.AuthHeader(token)

	// Удаляем машину в корзину.
	var u models.User
	require.NoError(t, db.Where("username = ?", "trashaudit1").First(&u).Error)
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID),
		fmt.Sprintf(`{"status":0,"user_id":%d,"table_id":%d}`, u.ID, tbl.ID), h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Восстанавливаем -> создаётся запись bulk_restored в audit_log.
	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/trash/restore", tbl.ID),
		fmt.Sprintf(`{"ids":[%d]}`, carID), h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Легаси-строка с более ранним временем (накоплена до cutover, ещё не перенесена).
	uid := u.ID
	legacy := models.SystemTableTrashHistory{
		SystemTableID: tbl.ID,
		ActionType:    models.TrashActionCleared,
		AffectedCount: 5,
		UserID:        &uid,
		CreatedAt:     time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// До backfill читатель видит только audit_log -> legacy-строка ещё невидима.
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash/history", tbl.ID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, testutil.ParseSlice(t, rec), 1, "до backfill видно только audit_log")

	// CleanDB-Seed уже выставил гард-флаг (backfill прогонялся на пустой таблице) -
	// снимаем, чтобы перенести только что вставленную legacy-строку.
	require.NoError(t, db.Where("key = ?", "audit_backfilled:"+models.AuditEntitySystemTableTrash).
		Delete(&models.SystemSetting{}).Error)
	require.NoError(t, database.BackfillAuditFromLegacy(db))

	// Легаси-строка физически скопирована в audit_log, а старая таблица цела (бэкап).
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").Where("entity_type = ? AND entity_id = ?", models.AuditEntitySystemTableTrash, tbl.ID).Count(&auditCount).Error)
	assert.Equal(t, int64(2), auditCount, "bulk_restored из API + перенесённая legacy-строка")
	require.NoError(t, db.Table("system_table_trash_histories").Where("system_table_id = ?", tbl.ID).Count(&legacyCount).Error)
	assert.Equal(t, int64(1), legacyCount, "старая таблица не тронута backfill'ом - read-only бэкап")

	// История корзины отдаёт обе записи из audit_log, новые сверху; affected_count свёрнут в details.
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash/history", tbl.ID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2, "после backfill видны и bulk_restored, и перенесённая legacy-строка")
	assert.Equal(t, "bulk_restored", hist[0]["action_type"], "новее сверху - bulk_restored из audit_log")
	assert.Equal(t, float64(1), hist[0]["affected_count"], "affected_count из audit_log")
	assert.Equal(t, "cleared", hist[1]["action_type"], "legacy cleared час назад - ниже")
	assert.Equal(t, float64(5), hist[1]["affected_count"], "перенесённый affected_count из details")

	// Идемпотентность: повторный backfill не дублирует (гард-флаг снова стоит).
	require.NoError(t, database.BackfillAuditFromLegacy(db))
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash/history", tbl.ID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, testutil.ParseSlice(t, rec), 2, "повторный backfill не создаёт дублей")
}

// TestSystemTables_WriteFlip_ColumnsUpdatedToAuditLog проверяет что оба endpoint-а
// обновления колонок (main и fact) пишут action='columns_updated' в audit_log (#870).
// UpdateFields -> PUT /system-tables/:id/fields (variant=main).
// UpdateFactFields -> PUT /system-tables/:id/fact-fields (variant=fact).
func TestSystemTables_WriteFlip_ColumnsUpdatedToAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	t.Run("main fields", func(t *testing.T) {
		rec := testutil.POST(t, e, "/system-tables",
			`{"name":"colupd_main","display_name":"ColUpdMain","table_type":"cars"}`, h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

		body := `{"fields":[{"field_name":"status","is_visible":false},{"field_name":"car_number","is_visible":true}]}`
		rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d/fields", tableID), body, h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var n int64
		require.NoError(t, db.Model(&models.AuditLog{}).
			Where("entity_type = ? AND entity_id = ? AND action = ?",
				models.AuditEntitySystemTable, tableID, models.SystemTableActionColumnsUpdated).
			Count(&n).Error)
		assert.GreaterOrEqual(t, n, int64(1), "PUT /fields должен писать columns_updated в audit_log")
	})

	t.Run("fact fields", func(t *testing.T) {
		// show_fact_table:true создаёт fact-поля по умолчанию (как в TestSystemTables_UpdateFactFields_PersistsVisibility).
		rec := testutil.POST(t, e, "/system-tables",
			`{"name":"colupd_fact","display_name":"ColUpdFact","table_type":"cars","show_fact_table":true}`, h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

		body := `{"fields":[{"field_name":"organization","is_visible":false},{"field_name":"car_number","is_visible":true}]}`
		rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d/fact-fields", tableID), body, h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var n int64
		require.NoError(t, db.Model(&models.AuditLog{}).
			Where("entity_type = ? AND entity_id = ? AND action = ?",
				models.AuditEntitySystemTable, tableID, models.SystemTableActionColumnsUpdated).
			Count(&n).Error)
		assert.GreaterOrEqual(t, n, int64(1), "PUT /fact-fields должен писать columns_updated в audit_log")
	})
}
