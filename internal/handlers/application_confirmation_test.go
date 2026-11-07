package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func seedApplicationConfirmation(t *testing.T, db *gorm.DB) services.ApplicationConfirmationInput {
	t.Helper()
	require.NoError(t, db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoNothing: true}).
		Create(&models.SystemSetting{Key: "legal.pd_consent_text", Type: "html", Value: "<p>Документ для подачи</p>"}).Error)
	revision, err := services.CurrentApplicationConfirmationRevision(context.Background(), db)
	require.NoError(t, err)
	return services.ApplicationConfirmationInput{DocumentRevision: revision}
}

func withApplicationConfirmation(t *testing.T, db *gorm.DB, body string) string {
	t.Helper()
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(body), &payload))
	revision, err := json.Marshal(seedApplicationConfirmation(t, db).DocumentRevision)
	require.NoError(t, err)
	payload["document_revision"] = revision
	result, err := json.Marshal(payload)
	require.NoError(t, err)
	return string(result)
}

func withSupplementConfirmation(t *testing.T, db *gorm.DB, body string) string {
	t.Helper()
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(withApplicationConfirmation(t, db, body)), &payload))
	payload["data_approval"] = json.RawMessage("true")
	result, err := json.Marshal(payload)
	require.NoError(t, err)
	return string(result)
}

func TestApplicationConfirmationSubmit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	actor, token := suppVoteUser(t, e, db, "confirmation_sender", td.OrgID, td.CompanyID)
	require.NoError(t, db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"})}).Create(&models.SystemSetting{
		Key: "legal.pd_consent_text", Type: "html", Value: "<p>Документ для подачи</p>",
	}).Error)
	revision, err := services.CurrentApplicationConfirmationRevision(context.Background(), db)
	require.NoError(t, err)
	req := operationSubmitRequest(t, db, operationTestKey(time.Now().UTC(), "c"), nil)
	req.DocumentRevision = models.PDConsentRevision{}
	path, headers := "/applications/submit-complete-application", testutil.AuthHeader(token)
	rejected := testutil.POST(t, e, path, transactionIntegrityJSON(t, req), headers)
	require.Equal(t, http.StatusConflict, rejected.Code, rejected.Body.String())
	var count int64
	require.NoError(t, db.Model(&models.Application{}).Where("sender_user_id = ?", actor).Count(&count).Error)
	require.Zero(t, count)

	req.DocumentRevision = revision
	accepted := testutil.POST(t, e, path, transactionIntegrityJSON(t, req), headers)
	require.Equal(t, http.StatusOK, accepted.Code, accepted.Body.String())
	app := testutil.ParseResponse[services.CompleteApplicationResponse](t, accepted)
	var confirmation models.ApplicationConfirmation
	require.NoError(t, db.Preload("Document").Where("application_id = ?", app.ApplicationID).Take(&confirmation).Error)
	require.Nil(t, confirmation.SupplementID)
	require.Equal(t, &actor, confirmation.ActorUserID)
	require.Equal(t, revision.Hash, confirmation.DocumentHash)
	require.Equal(t, "<p>Документ для подачи</p>", confirmation.Document.Text)
	var carID int
	require.NoError(t, db.Raw("SELECT c.id FROM cars c JOIN attachments a ON a.id = c.attachment_id WHERE a.application_id = ?", app.ApplicationID).Scan(&carID).Error)
	require.NotZero(t, carID)
	require.Equal(t, []models.ApplicationConfirmedRow{{Kind: "car", ID: carID}}, confirmation.Rows)
}

func TestApplicationConfirmationSupplement(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	actor, token := suppVoteUser(t, e, db, "confirmation_author", td.OrgID, td.CompanyID)
	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID: actor, PermissionKey: services.KeyActionSupplementApplication, Value: "allow", GrantedAt: time.Now(),
	}).Error)
	app := suppApp(t, db, td.OrgID, actor, "CONFIRMATION-1", models.ConfirmationApproved, models.StatusProcessing)
	attachment := suppAttachment(t, db, app, "cars", time.Now().UTC().AddDate(0, 1, 0).Format("2006-01-02"))
	setText := func(text string) {
		t.Helper()
		require.NoError(t, db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value"})}).Create(&models.SystemSetting{
			Key: "legal.pd_consent_text", Type: "html", Value: text,
		}).Error)
	}
	setText("<p>Первая редакция</p>")
	previous, err := services.CurrentApplicationConfirmationRevision(context.Background(), db)
	require.NoError(t, err)
	setText("<p>Обновлённый документ</p>")
	current, err := services.CurrentApplicationConfirmationRevision(context.Background(), db)
	require.NoError(t, err)
	require.Equal(t, previous.Version, current.Version)
	require.NotEqual(t, previous.Hash, current.Hash)

	commandKey := fmt.Sprintf("v1.%d.%032x", time.Now().UnixMilli(), 1)
	post := func(approval bool, revision models.PDConsentRevision, status int) {
		t.Helper()
		body := fmt.Sprintf(`{"request_idempotency_key":%q,"data_approval":%t,"document_revision":{"version":%d,"hash":%q},"additions":[{"attachment_id":%d,"vehicles":[{"car_number":"Б222ББ777","car_brand":"ГАЗель","unload_place":"Ворота 1","unload_places":[],"passage_tables":[]}]}]}`,
			commandKey, approval, revision.Version, revision.Hash, attachment)
		response := testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements", app), body, testutil.AuthHeader(token))
		require.Equal(t, status, response.Code, response.Body.String())
		if status == http.StatusConflict {
			require.Equal(t, "true", response.Header().Get("X-Document-Revision-Changed"))
		}
	}
	post(false, current, http.StatusBadRequest)
	post(true, previous, http.StatusConflict)
	var count int64
	require.NoError(t, db.Model(&models.ApplicationSupplement{}).Where("application_id = ?", app).Count(&count).Error)
	require.Zero(t, count)
	post(true, current, http.StatusOK)
	setText("<p>Следующий текст</p>")
	post(true, current, http.StatusOK)
	require.NoError(t, db.Model(&models.ApplicationConfirmation{}).Where("application_id = ?", app).Count(&count).Error)
	require.EqualValues(t, 1, count)

	var confirmation models.ApplicationConfirmation
	require.NoError(t, db.Preload("Document").Where("application_id = ?", app).Take(&confirmation).Error)
	require.NotNil(t, confirmation.SupplementID)
	require.Equal(t, &actor, confirmation.ActorUserID)
	require.Equal(t, current.Hash, confirmation.DocumentHash)
	require.Equal(t, "<p>Обновлённый документ</p>", confirmation.Document.Text)
	var car models.Car
	require.NoError(t, db.Where("supplement_id = ?", *confirmation.SupplementID).Take(&car).Error)
	require.Equal(t, []models.ApplicationConfirmedRow{{Kind: "car", ID: car.ID}}, confirmation.Rows)
}
