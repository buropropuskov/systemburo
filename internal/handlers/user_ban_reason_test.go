package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// POST /users/:id/ban с телом {reason} сохраняет причину в users.ban_reason,
// unban её очищает (#187 Фаза 3 -- причина показывается заблокированному в ЛК).
func TestUserBan_StoresAndClearsReason(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "banreasontarget", "password123", 1, td.OrgID, td.CompanyID)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "banreasontarget").Row().Scan(&userID))
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	reason := "грубое нарушение регламента"
	rec := testutil.POST(t, e, fmt.Sprintf("/users/%d/ban", userID), fmt.Sprintf(`{"reason":%q}`, reason), testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var stored *string
	require.NoError(t, db.Table("users").Select("ban_reason").Where("id = ?", userID).Row().Scan(&stored))
	require.NotNil(t, stored, "ban_reason должна сохраниться")
	assert.Equal(t, reason, *stored)

	// unban очищает причину.
	rec = testutil.POST(t, e, fmt.Sprintf("/users/%d/unban", userID), "", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	stored = nil
	require.NoError(t, db.Table("users").Select("ban_reason").Where("id = ?", userID).Row().Scan(&stored))
	assert.Nil(t, stored, "unban должен очистить ban_reason")
}

// Бан без тела (пустой запрос) не падает и трактуется как бан без причины.
func TestUserBan_NoReasonBodyOptional(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "bannoreason", "password123", 1, td.OrgID, td.CompanyID)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "bannoreason").Row().Scan(&userID))
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, fmt.Sprintf("/users/%d/ban", userID), "", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var stored *string
	require.NoError(t, db.Table("users").Select("ban_reason").Where("id = ?", userID).Row().Scan(&stored))
	assert.Nil(t, stored)

	var banned bool
	require.NoError(t, db.Table("users").Select("is_banned").Where("id = ?", userID).Row().Scan(&banned))
	assert.True(t, banned)
}
