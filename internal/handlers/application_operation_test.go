package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

func operationTestKey(now time.Time, suffix string) string {
	return fmt.Sprintf("v1.%d.%s", now.UnixMilli(), strings.Repeat(suffix, 32))
}

func operationSubmitRequest(t *testing.T, db *gorm.DB, key string, fileIDs []int) services.CompleteApplicationRequest {
	t.Helper()
	template := seedUniqueAttachment(t, db, "cars", "operation_cars", "Operation cars")
	vehicles := []services.VehicleInput{{CarNumber: "O700OO777", CarBrand: "Fixture"}}
	return services.CompleteApplicationRequest{
		ApplicationConfirmationInput: seedApplicationConfirmation(t, db),
		RequestIdempotencyKey:        key, Organization: "Test Organization", DataApproval: true,
		ResponsiblePerson: "Fixture", ContactPhone: "+79000000000", FileIDs: fileIDs,
		Attachments: []services.AttachmentData{{AttachmentType: "cars", AttachmentName: "operation_cars", AttachmentDisplayName: "Operation cars", UniqueAttachmentID: template,
			EntryDateFrom: testutil.Ptr("2026-01-01"), EntryDateTo: testutil.Ptr("2099-12-31"), EntryTimeFrom: testutil.Ptr("08:00"), EntryTimeTo: testutil.Ptr("18:00"), Data: services.AttachmentContentData{Vehicles: &vehicles}}},
	}
}

func TestApplicationOperation_SubmitReplayWithFilesAndCurrentAccess(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "operation_sender", "pass123", 1, td.OrgID, td.CompanyID)
	file := uploadDraftFile(t, e, token, "operation.png", realPNG(t))
	req := operationSubmitRequest(t, db, operationTestKey(time.Now().UTC(), "a"), []int{file.ID})
	body := transactionIntegrityJSON(t, req)
	first := testutil.POST(t, e, "/applications/submit-complete-application", withApplicationConfirmation(t, db, body), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	result := testutil.ParseResponse[services.CompleteApplicationResponse](t, first)
	before := transactionIntegritySnapshot(t, db)
	repeat := testutil.POST(t, e, "/applications/submit-complete-application", withApplicationConfirmation(t, db, body), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, repeat.Code, repeat.Body.String())
	require.Equal(t, result, testutil.ParseResponse[services.CompleteApplicationResponse](t, repeat))
	require.Equal(t, before, transactionIntegritySnapshot(t, db), "replay must not duplicate business rows, history or notifications")
	req.Message = testutil.Ptr("changed")
	mismatch := testutil.POST(t, e, "/applications/submit-complete-application", transactionIntegrityJSON(t, req), testutil.AuthHeader(token))
	require.Equal(t, http.StatusConflict, mismatch.Code, mismatch.Body.String())
	require.Equal(t, before, transactionIntegritySnapshot(t, db))
	var operation models.ApplicationOperation
	require.NoError(t, db.Where("key = ?", req.RequestIdempotencyKey).First(&operation).Error)
	require.NotContains(t, string(operation.Result), "contact_phone")
	require.NotContains(t, string(operation.Result), "file_ids")
	other := testutil.RegisterAndLogin(t, e, "operation_other_sender", "pass123", 1, td.OrgID, td.CompanyID)
	independent := req
	independent.FileIDs = nil
	separate := testutil.POST(t, e, "/applications/submit-complete-application", transactionIntegrityJSON(t, independent), testutil.AuthHeader(other))
	require.Equal(t, http.StatusOK, separate.Code, separate.Body.String())
	require.NotEqual(t, result.ApplicationID, testutil.ParseResponse[services.CompleteApplicationResponse](t, separate).ApplicationID)
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "operation_sender").Update("is_active", false).Error)
	denied := testutil.POST(t, e, "/applications/submit-complete-application", withApplicationConfirmation(t, db, body), testutil.AuthHeader(token))
	require.NotEqual(t, http.StatusOK, denied.Code)
}

