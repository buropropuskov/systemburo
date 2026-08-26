package handlers_test

// Учётные записи и чёрные списки -- разделы, выгрузку которых уже закрывали как дыру
// (#1528, #1531). Через сквозной поиск открылся бы тот же канал, поэтому проверяем: без
// права раздела нет вовсе, а в выдачу не попадает то, чего нет и в самом разделе, --
// хеш пароля у учётной записи и причина попадания в чёрный список.

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearch_Users_GatedByPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "Роголевучётка", "password123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "usr_plain", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "usr_plain")
	testutil.RegisterUser(t, e, "usr_admin", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "usr_admin")
	grantPermission(t, db, "usr_admin", "page.admin.users")

	t.Run("без права раздела нет", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "usr_plain", "password123")
		rec := testutil.GET(t, e, "/search?q=Роголевучётка", testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

		_, found := groupByType(decodeSearch(t, rec.Body.String()), "users")
		assert.False(t, found, "список учётных записей закрыт правом: %s", rec.Body.String())
	})

	t.Run("с правом учётка находится по логину", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "usr_admin", "password123")
		rec := testutil.GET(t, e, "/search?q=Роголевучётка", testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

		_, found := groupByType(decodeSearch(t, rec.Body.String()), "users")
		require.True(t, found, "учётная запись должна находиться: %s", rec.Body.String())

		body := rec.Body.String()
		assert.NotContains(t, body, "password", "в выдаче не должно быть полей пароля: %s", body)
		assert.NotContains(t, body, "$2a$", "в выдаче не должно быть хеша пароля: %s", body)
	})
}

func TestSearch_Blacklist_GatedByPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const secretReason = "инцидент на проходной 14 марта"
	require.NoError(t, db.Create(&models.PersonBlacklist{
		LastName:  "Роголевчс",
		FirstName: "Иван",
		Reason:    secretReason,
	}).Error)

	testutil.RegisterUser(t, e, "bl_plain", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "bl_plain")
	testutil.RegisterUser(t, e, "bl_admin", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "bl_admin")
	grantPermission(t, db, "bl_admin", "page.admin.blacklist")

	t.Run("без права раздела нет", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "bl_plain", "password123")
		rec := testutil.GET(t, e, "/search?q=Роголевчс", testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

		_, found := groupByType(decodeSearch(t, rec.Body.String()), "blacklist")
		assert.False(t, found, "чёрный список закрыт правом: %s", rec.Body.String())
	})

	t.Run("с правом запись находится, причина не выдаётся", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "bl_admin", "password123")
		rec := testutil.GET(t, e, "/search?q=Роголевчс", testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

		count, found := groupByType(decodeSearch(t, rec.Body.String()), "blacklist")
		require.True(t, found, "запись чёрного списка должна находиться: %s", rec.Body.String())
		assert.Equal(t, 1, count)
		assert.NotContains(t, rec.Body.String(), secretReason,
			"обстоятельства инцидента не место показывать в подсказке поиска")
	})
}

// Машины и люди приходят одной группой, но ведут на разные вкладки раздела.
func TestSearch_Blacklist_VehicleAndPersonHaveOwnTargets(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Корень слова свой, не пересекающийся с соседними тестами: нечёткое сравнение
	// находит записи с общим началом, и «Роголевоб…» ловилось бы запросом «Роголевчс».
	require.NoError(t, db.Exec("DELETE FROM person_blacklists WHERE last_name LIKE ?", "Барсуквкладка%").Error)
	require.NoError(t, db.Exec("DELETE FROM vehicle_blacklists WHERE car_number LIKE ?", "Барсуквкладка%").Error)
	token := searchDirToken("Барсуквкладка")
	require.NoError(t, db.Create(&models.PersonBlacklist{
		LastName: token, FirstName: "Иван", Reason: "причина",
	}).Error)
	require.NoError(t, db.Create(&models.VehicleBlacklist{
		CarNumber: token, MarkName: "BMW", Reason: "причина",
	}).Error)

	testutil.RegisterUser(t, e, "bl_kinds", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "bl_kinds")
	grantPermission(t, db, "bl_kinds", "page.admin.blacklist")

	authToken, _ := testutil.LoginUser(t, e, "bl_kinds", "password123")
	rec := testutil.GET(t, e, "/search?q="+token, testutil.AuthHeader(authToken))
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	body := rec.Body.String()
	assert.Contains(t, body, `"entity":"person_blacklist"`, "человек ведёт на свою вкладку: %s", body)
	assert.Contains(t, body, `"entity":"vehicle_blacklist"`, "машина ведёт на свою вкладку: %s", body)
}
