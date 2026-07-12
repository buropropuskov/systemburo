package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsers_BulkOperations(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	testutil.RegisterUser(t, e, "bulku1", "password123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "bulku2", "password123", 1, td.OrgID, td.CompanyID)

	renterTypeID := 0
	for _, ut := range testutil.ParseSlice(t, testutil.GET(t, e, "/user-types-management", h)) {
		if ut["code"] == "renter" {
			renterTypeID = int(ut["id"].(float64))
		}
	}
	require.Greater(t, renterTypeID, 0)

	// GetAll возвращает пользователей (по username), для проверки применённого.
	userField := func(username, field string) interface{} {
		for _, u := range testutil.ParseSlice(t, testutil.GET(t, e, "/users/all", h)) {
			if u["username"] == username {
				return u[field]
			}
		}
		return nil
	}

	t.Run("type", func(t *testing.T) {
		rec := testutil.POST(t, e, "/users/bulk/type", fmt.Sprintf(`{"usernames":["bulku1","bulku2"],"type_id":%d}`, renterTypeID), h)
		require.Equal(t, http.StatusOK, rec.Code)
		res := testutil.ParseMap(t, rec)
		assert.Equal(t, float64(2), res["success_count"])
		assert.Equal(t, float64(0), res["error_count"])
		assert.Equal(t, float64(renterTypeID), userField("bulku1", "type_id"))
		assert.Equal(t, float64(renterTypeID), userField("bulku2", "type_id"))

		// Дубли username не раздувают success_count.
		dup := testutil.ParseMap(t, testutil.POST(t, e, "/users/bulk/type", fmt.Sprintf(`{"usernames":["bulku1","bulku1"],"type_id":%d}`, renterTypeID), h))
		assert.Equal(t, float64(1), dup["success_count"], "дубли username дедуплицируются")

		// Несуществующий username -> в errors, статус 207.
		prec := testutil.POST(t, e, "/users/bulk/type", fmt.Sprintf(`{"usernames":["bulku1","nouser"],"type_id":%d}`, renterTypeID), h)
		require.Equal(t, http.StatusMultiStatus, prec.Code)
		pres := testutil.ParseMap(t, prec)
		assert.Equal(t, float64(1), pres["success_count"])
		assert.Equal(t, float64(1), pres["error_count"])
		errs := pres["errors"].([]interface{})
		require.Len(t, errs, 1)
		assert.Equal(t, "nouser", errs[0].(map[string]interface{})["name"])

		// Невалидный тип -> 400 на весь запрос (валидация до цикла).
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/users/bulk/type", `{"usernames":["bulku1"],"type_id":999999}`, h).Code)
		// Пустой список -> 400.
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/users/bulk/type", fmt.Sprintf(`{"usernames":[],"type_id":%d}`, renterTypeID), h).Code)
	})

	t.Run("organization", func(t *testing.T) {
		rec := testutil.POST(t, e, "/users/bulk/organization", fmt.Sprintf(`{"usernames":["bulku1","bulku2"],"organization_id":%d}`, td.OrgID), h)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, float64(2), testutil.ParseMap(t, rec)["success_count"])
		assert.Equal(t, float64(td.OrgID), userField("bulku1", "organization_id"))

		// Несуществующая организация -> 400 на весь запрос.
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/users/bulk/organization", `{"usernames":["bulku1"],"organization_id":999999}`, h).Code)
	})

	t.Run("company", func(t *testing.T) {
		rec := testutil.POST(t, e, "/users/bulk/company", fmt.Sprintf(`{"usernames":["bulku1","bulku2"],"company_id":%d}`, td.CompanyID), h)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, float64(2), testutil.ParseMap(t, rec)["success_count"])
		assert.Equal(t, float64(td.CompanyID), userField("bulku1", "company_id"))

		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/users/bulk/company", `{"usernames":["bulku1"],"company_id":999999}`, h).Code)
	})

	t.Run("archive-restore", func(t *testing.T) {
		// Супер-админ в наборе -> в errors (одиночный Delete его бережёт 403), не валит операцию.
		testutil.RegisterUser(t, e, "bulksuper", "password123", 1, td.OrgID, td.CompanyID)
		require.NoError(t, db.Table("users").Where("username = ?", "bulksuper").Update("is_super_admin", true).Error)
		srec := testutil.POST(t, e, "/users/bulk/archive", `{"usernames":["bulku1","bulksuper"]}`, h)
		require.Equal(t, http.StatusMultiStatus, srec.Code)
		sres := testutil.ParseMap(t, srec)
		assert.Equal(t, float64(1), sres["success_count"], "обычный юзер архивировался")
		assert.Equal(t, float64(1), sres["error_count"], "супер-админ в errors")
		serrs := sres["errors"].([]interface{})
		require.Len(t, serrs, 1)
		serr := serrs[0].(map[string]interface{})
		assert.Equal(t, "bulksuper", serr["name"])
		assert.Contains(t, serr["error"].(string), "супер-администратор")
		require.Equal(t, http.StatusOK, testutil.POST(t, e, "/users/bulk/restore", `{"usernames":["bulku1"]}`, h).Code) // вернуть для следующих проверок

		// Архив bulku1, bulku2 + несуществующий -> 207 (2 успех, 1 ошибка).
		arec := testutil.POST(t, e, "/users/bulk/archive", `{"usernames":["bulku1","bulku2","nouser"]}`, h)
		require.Equal(t, http.StatusMultiStatus, arec.Code)
		ares := testutil.ParseMap(t, arec)
		assert.Equal(t, float64(2), ares["success_count"])
		assert.Equal(t, float64(1), ares["error_count"])

		// bulku1 больше не в активном списке (is_active=false).
		assert.Nil(t, userField("bulku1", "username"), "архивный не в активном /users/all")

		// Восстановление -> снова активны.
		rrec := testutil.POST(t, e, "/users/bulk/restore", `{"usernames":["bulku1","bulku2"]}`, h)
		require.Equal(t, http.StatusOK, rrec.Code)
		assert.Equal(t, float64(2), testutil.ParseMap(t, rrec)["success_count"])
		assert.Equal(t, "bulku1", userField("bulku1", "username"))
	})
}

func TestUsers_Bulk_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "plainuser", "password123", 1, td.OrgID, td.CompanyID))

	// Гейт requireUsers (page.admin.users) бьёт до цикла - 403.
	assert.Equal(t, http.StatusForbidden,
		testutil.POST(t, e, "/users/bulk/archive", `{"usernames":["plainuser"]}`, h).Code)
	assert.Equal(t, http.StatusForbidden,
		testutil.POST(t, e, "/users/bulk/type", `{"usernames":["plainuser"],"type_id":1}`, h).Code)
}
