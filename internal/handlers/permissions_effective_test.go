package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GET /permissions/user/:id/effective -- эффективные права целевого юзера с источником
// для админ-экрана прав (#187 Фаза 3). Доступ только super-admin.
func TestPermissions_GetUserEffective_SuperAdminOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userToken := testutil.RegisterAndLogin(t, e, "effectiveuser", "password123", 1, td.OrgID, td.CompanyID)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "effectiveuser").Row().Scan(&userID))

	// Обычный юзер -> 403.
	rec := testutil.GET(t, e, fmt.Sprintf("/permissions/user/%d/effective", userID), testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// Super-admin -> 200, обычный юзер в режиме normal и не забанен.
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec = testutil.GET(t, e, fmt.Sprintf("/permissions/user/%d/effective", userID), testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)
	data := testutil.ParseResponse[models.MyPermissionsResponse](t, rec)
	assert.Equal(t, "normal", data.Mode)
	assert.False(t, data.Banned)
}

// Эндпоинт должен резолвить ЦЕЛЕВОГО юзера, а не вызывающего: super-admin запрашивает
// забаненного юзера и видит его banned-набор с причиной, а не свой super-доступ.
func TestPermissions_GetUserEffective_ResolvesTargetNotCaller(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "bannedtarget", "password123", 1, td.OrgID, td.CompanyID)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "bannedtarget").Row().Scan(&userID))
	reason := "нарушение регламента"
	require.NoError(t, db.Table("users").Where("id = ?", userID).
		Updates(map[string]any{"is_banned": true, "ban_reason": reason}).Error)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, fmt.Sprintf("/permissions/user/%d/effective", userID), testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)
	data := testutil.ParseResponse[models.MyPermissionsResponse](t, rec)
	assert.True(t, data.Banned)
	assert.Equal(t, reason, data.BanReason)
}
