package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApprovers_History_UnionLegacyAndAudit проверяет переходную модель #870:
// новые действия пишутся в audit_log, а строки из замороженной
// application_approver_histories по-прежнему видны через union.
func TestApprovers_History_UnionLegacyAndAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "auditapprover", "password123", 1, td.OrgID, td.CompanyID)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminH := testutil.AuthHeader(adminToken)

	// Получаем ID целевого пользователя.
	rec := testutil.GET(t, e, "/application-approvers/available-users", adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	available := testutil.ParseSlice(t, rec)
	var targetUserID int
	for _, u := range available {
		if u["username"] == "auditapprover" {
			targetUserID = int(u["id"].(float64))
			break
		}
	}
	require.Greater(t, targetUserID, 0)

	// Добавляем через API -> запись идёт уже в audit_log.
	rec = testutil.POST(t, e, "/application-approvers", fmt.Sprintf(`{"user_id":%d}`, targetUserID), adminH)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Новая запись физически в audit_log.
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ?", models.AuditEntityApprover, targetUserID).
		Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount, "created должен попасть в audit_log")
	require.NoError(t, db.Table("application_approver_histories").
		Where("approver_user_id = ?", targetUserID).
		Count(&legacyCount).Error)
	assert.Equal(t, int64(0), legacyCount, "старая таблица больше не пишется")

	// Легаси-строка напрямую в старую таблицу с более ранним временем (как до миграции).
	legacy := models.ApplicationApproverHistory{
		ApproverUserID: targetUserID,
		ApproverName:   "Тестовый Пользователь",
		ActionType:     models.ApproverActionCreated,
		CreatedAt:      time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// История объединяет обе таблицы, новые сверху.
	rec = testutil.GET(t, e, "/application-approvers/history", adminH)
	require.Equal(t, http.StatusOK, rec.Code)

	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2, "union должен отдать и audit_log, и legacy-строку")

	// Новее сверху - created из audit_log.
	assert.Equal(t, "created", hist[0]["action_type"])
	assert.Equal(t, float64(targetUserID), hist[0]["approver_user_id"])
	assert.NotEmpty(t, hist[0]["approver_name"])

	// Легаси-строка ниже.
	assert.Equal(t, "created", hist[1]["action_type"])
	assert.Equal(t, float64(targetUserID), hist[1]["approver_user_id"])
	assert.Equal(t, "Тестовый Пользователь", hist[1]["approver_name"])
}
