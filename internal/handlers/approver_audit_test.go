package handlers_test

import (
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

// TestApprovers_History_BackfillLegacyIntoAudit проверяет финал #870 (срез F.3):
// читатель глобальной истории принимающих переведён на audit_log-only, а до-cutover
// строки замороженной application_approver_histories поднимаются в audit_log разовым
// BackfillAuditFromLegacy. approver_name из плоской колонки сворачивается в
// details->>'approver_name' в той же форме, что пишет recorder.
func TestApprovers_History_BackfillLegacyIntoAudit(t *testing.T) {
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

	// Легаси-строка напрямую в замороженную таблицу с более ранним временем - как
	// строка, накопленная до cutover и ещё не перенесённая в audit_log.
	legacy := models.ApplicationApproverHistory{
		ApproverUserID: targetUserID,
		ApproverName:   "Тестовый Пользователь",
		ActionType:     models.ApproverActionCreated,
		CreatedAt:      time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// До backfill читатель видит только audit_log -> legacy-строка ещё невидима.
	rec = testutil.GET(t, e, "/application-approvers/history", adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, testutil.ParseSlice(t, rec), 1, "до backfill видно только audit_log")

	// CleanDB-Seed уже выставил гард-флаг (backfill прогонялся на пустой таблице) -
	// снимаем, чтобы перенести только что вставленную legacy-строку.
	require.NoError(t, db.Where("key = ?", "audit_backfilled:"+models.AuditEntityApprover).
		Delete(&models.SystemSetting{}).Error)
	require.NoError(t, database.BackfillAuditFromLegacy(db))

	// Легаси-строка физически скопирована в audit_log, а старая таблица цела (бэкап).
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ?", models.AuditEntityApprover, targetUserID).
		Count(&auditCount).Error)
	assert.Equal(t, int64(2), auditCount, "created из API + перенесённая legacy-строка")
	require.NoError(t, db.Table("application_approver_histories").
		Where("approver_user_id = ?", targetUserID).
		Count(&legacyCount).Error)
	assert.Equal(t, int64(1), legacyCount, "старая таблица не тронута backfill'ом - read-only бэкап")

	// История отдаёт обе записи из audit_log, новые сверху, approver_name сохранён.
	rec = testutil.GET(t, e, "/application-approvers/history", adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2, "после backfill видны и API-created, и перенесённая legacy-строка")

	// Новее сверху - created из API (recorder снял имя в details->>'approver_name').
	assert.Equal(t, "created", hist[0]["action_type"])
	assert.Equal(t, float64(targetUserID), hist[0]["approver_user_id"])
	assert.NotEmpty(t, hist[0]["approver_name"])

	// Перенесённая legacy-строка ниже - approver_name из плоской колонки в details.
	assert.Equal(t, "created", hist[1]["action_type"])
	assert.Equal(t, float64(targetUserID), hist[1]["approver_user_id"])
	assert.Equal(t, "Тестовый Пользователь", hist[1]["approver_name"])

	// Идемпотентность: повторный backfill не дублирует (гард-флаг снова стоит).
	require.NoError(t, database.BackfillAuditFromLegacy(db))
	rec = testutil.GET(t, e, "/application-approvers/history", adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, testutil.ParseSlice(t, rec), 2, "повторный backfill не создаёт дублей")
}
