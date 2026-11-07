package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// No parallel tests: GORM callbacks and the encryption key are process-wide fixtures.
var transactionIntegrityTime = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

func transactionIntegrityKey(t *testing.T) {
	t.Helper()
	crypto.SetGlobalKey([]byte("0123456789abcdef0123456789abcdef"))
	t.Cleanup(func() { crypto.SetGlobalKey(nil) })
}

func transactionIntegrityService(db *gorm.DB, publisher *fakePublisher) services.ApplicationService {
	recorder := services.NewAuditRecorder(db)
	return services.NewApplicationService(db, services.NewPermissionService(db),
		services.NewNotificationService(db), services.NewVehicleBlacklistService(db, recorder),
		services.NewPersonBlacklistService(db, recorder), recorder,
		services.WithRealtimePublisher(publisher))
}

// Include dependent rows and postcommit notifications: equal snapshots prove that
// existing records survived and that no partial operation or success notice escaped.
func transactionIntegritySnapshot(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	result := map[string]int64{}
	for _, table := range []string{"applications", "attachments", "employees", "cars", "items",
		"employee_target_tables", "car_target_tables", "car_unload_places", "attachment_unload_places",
		"application_supplements", "application_supplement_approvals", "application_responsible_users",
		"application_viewers", "audit_log", "notifications"} {
		var count int64
		require.NoError(t, db.Table(table).Count(&count).Error, table)
		result[table] = count
	}
	return result
}

func transactionIntegrityEmployees(n, citizenshipID int, tables []int) []services.EmployeeInput {
	rows := make([]services.EmployeeInput, n)
	for i := range rows {
		rows[i] = services.EmployeeInput{LastName: fmt.Sprintf("Integrity%04d", i), FirstName: "Fixture",
			CitizenshipID: citizenshipID, Position: "Worker", PassportSeriesNumber: fmt.Sprintf("9000 %06d", i),
			PatentNumber: testutil.Ptr(fmt.Sprintf("77%08d", i)), TargetTables: tables}
	}
	return rows
}

func transactionIntegrityRequest(t *testing.T, db *gorm.DB, kind string, n, citizenshipID int, tables []int, placeID int) services.CompleteApplicationRequest {
	t.Helper()
	uaID := seedUniqueAttachment(t, db, kind, "integrity_"+kind, "Integrity")
	att := services.AttachmentData{AttachmentType: kind, UniqueAttachmentID: uaID,
		AttachmentName: "integrity", AttachmentDisplayName: "Integrity",
		EntryDateFrom: testutil.Ptr("2026-01-01"), EntryDateTo: testutil.Ptr("2099-12-31"),
		EntryTimeFrom: testutil.Ptr("08:00"), EntryTimeTo: testutil.Ptr("18:00")}
	if kind == "people" {
		rows := transactionIntegrityEmployees(n, citizenshipID, tables)
		att.Data.Employees = &rows
	} else {
		rows := make([]services.VehicleInput, n)
		for i := range rows {
			rows[i] = services.VehicleInput{CarNumber: fmt.Sprintf("I%06d", i), CarBrand: "Fixture",
				TargetTables: tables, UnloadPlaces: []int{placeID}}
		}
		att.Data.Vehicles = &rows
	}
	return services.CompleteApplicationRequest{ApplicationConfirmationInput: seedApplicationConfirmation(t, db), Organization: "Test Organization", DataApproval: true,
		ResponsiblePerson: "Fixture", ContactPhone: "+79000000000", Attachments: []services.AttachmentData{att}}
}

func transactionIntegrityJSON(t *testing.T, value interface{}) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}

func transactionIntegrityCreateFault(t *testing.T, db *gorm.DB, table string, at int, fault func(*gorm.DB)) *int {
	t.Helper()
	calls := 0
	name := "test:transaction_integrity:" + strings.ReplaceAll(t.Name(), "/", ":")
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			calls++
			if calls == at {
				fault(tx)
			}
		}
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Create().Remove(name)) })
	return &calls
}

