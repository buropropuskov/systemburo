package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRelatedSQLIntegrity_EighteenRequiredStatements(t *testing.T) {
	e, originalDB, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, originalDB)
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	db := originalDB.Session(&gorm.Session{NowFunc: func() time.Time { return now }})
	td := testutil.SeedTestData(t, db)
	const sender, voter = "related_sender", "related_voter"
	ids := map[string]int{}
	for _, username := range []string{sender, voter, "related_resp1", "related_resp2", "related_view1", "related_view2"} {
		testutil.RegisterUser(t, e, username, "pass123", 1, td.OrgID, td.CompanyID)
		ids[username] = getUserID(t, db, username)
		if username == voter || strings.HasPrefix(username, "related_resp") {
			require.NoError(t, db.Create(&models.UserPermissionOverride{UserID: ids[username],
				PermissionKey: services.KeyActionApproveApplication, Value: "allow", GrantedAt: now}).Error)
		}
	}
	require.NoError(t, db.Create(&models.UserPermissionOverride{UserID: ids[sender],
		PermissionKey: services.KeyActionForwardApplication, Value: "allow", GrantedAt: now}).Error)
	resp1, resp2, view1, view2 := ids["related_resp1"], ids["related_resp2"], ids["related_view1"], ids["related_view2"]
	require.NoError(t, db.Exec(`INSERT INTO organization_users (organization_id,user_id,is_primary,required_approval)
		VALUES (?, ?, true, true), (?, ?, false, true)`, td.OrgID, resp1, td.OrgID, resp2).Error)
	require.NoError(t, db.Exec(`INSERT INTO companies_users (company_id,user_id,is_primary,required_approval)
		VALUES (?, ?, false, false)`, td.CompanyID, resp1).Error)
	for _, tc := range []struct {
		operation, point, sql string
		row                   bool
		at                    int
	}{
		{"create", "organization_read", "from organization_users where organization_id =", true, 1},
		{"create", "company_read", "from companies_users where company_id =", true, 1},
		{"create", "primary_assignment", "update applications set responsible_user_id =", false, 1},
		{"create", "second_responsible", "insert into application_responsible_users", false, 2},
		{"forward", "second_user_exists", "select exists(select 1 from users where id =", true, 2},
		{"forward", "second_responsible_exists", "select exists(select 1 from application_responsible_users where application_id =", true, 2},
		{"forward", "second_responsible_insert", "insert into application_responsible_users", false, 2},
		{"forward", "second_viewer_exists", "select exists(select 1 from application_viewers where application_id =", true, 2},
		{"forward", "second_viewer_insert", "insert into application_viewers", false, 2},
		{"forward", "attachment_scope_read", "select id from attachments where application_id =", true, 1},
		{"forward", "second_attachment_binding", "insert into forward_attachments", false, 2},
		{"forward", "new_confirmation", "select confirmation from applications where id =", true, 1},
		{"approve", "old_confirmation", "select confirmation from applications where id =", true, 1},
		{"approve", "vote_update", "update application_responsible_users set approval_status =", false, 1},
		{"approve", "new_confirmation", "select confirmation from applications where id =", true, 2},
		{"revoke", "old_confirmation", "select confirmation from applications where id =", true, 1},
		{"revoke", "vote_update", "update application_responsible_users set approval_status =", false, 1},
		{"revoke", "new_confirmation", "select confirmation from applications where id =", true, 2},
	} {
		t.Run(tc.operation+"_"+tc.point, func(t *testing.T) {
			confirmation, voteStatus := models.ConfirmationPending, "pending"
			if tc.operation == "revoke" || tc.operation == "forward" {
				confirmation, voteStatus = models.ConfirmationApproved, "approved"
			}
			appID := suppApp(t, db, td.OrgID, ids[sender], "RELATED-"+tc.operation+"-"+tc.point, confirmation, models.StatusProcessing)
			att1 := suppAttachment(t, db, appID, "people", "2099-12-31")
			att2 := suppAttachment(t, db, appID, "cars", "2099-12-31")
			suppNewEmployee(t, db, att1, "Related original", nil)
			responsible := models.ApplicationResponsibleUser{ApplicationID: appID, UserID: ids[voter], RequiredApproval: true,
				ApprovalStatus: &voteStatus, ApprovalComment: testutil.Ptr("Existing vote"), ApprovalDatetime: &now, CreatedAt: now}
			require.NoError(t, db.Create(&responsible).Error)
			require.NoError(t, db.Create(&models.ApplicationStatusView{ApplicationID: appID, UserID: ids[voter], SeenAt: now}).Error)
			before := remainingTransactionSnapshot(t, db)
			var applicationsBefore int64
			require.NoError(t, db.Model(&models.Application{}).Count(&applicationsBefore).Error)
			var existingApplicationIDs []int
			require.NoError(t, db.Model(&models.Application{}).Pluck("id", &existingApplicationIDs).Error)
			injected := errors.New("synthetic required pre-SQL error")
			hits := 0
			name := "test:related_sql:" + strings.ReplaceAll(t.Name(), "/", ":")
			fault := func(tx *gorm.DB) {
				sql := strings.ToLower(strings.Join(strings.Fields(tx.Statement.SQL.String()), " "))
				if !strings.Contains(sql, tc.sql) {
					return
				}
				hits++
				if hits != tc.at {
					return
				}
				// NewDB retains the transaction connection while discarding the
				// current Row/Raw statement. Model reads do not recurse into Row.
				if tc.operation == "create" {
					var count int64
					require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Model(&models.Application{}).Count(&count).Error)
					require.Equal(t, applicationsBefore+1, count)
				}
				if tc.at == 2 && strings.HasPrefix(tc.point, "second_") {
					table, column, expectedUser := "application_responsible_users", "user_id", resp1
					if strings.Contains(tc.point, "viewer") {
						table, expectedUser = "application_viewers", view1
					}
					if strings.Contains(tc.point, "attachment") {
						table, column, expectedUser = "forward_attachments", "recipient_user_id", resp1
					}
					var count int64
					query := tx.Session(&gorm.Session{NewDB: true}).Table(table)
					if tc.operation == "create" {
						query = query.Where("application_id NOT IN ?", existingApplicationIDs)
					} else {
						query = query.Where("application_id = ? AND "+column+" = ?", appID, expectedUser)
					}
					// Count only assignments outside the pre-operation application IDs.
					require.NoError(t, query.Count(&count).Error)
					require.EqualValues(t, 1, count, "the first loop write must exist before the second statement fails")
				}
				if tc.point == "new_confirmation" && tc.operation != "forward" {
					var vote models.ApplicationResponsibleUser
					require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).First(&vote, responsible.ID).Error)
					wantVote, wantConfirmation := "approved", models.ConfirmationApproved
					if tc.operation == "revoke" {
						wantVote, wantConfirmation = "pending", models.ConfirmationPending
					}
					require.Equal(t, wantVote, *vote.ApprovalStatus)
					var current models.Application
					require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).First(&current, appID).Error)
					require.Equal(t, wantConfirmation, *current.Confirmation, "failure follows the real vote and quorum writes")
				}
				tx.AddError(injected) // no PostgreSQL error/aborted state
			}
			if tc.row {
				require.NoError(t, db.Callback().Row().Before("gorm:row").Register(name, fault))
				t.Cleanup(func() { require.NoError(t, db.Callback().Row().Remove(name)) })
			} else {
				require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register(name, fault))
				t.Cleanup(func() { require.NoError(t, db.Callback().Raw().Remove(name)) })
			}
			pub := &fakePublisher{}
			spy := &remainingPermissionSpy{PermissionService: services.NewPermissionService(db)}
			recorder := services.NewAuditRecorder(db)
			svc := services.NewApplicationService(db, spy, services.NewNotificationService(db),
				services.NewVehicleBlacklistService(db, recorder), services.NewPersonBlacklistService(db, recorder), recorder,
				services.WithRealtimePublisher(pub))
			var err error
			switch tc.operation {
			case "create":
				var response *services.ApplicationCreateResponse
				response, err = svc.CreateApplication(context.Background(), sender,
					services.ApplicationCreateRequest{OrganizationID: &td.OrgID, CompanyID: &td.CompanyID, DataApproval: true}, false)
				require.Nil(t, response)
			case "forward":
				err = svc.ForwardApplication(context.Background(), sender, appID, false, services.ForwardApplicationRequest{
					Users: []services.ForwardUser{{UserID: resp1, RequiredApproval: true}, {UserID: resp2, RequiredApproval: true},
						{UserID: view1, CanView: true}, {UserID: view2, CanView: true}}, AttachmentIDs: []int{att1, att2}})
			case "approve":
				err = svc.ApproveApplicationByUser(context.Background(), voter, appID, services.UserApprovalRequest{Status: "approved"})
			case "revoke":
				var response *services.RevokeApprovalResponse
				response, err = svc.RevokeApproval(context.Background(), voter, appID, services.RevokeApprovalRequest{})
				require.Nil(t, response)
			}
			require.Error(t, err)
			var appError *apperr.Error
			require.ErrorAs(t, err, &appError)
			require.Equal(t, http.StatusInternalServerError, appError.Code)
			require.ErrorIs(t, err, injected)
			require.Equal(t, tc.at, hits, "precise site and occurrence must be reached")
			require.Equal(t, before, remainingTransactionSnapshot(t, db))
			require.Empty(t, pub.events)
			require.Zero(t, spy.grants, "permission grant belongs after a successful Forward commit")
			if tc.point == "attachment_scope_read" {
				require.False(t, svc.CanAccessApplication(context.Background(), appID, "related_resp1", false), "failed scope lookup must not grant unrestricted access")
				require.False(t, svc.CanAccessApplication(context.Background(), appID, "related_view1", false))
			}
		})
	}
}