func TestApplicationOperation_ConcurrentSubmitOneBusinessTransaction(t *testing.T) {
	previousParallelism := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(previousParallelism)
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "operation_concurrent", "pass123", 1, td.OrgID, td.CompanyID)
	file := uploadDraftFile(t, e, token, "concurrent.png", realPNG(t))
	req := operationSubmitRequest(t, db, operationTestKey(time.Now().UTC(), "b"), []int{file.ID})
	body := transactionIntegrityJSON(t, req)
	winnerInserted := make(chan struct{})
	secondInsert := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	var workers sync.WaitGroup
	var released sync.Once
	unblock := func() { released.Do(func() { close(release) }) }
	const before = "test:operation_concurrent_before"
	const after = "test:operation_concurrent_after"
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(before, func(tx *gorm.DB) {
		if tx.Statement.Table == "application_operations" && calls.Add(1) == 2 {
			close(secondInsert)
		}
	}))
	require.NoError(t, db.Callback().Create().After("gorm:create").Register(after, func(tx *gorm.DB) {
		if tx.Statement.Table == "application_operations" && tx.RowsAffected == 1 && tx.Error == nil {
			close(winnerInserted)
			<-release
		}
	}))
	defer db.Callback().Create().Remove(before)
	defer db.Callback().Create().Remove(after)
	defer func() { unblock(); workers.Wait() }()
	results := make(chan *httptest.ResponseRecorder, 2)
	run := func() {
		defer workers.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		r := httptest.NewRequest(http.MethodPost, "/api/applications/submit-complete-application", strings.NewReader(body))
		r = r.WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, r)
		results <- rec
	}
	workers.Add(1)
	go run()
	select {
	case <-winnerInserted:
	case <-time.After(10 * time.Second):
		unblock()
		t.Fatal("winner did not reserve")
	}
	workers.Add(1)
	go run()
	select {
	case <-secondInsert:
	case <-time.After(10 * time.Second):
		unblock()
		t.Fatal("contender did not enter reservation")
	}
	unblock()
	var one, two *httptest.ResponseRecorder
	select {
	case one = <-results:
	case <-time.After(10 * time.Second):
		t.Fatal("winner did not finish")
	}
	select {
	case two = <-results:
	case <-time.After(10 * time.Second):
		t.Fatal("contender did not finish")
	}
	require.Equal(t, http.StatusOK, one.Code, one.Body.String())
	require.Equal(t, http.StatusOK, two.Code, two.Body.String())
	require.Equal(t, testutil.ParseResponse[services.CompleteApplicationResponse](t, one), testutil.ParseResponse[services.CompleteApplicationResponse](t, two))
	var count int64
	require.NoError(t, db.Model(&models.ApplicationOperation{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Model(&models.Application{}).Where("sender_user_id = ?", getUserID(t, db, "operation_concurrent")).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestApplicationOperation_ResultFailureRollsBackFilesAndReservation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "operation_rollback", "pass123", 1, td.OrgID, td.CompanyID)
	file := uploadDraftFile(t, e, token, "rollback.png", realPNG(t))
	req := operationSubmitRequest(t, db, operationTestKey(time.Now().UTC(), "c"), []int{file.ID})
	before := transactionIntegritySnapshot(t, db)
	name := "test:operation_result_failure"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "application_operations" {
			tx.AddError(errors.New("synthetic result failure"))
		}
	}))
	first := testutil.POST(t, e, "/applications/submit-complete-application", transactionIntegrityJSON(t, req), testutil.AuthHeader(token))
	require.NoError(t, db.Callback().Update().Remove(name))
	require.Equal(t, http.StatusInternalServerError, first.Code, first.Body.String())
	require.Equal(t, before, transactionIntegritySnapshot(t, db))
	var stored models.ApplicationFile
	require.NoError(t, db.First(&stored, file.ID).Error)
	require.Nil(t, stored.ApplicationID)
	var count int64
	require.NoError(t, db.Model(&models.ApplicationOperation{}).Where("key = ?", req.RequestIdempotencyKey).Count(&count).Error)
	require.Zero(t, count)
	retry := testutil.POST(t, e, "/applications/submit-complete-application", transactionIntegrityJSON(t, req), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, retry.Code, retry.Body.String())
}

