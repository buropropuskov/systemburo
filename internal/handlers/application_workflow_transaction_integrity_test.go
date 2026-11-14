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

type workflowTransactionState struct {
	Counts      map[string]int64
	App         models.Application
	Attachments []models.Attachment
	Cars        []models.Car
	Employees   []models.Employee
	Supplements []models.ApplicationSupplement
	Views       []models.ApplicationStatusView
	Audit       []models.AuditLog
}

func workflowTransactionSnapshot(t *testing.T, db *gorm.DB, appID int) workflowTransactionState {
	t.Helper()
	state := workflowTransactionState{Counts: transactionIntegritySnapshot(t, db), App: suppReadApplication(t, db, appID)}
	require.NoError(t, db.Where("application_id = ?", appID).Order("id").Find(&state.Attachments).Error)
	attIDs := db.Model(&models.Attachment{}).Select("id").Where("application_id = ?", appID)
	require.NoError(t, db.Where("attachment_id IN (?)", attIDs).Order("id").Find(&state.Cars).Error)
	require.NoError(t, db.Where("attachment_id IN (?)", attIDs).Order("id").Find(&state.Employees).Error)
	require.NoError(t, db.Where("application_id = ?", appID).Order("id").Find(&state.Supplements).Error)
	require.NoError(t, db.Where("application_id = ?", appID).Order("id").Find(&state.Views).Error)
	require.NoError(t, db.Order("id").Find(&state.Audit).Error)
	return state
}

func workflowTransactionRawFault(t *testing.T, db *gorm.DB, prefix string, fault func(*gorm.DB)) *int {
	t.Helper()
	name := "test:workflow_raw:" + strings.ReplaceAll(t.Name(), "/", ":")
	calls := 0
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register(name, func(tx *gorm.DB) {
		if strings.HasPrefix(strings.ToLower(strings.Join(strings.Fields(tx.Statement.SQL.String()), " ")), prefix) {
			calls++
			if calls == 1 {
				fault(tx)
			}
		}
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Raw().Remove(name)) })
	return &calls
}

