package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestDocumentAtPostRuleAndSubmission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	admin := testutil.RegisterAdmin(t, e, 0, 0)
	org := models.Organization{Name: "Учебная организация"}
	require.NoError(t, db.Create(&org).Error)
	testutil.RegisterUser(t, e, "documentcheck", "Password123", 1, org.ID, 0)
	applicant, _ := testutil.LoginUser(t, e, "documentcheck", "Password123")
	testutil.RegisterUser(t, e, "documentpeer", "Password123", 1, org.ID, 0)
	peer, _ := testutil.LoginUser(t, e, "documentpeer", "Password123")
	userID := getUserID(t, db, "documentcheck")
	citizenship := models.Citizenship{Name: "Россия"}
	require.NoError(t, db.Create(&citizenship).Error)
	template := seedUniqueAttachment(t, db, "people", "doc_at_post", "Проведение работ")
	other := seedUniqueAttachment(t, db, "people", "doc_at_post_other", "Другой вид")
	rule := fmt.Sprintf("/users/%d/document-rule/%d", userID, template)
	read := fmt.Sprintf("/attachments/%d/document-at-post", template)

	for _, actor := range []struct {
		token string
		code  int
	}{{applicant, http.StatusForbidden}, {admin, http.StatusOK}} {
		rec := testutil.PUT(t, e, rule, `{"enabled":true}`, testutil.AuthHeader(actor.token))
		require.Equal(t, actor.code, rec.Code, rec.Body.String())
	}
	rec := testutil.GET(t, e, read, testutil.AuthHeader(applicant))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.True(t, testutil.ParseResponse[map[string]bool](t, rec)["allowed"])
	legacy := models.AttachmentDocumentRule{UniqueAttachmentID: template, Directory: "organization", DirectoryID: org.ID}
	require.NoError(t, db.Create(&legacy).Error)
	rec = testutil.GET(t, e, read, testutil.AuthHeader(peer))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.False(t, testutil.ParseResponse[map[string]bool](t, rec)["allowed"])

	config := fmt.Sprintf("/attachments/%d/field-config", template)
	rec = testutil.PUT(t, e, config, `{"base":[{"key":"passport","visible":true,"required":true}]}`, testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	for _, tc := range []struct {
		name     string
		template int
		atPost   bool
		passport string
		code     int
		token    string
	}{
		{"other-template", other, true, "", http.StatusBadRequest, applicant},
		{"contradictory-payload", template, true, "учебный номер", http.StatusBadRequest, applicant},
		{"required-passport", template, false, "", http.StatusBadRequest, applicant},
		{"free-text", template, false, "-", http.StatusOK, applicant},
		{"document-at-post", template, true, "", http.StatusOK, applicant},
		{"same-organization-other-account", template, true, "", http.StatusBadRequest, peer},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{
				"message": %q, "organization_id": %d, "responsible_person": "Учебный заявитель",
				"contact_phone": "+79001234567", "data_approval": true,
				"attachments": [{"attachment_type":"people","unique_attachment_id":%d,
				"attachment_name":"doc_at_post","attachment_display_name":"Проведение работ",
				"entry_date_from":"2026-01-01","entry_date_to":"2099-12-31",
				"entry_time_from":"08:00","entry_time_to":"18:00",
				"data":{"employees":[{"last_name":"Учебный","first_name":"Сотрудник",
				"citizenship_id":%d,"passport_series_number":%q,"document_at_post":%t}]}}]
			}`, tc.name, org.ID, tc.template, citizenship.ID, tc.passport, tc.atPost)
			rec := testutil.POST(t, e, "/applications/submit-complete-application", withApplicationConfirmation(t, db, body), testutil.AuthHeader(tc.token))
			require.Equal(t, tc.code, rec.Code, rec.Body.String())
			if tc.code != http.StatusOK {
				return
			}
			appID := submittedAppID(t, rec)
			var row struct {
				DocumentAtPost        bool
				DocumentAtPostAllowed bool
				PassportSeriesNumber  *string
			}
			require.NoError(t, db.Raw(`SELECT e.document_at_post, a.document_at_post_allowed, e.passport_series_number
				FROM employees e JOIN attachments a ON a.id = e.attachment_id WHERE a.application_id = ?`, appID).Scan(&row).Error)
			require.True(t, row.DocumentAtPostAllowed)
			require.Equal(t, tc.atPost, row.DocumentAtPost)
			if tc.atPost {
				require.Nil(t, row.PassportSeriesNumber)
			} else {
				require.NotNil(t, row.PassportSeriesNumber)
			}
		})
	}
	rec = testutil.PUT(t, e, rule, `{"enabled":false}`, testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = testutil.GET(t, e, read, testutil.AuthHeader(applicant))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.False(t, testutil.ParseResponse[map[string]bool](t, rec)["allowed"])
	var preserved int64
	require.NoError(t, db.Model(&models.Employee{}).Where("document_at_post").Count(&preserved).Error)
	require.EqualValues(t, 1, preserved)
}
