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

// TestVehicleBlacklist_History_UnionLegacyAndAudit проверяет переходную модель #870:
// новые действия пишутся в audit_log, а строки из замороженной vehicle_blacklist_histories
// по-прежнему видны в истории через union. Гарантирует "история та же" до backfill.
func TestVehicleBlacklist_History_UnionLegacyAndAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	mark := seedMark(t, db, "VBL_AuditMark")

	// Создание через API -> created уходит уже в audit_log (cutover записи).
	body := fmt.Sprintf(`{"car_number":"T001TT799","mark_id":%d,"reason":"audit-test"}`, mark.ID)
	rec := testutil.POST(t, e, "/vehicle-blacklist", body, h)
	require.Equal(t, http.StatusCreated, rec.Code)
	entryMap := testutil.ParseMap(t, rec)
	id := int(entryMap["id"].(float64))

	// Подтверждаем: новая запись физически в audit_log, а не в старой таблице.
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").Where("entity_type = ? AND entity_id = ?", models.AuditEntityVehicleBlacklist, id).Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount, "created должен попасть в audit_log")
	require.NoError(t, db.Table("vehicle_blacklist_histories").Where("entity_id = ?", id).Count(&legacyCount).Error)
	assert.Equal(t, int64(0), legacyCount, "старая таблица больше не пишется")

	// Легаси-строка напрямую в старую таблицу с более ранним временем (как до миграции).
	legacy := models.VehicleBlacklistHistory{
		EntityID:   id,
		ActionType: models.BlacklistActionArchived,
		Details:    json.RawMessage(`{"car_number":"T001TT799","mark_name":"VBL_AuditMark"}`),
		CreatedAt:  time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// История per-id объединяет обе таблицы, новые сверху.
	rec = testutil.GET(t, e, fmt.Sprintf("/vehicle-blacklist/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2, "union должен отдать и audit_log, и legacy-строку")
	assert.Equal(t, "created", hist[0]["action_type"], "новее сверху - created из audit_log")
	assert.Equal(t, "archived", hist[1]["action_type"], "legacy archived час назад - ниже")
}

// TestVehicleBlacklist_AllHistory_UnionLegacyAndAudit проверяет глобальный журнал (#870):
// GET /vehicle-blacklist/history отдаёт события из обеих таблиц через union.
func TestVehicleBlacklist_AllHistory_UnionLegacyAndAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	mark := seedMark(t, db, "VBL_AllAuditMark")

	// Создать запись через API (goes to audit_log).
	body := fmt.Sprintf(`{"car_number":"T002TT799","mark_id":%d,"reason":"all-audit-test"}`, mark.ID)
	rec := testutil.POST(t, e, "/vehicle-blacklist", body, h)
	require.Equal(t, http.StatusCreated, rec.Code)
	id := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Легаси-строка на другую (уже удалённую) запись с entity_id=0 (нет FK).
	legacyEntityID := 999999
	legacy := models.VehicleBlacklistHistory{
		EntityID:   legacyEntityID,
		ActionType: models.BlacklistActionPurged,
		Details:    json.RawMessage(`{"car_number":"OLD999","mark_name":"OldMark"}`),
		CreatedAt:  time.Now().Add(-2 * time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// GET /vehicle-blacklist/history объединяет обе таблицы.
	rec = testutil.GET(t, e, "/vehicle-blacklist/history", h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)

	var sawCreated, sawLegacyPurged bool
	for _, item := range hist {
		eid := int(item["entity_id"].(float64))
		action := item["action_type"].(string)
		if eid == id && action == models.BlacklistActionCreated {
			sawCreated = true
		}
		if eid == legacyEntityID && action == models.BlacklistActionPurged {
			sawLegacyPurged = true
		}
	}
	assert.True(t, sawCreated, "должна быть запись created из audit_log")
	assert.True(t, sawLegacyPurged, "должна быть legacy-строка purged")
}

// TestVehicleBlacklist_WriteFlip_RestoreToAuditLog: update/archive/restore через API
// пишут соответствующие действия в audit_log (#870): 'updated', 'archived', 'restored'.
func TestVehicleBlacklist_WriteFlip_RestoreToAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	mark := seedMark(t, db, "VBL_RestoreMark")

	body := fmt.Sprintf(`{"car_number":"T003TT799","mark_id":%d,"reason":"restore-test"}`, mark.ID)
	rec := testutil.POST(t, e, "/vehicle-blacklist", body, h)
	require.Equal(t, http.StatusCreated, rec.Code, "create: %s", rec.Body.String())
	id := int(testutil.ParseMap(t, rec)["id"].(float64))

	// PUT update -> 'updated' в audit_log.
	updateBody := fmt.Sprintf(`{"car_number":"T003TT799","mark_id":%d,"reason":"обновлённая причина"}`, mark.ID)
	rec = testutil.PUT(t, e, fmt.Sprintf("/vehicle-blacklist/%d", id), updateBody, h)
	require.Equal(t, http.StatusOK, rec.Code, "update: %s", rec.Body.String())

	var updCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityVehicleBlacklist, id, models.BlacklistActionUpdated).
		Count(&updCount).Error)
	assert.Equal(t, int64(1), updCount, "update должен попасть в audit_log")

	// DELETE (archive) -> 'archived'.
	rec = testutil.DELETE(t, e, fmt.Sprintf("/vehicle-blacklist/%d", id), h)
	require.Equal(t, http.StatusOK, rec.Code, "archive: %s", rec.Body.String())

	var archCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityVehicleBlacklist, id, models.BlacklistActionArchived).
		Count(&archCount).Error)
	assert.Equal(t, int64(1), archCount, "archive должен попасть в audit_log")

	// POST restore -> 'restored'.
	rec = testutil.POST(t, e, fmt.Sprintf("/vehicle-blacklist/%d/restore", id), "", h)
	require.Equal(t, http.StatusOK, rec.Code, "restore: %s", rec.Body.String())

	var restCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityVehicleBlacklist, id, models.BlacklistActionRestored).
		Count(&restCount).Error)
	assert.Equal(t, int64(1), restCount, "restore должен попасть в audit_log")
}
