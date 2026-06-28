package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSystemTable_History_UnionLegacyAndAudit проверяет переходную модель #870:
// новые действия пишутся в audit_log, а строки из замороженной system_table_histories
// по-прежнему видны в истории через union.
func TestSystemTable_History_UnionLegacyAndAudit(t *testing.T) {
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

	// Подтверждаем что новая запись физически в audit_log (не в старой таблице).
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").Where("entity_type = ? AND entity_id = ?", models.AuditEntitySystemTable, id).Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount, "created должен попасть в audit_log")
	require.NoError(t, db.Table("system_table_histories").Where("system_table_id = ?", id).Count(&legacyCount).Error)
	assert.Equal(t, int64(0), legacyCount, "старая таблица больше не пишется")

	// Легаси-строка напрямую в старую таблицу с более ранним временем (как до cutover).
	legacy := models.SystemTableHistory{
		SystemTableID: id,
		ActionType:    models.SystemTableActionUpdated,
		Details:       json.RawMessage(`{"display_name":"Old Name"}`),
		CreatedAt:     time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// История endpoint-а объединяет обе таблицы, новые сверху.
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2, "union должен отдать и audit_log, и legacy-строку")
	assert.Equal(t, "created", hist[0]["action_type"], "новее сверху - created из audit_log")
	assert.Equal(t, "updated", hist[1]["action_type"], "legacy updated час назад - ниже")
	assert.NotEmpty(t, hist[0]["user_name"], "user_name должен заполняться")
}

// TestTrash_History_UnionLegacyAndAudit проверяет что история корзины (#870)
// объединяет legacy system_table_trash_histories и новые записи audit_log.
func TestTrash_History_UnionLegacyAndAudit(t *testing.T) {
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

	// Подтверждаем что запись попала в audit_log.
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").Where("entity_type = ? AND entity_id = ?", models.AuditEntitySystemTableTrash, tbl.ID).Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount, "bulk_restored должен попасть в audit_log")
	require.NoError(t, db.Table("system_table_trash_histories").Where("system_table_id = ?", tbl.ID).Count(&legacyCount).Error)
	assert.Equal(t, int64(0), legacyCount, "старая таблица больше не пишется")

	// Легаси-строка с более ранним временем.
	uid := u.ID
	legacy := models.SystemTableTrashHistory{
		SystemTableID: tbl.ID,
		ActionType:    models.TrashActionCleared,
		AffectedCount: 5,
		UserID:        &uid,
		CreatedAt:     time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// История корзины объединяет обе таблицы.
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/trash/history", tbl.ID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2, "union должен отдать и audit_log, и legacy-строку")
	assert.Equal(t, "bulk_restored", hist[0]["action_type"], "новее сверху - bulk_restored из audit_log")
	assert.Equal(t, float64(1), hist[0]["affected_count"], "affected_count из audit_log")
	assert.Equal(t, "cleared", hist[1]["action_type"], "legacy cleared час назад - ниже")
	assert.Equal(t, float64(5), hist[1]["affected_count"], "affected_count из legacy")
}