func TestApplicationTransactionIntegrity_SecondBindingBatchRollsBackAndKeepsDraft(t *testing.T) {
	e, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	transactionIntegrityKey(t)
	token := testutil.RegisterAndLogin(t, e, "integrity_batch", "pass123", 1, td.OrgID, td.CompanyID)
	citizenship := seedCitizenship(t, db)
	peopleTable := seedSystemTable(t, db)
	carTable := seedCarsTable(t, db, "integrity_cars", "Integrity cars")
	place := seedPlace(t, db, "Integrity unload")
	file := uploadDraftFile(t, e, token, "fixture.png", realPNG(t))
	var draft models.ApplicationFile
	require.NoError(t, db.First(&draft, file.ID).Error)
	diskPath := filepath.Join(uploadDir, "application_files", draft.StoredName)
	beforeBytes, err := os.ReadFile(diskPath)
	require.NoError(t, err)
	// A committed application/entity must survive each subsequent failed submit.
	oldApp := suppApp(t, db, td.OrgID, getUserID(t, db, "integrity_batch"), "INTEGRITY-OLD", models.ConfirmationApproved, models.StatusProcessing)
	oldAtt := suppAttachment(t, db, oldApp, "people", "2099-12-31")
	oldEmployee := suppNewEmployee(t, db, oldAtt, "Original", nil)
	for _, tc := range []struct {
		table, kind string
		tableID     int
	}{
		{"car_unload_places", "cars", carTable}, {"car_target_tables", "cars", carTable},
		{"employee_target_tables", "people", peopleTable},
	} {
		t.Run(tc.table, func(t *testing.T) {
			// Mandatory bindings use batches of 1000 (employees use 500).
			req := transactionIntegrityRequest(t, db, tc.kind, 1001, citizenship, []int{tc.tableID}, place)
			req.FileIDs = []int{file.ID}
			before := transactionIntegritySnapshot(t, db)
			calls := transactionIntegrityCreateFault(t, db, tc.table, 2, func(tx *gorm.DB) {
				tx.AddError(errors.New("synthetic binding failure"))
			})
			rec := testutil.POST(t, e, "/applications/submit-complete-application", transactionIntegrityJSON(t, req), testutil.AuthHeader(token))
			require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
			require.Equal(t, 2, *calls, "fault must occur after the first batch was written")
			require.Equal(t, before, transactionIntegritySnapshot(t, db))
			var got models.ApplicationFile
			require.NoError(t, db.First(&got, file.ID).Error)
			require.Equal(t, draft, got, "file must remain the same unattached draft")
			afterBytes, err := os.ReadFile(diskPath)
			require.NoError(t, err)
			require.Equal(t, beforeBytes, afterBytes)
			var original models.Employee
			require.NoError(t, db.First(&original, oldEmployee).Error)
			require.Equal(t, "Original", *original.LastName)
		})
	}
}

func TestApplicationTransactionIntegrity_PanicPropagatesAndHTTPRecoverRejectsSuccess(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "integrity_panic", "pass123", 1, td.OrgID, td.CompanyID)
	req := transactionIntegrityRequest(t, db, "people", 1, seedCitizenship(t, db), []int{seedSystemTable(t, db)}, 0)
	// SetupTestApp has no Recover; explicitly exercise production Echo middleware.
	e.Use(echomw.Recover())
	for _, viaHTTP := range []bool{false, true} {
		t.Run(fmt.Sprintf("HTTP=%t", viaHTTP), func(t *testing.T) {
			before := transactionIntegritySnapshot(t, db)
			calls := transactionIntegrityCreateFault(t, db, "employee_target_tables", 1, func(tx *gorm.DB) {
				var count int64
				// Keep this read on the transaction connection, but discard the
				// binding Create statement's table/model/clauses.
				require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Model(&models.Employee{}).Count(&count).Error)
				require.Positive(t, count, "panic must follow primary/entity writes")
				panic("synthetic transaction panic")
			})
			publisher := &fakePublisher{}
			if viaHTTP {
				rec := testutil.POST(t, e, "/applications/submit-complete-application", transactionIntegrityJSON(t, req), testutil.AuthHeader(token))
				require.Equal(t, http.StatusInternalServerError, rec.Code)
			} else {
				svc := transactionIntegrityService(db, publisher)
				require.PanicsWithValue(t, "synthetic transaction panic", func() {
					_, _ = svc.SubmitCompleteApplication(context.Background(), "integrity_panic", req, false)
				})
				require.Empty(t, publisher.events, "no postcommit realtime signal on rollback")
			}
			require.Equal(t, 1, *calls)
			require.Equal(t, before, transactionIntegritySnapshot(t, db))
		})
	}
}

