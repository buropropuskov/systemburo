package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSupplementTransactionIntegrity_BothBranchesRollbackAndRecover(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	transactionIntegrityKey(t)
	e.Use(echomw.Recover()) // SetupTestApp intentionally does not install Recover.
	authorID, token := suppAuthor(t, e, db, "integrity_supp_author", td.OrgID, td.CompanyID)
	approverID, _ := suppAuthor(t, e, db, "integrity_supp_approver", td.OrgID, td.CompanyID)
	citizenship := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)
	for _, status := range []string{models.StatusProcessing, models.StatusInWork} {
		t.Run(status, func(t *testing.T) {
			appID := suppApp(t, db, td.OrgID, authorID, "INTEGRITY-SUPP-"+status, models.ConfirmationApproved, status)
			attID := suppAttachment(t, db, appID, "people", "2099-12-31")
			originalID := suppNewEmployee(t, db, attID, "Original", nil)
			vote := models.ApplicationResponsibleUser{ApplicationID: appID, UserID: approverID,
				RequiredApproval: true, ApprovalStatus: testutil.Ptr("approved"),
				ApprovalComment: testutil.Ptr("Original vote"), ApprovalDatetime: testutil.Ptr(transactionIntegrityTime),
				CreatedAt: transactionIntegrityTime}
			require.NoError(t, db.Create(&vote).Error)
			// Compare database representations on both sides: pgx may decode
			// UTC timestamps using time.Local even when the instant is identical.
			require.NoError(t, db.First(&vote, vote.ID).Error)
			req := services.CreateSupplementRequest{Additions: []services.SupplementAddition{{
				AttachmentID: attID, Employees: transactionIntegrityEmployees(2, citizenship, []int{tableID}),
			}}}
			originalApp := suppReadApplication(t, db, appID)
			var originalEmployee models.Employee
			require.NoError(t, db.First(&originalEmployee, originalID).Error)
			for _, mode := range []string{"error_second_employee", "panic_service", "panic_HTTP"} {
				t.Run(mode, func(t *testing.T) {
					before := transactionIntegritySnapshot(t, db)
					publisher := &fakePublisher{}
					var calls *int
					latePanic := false
					if mode == "error_second_employee" {
						calls = transactionIntegrityCreateFault(t, db, "employees", 2, func(tx *gorm.DB) {
							tx.AddError(errors.New("synthetic supplement employee failure"))
						})
					} else {
						// Panic after entities AND branch-specific vote changes, immediately
						// before the final audit write. This does not change audit policy.
						name := "test:supplement_late_panic:" + strings.ReplaceAll(t.Name(), "/", ":")
						require.NoError(t, db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
							entry, ok := tx.Statement.Dest.(*models.AuditLog)
							if !ok || entry.Action != "supplement_created" {
								return
							}
							latePanic = true
							readTx := tx.Session(&gorm.Session{NewDB: true})
							if status == models.StatusProcessing {
								var current models.ApplicationResponsibleUser
								require.NoError(t, readTx.Where("application_id = ? AND user_id = ?", appID, approverID).First(&current).Error)
								require.Equal(t, "pending", *current.ApprovalStatus, "must reach branch reset before panic")
							} else {
								var count int64
								require.NoError(t, readTx.Raw(`SELECT COUNT(*) FROM application_supplement_approvals
									WHERE supplement_id IN (SELECT id FROM application_supplements WHERE application_id = ?)`, appID).Scan(&count).Error)
								require.EqualValues(t, 1, count, "must reach branch snapshot before panic")
							}
							panic("synthetic supplement panic")
						}))
						t.Cleanup(func() { require.NoError(t, db.Callback().Create().Remove(name)) })
					}
					if mode == "panic_service" {
						require.PanicsWithValue(t, "synthetic supplement panic", func() {
							_, _ = transactionIntegrityService(db, publisher).CreateSupplement(context.Background(), "integrity_supp_author", appID, false, req)
						})
					} else {
						rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements", appID), transactionIntegrityJSON(t, req), testutil.AuthHeader(token))
						require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
					}
					if calls != nil {
						require.Equal(t, 2, *calls)
					} else {
						require.True(t, latePanic)
					}
					require.Equal(t, before, transactionIntegritySnapshot(t, db))
					if mode == "panic_service" {
						require.Empty(t, publisher.events)
					}
					require.Equal(t, originalApp, suppReadApplication(t, db, appID))
					var gotEmployee models.Employee
					require.NoError(t, db.First(&gotEmployee, originalID).Error)
					require.Equal(t, originalEmployee, gotEmployee)
					var gotVote models.ApplicationResponsibleUser
					require.NoError(t, db.Where("application_id = ? AND user_id = ?", appID, approverID).First(&gotVote).Error)
					require.Equal(t, vote, gotVote, "rollback must restore prior approval, comment and timestamp")
				})
			}
			// The same application remains usable after each rolled-back attempt.
			rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements", appID), transactionIntegrityJSON(t, req), testutil.AuthHeader(token))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			result := testutil.ParseResponse[services.CreateSupplementResponse](t, rec)
			require.Equal(t, 1, result.Number, "failed attempts must not leave a round")
			wantStatus := models.SupplementMerged
			if status == models.StatusInWork {
				wantStatus = models.SupplementPending
			}
			require.Equal(t, wantStatus, result.Status)
			require.Equal(t, 2, result.Counts.Employees)
			var employees []models.Employee
			require.NoError(t, db.Where("supplement_id = ?", result.SupplementID).Order("id").Find(&employees).Error)
			require.Len(t, employees, 2)
			for i, employee := range employees {
				require.Equal(t, fmt.Sprintf("9000 %06d", i), *employee.PassportSeriesNumber)
				require.Equal(t, 0, *employee.Status)
			}
			gotApp := suppReadApplication(t, db, appID)
			require.Equal(t, status, *gotApp.Status)
			var gotVote models.ApplicationResponsibleUser
			require.NoError(t, db.Where("application_id = ? AND user_id = ?", appID, approverID).First(&gotVote).Error)
			if status == models.StatusInWork {
				require.Equal(t, models.ConfirmationApproved, *gotApp.Confirmation)
				require.Equal(t, vote, gotVote)
				var count int64
				require.NoError(t, db.Table("application_supplement_approvals").Where("supplement_id = ?", result.SupplementID).Count(&count).Error)
				require.EqualValues(t, 1, count)
			} else {
				require.Equal(t, models.ConfirmationPending, *gotApp.Confirmation)
				require.Equal(t, "pending", *gotVote.ApprovalStatus)
				require.Nil(t, gotVote.ApprovalComment)
				require.Nil(t, gotVote.ApprovalDatetime)
			}
		})
	}
}
