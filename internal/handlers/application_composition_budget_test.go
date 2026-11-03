package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type compositionSQLCounts struct{ writes, employeeStatements, bindingStatements int }

// Test-local SQL accounting; this adds no production metrics or timing claim.
// Successful Create callbacks represent actual INSERT statements, including batches.
func compositionCountSQL(t *testing.T, db *gorm.DB) *compositionSQLCounts {
	t.Helper()
	counts := &compositionSQLCounts{}
	name := "test:composition_sql:" + strings.ReplaceAll(t.Name(), "/", ":")
	businessTable := func(table string) bool {
		return table == "applications" || table == "attachments" || table == "cars" || table == "employees" || table == "items" ||
			strings.HasPrefix(table, "application_") || strings.HasPrefix(table, "employee_") || strings.HasPrefix(table, "car_") || table == "audit_log" || table == "notifications"
	}
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(name+":attempt", func(tx *gorm.DB) {
		if businessTable(tx.Statement.Table) {
			counts.writes++
		}
	}))
	require.NoError(t, db.Callback().Create().After("gorm:create").Register(name+":insert", func(tx *gorm.DB) {
		if tx.Error != nil {
			return
		}
		if tx.Statement.Table == "employees" {
			counts.employeeStatements++
		}
		if tx.Statement.Table == "employee_target_tables" {
			counts.bindingStatements++
		}
	}))
	raw := func(tx *gorm.DB) {
		fields := strings.Fields(strings.ToLower(tx.Statement.SQL.String()))
		if len(fields) < 3 {
			return
		}
		table := ""
		switch fields[0] {
		case "insert":
			table = fields[2]
		case "update":
			table = fields[1]
		case "delete":
			table = fields[2]
		}
		if businessTable(strings.Trim(table, `"`)) {
			counts.writes++
		}
	}
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register(name, raw))
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register(name, raw))
	t.Cleanup(func() {
		require.NoError(t, db.Callback().Create().Remove(name+":attempt"))
		require.NoError(t, db.Callback().Create().Remove(name+":insert"))
		require.NoError(t, db.Callback().Raw().Remove(name))
		require.NoError(t, db.Callback().Row().Remove(name))
	})
	return counts
}

func compositionOversize(kind string) (services.CompleteApplicationRequest, services.CreateSupplementRequest) {
	submit := services.CompleteApplicationRequest{Organization: "Test Organization", DataApproval: true,
		ResponsiblePerson: "Fixture", ContactPhone: "+79000000000"}
	supplement := services.CreateSupplementRequest{}
	for _, n := range []int{1000, 1001} {
		att := services.AttachmentData{AttachmentType: kind}
		addition := services.SupplementAddition{AttachmentID: 1}
		switch kind {
		case "people":
			rows := make([]services.EmployeeInput, n)
			att.Data.Employees = &rows
			addition.Employees = rows
		case "cars":
			rows := make([]services.VehicleInput, n)
			att.Data.Vehicles = &rows
			addition.Vehicles = rows
		case "items":
			rows := make([]services.ItemInput, n)
			att.Data.Items = &rows
			addition.Items = rows
		}
		submit.Attachments = append(submit.Attachments, att)
		supplement.Additions = append(supplement.Additions, addition)
	}
	return submit, supplement
}

func TestCompositionBudget_HTTPAndServiceRejectBeforeWrites(t *testing.T) {
	e, originalDB, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, originalDB)
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	db := originalDB.Session(&gorm.Session{NowFunc: func() time.Time { return now }})
	td := testutil.SeedTestData(t, db)
	authorID, token := suppAuthor(t, e, db, "budget_author", td.OrgID, td.CompanyID)
	_, strangerToken := suppAuthor(t, e, db, "budget_stranger", td.OrgID, td.CompanyID)
	appID := suppApp(t, db, td.OrgID, authorID, "BUDGET-OWN", models.ConfirmationApproved, models.StatusProcessing)
	closedID := suppApp(t, db, td.OrgID, authorID, "BUDGET-CLOSED", models.ConfirmationApproved, models.StatusCompleted)
	before := remainingTransactionSnapshot(t, db)
	counts := compositionCountSQL(t, db)
	pub := &fakePublisher{}
	svc := transactionIntegrityService(db, pub)
	for _, kind := range []string{"people", "cars", "items"} {
		t.Run(kind, func(t *testing.T) {
			submit, supplement := compositionOversize(kind)
			response, err := svc.SubmitCompleteApplication(context.Background(), "budget_author", submit, false)
			require.Nil(t, response)
			var validation *apperr.Error
			require.ErrorAs(t, err, &validation)
			require.Equal(t, http.StatusBadRequest, validation.Code)
			require.Contains(t, validation.Message, "2001")
			supplementResponse, err := svc.CreateSupplement(context.Background(), "budget_author", appID, false, supplement)
			require.Nil(t, supplementResponse)
			require.ErrorAs(t, err, &validation)
			require.Equal(t, http.StatusBadRequest, validation.Code)
			require.Contains(t, validation.Message, "2001")
			for _, request := range []struct {
				path string
				body interface{}
			}{
				{"/applications/submit-complete-application", submit}, {fmt.Sprintf("/applications/%d/supplements", appID), supplement},
			} {
				rec := testutil.POST(t, e, request.path, transactionIntegrityJSON(t, request.body), testutil.AuthHeader(token))
				require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
				require.Contains(t, rec.Body.String(), "2001")
			}
			require.Zero(t, counts.writes, "over-budget composition must not attempt business writes")
			require.Equal(t, before, remainingTransactionSnapshot(t, db))
			require.Empty(t, pub.events)
		})
	}
	_, oversize := compositionOversize("people")
	for _, tc := range []struct {
		username, token string
		appID, code     int
	}{
		{"budget_stranger", strangerToken, appID, http.StatusForbidden},
		{"budget_author", token, closedID, http.StatusConflict},
	} {
		response, err := svc.CreateSupplement(context.Background(), tc.username, tc.appID, false, oversize)
		require.Error(t, err)
		require.Nil(t, response)
		var accessError *echo.HTTPError
		require.ErrorAs(t, err, &accessError)
		require.Equal(t, tc.code, accessError.Code)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements", tc.appID), transactionIntegrityJSON(t, oversize), testutil.AuthHeader(tc.token))
		require.Equal(t, tc.code, rec.Code, "ownership/status must precede the budget error: %s", rec.Body.String())
		require.Zero(t, counts.writes)
	}
	require.Equal(t, before, remainingTransactionSnapshot(t, db))
}
