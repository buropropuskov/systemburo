package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type periodRoutes2665World struct {
	e                                                                              *echo.Echo
	db                                                                             *gorm.DB
	actor, otherActor, application, employee, car, peopleAttachment, carAttachment int
	token                                                                          string
	clock                                                                          time.Time
}

// One real application/router stack per scenario; no direct handler calls or
// fabricated JWT contexts. RegisterUser hashes the password, LoginUser uses /login.
func setupPeriodRoutes2665World(t *testing.T, group, deniedAdmin bool) periodRoutes2665World {
	t.Helper()
	e, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	seed := testutil.SeedTestData(t, db)
	const password = "period_route2665_password_long_enough!"
	testutil.RegisterUser(t, e, "period_route2665_actor", password, 1, seed.OrgID, 0)
	testutil.RegisterUser(t, e, "period_route2665_other", password, 1, seed.OrgID, 0)
	var actor, other models.User
	require.NoError(t, db.Where("username=?", "period_route2665_actor").First(&actor).Error)
	require.NoError(t, db.Where("username=?", "period_route2665_other").First(&other).Error)
	require.NoError(t, db.Model(&other).Update("is_super_admin", true).Error)
	if deniedAdmin {
		require.NoError(t, db.Model(&actor).Update("is_admin", true).Error)
	}
	for _, key := range []string{services.KeyDetailPeriodChange, services.KeyApplicationPeriodChange} {
		if deniedAdmin {
			testutil.DenyPermission(t, actor.ID, key)
		} else {
			testutil.GrantPermission(t, actor.ID, key)
		}
	}
	now := time.Now().In(services.MoscowLocation())
	status, confirmation := models.StatusInWork, models.ConfirmationApproved
	if group {
		status = models.StatusProcessing
	}
	app := models.Application{OrganizationID: seed.OrgID, SenderUserID: actor.ID, Status: &status, Confirmation: &confirmation, SendingDatetime: &now}
	require.NoError(t, db.Create(&app).Error)
	from, to, start, end, active := now.AddDate(0, 0, -1).Format("2006-01-02"), now.AddDate(0, 0, 2).Format("2006-01-02"), "08:00:00", "22:00:00", 1
	people := models.Attachment{ApplicationID: &app.ID, AttachmentType: "people", Status: &active, EntryDateFrom: &from, EntryDateTo: &to, EntryTimeFrom: &start, EntryTimeTo: &end}
	cars := people
	cars.AttachmentType = "cars"
	require.NoError(t, db.Create(&people).Error)
	require.NoError(t, db.Create(&cars).Error)
	employee := models.Employee{AttachmentID: &people.ID, Status: &active}
	plate := "ROUTE2665"
	car := models.Car{AttachmentID: cars.ID, CarNumber: &plate, Status: &active, EntryDateFrom: &from, EntryDateTo: &to, EntryTimeFrom: &start, EntryTimeTo: &end}
	require.NoError(t, db.Create(&employee).Error)
	require.NoError(t, db.Create(&car).Error)
	token, _ := testutil.LoginUser(t, e, actor.Username, password)
	require.NotEmpty(t, token)
	return periodRoutes2665World{e: e, db: db, actor: actor.ID, otherActor: other.ID, application: app.ID, employee: employee.ID, car: car.ID, peopleAttachment: people.ID, carAttachment: cars.ID, token: token, clock: now}
}

func (w periodRoutes2665World) period() *services.EntityPeriodInput {
	return &services.EntityPeriodInput{EntryDateFrom: w.clock.AddDate(0, 0, -1).Format("2006-01-02"), EntryDateTo: w.clock.AddDate(0, 0, 7).Format("2006-01-02"), EntryTimeFrom: "08:00", EntryTimeTo: "22:00"}
}

func periodRoutes2665Body(t *testing.T, value any, spoofActor int) string {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	body["actor_id"], body["user_id"], body["actor_user_id"] = spoofActor, spoofActor, spoofActor
	raw, err = json.Marshal(body)
	require.NoError(t, err)
	return string(raw)
}

