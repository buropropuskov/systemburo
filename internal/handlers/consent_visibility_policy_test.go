package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestConsentVisibilityCrossSurface(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seed := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, seed.OrgID, seed.CompanyID)
	username := "visibility_http_subject"
	user := testutil.RegisterAndLogin(t, e, username, "password123456789012345678901234", 1, seed.OrgID, seed.CompanyID)
	id := setUserName(t, db, username, "VisibilityName", "Subject", "Fixture")
	alias := "VisibilityName Subject Fixture"
	require.NoError(t, db.Create(&models.ApplicationApprover{UserID: id, DisplayName: &alias}).Error)
	enableConsent(t, e, admin, "<p>Consent</p>")
	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, displayedConsentBody(t, e, user), testutil.AuthHeader(user)).Code)
	require.Equal(t, http.StatusOK, testutil.DELETE(t, e, "/consents/pd_processing", testutil.AuthHeader(user)).Code)
	check := func(t *testing.T) {
		t.Helper()
		r := testutil.GET(t, e, "/users/all?include_archived=true", testutil.AuthHeader(admin))
		require.Equal(t, http.StatusOK, r.Code, r.Body.String())
		found := false
		for _, row := range testutil.ParseResponse[[]models.UserInfoResponse](t, r) {
			if row.ID == id {
				found = true
				require.Nil(t, row.LastName)
				require.True(t, row.PDHidden)
			}
		}
		require.True(t, found)
		r = testutil.GET(t, e, "/application-approvers", testutil.AuthHeader(admin))
		require.Equal(t, http.StatusOK, r.Code, r.Body.String())
		rows := testutil.ParseResponse[[]models.ApplicationApproverWithUser](t, r)
		require.Len(t, rows, 1)
		require.Nil(t, rows[0].LastName)
		require.Equal(t, "@"+username, *rows[0].DisplayName)
	}
	t.Run("active", check)
	require.Equal(t, http.StatusOK, testutil.DELETE(t, e, "/users/"+username, testutil.AuthHeader(admin)).Code)
	t.Run("archived", check)
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, pdConsentPath+"/required", `{"required":false}`, testutil.AuthHeader(admin)).Code)
	t.Run("prompt-disabled", check)
}
