package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

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
