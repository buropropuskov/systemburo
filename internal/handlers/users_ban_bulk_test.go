package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Групповая блокировка/разблокировка по списку username: успех, частичный успех
// (супер-админ и самобан в errors), дедуп, очистка причины при разбане.
func TestUsers_BulkBanUnban(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	testutil.RegisterUser(t, e, "banu1", "password123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "banu2", "password123", 1, td.OrgID, td.CompanyID)

	banned := func(username string) *bool {
		var b *bool
		require.NoError(t, db.Table("users").Select("is_banned").Where("username = ?", username).Row().Scan(&b))
		return b
	}
	reasonOf := func(username string) *string {
		var r *string
		require.NoError(t, db.Table("users").Select("ban_reason").Where("username = ?", username).Row().Scan(&r))
		return r
	}

	t.Run("ban полный успех с причиной", func(t *testing.T) {
		rec := testutil.POST(t, e, "/users/bulk/ban", `{"usernames":["banu1","banu2"],"reason":"нарушение"}`, h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		res := testutil.ParseMap(t, rec)
		assert.Equal(t, float64(2), res["success_count"])
		assert.Equal(t, float64(0), res["error_count"])
		require.NotNil(t, banned("banu1"))
		assert.True(t, *banned("banu1"))
		assert.True(t, *banned("banu2"))
		require.NotNil(t, reasonOf("banu1"))
		assert.Equal(t, "нарушение", *reasonOf("banu1"))
	})

	t.Run("дубли username дедуплицируются", func(t *testing.T) {
		dup := testutil.ParseMap(t, testutil.POST(t, e, "/users/bulk/ban", `{"usernames":["banu1","banu1"]}`, h))
		assert.Equal(t, float64(1), dup["success_count"])
	})

	t.Run("unban полный успех очищает причину", func(t *testing.T) {
		rec := testutil.POST(t, e, "/users/bulk/unban", `{"usernames":["banu1","banu2"]}`, h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, float64(2), testutil.ParseMap(t, rec)["success_count"])
		assert.False(t, *banned("banu1"))
		assert.Nil(t, reasonOf("banu1"))
	})

	t.Run("супер-админ в наборе -> в errors, не валит операцию", func(t *testing.T) {
		testutil.RegisterUser(t, e, "bansuper", "password123", 1, td.OrgID, td.CompanyID)
		require.NoError(t, db.Table("users").Where("username = ?", "bansuper").Update("is_super_admin", true).Error)
		rec := testutil.POST(t, e, "/users/bulk/ban", `{"usernames":["banu1","bansuper"]}`, h)
		require.Equal(t, http.StatusMultiStatus, rec.Code)
		res := testutil.ParseMap(t, rec)
		assert.Equal(t, float64(1), res["success_count"])
		assert.Equal(t, float64(1), res["error_count"])
		errs := res["errors"].([]interface{})
		require.Len(t, errs, 1)
		e0 := errs[0].(map[string]interface{})
		assert.Equal(t, "bansuper", e0["name"])
		assert.Contains(t, e0["error"].(string), "супер-администратор")
		testutil.POST(t, e, "/users/bulk/unban", `{"usernames":["banu1"]}`, h) // вернуть
	})

	t.Run("самобан актора -> в errors", func(t *testing.T) {
		rec := testutil.POST(t, e, "/users/bulk/ban", `{"usernames":["banu1","testadmin"]}`, h)
		require.Equal(t, http.StatusMultiStatus, rec.Code)
		res := testutil.ParseMap(t, rec)
		assert.Equal(t, float64(1), res["success_count"])
		errs := res["errors"].([]interface{})
		require.Len(t, errs, 1)
		e0 := errs[0].(map[string]interface{})
		assert.Equal(t, "testadmin", e0["name"])
		assert.Contains(t, e0["error"].(string), "самого себя")
		testutil.POST(t, e, "/users/bulk/unban", `{"usernames":["banu1"]}`, h)
	})

	t.Run("несуществующий username -> в errors (207)", func(t *testing.T) {
		rec := testutil.POST(t, e, "/users/bulk/ban", `{"usernames":["banu1","nouser"]}`, h)
		require.Equal(t, http.StatusMultiStatus, rec.Code)
		res := testutil.ParseMap(t, rec)
		assert.Equal(t, float64(1), res["success_count"])
		assert.Equal(t, float64(1), res["error_count"])
		testutil.POST(t, e, "/users/bulk/unban", `{"usernames":["banu1"]}`, h)
	})

	t.Run("пустой список -> 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/users/bulk/ban", `{"usernames":[]}`, h).Code)
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/users/bulk/unban", `{"usernames":[]}`, h).Code)
	})
}

// Гейт action.ban.user: обычный пользователь без права получает 403 до цикла.
func TestUsers_BulkBan_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "banplain", "password123", 1, td.OrgID, td.CompanyID))

	assert.Equal(t, http.StatusForbidden,
		testutil.POST(t, e, "/users/bulk/ban", fmt.Sprintf(`{"usernames":[%q]}`, "banplain"), h).Code)
	assert.Equal(t, http.StatusForbidden,
		testutil.POST(t, e, "/users/bulk/unban", fmt.Sprintf(`{"usernames":[%q]}`, "banplain"), h).Code)
}