func periodRoutes2665Envelope(t *testing.T, rec *httptest.ResponseRecorder, status int, result any) {
	t.Helper()
	require.Equal(t, status, rec.Code, rec.Body.String())
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   string          `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, status == http.StatusOK, envelope.Success)
	if status == http.StatusOK {
		require.Empty(t, envelope.Error)
		require.NotEmpty(t, envelope.Data)
		if result != nil {
			require.NoError(t, json.Unmarshal(envelope.Data, result))
		}
	} else {
		require.NotEmpty(t, envelope.Error)
		require.Empty(t, envelope.Data)
	}
}

func (w periodRoutes2665World) entityPath(kind services.ElementKind) (string, int) {
	id := w.employee
	if kind == services.ElementCar {
		id = w.car
	}
	return fmt.Sprintf("/%s/%d/period", kind, id), id
}

func (w periodRoutes2665World) snapshot(t *testing.T) string {
	t.Helper()
	var value string
	require.NoError(t, w.db.Raw(`SELECT jsonb_build_object(
		'application',(SELECT to_jsonb(a) FROM applications a WHERE id=?),
		'attachments',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM attachments a WHERE application_id=?),
		'employee',(SELECT to_jsonb(e) FROM employees e WHERE id=?),
		'car',(SELECT to_jsonb(c) FROM cars c WHERE id=?)
	)::text`, w.application, w.application, w.employee, w.car).Scan(&value).Error)
	return value
}

func TestEntityPeriod2665HTTPAnonymousAndManagedDeny(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(fmt.Sprintf("denied_admin_%t", denied), func(t *testing.T) {
			w := setupPeriodRoutes2665World(t, true, denied)
			before := w.snapshot(t)
			var headers http.Header
			status := http.StatusUnauthorized
			if denied {
				headers, status = testutil.AuthHeader(w.token), http.StatusForbidden
			}
			for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
				path, _ := w.entityPath(kind)
				periodRoutes2665Envelope(t, testutil.GET(t, w.e, path, headers), status, nil)
				request := services.ChangeEntityPeriodRequest{PeriodMode: models.PeriodIndividual, Period: w.period(), Reason: "HTTP permission test", ExpectedRevision: "untrusted"}
				periodRoutes2665Envelope(t, testutil.PUT(t, w.e, path, periodRoutes2665Body(t, request, w.otherActor), headers), status, nil)
			}
			path := fmt.Sprintf("/applications/%d/attachment-period", w.application)
			request := services.ChangeAttachmentPeriodRequest{AttachmentIDs: []int{w.peopleAttachment, w.carAttachment}, Period: w.period(), Reason: "HTTP permission test", ExpectedRevision: "untrusted"}
			body := periodRoutes2665Body(t, request, w.otherActor)
			periodRoutes2665Envelope(t, testutil.POST(t, w.e, path+"/preview", body, headers), status, nil)
			periodRoutes2665Envelope(t, testutil.PUT(t, w.e, path, body, headers), status, nil)
			var count int64
			require.NoError(t, w.db.Model(&models.AuditLog{}).Where("action=?", models.AuditActionDatesChanged).Count(&count).Error)
			require.Zero(t, count)
			require.Equal(t, before, w.snapshot(t))
		})
	}
}

func TestEntityPeriod2665HTTPEntityRoundtripAndJWTActor(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPeriodRoutes2665World(t, false, false)
			path, id := w.entityPath(kind)
			headers := testutil.AuthHeader(w.token)
			var inspect, changed, reloaded services.EntityPeriodCommandResult
			periodRoutes2665Envelope(t, testutil.GET(t, w.e, path, headers), http.StatusOK, &inspect)
			require.Equal(t, id, inspect.EntityID)
			require.NotEmpty(t, inspect.Revision)
			request := services.ChangeEntityPeriodRequest{PeriodMode: models.PeriodIndividual, Period: w.period(), Reason: "HTTP entity change", ExpectedRevision: inspect.Revision}
			body := periodRoutes2665Body(t, request, w.otherActor)
			periodRoutes2665Envelope(t, testutil.PUT(t, w.e, path, body, headers), http.StatusOK, &changed)
			require.Equal(t, models.PeriodIndividual, changed.PeriodMode)
			require.False(t, changed.ApprovalsReset)
			require.Equal(t, request.Period.EntryDateTo, *changed.Effective.EntryDateTo)
			periodRoutes2665Envelope(t, testutil.GET(t, w.e, path, headers), http.StatusOK, &reloaded)
			require.Equal(t, changed.Revision, reloaded.Revision)
			beforeStale := w.snapshot(t)
			periodRoutes2665Envelope(t, testutil.PUT(t, w.e, path, body, headers), http.StatusConflict, nil)
			require.Equal(t, beforeStale, w.snapshot(t))
			var audits []models.AuditLog
			entityType := models.AuditEntityEmployee
			if kind == services.ElementCar {
				entityType = models.AuditEntityCar
			}
			require.NoError(t, w.db.Where("entity_type=? AND entity_id=? AND action=?", entityType, id, models.AuditActionDatesChanged).Find(&audits).Error)
			require.Len(t, audits, 1)
			require.NotNil(t, audits[0].ActorUserID)
			require.Equal(t, w.actor, *audits[0].ActorUserID, "body identity must never replace the login/JWT actor")
		})
	}
}

func TestEntityPeriod2665HTTPGroupRoundtripAndForeignSelection(t *testing.T) {
	for _, foreignKind := range []services.ElementKind{"", services.ElementEmployee, services.ElementCar} {
		t.Run("foreign_"+string(foreignKind), func(t *testing.T) {
			w := setupPeriodRoutes2665World(t, true, false)
			path := fmt.Sprintf("/applications/%d/attachment-period", w.application)
			headers := testutil.AuthHeader(w.token)
			request := services.ChangeAttachmentPeriodRequest{AttachmentIDs: []int{w.peopleAttachment, w.carAttachment}, Period: w.period(), Reason: "HTTP selected attachments", IndividualPolicy: services.IndividualPolicyPreserve}
			if foreignKind != "" {
				var app models.Application
				require.NoError(t, w.db.First(&app, w.application).Error)
				foreignApp := models.Application{OrganizationID: app.OrganizationID, SenderUserID: w.otherActor, Status: app.Status, Confirmation: app.Confirmation}
				require.NoError(t, w.db.Create(&foreignApp).Error)
				attachmentType, active := "people", 1
				if foreignKind == services.ElementCar {
					attachmentType = "cars"
				}
				foreign := models.Attachment{ApplicationID: &foreignApp.ID, AttachmentType: attachmentType, Status: &active}
				require.NoError(t, w.db.Create(&foreign).Error)
				request.AttachmentIDs = append(request.AttachmentIDs, foreign.ID)
				before := w.snapshot(t)
				body := periodRoutes2665Body(t, request, w.otherActor)
				periodRoutes2665Envelope(t, testutil.POST(t, w.e, path+"/preview", body, headers), http.StatusForbidden, nil)
				// Valid SHA-256 shape reaches the foreign-resource gate. The
				// membership denial must precede checking revision contents.
				request.ExpectedRevision = "0000000000000000000000000000000000000000000000000000000000000000"
				periodRoutes2665Envelope(t, testutil.PUT(t, w.e, path, periodRoutes2665Body(t, request, w.otherActor), headers), http.StatusForbidden, nil)
				var count int64
				require.NoError(t, w.db.Model(&models.AuditLog{}).Where("action=?", models.AuditActionDatesChanged).Count(&count).Error)
				require.Zero(t, count)
				require.Equal(t, before, w.snapshot(t), "foreign selection may not partially write the valid selected attachments")
				return
			}
			var preview, changed services.AttachmentPeriodCommandResult
			periodRoutes2665Envelope(t, testutil.POST(t, w.e, path+"/preview", periodRoutes2665Body(t, request, w.otherActor), headers), http.StatusOK, &preview)
			require.NotEmpty(t, preview.Revision)
			require.ElementsMatch(t, request.AttachmentIDs, preview.AttachmentIDs)
			request.ExpectedRevision = preview.Revision
			body := periodRoutes2665Body(t, request, w.otherActor)
			periodRoutes2665Envelope(t, testutil.PUT(t, w.e, path, body, headers), http.StatusOK, &changed)
			require.False(t, changed.ApprovalsReset)
			require.Equal(t, []int{w.employee}, changed.ChangedEmployeeIDs)
			require.Equal(t, []int{w.car}, changed.ChangedCarIDs)
			for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
				_, id := w.entityPath(kind)
				attachmentID := w.peopleAttachment
				if kind == services.ElementCar {
					attachmentID = w.carAttachment
				}
				var end string
				require.NoError(t, w.db.Table("attachments").Select("entry_date_to").Where("id=?", attachmentID).Scan(&end).Error)
				require.Equal(t, request.Period.EntryDateTo, end)
				var mode models.PeriodMode
				require.NoError(t, w.db.Table(string(kind)).Select("period_mode").Where("id=?", id).Scan(&mode).Error)
				require.Equal(t, models.PeriodInherit, mode)
			}
			beforeStale := w.snapshot(t)
			periodRoutes2665Envelope(t, testutil.PUT(t, w.e, path, body, headers), http.StatusConflict, nil)
			require.Equal(t, beforeStale, w.snapshot(t))
			var audits []models.AuditLog
			require.NoError(t, w.db.Where("entity_type=? AND entity_id=? AND action=?", models.AuditEntityApplication, w.application, models.AuditActionDatesChanged).Find(&audits).Error)
			require.Len(t, audits, 1)
			require.NotNil(t, audits[0].ActorUserID)
			require.Equal(t, w.actor, *audits[0].ActorUserID)
		})
	}
}
