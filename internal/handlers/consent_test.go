package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsentEndpoints_Unauthorized(t *testing.T) {
	e, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	rec := testutil.POST(t, e, "/consents", `{"consent_type":"pd_processing"}`, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGrantConsent_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "consent_user", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/consents", `{"consent_type":"pd_processing"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseMap(t, rec)
	assert.Equal(t, "pd_processing", resp["consent_type"])
	assert.Equal(t, true, resp["granted"])
}

func TestRevokeConsent_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "revoke_user", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	testutil.POST(t, e, "/consents", `{"consent_type":"pd_processing"}`, testutil.AuthHeader(token))

	rec := testutil.DELETE(t, e, "/consents/pd_processing", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCheckConsent_Active(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "check_user", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	testutil.POST(t, e, "/consents", `{"consent_type":"pd_processing"}`, testutil.AuthHeader(token))

	rec := testutil.GET(t, e, "/consents/check/pd_processing", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseMap(t, rec)
	assert.Equal(t, true, resp["active"])
}

func TestListConsents_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "list_user", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	testutil.POST(t, e, "/consents", `{"consent_type":"pd_processing"}`, testutil.AuthHeader(token))

	rec := testutil.GET(t, e, "/consents", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
}
