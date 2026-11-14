package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	echomw "github.com/labstack/echo/v5/middleware"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type remainingPermissionSpy struct {
	services.PermissionService
	grants int
}

func (s *remainingPermissionSpy) GrantPermission(ctx context.Context, userID int, key, value string) error {
	s.grants++
	return s.PermissionService.GrantPermission(ctx, userID, key, value)
}

// Compare complete stored rows, including old comments/timestamps and both
// permission stores. Counts alone cannot detect an overwritten vote or marker.
func remainingTransactionSnapshot(t *testing.T, db *gorm.DB) map[string]string {
	t.Helper()
	state := map[string]string{}
	for _, table := range []string{"applications", "attachments", "cars", "employees", "items",
		"application_responsible_users", "application_viewers", "forward_attachments",
		"application_status_views", "application_supplements", "application_supplement_approvals",
		"audit_log", "notifications", "user_permissions", "user_permission_overrides"} {
		var rows string
		// Table identifiers are the fixed allow-list above; no request input is used.
		require.NoError(t, db.Raw(`SELECT COALESCE(jsonb_agg(record ORDER BY record::text), '[]'::jsonb)::text
			FROM (SELECT to_jsonb(t) AS record FROM `+table+` t) snapshot`).Scan(&rows).Error, table)
		state[table] = rows
	}
	return state
}

