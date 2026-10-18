package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestS5Action_ForwardAndAssignment(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	sender := testutil.RegisterAndLogin(t, e, "s5_sender", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "s5_target", "pass123", 1, td.OrgID, td.CompanyID)
	senderID, targetID := getUserID(t, db, "s5_sender"), getUserID(t, db, "s5_target")
	appID := createSimpleApplication(t, e, sender, td.OrgID)
	url := fmt.Sprintf("/applications/%d/forward", appID)
	deny := models.UserPermissionOverride{UserID: senderID, PermissionKey: services.KeyActionForwardApplication, Value: "deny"}
	require.NoError(t, db.Create(&deny).Error)
	rec := testutil.POST(t, e, url, forwardBody(targetID, true, false), testutil.AuthHeader(sender))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	var count int64
	require.NoError(t, db.Table("application_responsible_users").Where("application_id = ? AND user_id = ?", appID, targetID).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Where("user_id = ? AND permission_key = ?", senderID, deny.PermissionKey).Delete(&models.UserPermissionOverride{}).Error)
	deny = models.UserPermissionOverride{UserID: targetID, PermissionKey: services.KeyActionApproveApplication, Value: "deny"}
	require.NoError(t, db.Create(&deny).Error)
	for _, required := range []bool{false, true} {
		rec = testutil.POST(t, e, url, forwardBody(targetID, required, false), testutil.AuthHeader(sender))
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		require.NotContains(t, rec.Body.String(), "настройк")
	}
	require.NoError(t, db.Table("application_responsible_users").Where("application_id = ? AND user_id = ?", appID, targetID).Count(&count).Error)
	require.Zero(t, count)
	rec = testutil.POST(t, e, url, forwardBody(targetID, false, true), testutil.AuthHeader(sender))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, db.Table("application_viewers").Where("application_id = ? AND user_id = ?", appID, targetID).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestS5Action_VoteUsesCurrentPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	sender := testutil.RegisterAndLogin(t, e, "s5_vote_sender", "pass123", 1, td.OrgID, td.CompanyID)
	voter := testutil.RegisterAndLogin(t, e, "s5_voter", "pass123", 1, td.OrgID, td.CompanyID)
	id := getUserID(t, db, "s5_voter")
	appID := createSimpleApplication(t, e, sender, td.OrgID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), forwardBody(id, true, false), testutil.AuthHeader(sender))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, db.Create(&models.UserPermissionOverride{UserID: id, PermissionKey: services.KeyActionApproveApplication, Value: "deny"}).Error)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), `{"status":"approved"}`, testutil.AuthHeader(voter))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	var status string
	require.NoError(t, db.Table("application_responsible_users").Select("approval_status").Where("application_id = ? AND user_id = ?", appID, id).Scan(&status).Error)
	require.Equal(t, "pending", status)
	require.NoError(t, db.Where("user_id = ? AND permission_key = ?", id, services.KeyActionApproveApplication).Delete(&models.UserPermissionOverride{}).Error)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), `{"status":"approved"}`, testutil.AuthHeader(voter))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, db.Create(&models.UserPermissionOverride{UserID: id, PermissionKey: services.KeyActionApproveApplication, Value: "deny"}).Error)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/revoke-approval", appID), `{}`, testutil.AuthHeader(voter))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	require.NoError(t, db.Table("application_responsible_users").Select("approval_status").Where("application_id = ? AND user_id = ?", appID, id).Scan(&status).Error)
	require.Equal(t, "approved", status, "permission change must preserve the recorded vote")
}

func TestS5Action_AutomaticAssignments(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	sender := testutil.RegisterAndLogin(t, e, "s5_auto_sender", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "s5_auto_target", "pass123", 1, td.OrgID, td.CompanyID)
	id := getUserID(t, db, "s5_auto_target")
	require.NoError(t, db.Create(&models.UserPermissionOverride{UserID: id, PermissionKey: services.KeyActionApproveApplication, Value: "deny"}).Error)
	membership := models.OrganizationUser{OrganizationID: td.OrgID, UserID: id, IsPrimary: true, RequiredApproval: false}
	require.NoError(t, db.Create(&membership).Error)
	appID := createSimpleApplication(t, e, sender, td.OrgID)
	var count int64
	require.NoError(t, db.Table("application_responsible_users").Where("application_id = ? AND user_id = ?", appID, id).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Table("application_viewers").Where("application_id = ? AND user_id = ?", appID, id).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Model(&membership).Update("required_approval", true).Error)
	var before, after int64
	require.NoError(t, db.Table("applications").Count(&before).Error)
	body := fmt.Sprintf(`{"organization_id":%d,"data_approval":true,"message":"synthetic"}`, td.OrgID)
	rec := testutil.POST(t, e, "/applications", body, testutil.AuthHeader(sender))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "настройк")
	require.NoError(t, db.Table("applications").Count(&after).Error)
	require.Equal(t, before, after, "failed assignment must roll back application")
}

func TestS5Action_ForwardBatchRollsBack(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	sender := testutil.RegisterAndLogin(t, e, "s5_batch_sender", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "s5_batch_one", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "s5_batch_two", "pass123", 1, td.OrgID, td.CompanyID)
	one, two := getUserID(t, db, "s5_batch_one"), getUserID(t, db, "s5_batch_two")
	appID := createSimpleApplication(t, e, sender, td.OrgID)
	require.NoError(t, db.Create(&models.UserPermissionOverride{UserID: two, PermissionKey: services.KeyActionApproveApplication, Value: "deny"}).Error)
	body := fmt.Sprintf(`{"users":[{"user_id":%d,"can_view":false,"required_approval":true},{"user_id":%d,"can_view":false,"required_approval":true}]}`, one, two)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(sender))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var count int64
	require.NoError(t, db.Table("application_responsible_users").Where("application_id = ? AND user_id IN ?", appID, []int{one, two}).Count(&count).Error)
	require.Zero(t, count)
}

