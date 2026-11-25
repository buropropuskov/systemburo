package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/entityarchive"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestApplicationNumbersSurvivePurgeAcrossSubmissionPaths(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	data := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "number-owner", "pass123", 1, data.OrgID, data.CompanyID)
	templateID := seedUniqueAttachment(t, db, "cars", "number_cars", "Транспорт")
	body := withApplicationConfirmation(t, db, fmt.Sprintf(`{"organization_id":%d,"data_approval":true}`, data.OrgID))
	seen := make(map[string]bool)
	create := func() int {
		t.Helper()
		rec := testutil.POST(t, e, "/applications", body, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		app := testutil.ParseResponse[services.ApplicationCreateResponse](t, rec)
		require.NotEmpty(t, app.ApplicationNumber)
		require.False(t, seen[app.ApplicationNumber], "повтор номера %s", app.ApplicationNumber)
		seen[app.ApplicationNumber] = true
		return app.ApplicationID
	}
	purge := func(id int) {
		t.Helper()
		_, err := entityarchive.PurgeApplication(context.Background(), db, services.NewAuditRecorder(db),
			id, entityarchive.DestructionOptions{Basis: entityarchive.BasisOperator, Apply: true})
		require.NoError(t, err)
	}
	create()
	middle := create()
	last := submitCompleteApplication(t, db, e, token, "Test Organization", templateID)
	var number string
	require.NoError(t, db.Raw("SELECT application_number FROM applications WHERE id = ?", last).Row().Scan(&number))
	require.NotEmpty(t, number)
	require.False(t, seen[number])
	seen[number] = true
	purge(middle)
	newLast := create()
	purge(newLast)
	create()
}

func TestGeneratedApplicationNumberSharesSendingTimeAndAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	data := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "number-clock", "pass123", 1, data.OrgID, data.CompanyID)
	templateID := seedUniqueAttachment(t, db, "items", "number_items", "Имущество")
	body := withApplicationConfirmation(t, db, fmt.Sprintf(`{
		"organization_id": %d, "data_approval": true, "generatedSendingTime": "2030-12-31T21:00:00Z",
		"responsible_person":"Учебный сотрудник", "contact_phone":"+79000000000",
		"attachments": [{"attachment_type":"items","attachment_name":"number_items","attachment_display_name":"Имущество",
			"unique_attachment_id":%d, "entry_date_from":"2031-01-01","entry_date_to":"2031-01-02",
			"data":{"items":[{"name":"Коробка","count":1,"order_index":1}]}}]}`, data.OrgID, templateID))
	var req services.CompleteApplicationRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))
	recorder := services.NewAuditRecorder(db)
	svc := services.NewApplicationService(db, services.NewPermissionService(db), services.NewNotificationService(db),
		services.NewVehicleBlacklistService(db, recorder), services.NewPersonBlacklistService(db, recorder), recorder)
	sentAt := time.Date(2030, 12, 31, 21, 0, 0, 0, time.UTC)
	app, err := services.SubmitGeneratedApplication(context.Background(), svc, "number-clock", req, sentAt)
	require.NoError(t, err)
	require.Equal(t, "№ 20310101/001", app.ApplicationNumber)
	var storedTime time.Time
	var number, history string
	require.NoError(t, db.Raw(`SELECT a.sending_datetime, a.application_number, l.details->>'new_value'
		FROM applications a JOIN audit_log l ON l.entity_type = 'application' AND l.entity_id = a.id AND l.action = 'create'
		WHERE a.id = ?`, app.ApplicationID).Row().Scan(&storedTime, &number, &history))
	require.True(t, sentAt.Equal(storedTime))
	require.Equal(t, app.ApplicationNumber, number)
	require.Equal(t, number, history)

	// HTTP не принимает учебное время. Проверяем по моментам начала и конца
	// запроса, в том числе если он был обработан во время смены суток.
	start := time.Now()
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	end := time.Now()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	ordinary := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec)
	require.NoError(t, db.Raw("SELECT sending_datetime FROM applications WHERE id = ?", ordinary.ApplicationID).Row().Scan(&storedTime))
	require.True(t, !storedTime.Before(start.Truncate(time.Microsecond)) && !storedTime.After(end.Add(time.Microsecond)))
}
