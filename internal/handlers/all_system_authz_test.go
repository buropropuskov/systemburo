package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllSystem_ForbiddenForNonAdmin: системный срез (filter_type=all_system)
// закрыт для обычного юзера (не super, не admin) -> 403, а свой scope доступен.
// Закрывает broken access control: раньше all_system шёл без фильтра для всех.
func TestAllSystem_ForbiddenForNonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "plainuser", "password123", 1, td.OrgID, td.CompanyID)
	token, _ := testutil.LoginUser(t, e, "plainuser", "password123")
	h := testutil.AuthHeader(token)

	for _, path := range []string{"/unique-cars", "/unique-employees"} {
		rec := testutil.GET(t, e, path+"?filter_type=all_system", h)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s all_system без прав -> 403: %s", path, rec.Body.String())

		rec = testutil.GET(t, e, path+"?filter_type=user", h)
		assert.Equal(t, http.StatusOK, rec.Code, "%s свой scope -> 200: %s", path, rec.Body.String())
	}
}

// TestAllSystem_AllowedForAdminFlag: администратор (is_admin) видит системный срез.
func TestAllSystem_AllowedForAdminFlag(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "adminflag", "password123", 1, td.OrgID, td.CompanyID)
	require.NoError(t, db.Table("users").Where("username = ?", "adminflag").Update("is_admin", true).Error)
	token, _ := testutil.LoginUser(t, e, "adminflag", "password123")
	h := testutil.AuthHeader(token)

	for _, path := range []string{"/unique-cars", "/unique-employees"} {
		rec := testutil.GET(t, e, path+"?filter_type=all_system", h)
		assert.Equal(t, http.StatusOK, rec.Code, "%s all_system у админа -> 200: %s", path, rec.Body.String())
	}
}