func TestRemainingTransactionIntegrity_EightCallbacksAndHTTPRecovery(t *testing.T) {
	e, originalDB, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, originalDB)
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	db := originalDB.Session(&gorm.Session{NowFunc: func() time.Time { return now }})
	td := testutil.SeedTestData(t, db)
	e.Use(echomw.Recover())
	const sender, voter, recipient, viewer = "remaining_sender", "remaining_voter", "remaining_recipient", "remaining_viewer"
	tokens := map[string]string{}
	ids := map[string]int{}
	for _, username := range []string{sender, voter, recipient, viewer} {
		tokens[username] = testutil.RegisterAndLogin(t, e, username, "pass123", 1, td.OrgID, td.CompanyID)
		ids[username] = getUserID(t, db, username)
	}
	makeApprover(t, db, voter)
	for _, username := range []string{voter, recipient} {
		require.NoError(t, db.Create(&models.UserPermissionOverride{UserID: ids[username],
			PermissionKey: services.KeyActionApproveApplication, Value: "allow", GrantedAt: now}).Error)
	}
	require.NoError(t, db.Create(&models.UserPermissionOverride{UserID: ids[sender],
		PermissionKey: services.KeyActionForwardApplication, Value: "allow", GrantedAt: now}).Error)
	for _, tc := range []struct{ operation, mode string }{
		{"create", "service_panic"}, {"forward", "service_panic"}, {"approve", "service_panic"}, {"revoke", "service_panic"},
		{"supplement_approve", "service_panic"}, {"supplement_revoke", "service_panic"}, {"decide", "service_panic"}, {"cancel", "service_panic"},
		{"create", "HTTP_panic"}, {"forward", "HTTP_panic"}, {"supplement_approve", "HTTP_panic"}, {"decide", "HTTP_panic"},
		{"decide", "late_seen_error"},
	} {
		t.Run(tc.operation+"_"+tc.mode, func(t *testing.T) {
			status, confirmation := models.StatusProcessing, models.ConfirmationPending
			if strings.HasPrefix(tc.operation, "supplement_") || tc.operation == "decide" || tc.operation == "cancel" {
				status, confirmation = models.StatusInWork, models.ConfirmationApproved
			}
			if tc.operation == "revoke" {
				confirmation = models.ConfirmationApproved
			}
			appID := suppApp(t, db, td.OrgID, ids[sender], "REMAINING-"+tc.operation+"-"+tc.mode, confirmation, status)
			attID := suppAttachment(t, db, appID, "people", "2099-12-31")
			originalID := suppNewEmployee(t, db, attID, "Original admitted", nil)
			require.NoError(t, db.Model(&models.Employee{}).Where("id = ?", originalID).Update("status", 1).Error)
			voteStatus := "pending"
			if tc.operation == "revoke" {
				voteStatus = "approved"
			}
			responsible := models.ApplicationResponsibleUser{ApplicationID: appID, UserID: ids[voter], RequiredApproval: true,
				ApprovalStatus: &voteStatus, ApprovalComment: testutil.Ptr("Original vote comment"), ApprovalDatetime: &now, CreatedAt: now}
			require.NoError(t, db.Create(&responsible).Error)
			roundStatus := models.SupplementPending
			if tc.operation == "decide" || tc.operation == "supplement_revoke" {
				roundStatus = models.SupplementApproved
			}
			round := models.ApplicationSupplement{ApplicationID: appID, Number: 1, Status: roundStatus, CreatedByUserID: ids[sender], CreatedAt: now}
			require.NoError(t, db.Create(&round).Error)
			suppVoteStatus := "pending"
			if tc.operation == "supplement_revoke" || tc.operation == "decide" {
				suppVoteStatus = "approved"
			}
			require.NoError(t, db.Create(&models.ApplicationSupplementApproval{SupplementID: round.ID, UserID: ids[voter],
				RequiredApproval: true, ApprovalStatus: &suppVoteStatus, ApprovalComment: testutil.Ptr("Original supplement vote"), ApprovalDatetime: &now, CreatedAt: now}).Error)
			pendingID := suppNewEmployee(t, db, attID, "Pending addition", &round.ID)
			require.NoError(t, db.Create(&models.ApplicationStatusView{ApplicationID: appID, UserID: ids[voter], SeenAt: now}).Error)
			before := remainingTransactionSnapshot(t, db)
			var applicationCount int64
			require.NoError(t, db.Model(&models.Application{}).Count(&applicationCount).Error)
			checkPrimary := func(tx *gorm.DB) {
				readTx := tx.Session(&gorm.Session{NewDB: true})
				switch tc.operation {
				case "create":
					var count int64
					require.NoError(t, readTx.Model(&models.Application{}).Count(&count).Error)
					require.Equal(t, applicationCount+1, count, "panic must follow INSERT applications")
				case "forward":
					var count int64
					require.NoError(t, readTx.Table("application_responsible_users").Where("application_id = ? AND user_id = ?", appID, ids[recipient]).Count(&count).Error)
					require.EqualValues(t, 1, count)
					var links int64
					require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Table("forward_attachments").Where("application_id = ?", appID).Count(&links).Error)
					require.EqualValues(t, 2, links, "both recipients must already have their attachment binding")
				case "approve", "revoke":
					var vote models.ApplicationResponsibleUser
					require.NoError(t, readTx.First(&vote, responsible.ID).Error)
					want := "approved"
					if tc.operation == "revoke" {
						want = "pending"
					}
					require.Equal(t, want, *vote.ApprovalStatus)
				case "supplement_approve", "supplement_revoke":
					var vote models.ApplicationSupplementApproval
					require.NoError(t, readTx.Where("supplement_id = ? AND user_id = ?", round.ID, ids[voter]).First(&vote).Error)
					want := "approved"
					if tc.operation == "supplement_revoke" {
						want = "pending"
					}
					require.Equal(t, want, *vote.ApprovalStatus)
				case "decide", "cancel":
					var current models.ApplicationSupplement
					require.NoError(t, readTx.First(&current, round.ID).Error)
					want := models.SupplementAccepted
					if tc.operation == "cancel" {
						want = models.SupplementCancelled
					}
					require.Equal(t, want, current.Status)
					require.NotNil(t, current.DecidedAt)
					if tc.operation == "decide" {
						var employee models.Employee
						require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).First(&employee, pendingID).Error)
						require.Equal(t, 1, *employee.Status, "round entities must already be activated")
					}
				}
			}
			name := "test:remaining_callback:" + strings.ReplaceAll(t.Name(), "/", ":")
			hits := 0
			if tc.mode == "late_seen_error" {
				require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register(name, func(tx *gorm.DB) {
					if strings.HasPrefix(strings.ToLower(strings.Join(strings.Fields(tx.Statement.SQL.String()), " ")), "insert into application_status_views") {
						hits++
						checkPrimary(tx)
						tx.AddError(errors.New("synthetic late seen failure"))
					}
				}))
				t.Cleanup(func() { require.NoError(t, db.Callback().Raw().Remove(name)) })
			} else {
				actions := map[string]string{"forward": models.AuditActionForwarded, "approve": "approve", "revoke": "revoke_approval",
					"supplement_approve": models.AuditActionSupplementApprove, "supplement_revoke": models.AuditActionSupplementRevokeApproval,
					"decide": models.AuditActionSupplementAccepted, "cancel": models.AuditActionSupplementCancelledByAuthor}
				processor := db.Callback().Create().Before("gorm:create")
				if tc.operation == "create" {
					processor = db.Callback().Create().After("gorm:create")
				}
				require.NoError(t, processor.Register(name, func(tx *gorm.DB) {
					if tc.operation == "create" {
						if tx.Statement.Table != "applications" {
							return
						}
					} else {
						entry, ok := tx.Statement.Dest.(*models.AuditLog)
						if !ok || entry.Action != actions[tc.operation] {
							return
						}
					}
					hits++
					checkPrimary(tx)
					panic("synthetic remaining transaction panic")
				}))
				t.Cleanup(func() { require.NoError(t, db.Callback().Create().Remove(name)) })
			}
			pub := &fakePublisher{}
			permissionSpy := &remainingPermissionSpy{PermissionService: services.NewPermissionService(db)}
			recorder := services.NewAuditRecorder(db)
			svc := services.NewApplicationService(db, permissionSpy, services.NewNotificationService(db),
				services.NewVehicleBlacklistService(db, recorder), services.NewPersonBlacklistService(db, recorder), recorder,
				services.WithRealtimePublisher(pub))
			createReq := services.ApplicationCreateRequest{ApplicationConfirmationInput: seedApplicationConfirmation(t, db), OrganizationID: &td.OrgID, DataApproval: true}
			forwardReq := services.ForwardApplicationRequest{Users: []services.ForwardUser{{UserID: ids[recipient], RequiredApproval: true}, {UserID: ids[viewer], CanView: true}}, AttachmentIDs: []int{attID}}
			invoke := func() error {
				switch tc.operation {
				case "create":
					_, err := svc.CreateApplication(context.Background(), sender, createReq, false)
					return err
				case "forward":
					return svc.ForwardApplication(context.Background(), sender, appID, false, forwardReq)
				case "approve":
					return svc.ApproveApplicationByUser(context.Background(), voter, appID, services.UserApprovalRequest{Status: "approved"})
				case "revoke":
					_, err := svc.RevokeApproval(context.Background(), voter, appID, services.RevokeApprovalRequest{})
					return err
				case "supplement_approve":
					_, err := svc.ApproveSupplement(context.Background(), voter, appID, round.ID, services.SupplementApprovalRequest{Status: "approved"})
					return err
				case "supplement_revoke":
					_, err := svc.RevokeSupplementApproval(context.Background(), voter, appID, round.ID, services.SupplementRevokeApprovalRequest{})
					return err
				case "decide":
					_, err := svc.DecideSupplement(context.Background(), voter, appID, round.ID, services.SupplementDecisionRequest{Action: "accept"})
					return err
				default:
					_, err := svc.CancelSupplement(context.Background(), sender, appID, round.ID, false, services.SupplementCancelRequest{})
					return err
				}
			}
			if tc.mode == "HTTP_panic" {
				url, token, body := "/applications", tokens[sender], transactionIntegrityJSON(t, createReq)
				switch tc.operation {
				case "forward":
					url, body = fmt.Sprintf("/applications/%d/forward", appID), transactionIntegrityJSON(t, forwardReq)
				case "supplement_approve":
					url, token, body = fmt.Sprintf("/applications/%d/supplements/%d/approve", appID, round.ID), tokens[voter], `{"status":"approved"}`
				case "decide":
					url, token, body = fmt.Sprintf("/applications/%d/supplements/%d/take-to-work", appID, round.ID), tokens[voter], `{"action":"accept"}`
				}
				rec := testutil.POST(t, e, url, body, testutil.AuthHeader(token))
				require.Equal(t, http.StatusInternalServerError, rec.Code)
			} else {
				if tc.mode == "late_seen_error" {
					require.Error(t, invoke())
				} else {
					require.PanicsWithValue(t, "synthetic remaining transaction panic", func() { _ = invoke() })
				}
				require.Empty(t, pub.events)
				require.Zero(t, permissionSpy.grants, "forward may grant permission only after a successful commit")
			}
			require.Equal(t, 1, hits)
			require.Equal(t, before, remainingTransactionSnapshot(t, db), "rollback restores all stored state and prevents success notification/permission effects")
		})
	}
}
