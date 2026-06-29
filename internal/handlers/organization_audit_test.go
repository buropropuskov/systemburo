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

// TestOrganizations_History_BackfillLegacyIntoAudit проверяет финал #870 (срез F.2):
// читатель истории переведён на audit_log-only, а до-cutover строки замороженной
// organization_histories поднимаются в audit_log разовым BackfillAuditFromLegacy.
// Так гарантируется "история та же" уже без union: новые действия и перенесённые
// старые видны вместе, старая таблица остаётся бэкапом и больше не читается.
func TestOrganizations_History_BackfillLegacyIntoAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Создание через API -> запись created уходит в audit_log (cutover записи).
	rec := testutil.POST(t, e, "/organizations", `{"name":"Ауди Орг"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	id := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Легаси-строка напрямую в замороженную таблицу с более ранним временем - как
	// строка, накопленная до cutover и ещё не перенесённая в audit_log.
	legacy := models.OrganizationHistory{
		OrganizationID: id,
		ActionType:     models.OrganizationActionRenamed,
		Details:        json.RawMessage(`{"name":"Старое Имя"}`),
		CreatedAt:      time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// До backfill читатель видит только audit_log -> legacy-строка ещё невидима.
	rec = testutil.GET(t, e, fmt.Sprintf("/organizations/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, testutil.ParseSlice(t, rec), 1, "до backfill видно только audit_log")

	// CleanDB-Seed уже выставил гард-флаг (backfill прогонялся на пустой таблице) -
	// снимаем, чтобы перенести только что вставленную legacy-строку.
	require.NoError(t, db.Where("key = ?", "audit_backfilled:"+models.AuditEntityOrganization).
		Delete(&models.SystemSetting{}).Error)
	require.NoError(t, database.BackfillAuditFromLegacy(db))

	// Легаси-строка физически скопирована в audit_log, а старая таблица цела (бэкап).
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityOrganization, id, models.OrganizationActionRenamed).
		Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount, "legacy renamed перенесён в audit_log")
	require.NoError(t, db.Table("organization_histories").Where("organization_id = ?", id).Count(&legacyCount).Error)
	assert.Equal(t, int64(1), legacyCount, "старая таблица не тронута backfill'ом - read-only бэкап")

	// История отдаёт обе записи из audit_log, новые сверху.
	rec = testutil.GET(t, e, fmt.Sprintf("/organizations/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2, "после backfill видны и created, и перенесённая legacy-строка")
	assert.Equal(t, "created", hist[0]["action_type"], "новее сверху - created из audit_log")
	assert.Equal(t, "renamed", hist[1]["action_type"], "legacy renamed час назад - ниже")
	// details перенесены verbatim (renamed хранит только {name:new}).
	renamed := hist[1]["details"].(map[string]interface{})
	assert.Equal(t, "Старое Имя", renamed["name"])

	// Идемпотентность: повторный backfill не дублирует (гард-флаг снова стоит).
	require.NoError(t, database.BackfillAuditFromLegacy(db))
	rec = testutil.GET(t, e, fmt.Sprintf("/organizations/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, testutil.ParseSlice(t, rec), 2, "повторный backfill не создаёт дублей")
}