func TestS5Action_DuplicateMandatoryAssignment(t *testing.T) {
	for _, path := range []string{"/applications", "/applications/submit-complete-application"} {
		t.Run(path, func(t *testing.T) {
			e, db, cleanup := testutil.SetupTestApp(t)
			defer cleanup()
			testutil.CleanDB(t, db)
			td := testutil.SeedTestData(t, db)
			sender := testutil.RegisterAndLogin(t, e, "s5_duplicate_sender", "pass123", 1, td.OrgID, td.CompanyID)
			testutil.RegisterUser(t, e, "s5_duplicate_target", "pass123", 1, td.OrgID, td.CompanyID)
			id := getUserID(t, db, "s5_duplicate_target")
			require.NoError(t, db.Create(&models.OrganizationUser{OrganizationID: td.OrgID, UserID: id, RequiredApproval: false}).Error)
			require.NoError(t, db.Create(&models.CompaniesUser{CompanyID: td.CompanyID, UserID: id, RequiredApproval: true}).Error)
			now := time.Now().UTC()
			uaID := seedUniqueAttachment(t, db, "cars", "s5_duplicate", "Synthetic")
			body := map[string]any{"organization_id": td.OrgID, "organization": "Test Organization", "company_id": td.CompanyID, "data_approval": true, "message": "synthetic", "responsible_person": "Synthetic", "contact_phone": "+79000000000", "attachments": []any{map[string]any{"attachment_type": "cars", "attachment_name": "cars_template", "attachment_display_name": "Synthetic", "unique_attachment_id": uaID, "entry_date_from": now.AddDate(0, 0, -7).Format("2006-01-02"), "entry_date_to": now.AddDate(0, 0, 7).Format("2006-01-02"), "entry_time_from": "00:00", "entry_time_to": "23:59", "data": map[string]any{"vehicles": []any{map[string]any{"car_number": "A123AA777", "car_brand": "Synthetic"}}}}}}
			data, err := json.Marshal(body)
			require.NoError(t, err)
			rec := testutil.POST(t, e, path, string(data), testutil.AuthHeader(sender))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var assignments int64
			require.NoError(t, db.Table("application_responsible_users").Where("user_id = ? AND required_approval = true", id).Count(&assignments).Error)
			require.EqualValues(t, 1, assignments)
			require.NoError(t, db.Create(&models.UserPermissionOverride{UserID: id, PermissionKey: services.KeyActionApproveApplication, Value: "deny"}).Error)
			var before, after int64
			require.NoError(t, db.Table("applications").Count(&before).Error)
			rec = testutil.POST(t, e, path, string(data), testutil.AuthHeader(sender))
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), "не может согласовать")
			require.NoError(t, db.Table("applications").Count(&after).Error)
			require.Equal(t, before, after)
		})
	}
}

func TestS5Action_DisplayCacheAndFreshGuard(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	sender := testutil.RegisterAndLogin(t, e, "s5_cache_sender", "pass123", 1, td.OrgID, td.CompanyID)
	voter := testutil.RegisterAndLogin(t, e, "s5_cache_voter", "pass123", 1, td.OrgID, td.CompanyID)
	id := getUserID(t, db, "s5_cache_voter")
	appID := createSimpleApplication(t, e, sender, td.OrgID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), forwardBody(id, true, false), testutil.AuthHeader(sender))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	counter := &sessionQueryCounter{Interface: logger.Discard}
	readDB := db.Session(&gorm.Session{Logger: counter})
	resolver := services.NewPermissionResolver(readDB)
	svc := services.NewApplicationService(readDB, nil, nil, nil, nil, nil, services.WithApplicationPermissionResolver(resolver))
	rows, err := svc.GetApplicationResponsibleUsers(context.Background(), appID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].CanApprove)
	require.True(t, *rows[0].CanApprove)
	cold := counter.count.Swap(0)
	_, err = svc.GetApplicationResponsibleUsers(context.Background(), appID)
	require.NoError(t, err)
	warm := counter.count.Swap(0)
	require.Less(t, warm, cold)
	t.Logf("availability SQL cold=%d warm=%d", cold, warm)
	require.NoError(t, db.Create(&models.UserPermissionOverride{UserID: id, PermissionKey: services.KeyActionApproveApplication, Value: "deny"}).Error)
	// Do not invalidate the display cache: it must never authorize a mutation.
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), `{"status":"approved"}`, testutil.AuthHeader(voter))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	resolver.Invalidate(id)
	rows, err = svc.GetApplicationResponsibleUsers(context.Background(), appID)
	require.NoError(t, err)
	require.False(t, *rows[0].CanApprove)
}

func TestS5Action_PermissionSourceFailure(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	sender := testutil.RegisterAndLogin(t, e, "s5_error_sender", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, sender, td.OrgID)
	name := "s5_synthetic_permission_error"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "user_permission_overrides" {
			tx.AddError(fmt.Errorf("synthetic permission source unavailable"))
		}
	}))
	defer db.Callback().Query().Remove(name)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), `{"users":[]}`, testutil.AuthHeader(sender))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "synthetic")
}
