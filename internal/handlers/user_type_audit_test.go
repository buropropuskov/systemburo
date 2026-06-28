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

// TestUserTypes_History_UnionLegacyAndAudit проверяет переходную модель #870:
// новые действия пишутся в audit_log, а строки из замороженной user_type_histories
// по-прежнему видны в истории через union. Гарантирует "история та же" до backfill.
func TestUserTypes_History_UnionLegacyAndAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Создание через API -> запись created уходит уже в audit_log (cutover записи).
	rec := testutil.POST(t, e, "/user-types-management", `{"name":"Аудит тип","code":"audit_type"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	id := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Подтверждаем, что новая запись физически в audit_log (а не в старой таблице).
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").Where("entity_type = ? AND entity_id = ?", models.AuditEntityUserType, id).Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount, "created должен попасть в audit_log")
	require.NoError(t, db.Table("user_type_histories").Where("user_type_id = ?", id).Count(&legacyCount).Error)
	assert.Equal(t, int64(0), legacyCount, "старая таблица больше не пишется")

	// Легаси-строка напрямую в старую таблицу с более ранним временем (как до миграции).
	legacy := models.UserTypeHistory{
		UserTypeID: id,
		ActionType: models.UserTypeActionRenamed,
		Details:    json.RawMessage(`{"name":"Старое Имя"}`),
		CreatedAt:  time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// История endpoint-а объединяет обе таблицы, новые сверху.
	rec = testutil.GET(t, e, fmt.Sprintf("/user-types-management/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2, "union должен отдать и audit_log, и legacy-строку")
	assert.Equal(t, "created", hist[0]["action_type"], "новее сверху - created из audit_log")
	assert.Equal(t, "renamed", hist[1]["action_type"], "legacy renamed час назад - ниже")
}
