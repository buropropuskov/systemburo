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

func TestDoubleGrant_CreatesTwoConsents(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "double_grant_user", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec1 := testutil.POST(t, e, "/consents", `{"consent_type":"pd_processing"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec1.Code)

	rec2 := testutil.POST(t, e, "/consents", `{"consent_type":"pd_processing"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec2.Code)

	recList := testutil.GET(t, e, "/consents", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, recList.Code)
	items := testutil.ParseSlice(t, recList)
	assert.Len(t, items, 2)

	recCheck := testutil.GET(t, e, "/consents/check/pd_processing", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, recCheck.Code)
	checkResp := testutil.ParseMap(t, recCheck)
	assert.Equal(t, true, checkResp["active"])
}

func TestHasActive_FalseAfterRevoke(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "revoke_check_user", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/consents", `{"consent_type":"pd_processing"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	recCheckBefore := testutil.GET(t, e, "/consents/check/pd_processing", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, recCheckBefore.Code)
	assert.Equal(t, true, testutil.ParseMap(t, recCheckBefore)["active"])

	recRevoke := testutil.DELETE(t, e, "/consents/pd_processing", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, recRevoke.Code)

	recCheckAfter := testutil.GET(t, e, "/consents/check/pd_processing", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, recCheckAfter.Code)
	assert.Equal(t, false, testutil.ParseMap(t, recCheckAfter)["active"])

	recList := testutil.GET(t, e, "/consents", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, recList.Code)
	items := testutil.ParseSlice(t, recList)
	assert.Len(t, items, 1, "revoked consent should still be in the list")
	assert.Equal(t, false, items[0]["granted"])
}

func TestRevokeNonExistent_Returns404(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "no_consent_user", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.DELETE(t, e, "/consents/pd_processing", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGrantConsent_InvalidType(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "invalid_type_user", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/consents", `{"consent_type":"invalid_type"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListConsents_EmptyForNewUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "empty_list_user", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/consents", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	items := testutil.ParseSlice(t, rec)
	assert.Empty(t, items)
}

func TestGrantRevokeRegrant_Lifecycle(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "lifecycle_user", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	// Grant
	rec := testutil.POST(t, e, "/consents", `{"consent_type":"pd_processing"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	// Revoke
	rec = testutil.DELETE(t, e, "/consents/pd_processing", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	// Check → false
	rec = testutil.GET(t, e, "/consents/check/pd_processing", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, false, testutil.ParseMap(t, rec)["active"])

	// Re-grant
	rec = testutil.POST(t, e, "/consents", `{"consent_type":"pd_processing"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	// Check → true
	rec = testutil.GET(t, e, "/consents/check/pd_processing", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, true, testutil.ParseMap(t, rec)["active"])

	// List → 3 records (grant, revoke updates first, re-grant creates second + original)
	rec = testutil.GET(t, e, "/consents", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	items := testutil.ParseSlice(t, rec)
	assert.Len(t, items, 2, "should have 2 consent records: original (revoked) + re-granted")
}
