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

// TestUsers_History_UnionLegacyAndAudit проверяет переходную модель #870:
// новые действия пишутся в audit_log, старые строки из user_histories видны
// через union. Гарантирует "история та же" до финального backfill.
func TestUsers_History_UnionLegacyAndAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Создание пользователя через API -> запись created уходит уже в audit_log.
	body := fmt.Sprintf(`{"username":"audituser","password":"password123","type_id":1,"organization_id":%d}`, td.OrgID)
	rec := testutil.POST(t, e, "/users", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Находим ID созданного пользователя
	rec = testutil.GET(t, e, "/users/all", h)
	require.Equal(t, http.StatusOK, rec.Code)
	var userID int
	for _, u := range testutil.ParseSlice(t, rec) {
		if u["username"] == "audituser" {
			userID = int(u["id"].(float64))
			break
		}
	}
	require.Greater(t, userID, 0, "созданный пользователь должен иметь ID")

	// Подтверждаем, что новая запись физически в audit_log (а не в старой таблице).
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ?", models.AuditEntityUser, userID).
		Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount, "created должен попасть в audit_log")
	require.NoError(t, db.Table("user_histories").
		Where("target_user_id = ?", userID).
		Count(&legacyCount).Error)
	assert.Equal(t, int64(0), legacyCount, "старая таблица больше не пишется")

	// Легаси-строка напрямую в старую таблицу с более ранним временем (как до миграции).
	legacy := models.UserHistory{
		TargetUserID: userID,
		ActionType:   models.UserActionUpdated,
		Details:      json.RawMessage(`{"last_name":{"old":"","new":"Иванов"}}`),
		CreatedAt:    time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// История endpoint-а объединяет обе таблицы, новые сверху.
	rec = testutil.GET(t, e, "/users/audituser/history", h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2, "union должен отдать и audit_log, и legacy-строку")
	assert.Equal(t, "created", hist[0]["action_type"], "новее сверху - created из audit_log")
	assert.Equal(t, "updated", hist[1]["action_type"], "legacy updated час назад - ниже")
}
