package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSupplementBatch_BoundariesBranchesAndRollback(t *testing.T) {
	e, originalDB, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, originalDB)
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	// A Session shares callbacks with the HTTP DB. Its asynchronous PDAudit
	// writer must not race with this test's SQL observers and fault callbacks.
	// Keep the same pool/schema, but give the service its own callback registry.
	sqlDB, err := originalDB.DB()
	require.NoError(t, err)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger: originalDB.Logger, NowFunc: func() time.Time { return now },
	})
	require.NoError(t, err)
	td := testutil.SeedTestData(t, db)
	transactionIntegrityKey(t)
	authorID, token := suppAuthor(t, e, db, "batch_budget_author", td.OrgID, td.CompanyID)
	voterID, _ := suppAuthor(t, e, db, "batch_budget_voter", td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tables := []int{}
	for i := 0; i < 3; i++ {
		table := models.SystemTable{Name: fmt.Sprintf("batch_budget_post_%d", i), TableType: "people", IsActive: true}
		require.NoError(t, db.Create(&table).Error)
		tables = append(tables, table.ID)
	}
	fixtureNumber := 0
	fixture := func(t *testing.T, status string, n int, posts []int) (int, int, int, services.CreateSupplementRequest) {
		t.Helper()
		fixtureNumber++
		appID := suppApp(t, db, td.OrgID, authorID, fmt.Sprintf("BATCH-%02d", fixtureNumber), models.ConfirmationApproved, status)
		attID := suppAttachment(t, db, appID, "people", "2099-12-31")
		oldID := suppNewEmployee(t, db, attID, "Original before supplement", nil)
		require.NoError(t, db.Create(&models.ApplicationResponsibleUser{ApplicationID: appID, UserID: voterID,
			RequiredApproval: true, ApprovalStatus: testutil.Ptr("approved"), ApprovalComment: testutil.Ptr("Existing vote"),
			ApprovalDatetime: &now, CreatedAt: now}).Error)
		rows := transactionIntegrityEmployees(n, citizenshipID, posts)
		for i := range rows {
			rows[i].PDConsent = true
		}
		return appID, attID, oldID, services.CreateSupplementRequest{Additions: []services.SupplementAddition{{AttachmentID: attID, Employees: rows}}}
	}
	for _, tc := range []struct {
		status    string
		n, posts  int
		duplicate bool
	}{
		{models.StatusProcessing, 500, 3, false}, {models.StatusProcessing, 501, 1, false},
		{models.StatusInWork, 500, 3, false}, {models.StatusInWork, 501, 1, false},
		// Exactly 2000 new rows, plus an existing employee: this is a per-request
		// budget, not a lifetime cap on the attachment or application.
		{models.StatusProcessing, 2000, 1, false}, {models.StatusProcessing, 1, 1, true},
	} {
		t.Run(fmt.Sprintf("success_%s_%dx%d_duplicate=%t", tc.status, tc.n, tc.posts, tc.duplicate), func(t *testing.T) {
			posts := append([]int(nil), tables[:tc.posts]...)
			if tc.duplicate {
				posts = append(posts, posts[0])
			}
			appID, attID, oldID, req := fixture(t, tc.status, tc.n, posts)
			req.Additions[0].Employees[0].OtherPermission = testutil.Ptr("Учебное разрешение на работы")
			beforeApp := suppReadApplication(t, db, appID)
			var oldEmployee models.Employee
			require.NoError(t, db.First(&oldEmployee, oldID).Error)
			var oldVote models.ApplicationResponsibleUser
			require.NoError(t, db.Where("application_id = ? AND user_id = ?", appID, voterID).First(&oldVote).Error)
			counts := compositionCountSQL(t, db)
			pub := &fakePublisher{}
			response, err := transactionIntegrityService(db, pub).CreateSupplement(context.Background(), "batch_budget_author", appID, false, req)
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, tc.n, response.Counts.Employees)
			require.Equal(t, 1, response.Number)
			wantStatus := models.SupplementMerged
			if tc.status == models.StatusInWork {
				wantStatus = models.SupplementPending
			}
			require.Equal(t, wantStatus, response.Status)
			// Count observed INSERT statements, not elapsed time. The former
			// per-row path issued n + n*posts entity/binding INSERTs.
			require.Equal(t, (tc.n+499)/500, counts.employeeStatements)
			require.Equal(t, (tc.n*len(posts)+999)/1000, counts.bindingStatements)
			t.Logf("employee/binding INSERTs observed=%d+%d; previous per-row shape=%d+%d (structural comparison, no timing benchmark)", counts.employeeStatements, counts.bindingStatements, tc.n, tc.n*len(posts))
			var employees []models.Employee
			require.NoError(t, db.Where("supplement_id = ?", response.SupplementID).Order("id").Find(&employees).Error)
			require.Len(t, employees, tc.n)
			var links []models.EmployeeTargetTable
			require.NoError(t, db.Where("employee_id IN (?)", db.Model(&models.Employee{}).Select("id").Where("supplement_id = ?", response.SupplementID)).Find(&links).Error)
			require.Len(t, links, tc.n*len(posts))
			linksByEmployee := map[int][]int{}
			for _, link := range links {
				require.Equal(t, "application", link.Source)
				require.NotNil(t, link.OrderIndex)
				require.Equal(t, 1, *link.OrderIndex)
				linksByEmployee[link.EmployeeID] = append(linksByEmployee[link.EmployeeID], link.TableID)
			}
			var history []models.AuditLog
			require.NoError(t, db.Where("entity_type = ? AND action = ? AND entity_id IN (?)", models.AuditEntityEmployee, "create",
				db.Model(&models.Employee{}).Select("id").Where("supplement_id = ?", response.SupplementID)).Find(&history).Error)
			require.Len(t, history, tc.n, "per-employee audit remains individual")
			historyByEmployee := map[int]models.AuditLog{}
			for _, entry := range history {
				historyByEmployee[*entry.EntityID] = entry
			}
			for i, employee := range employees {
				input := req.Additions[0].Employees[i]
				require.NotZero(t, employee.ID)
				require.Equal(t, attID, *employee.AttachmentID)
				require.Equal(t, response.SupplementID, *employee.SupplementID)
				require.Equal(t, input.LastName, *employee.LastName)
				require.Equal(t, input.PassportSeriesNumber, *employee.PassportSeriesNumber)
				require.Equal(t, *input.PatentNumber, *employee.PatentNumber)
				require.Equal(t, crypto.HMACOptional(&input.PassportSeriesNumber), employee.PassportSeriesNumberHMAC)
				require.Equal(t, crypto.HMACOptional(input.PatentNumber), employee.PatentNumberHMAC)
				require.Equal(t, input.OtherPermission, employee.OtherPermission)
				require.Equal(t, models.PeriodInherit, employee.PeriodMode, "BeforeCreate must run for every batched employee")
				require.Equal(t, 0, *employee.Status)
				require.NotNil(t, employee.PDConsentAt)
				require.Equal(t, authorID, *employee.PDConsentByUserID)
				require.ElementsMatch(t, posts, linksByEmployee[employee.ID])
				var details struct {
					Comment string `json:"comment"`
				}
				require.NoError(t, json.Unmarshal(historyByEmployee[employee.ID].Details, &details))
				require.Contains(t, details.Comment, input.LastName, "audit must follow the matching ID and plaintext input")
			}
			var plaintext int64
			var rawOtherPermission string
			require.NoError(t, db.Raw("SELECT other_permission FROM employees WHERE id = ?", employees[0].ID).Scan(&rawOtherPermission).Error)
			require.NotEqual(t, *req.Additions[0].Employees[0].OtherPermission, rawOtherPermission)
			require.NoError(t, db.Table("employees").Where("supplement_id = ? AND passport_series_number LIKE '9000 %'", response.SupplementID).Count(&plaintext).Error)
			require.Zero(t, plaintext)
			var currentOld models.Employee
			require.NoError(t, db.First(&currentOld, oldID).Error)
			require.Equal(t, oldEmployee, currentOld)
			app := suppReadApplication(t, db, appID)
			require.Equal(t, tc.status, *app.Status)
			var vote models.ApplicationResponsibleUser
			require.NoError(t, db.Where("application_id = ? AND user_id = ?", appID, voterID).First(&vote).Error)
			if tc.status == models.StatusInWork {
				require.Equal(t, beforeApp.Confirmation, app.Confirmation)
				require.Equal(t, oldVote, vote)
			} else {
				require.Equal(t, models.ConfirmationPending, *app.Confirmation)
				require.Equal(t, "pending", *vote.ApprovalStatus)
				require.Nil(t, vote.ApprovalDatetime)
			}
			rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), testutil.AuthHeader(token))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.NotEmpty(t, pub.events)
		})
	}
	for _, status := range []string{models.StatusProcessing, models.StatusInWork} {
		for _, table := range []string{"employees", "employee_target_tables"} {
			t.Run("rollback_"+status+"_"+table, func(t *testing.T) {
				n, posts := 501, tables[:1]
				if table == "employee_target_tables" {
					n, posts = 500, tables
				}
				appID, _, _, req := fixture(t, status, n, posts)
				before := remainingTransactionSnapshot(t, db)
				baseCounts := transactionIntegritySnapshot(t, db)
				calls := transactionIntegrityCreateFault(t, db, table, 2, func(tx *gorm.DB) {
					var count int64
					require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Table(table).Count(&count).Error)
					firstBatch := int64(500)
					if table == "employee_target_tables" {
						firstBatch = 1000
					}
					require.Equal(t, baseCounts[table]+firstBatch, count, "first batch must exist before second batch fails")
					tx.AddError(errors.New("synthetic second supplement batch failure"))
				})
				pub := &fakePublisher{}
				response, err := transactionIntegrityService(db, pub).CreateSupplement(context.Background(), "batch_budget_author", appID, false, req)
				require.Error(t, err)
				require.Nil(t, response)
				require.Equal(t, 2, *calls)
				require.Equal(t, before, remainingTransactionSnapshot(t, db))
				require.Equal(t, baseCounts, transactionIntegritySnapshot(t, db), "bindings and notifications must roll back too")
				require.Empty(t, pub.events)
			})
		}
	}
}
