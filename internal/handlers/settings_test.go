package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
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

func TestSettings_GetNotifications_ReturnsDurations(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	// Доступно любому авторизованному (не только супер-админу).
	token := testutil.RegisterAndLogin(t, e, "notif_reader", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/settings/notifications", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	resp := testutil.ParseMap(t, rec)
	assert.Equal(t, float64(10), resp["delete_duration"], "дефолт длительности удаления - 10 сек")
	assert.Equal(t, float64(5), resp["restore_duration"], "дефолт длительности восстановления - 5 сек")
}

func TestSettings_UpdateNotificationDuration_ValidPersists(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/settings/notifications.delete_duration", `{"value":"15"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = testutil.PUT(t, e, "/settings/notifications.restore_duration", `{"value":"3"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// GET отражает сохранённые значения.
	rec = testutil.GET(t, e, "/settings/notifications", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	resp := testutil.ParseMap(t, rec)
	assert.Equal(t, float64(15), resp["delete_duration"])
	assert.Equal(t, float64(3), resp["restore_duration"])
}

func TestSettings_UpdateNotificationDuration_OutOfRangeRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	for _, v := range []string{"2", "61", "abc"} {
		rec := testutil.PUT(t, e, "/settings/notifications.delete_duration", `{"value":"`+v+`"}`, testutil.AuthHeader(token))
		assert.Equal(t, http.StatusBadRequest, rec.Code, "значение %q должно отклоняться (диапазон 3-60)", v)
	}
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

func TestGetPasswordPolicy(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "policy_reader", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/settings/password-policy", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	policy := testutil.ParseResponse[models.PasswordPolicy](t, rec)
	assert.Equal(t, 8, policy.MinLength)
	assert.True(t, policy.RequireDigit)
}

func TestSettings_PasswordPolicyValidation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// min_length вне диапазона -> 400
	rec := testutil.PUT(t, e, "/settings/password.min_length", `{"value":"3"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// валидный min_length -> 200
	rec = testutil.PUT(t, e, "/settings/password.min_length", `{"value":"10"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	// bool-ключ с мусором -> 400
	rec = testutil.PUT(t, e, "/settings/password.require_digit", `{"value":"maybe"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// валидный bool -> 200
	rec = testutil.PUT(t, e, "/settings/password.require_special", `{"value":"true"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
}