func TestWorkflowTransactionIntegrity_RollbackActionsAndPropagatePanics(t *testing.T) {
	e, originalDB, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, originalDB)
	// Fixture timestamps are fixed; comparisons use round-tripped DB values.
	// Business SQL NOW() stays authoritative and is never compared to wall-clock Go time.
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	previousStatusTime := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
	db := originalDB.Session(&gorm.Session{NowFunc: func() time.Time { return now }})
	td := testutil.SeedTestData(t, db)
	e.Use(echomw.Recover())
	const sender, actor = "workflow_integrity_sender", "workflow_integrity_actor"
	senderToken := testutil.RegisterAndLogin(t, e, sender, "pass123", 1, td.OrgID, td.CompanyID)
	actorToken := testutil.RegisterAndLogin(t, e, actor, "pass123", 1, td.OrgID, td.CompanyID)
	senderID, actorID := getUserID(t, db, sender), getUserID(t, db, actor)
	makeApprover(t, db, actor)
	carTable := seedCarsTable(t, db, "workflow_integrity_cars", "Workflow cars")
	peopleTable := seedSystemTable(t, db)
	for _, tc := range []struct{ action, mode string }{
		{"accept", "panic_service"}, {"reject", "panic_service"},
		{"revoke", "panic_service"}, {"restore", "panic_service"}, {"withdraw", "panic_service"},
		{"accept", "panic_HTTP"}, {"revoke", "panic_HTTP"}, {"withdraw", "panic_HTTP"},
		{"accept", "primary_update_error"}, {"reject", "primary_update_error"}, {"revoke", "primary_update_error"},
		{"accept", "bump_seen_error"}, {"withdraw", "last_attachment_update_error"},
	} {
		t.Run(tc.action+"_"+tc.mode, func(t *testing.T) {
			initial, expected, auditAction := models.StatusInWork, models.StatusProcessing, "revoke_from_work"
			switch tc.action {
			case "accept":
				initial, expected, auditAction = models.StatusProcessing, models.StatusInWork, models.AuditActionTakeToWork
			case "reject":
				expected, auditAction = models.StatusRefused, models.AuditActionReject
			case "restore":
				initial, auditAction = models.StatusRefused, "restore_to_work"
			case "withdraw":
				expected, auditAction = models.StatusWithdrawn, models.AuditActionWithdraw
			}
			appID := suppApp(t, db, td.OrgID, senderID, "WORKFLOW-"+tc.action+"-"+tc.mode, models.ConfirmationApproved, initial)
			fields := map[string]interface{}{"status_updated_at": previousStatusTime, "confirmation_datetime": now,
				"responsible_user_id": actorID, "responsible_comment": "Original decision"}
			if tc.action != "accept" {
				fields["accepted_at"] = now
			}
			require.NoError(t, db.Model(&models.Application{}).Where("id = ?", appID).Updates(fields).Error)
			carsAtt := suppAttachment(t, db, appID, "cars", "2099-12-31")
			peopleAtt := suppAttachment(t, db, appID, "people", "2099-12-31")
			active := 1
			if tc.action == "accept" || tc.action == "restore" {
				active = 0
			}
			car := models.Car{AttachmentID: carsAtt, CarNumber: testutil.Ptr("W000001"), Status: &active}
			require.NoError(t, db.Create(&car).Error)
			require.NoError(t, db.Create(&models.CarTargetTable{CarID: car.ID, TableID: carTable}).Error)
			empID := suppNewEmployee(t, db, peopleAtt, "Workflow original", nil)
			require.NoError(t, db.Model(&models.Employee{}).Where("id = ?", empID).Update("status", active).Error)
			require.NoError(t, db.Create(&models.EmployeeTargetTable{EmployeeID: empID, TableID: peopleTable}).Error)
			round := models.ApplicationSupplement{ApplicationID: appID, Number: 1, Status: models.SupplementPending,
				CreatedByUserID: senderID, CreatedAt: now}
			require.NoError(t, db.Create(&round).Error)
			suppNewEmployee(t, db, peopleAtt, "Workflow pending", &round.ID)
			for _, userID := range []int{senderID, actorID} {
				require.NoError(t, db.Create(&models.ApplicationStatusView{ApplicationID: appID, UserID: userID, SeenAt: previousStatusTime}).Error)
			}
			before := workflowTransactionSnapshot(t, db, appID)
			checkPrimary := func(tx *gorm.DB) {
				var current models.Application
				require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).First(&current, appID).Error)
				require.Equal(t, expected, *current.Status, "fault must follow the primary status write")
				if tc.action == "accept" {
					require.NotNil(t, current.AcceptedAt)
				}
				if tc.action == "withdraw" {
					require.NotNil(t, current.WithdrawnAt)
				}
			}
			var calls *int
			panicMode := strings.HasPrefix(tc.mode, "panic_")
			if panicMode {
				name := "test:workflow_panic:" + strings.ReplaceAll(t.Name(), "/", ":")
				n := 0
				calls = &n
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
					entry, ok := tx.Statement.Dest.(*models.AuditLog)
					if !ok || entry.Action != auditAction {
						return
					}
					n++
					checkPrimary(tx)
					panic("synthetic workflow panic")
				}))
				t.Cleanup(func() { require.NoError(t, db.Callback().Create().Remove(name)) })
			} else {
				// Match the common prefix before driver-specific placeholders ($1).
				// Each action executes exactly one primary status UPDATE.
				prefix := "update applications set status ="
				if tc.mode == "bump_seen_error" {
					prefix = "insert into application_status_views"
				}
				if tc.mode == "last_attachment_update_error" {
					prefix = "update attachments set status = 0 where application_id ="
				}
				calls = workflowTransactionRawFault(t, db, prefix, func(tx *gorm.DB) {
					if tc.mode != "primary_update_error" {
						checkPrimary(tx)
						var current models.Application
						require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).First(&current, appID).Error)
						require.NotNil(t, current.StatusUpdatedAt)
						require.False(t, current.StatusUpdatedAt.Equal(*before.App.StatusUpdatedAt), "bump must precede the injected error")
					}
					if tc.mode == "last_attachment_update_error" {
						var currentCar models.Car
						require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).First(&currentCar, car.ID).Error)
						require.Equal(t, 0, *currentCar.Status)
						var currentEmployee models.Employee
						require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).First(&currentEmployee, empID).Error)
						require.Equal(t, 0, *currentEmployee.Status)
						var currentRound models.ApplicationSupplement
						require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).First(&currentRound, round.ID).Error)
						require.Equal(t, models.SupplementCancelled, currentRound.Status, "late fault must follow cancellation")
					}
					tx.AddError(errors.New("synthetic pre-SQL workflow failure"))
				})
			}
			pub := &fakePublisher{}
			svc := transactionIntegrityService(db, pub)
			invoke := func() error {
				comment := testutil.Ptr("Integrity transition")
				switch tc.action {
				case "accept", "reject":
					return svc.TakeApplicationToWork(context.Background(), actor, appID, services.TakeToWorkRequest{Action: tc.action, Comment: comment})
				case "revoke":
					return svc.RevokeApplicationFromWork(context.Background(), actor, appID, services.RevokeFromWorkRequest{Comment: comment})
				case "restore":
					return svc.RestoreApplicationToWork(context.Background(), actor, appID, services.RevokeFromWorkRequest{Comment: comment})
				default:
					return svc.WithdrawApplication(context.Background(), sender, appID)
				}
			}
			if tc.mode == "panic_HTTP" {
				path, token, body := "take-to-work", actorToken, `{"action":"accept"}`
				if tc.action == "revoke" {
					path, body = "revoke-from-work", `{}`
				}
				if tc.action == "withdraw" {
					path, token, body = "withdraw", senderToken, ""
				}
				rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/%s", appID, path), body, testutil.AuthHeader(token))
				require.Equal(t, http.StatusInternalServerError, rec.Code)
			} else {
				if panicMode {
					require.PanicsWithValue(t, "synthetic workflow panic", func() { _ = invoke() })
				} else {
					require.Error(t, invoke())
				}
				require.Empty(t, pub.events, "service publisher must not emit success on rollback")
			}
			require.Equal(t, 1, *calls, "fault must reach its intended write")
			require.Equal(t, before, workflowTransactionSnapshot(t, db, appID), "rollback must restore timestamps, flags, entities, attachments, supplement and audit; no notification may escape")
		})
	}
}
