package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBlacklistOverrideRevoke_CarCascadeToAuditLog: отмена пропуска (DELETE blacklist-override)
// пишет в audit_log с entity_type=car, а не только в историю заявки (#870, срез 1.12c).
// TestBlacklistOverride_Delete ассертирует только application; этот тест закрывает пробел.
func TestBlacklistOverrideRevoke_CarCascadeToAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	ctx := context.Background()

	const orgName = "Test Organization"
	senderToken := testutil.RegisterAndLogin(t, e, "bcr_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "bcr_sender")
	mark := seedMark(t, db, "BCR_Mark")

	_, err := newVehicleBlacklistService(db).Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "E888EE799", MarkID: mark.ID, Reason: "розыск",
	}, senderID)
	require.NoError(t, err)

	rec := submitCarApp(t, e, db, senderToken, orgName, "rcr", "E888EE798", mark.ID)
	require.Equal(t, http.StatusOK, rec.Code, "submit: %s", rec.Body.String())
	carID := latestElementID(t, db, "cars", "car_number = ?", "E888EE798")
	flag, ok := blacklistFlagFor(t, db, models.BlacklistElementCar, carID)
	require.True(t, ok, "ожидался флаг похожести")
	appID := flag.ApplicationID

	testutil.RegisterUser(t, e, "bcr_appr", "pass123", 1, td.OrgID, td.CompanyID)
	apprID := getUserID(t, db, "bcr_appr")
	fwd := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, apprID)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), fwd, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())
	apprToken, _ := testutil.LoginUser(t, e, "bcr_appr", "pass123")

	overridePath := fmt.Sprintf("/applications/%d/blacklist-overrides", appID)
	rec = testutil.POST(t, e, overridePath, fmt.Sprintf(`{"flag_id":%d,"comment":"проверил лично"}`, flag.ID), testutil.AuthHeader(apprToken))
	require.Equal(t, http.StatusOK, rec.Code, "override: %s", rec.Body.String())

	var overrideCarCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = 'blacklist_override'", models.AuditEntityCar, carID).
		Count(&overrideCarCount).Error)
	assert.Equal(t, int64(1), overrideCarCount, "blacklist_override должен записаться в audit_log машины")

	deletePath := fmt.Sprintf("%s?flag_id=%d", overridePath, flag.ID)
	rec = testutil.DELETE(t, e, deletePath, testutil.AuthHeader(apprToken))
	require.Equal(t, http.StatusOK, rec.Code, "delete override: %s", rec.Body.String())

	var revokeCarCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = 'blacklist_override_revoke'", models.AuditEntityCar, carID).
		Count(&revokeCarCount).Error)
	assert.Equal(t, int64(1), revokeCarCount, "blacklist_override_revoke должен записаться в audit_log машины (cascade)")
}

// TestBlacklistOverride_EmployeeCascadeToAuditLog: подтверждение и отмена пропуска по
// похожему сотруднику пишут в audit_log с entity_type=employee (#870, срез 1.12c).
func TestBlacklistOverride_EmployeeCascadeToAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	ctx := context.Background()

	const orgName = "Test Organization"
	senderToken := testutil.RegisterAndLogin(t, e, "bemp_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "bemp_sender")

	citizenshipID := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)

	_, err := newPersonBlacklistService(db).Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Ivanov", FirstName: "Ivan", MiddleName: "Ivanovich", Reason: "нарушение",
	}, senderID)
	require.NoError(t, err)

	// Похожий (без отчества) - получает флаг, не 409.
	rec := submitPersonApp(t, e, db, senderToken, orgName, "emp", "Ivanov", "Ivan", "", citizenshipID, tableID)
	require.Equal(t, http.StatusOK, rec.Code, "submit: %s", rec.Body.String())

	empID := latestElementID(t, db, "employees", "last_name = ? AND first_name = ?", "Ivanov", "Ivan")
	flag, ok := blacklistFlagFor(t, db, models.BlacklistElementEmployee, empID)
	require.True(t, ok, "ожидался флаг похожести для сотрудника")
	appID := flag.ApplicationID

	testutil.RegisterUser(t, e, "bemp_appr", "pass123", 1, td.OrgID, td.CompanyID)
	apprID := getUserID(t, db, "bemp_appr")
	fwd := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, apprID)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), fwd, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())
	apprToken, _ := testutil.LoginUser(t, e, "bemp_appr", "pass123")

	overridePath := fmt.Sprintf("/applications/%d/blacklist-overrides", appID)
	rec = testutil.POST(t, e, overridePath, fmt.Sprintf(`{"flag_id":%d,"comment":"проверил лично"}`, flag.ID), testutil.AuthHeader(apprToken))
	require.Equal(t, http.StatusOK, rec.Code, "override: %s", rec.Body.String())

	var overrideEmpCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = 'blacklist_override'", models.AuditEntityEmployee, empID).
		Count(&overrideEmpCount).Error)
	assert.Equal(t, int64(1), overrideEmpCount, "blacklist_override должен записаться в audit_log сотрудника")

	deletePath := fmt.Sprintf("%s?flag_id=%d", overridePath, flag.ID)
	rec = testutil.DELETE(t, e, deletePath, testutil.AuthHeader(apprToken))
	require.Equal(t, http.StatusOK, rec.Code, "delete override: %s", rec.Body.String())

	var revokeEmpCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = 'blacklist_override_revoke'", models.AuditEntityEmployee, empID).
		Count(&revokeEmpCount).Error)
	assert.Equal(t, int64(1), revokeEmpCount, "blacklist_override_revoke должен записаться в audit_log сотрудника (cascade)")
}
