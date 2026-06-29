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

// TestUsers_WriteFlip_ChangesToAuditLog проверяет, что 5 действий над пользователем
// (updated, password_reset, org_changed, company_changed, restored) пишутся в audit_log.
// Каждый action проверяется направленным Count по (entity_type, entity_id, action).
func TestUsers_WriteFlip_ChangesToAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Целевой пользователь — его будем менять через эндпоинты.
	testutil.RegisterUser(t, e, "flipuser", "flippass123", 1, td.OrgID, td.CompanyID)
	targetUserID := getUserID(t, db, "flipuser")

	// --- updated: изменение ФИО (значения отличаются от пустых дефолтов, diff непуст) ---
	rec := testutil.PUT(t, e, "/users/flipuser/info",
		`{"last_name":"Новиков","first_name":"Иван","middle_name":"Петрович"}`, h)
	require.Equal(t, http.StatusOK, rec.Code, "updated info: %s", rec.Body.String())

	var n int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?",
			models.AuditEntityUser, targetUserID, models.UserActionUpdated).
		Count(&n).Error)
	assert.GreaterOrEqual(t, n, int64(1), "audit_log должен содержать updated")

	// --- password_reset ---
	rec = testutil.PUT(t, e, "/users/flipuser/password", `{"password":"newflippass999"}`, h)
	require.Equal(t, http.StatusOK, rec.Code, "password_reset: %s", rec.Body.String())

	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?",
			models.AuditEntityUser, targetUserID, models.UserActionPasswordReset).
		Count(&n).Error)
	assert.GreaterOrEqual(t, n, int64(1), "audit_log должен содержать password_reset")

	// --- org_changed: вторая организация (отличная от td.OrgID, иначе no-op) ---
	org2 := models.Organization{Name: "FlipOrg2", IsActive: true}
	require.NoError(t, db.Create(&org2).Error)
	rec = testutil.PUT(t, e, "/users/flipuser/organization",
		fmt.Sprintf(`{"organization_id":%d}`, org2.ID), h)
	require.Equal(t, http.StatusOK, rec.Code, "org_changed: %s", rec.Body.String())

	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?",
			models.AuditEntityUser, targetUserID, models.UserActionOrgChanged).
		Count(&n).Error)
	assert.GreaterOrEqual(t, n, int64(1), "audit_log должен содержать org_changed")

	// --- company_changed: вторая компания (отличная от td.CompanyID, иначе no-op) ---
	comp2 := models.Company{Name: "FlipComp2", IsActive: true}
	require.NoError(t, db.Create(&comp2).Error)
	rec = testutil.PUT(t, e, "/users/flipuser/company",
		fmt.Sprintf(`{"company_id":%d}`, comp2.ID), h)
	require.Equal(t, http.StatusOK, rec.Code, "company_changed: %s", rec.Body.String())

	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?",
			models.AuditEntityUser, targetUserID, models.UserActionCompanyChanged).
		Count(&n).Error)
	assert.GreaterOrEqual(t, n, int64(1), "audit_log должен содержать company_changed")

	// --- restored: сначала архивировать, потом восстановить ---
	rec = testutil.DELETE(t, e, "/users/flipuser", h)
	require.Equal(t, http.StatusOK, rec.Code, "archive before restore: %s", rec.Body.String())

	rec = testutil.POST(t, e, "/users/flipuser/restore", "", h)
	require.Equal(t, http.StatusOK, rec.Code, "restored: %s", rec.Body.String())

	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?",
			models.AuditEntityUser, targetUserID, models.UserActionRestored).
		Count(&n).Error)
	assert.GreaterOrEqual(t, n, int64(1), "audit_log должен содержать restored")
}

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