func TestApplicationTransactionIntegrity_ContextCancelledBeforeCommit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	testutil.RegisterAndLogin(t, e, "integrity_cancel", "pass123", 1, td.OrgID, td.CompanyID)
	req := transactionIntegrityRequest(t, db, "people", 1, seedCitizenship(t, db), []int{seedSystemTable(t, db)}, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	before := transactionIntegritySnapshot(t, db)
	calls := transactionIntegrityCreateFault(t, db, "employee_target_tables", 1, func(_ *gorm.DB) { cancel() })
	pub := &fakePublisher{}
	resp, err := transactionIntegrityService(db, pub).SubmitCompleteApplication(ctx, "integrity_cancel", req, false)
	require.Error(t, err)
	require.Nil(t, resp)
	require.Equal(t, 1, *calls)
	require.Equal(t, before, transactionIntegritySnapshot(t, db))
	require.Empty(t, pub.events)
}

func TestApplicationTransactionIntegrity_BatchBoundariesPreserveEmployees(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	transactionIntegrityKey(t)
	token := testutil.RegisterAndLogin(t, e, "integrity_success", "pass123", 1, td.OrgID, td.CompanyID)
	citizenship := seedCitizenship(t, db)
	tables := []int{}
	for i := 0; i < 3; i++ {
		table := models.SystemTable{Name: fmt.Sprintf("integrity_people_%d", i), TableType: "people", IsActive: true}
		require.NoError(t, db.Create(&table).Error)
		tables = append(tables, table.ID)
	}
	// 500 x 3 stresses binding batches; 501 x 1 crosses the employee batch edge.
	for _, tc := range []struct{ n, posts int }{{500, 3}, {501, 1}} {
		t.Run(fmt.Sprintf("%dx%d", tc.n, tc.posts), func(t *testing.T) {
			req := transactionIntegrityRequest(t, db, "people", tc.n, citizenship, tables[:tc.posts], 0)
			rec := testutil.POST(t, e, "/applications/submit-complete-application", transactionIntegrityJSON(t, req), testutil.AuthHeader(token))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			appID := submittedApplicationID(t, rec)
			var app models.Application
			require.NoError(t, db.First(&app, appID).Error)
			require.NotNil(t, app.ApplicationNumber)
			require.NotEmpty(t, *app.ApplicationNumber)
			var att models.Attachment
			require.NoError(t, db.Where("application_id = ?", appID).First(&att).Error)
			var rows []models.Employee
			require.NoError(t, db.Where("attachment_id = ?", att.ID).Order("id").Find(&rows).Error)
			require.Len(t, rows, tc.n)
			for i, row := range rows {
				require.Equal(t, fmt.Sprintf("Integrity%04d", i), *row.LastName)
				require.Equal(t, fmt.Sprintf("9000 %06d", i), *row.PassportSeriesNumber)
				require.Equal(t, fmt.Sprintf("77%08d", i), *row.PatentNumber)
				var links []models.EmployeeTargetTable
				require.NoError(t, db.Where("employee_id = ?", row.ID).Find(&links).Error)
				got := make([]int, 0, len(links))
				for _, link := range links {
					got = append(got, link.TableID)
				}
				require.ElementsMatch(t, tables[:tc.posts], got)
			}
			var plaintext int64
			require.NoError(t, db.Table("employees").Where("attachment_id = ? AND passport_series_number LIKE '9000 %'", att.ID).Count(&plaintext).Error)
			require.Zero(t, plaintext, "BeforeSave must encrypt every employee batch")
			card := testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), testutil.AuthHeader(token))
			require.Equal(t, http.StatusOK, card.Code, card.Body.String())
			var notices int64
			require.NoError(t, db.Table("notifications").Where("user_id = ? AND type = ? AND data->>'application_id' = ?",
				getUserID(t, db, "integrity_success"), services.NotificationTypeApplicationCreated, fmt.Sprint(appID)).Count(&notices).Error)
			require.EqualValues(t, 1, notices, "successful commit keeps the sender notification")
		})
	}
}
