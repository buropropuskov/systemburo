package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettings_GetAll_Unauthorized(t *testing.T) {
	e, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	rec := testutil.GET(t, e, "/settings", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSettings_GetAll_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "regular_user", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/settings", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestSettings_GetAll_Admin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/settings", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	data := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(data), 6)
}

func TestSettings_Update_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.PUT(t, e, "/settings/upload.max_file_size", `{"value":"5242880"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestSettings_Update_InvalidKey(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.PUT(t, e, "/settings/nonexistent.key", `{"value":"test"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSettings_Update_InvalidValue(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.PUT(t, e, "/settings/upload.max_file_size", `{"value":"not-a-number"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSettings_GetUploadSettings_AuthUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "regular", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/settings/upload", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	resp := testutil.ParseMap(t, rec)
	assert.NotNil(t, resp["max_file_size"])
	assert.NotNil(t, resp["allowed_image_types"])
}
