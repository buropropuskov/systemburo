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

// TestPersonBlacklist_History_BackfillLegacyIntoAudit проверяет финал #870 (срез F.4):
// читатель истории переведён на audit_log-only, а до-cutover строки замороженной
// person_blacklist_histories поднимаются в audit_log разовым BackfillAuditFromLegacy.
// details ЧС людей уже jsonb - переносится verbatim.
func TestPersonBlacklist_History_BackfillLegacyIntoAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Создание через API -> created уходит уже в audit_log (cutover записи).
	rec := testutil.POST(t, e, "/person-blacklist", `{"last_name":"Аудитов","first_name":"Аудит","reason":"audit-test"}`, h)
	require.Equal(t, http.StatusCreated, rec.Code)
	entryMap := testutil.ParseMap(t, rec)
	id := int(entryMap["id"].(float64))

	// Легаси-строка напрямую в замороженную таблицу с более ранним временем - как
	// строка, накопленная до cutover и ещё не перенесённая в audit_log.
	legacy := models.PersonBlacklistHistory{
		EntityID:   id,
		ActionType: models.BlacklistActionArchived,
		Details:    json.RawMessage(`{"full_name":"Аудитов Аудит"}`),
		CreatedAt:  time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// До backfill читатель видит только audit_log -> legacy-строка ещё невидима.
	rec = testutil.GET(t, e, fmt.Sprintf("/person-blacklist/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, testutil.ParseSlice(t, rec), 1, "до backfill видно только audit_log")

	// CleanDB-Seed уже выставил гард-флаг (backfill прогонялся на пустой таблице) -
	// снимаем, чтобы перенести только что вставленную legacy-строку.
	require.NoError(t, db.Where("key = ?", "audit_backfilled:"+models.AuditEntityPersonBlacklist).
		Delete(&models.SystemSetting{}).Error)
	require.NoError(t, database.BackfillAuditFromLegacy(db))

	// Легаси-строка физически скопирована в audit_log, старая таблица цела (бэкап).
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityPersonBlacklist, id, models.BlacklistActionArchived).
		Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount, "legacy archived перенесён в audit_log")
	require.NoError(t, db.Table("person_blacklist_histories").Where("entity_id = ?", id).Count(&legacyCount).Error)
	assert.Equal(t, int64(1), legacyCount, "старая таблица не тронута backfill'ом - read-only бэкап")

	// История per-id отдаёт обе записи из audit_log, новые сверху, details verbatim.
	rec = testutil.GET(t, e, fmt.Sprintf("/person-blacklist/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2, "после backfill видны и created, и перенесённая legacy-строка")
	assert.Equal(t, "created", hist[0]["action_type"], "новее сверху - created из audit_log")
	assert.Equal(t, "archived", hist[1]["action_type"], "legacy archived час назад - ниже")
	details := hist[1]["details"].(map[string]interface{})
	assert.Equal(t, "Аудитов Аудит", details["full_name"], "details перенесены verbatim")

	// Идемпотентность: повторный backfill не дублирует (гард-флаг снова стоит).
	require.NoError(t, database.BackfillAuditFromLegacy(db))
	rec = testutil.GET(t, e, fmt.Sprintf("/person-blacklist/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, testutil.ParseSlice(t, rec), 2, "повторный backfill не создаёт дублей")
}

// TestPersonBlacklist_AllHistory_BackfillLegacyIntoAudit проверяет глобальный журнал (#870, F.4):
// GET /person-blacklist/history читает audit_log-only, до-cutover строки (в т.ч. по уже
// удалённым записям) поднимаются backfill'ом.
func TestPersonBlacklist_AllHistory_BackfillLegacyIntoAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Создать запись через API (goes to audit_log).
	rec := testutil.POST(t, e, "/person-blacklist", `{"last_name":"Журналов","first_name":"Журнал","reason":"all-audit-test"}`, h)
	require.Equal(t, http.StatusCreated, rec.Code)
	id := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Легаси-строка на другую (уже удалённую) запись с entity_id без FK.
	legacyEntityID := 999998
	legacy := models.PersonBlacklistHistory{
		EntityID:   legacyEntityID,
		ActionType: models.BlacklistActionPurged,
		Details:    json.RawMessage(`{"full_name":"Удалённый Иван"}`),
		CreatedAt:  time.Now().Add(-2 * time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// До backfill глобальный журнал видит только created из audit_log.
	rec = testutil.GET(t, e, "/person-blacklist/history", h)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, hasBlacklistEntry(testutil.ParseSlice(t, rec), legacyEntityID, models.BlacklistActionPurged),
		"до backfill legacy-строка ещё невидима")

	// Снять гард-флаг и перенести legacy-строку.
	require.NoError(t, db.Where("key = ?", "audit_backfilled:"+models.AuditEntityPersonBlacklist).
		Delete(&models.SystemSetting{}).Error)
	require.NoError(t, database.BackfillAuditFromLegacy(db))

	// После backfill глобальный журнал отдаёт обе записи.
	rec = testutil.GET(t, e, "/person-blacklist/history", h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	assert.True(t, hasBlacklistEntry(hist, id, models.BlacklistActionCreated), "должна быть запись created из audit_log")
	assert.True(t, hasBlacklistEntry(hist, legacyEntityID, models.BlacklistActionPurged), "перенесённая legacy-строка purged видна")
}

// hasBlacklistEntry ищет в журнале ЧС строку по entity_id+action_type.
func hasBlacklistEntry(hist []map[string]interface{}, entityID int, action string) bool {
	for _, item := range hist {
		if int(item["entity_id"].(float64)) == entityID && item["action_type"].(string) == action {
			return true
		}
	}
	return false
}

// TestPersonBlacklist_WriteFlip_UpdateRestoreToAuditLog: update/archive/restore через API
// пишут соответствующие действия в audit_log (#870): 'updated', 'archived', 'restored'.
func TestPersonBlacklist_WriteFlip_UpdateRestoreToAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/person-blacklist", `{"last_name":"Правков","first_name":"Правк","reason":"update-test"}`, h)
	require.Equal(t, http.StatusCreated, rec.Code, "create: %s", rec.Body.String())
	id := int(testutil.ParseMap(t, rec)["id"].(float64))

	// PUT update - меняем только reason, identity не меняем.
	rec = testutil.PUT(t, e, fmt.Sprintf("/person-blacklist/%d", id), `{"last_name":"Правков","first_name":"Правк","reason":"новая причина"}`, h)
	require.Equal(t, http.StatusOK, rec.Code, "update: %s", rec.Body.String())

	var updCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityPersonBlacklist, id, models.BlacklistActionUpdated).
		Count(&updCount).Error)
	assert.Equal(t, int64(1), updCount, "update должен попасть в audit_log")

	// DELETE (archive) -> 'archived'.
	rec = testutil.DELETE(t, e, fmt.Sprintf("/person-blacklist/%d", id), h)
	require.Equal(t, http.StatusOK, rec.Code, "archive: %s", rec.Body.String())

	var archCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityPersonBlacklist, id, models.BlacklistActionArchived).
		Count(&archCount).Error)
	assert.Equal(t, int64(1), archCount, "archive должен попасть в audit_log")

	// POST restore -> 'restored'.
	rec = testutil.POST(t, e, fmt.Sprintf("/person-blacklist/%d/restore", id), "", h)
	require.Equal(t, http.StatusOK, rec.Code, "restore: %s", rec.Body.String())

	var restCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityPersonBlacklist, id, models.BlacklistActionRestored).
		Count(&restCount).Error)
	assert.Equal(t, int64(1), restCount, "restore должен попасть в audit_log")
}