func TestApplicationOperation_SupplementReplayPrecedesMutationStateAndDuplicateChecks(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	author, token := suppAuthor(t, e, db, "operation_supplement", td.OrgID, td.CompanyID)
	app := suppApp(t, db, td.OrgID, author, "OP-SUPP", models.ConfirmationApproved, models.StatusInWork)
	att := suppAttachment(t, db, app, "cars", "2099-12-31")
	req := services.CreateSupplementRequest{
		ApplicationConfirmationInput: seedApplicationConfirmation(t, db),
		DataApproval:                 true, RequestIdempotencyKey: operationTestKey(time.Now().UTC(), "d"),
		Additions: []services.SupplementAddition{{AttachmentID: att, Vehicles: []services.VehicleInput{{CarNumber: "S700SS777", CarBrand: "Fixture"}}}},
	}
	route := fmt.Sprintf("/applications/%d/supplements", app)
	body := transactionIntegrityJSON(t, req)
	first := testutil.POST(t, e, route, body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	before := transactionIntegritySnapshot(t, db)
	require.NoError(t, db.Model(&models.Application{}).Where("id = ?", app).Update("status", models.StatusCompleted).Error)
	repeat := testutil.POST(t, e, route, body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, repeat.Code, repeat.Body.String())
	require.Equal(t, testutil.ParseResponse[services.CreateSupplementResponse](t, first), testutil.ParseResponse[services.CreateSupplementResponse](t, repeat))
	require.Equal(t, before, transactionIntegritySnapshot(t, db))
	_, other := suppAuthor(t, e, db, "operation_supplement_other", td.OrgID, td.CompanyID)
	denied := testutil.POST(t, e, route, body, testutil.AuthHeader(other))
	require.Equal(t, http.StatusForbidden, denied.Code, denied.Body.String())
}

func TestApplicationOperation_StatusLookupRecoversExpiredMetadataWithoutPayload(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "operation_status", "pass123", 1, td.OrgID, td.CompanyID)
	actor := getUserID(t, db, "operation_status")
	app := suppApp(t, db, td.OrgID, actor, "OP-RECOVERY", models.ConfirmationApproved, models.StatusProcessing)
	key := operationTestKey(time.Now().UTC().Add(-24*time.Hour-30*time.Minute), "e")
	minimal := services.CompleteApplicationResponse{Success: true, Message: "Application created successfully", ApplicationID: app, ApplicationNumber: "OP-RECOVERY"}
	encoded := []byte(transactionIntegrityJSON(t, minimal))
	require.NoError(t, db.Create(&models.ApplicationOperation{OwnerUserID: actor, EffectiveUserID: actor, Scope: "application.submit", Key: key, Fingerprint: strings.Repeat("f", 64), Result: encoded, CreatedAt: time.Now().UTC().Add(-24*time.Hour - 30*time.Minute)}).Error)
	query := transactionIntegrityJSON(t, services.ApplicationOperationStatusRequest{Key: key, Scope: "application.submit"})
	before := transactionIntegritySnapshot(t, db)
	found := testutil.POST(t, e, "/applications/operations/status", query, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, found.Code, found.Body.String())
	var body struct {
		State  string                               `json:"state"`
		Result services.CompleteApplicationResponse `json:"result"`
	}
	body = testutil.ParseResponse[struct {
		State  string                               `json:"state"`
		Result services.CompleteApplicationResponse `json:"result"`
	}](t, found)
	require.Equal(t, "committed", body.State)
	require.Equal(t, minimal, body.Result)
	require.NotContains(t, found.Body.String(), "fingerprint")
	require.NotContains(t, found.Body.String(), "file_ids")
	require.Equal(t, before, transactionIntegritySnapshot(t, db))
	other := testutil.RegisterAndLogin(t, e, "operation_status_other", "pass123", 1, td.OrgID, td.CompanyID)
	absent := testutil.POST(t, e, "/applications/operations/status", query, testutil.AuthHeader(other))
	require.Equal(t, http.StatusOK, absent.Code, absent.Body.String())
	unknown := testutil.ParseResponse[services.ApplicationOperationStatusResponse](t, absent)
	require.Equal(t, "unknown", unknown.State)
	require.Nil(t, unknown.Result)
	require.NotContains(t, absent.Body.String(), "OP-RECOVERY")
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", actor).Update("is_active", false).Error)
	denied := testutil.POST(t, e, "/applications/operations/status", query, testutil.AuthHeader(token))
	require.NotEqual(t, http.StatusOK, denied.Code)
}

func TestApplicationOperation_StatusSupplementHasCurrentOwnerAndPermissionGates(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	actor, token := suppAuthor(t, e, db, "operation_status_supp", td.OrgID, td.CompanyID)
	app := suppApp(t, db, td.OrgID, actor, "OP-STATUS-SUPP", models.ConfirmationApproved, models.StatusCompleted)
	key := operationTestKey(time.Now().UTC(), "f")
	response := services.CreateSupplementResponse{SupplementID: 77, Number: 1, Status: models.SupplementPending, Counts: services.SupplementCounts{Vehicles: 1}}
	scope := fmt.Sprintf("application.supplement:%d", app)
	require.NoError(t, db.Create(&models.ApplicationOperation{OwnerUserID: actor, EffectiveUserID: actor, Scope: scope, Key: key, Fingerprint: strings.Repeat("f", 64), Result: []byte(transactionIntegrityJSON(t, response)), CreatedAt: time.Now().UTC()}).Error)
	body := transactionIntegrityJSON(t, services.ApplicationOperationStatusRequest{Key: key, Scope: scope})
	result := testutil.POST(t, e, "/applications/operations/status", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, result.Code, result.Body.String())
	_, other := suppAuthor(t, e, db, "operation_status_supp_other", td.OrgID, td.CompanyID)
	denied := testutil.POST(t, e, "/applications/operations/status", body, testutil.AuthHeader(other))
	require.Equal(t, http.StatusForbidden, denied.Code)
	noPermission := testutil.RegisterAndLogin(t, e, "operation_status_no_permission", "pass123", 1, td.OrgID, td.CompanyID)
	permissionDenied := testutil.POST(t, e, "/applications/operations/status", body, testutil.AuthHeader(noPermission))
	require.Equal(t, http.StatusForbidden, permissionDenied.Code)
	malformed := testutil.POST(t, e, "/applications/operations/status", transactionIntegrityJSON(t, services.ApplicationOperationStatusRequest{Key: key, Scope: "other.scope"}), testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, malformed.Code)
}
