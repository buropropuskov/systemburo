package handlers_test

// Опечатки. Человек набирает по памяти и промахивается мимо буквы, а система обязана
// понять: «Ивонов» это Иванов. Нечёткое сравнение идёт оператором %>> с порогом 0.3 --
// тем же, что в поиске Центра заявок, -- и опирается на индекс, в отличие от функции
// strict_word_similarity, которая заставляла бы просматривать таблицу целиком.
//
// Обратная сторона проверяется тут же: похожесть не должна превращать поиск в выдачу
// всего подряд, поэтому заведомо чужая фамилия не находится.

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearch_Typos_Employee(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "typo_user", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "typo_user")
	userID := userIDByName(t, db, "typo_user")
	seedSearchEmployee(t, db, "Роголев", userID, td.OrgID, "4510 555555")

	token, _ := testutil.LoginUser(t, e, "typo_user", "password123")

	cases := []struct {
		name, query string
		want        bool
	}{
		{"точное написание", "Роголев", true},
		{"одна буква мимо", "Рогалев", true},
		{"буква мимо в середине", "Роголев", true},
		{"пропущена буква", "Роголв", true},
		{"совсем другая фамилия не находится", "Кузнецов", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := testutil.GET(t, e, "/search?q="+urlQuery(tc.query), testutil.AuthHeader(token))
			require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

			_, found := groupByType(decodeSearch(t, rec.Body.String()), "employees")
			assert.Equal(t, tc.want, found, "запрос %q: %s", tc.query, rec.Body.String())
		})
	}
}

// Номера машин ищутся нечётко по отдельной просьбе. Оговорка существенная: «А111ОО77» и
// «А111ОО78» -- разные машины, и похожесть выдаст их друг за друга. Тест фиксирует
// принятое поведение, чтобы оно не изменилось молча.
func TestSearch_Typos_CarNumber(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "typo_car", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "typo_car")
	userID := userIDByName(t, db, "typo_car")

	require.NoError(t, db.Create(&models.UniqueCar{
		Number:         searchStrPtr("В 543 НЕ 654"),
		Mark:           searchStrPtr("Мерседес"),
		UserID:         &userID,
		OrganizationID: &td.OrgID,
	}).Error)

	token, _ := testutil.LoginUser(t, e, "typo_car", "password123")

	t.Run("опечатка в марке", func(t *testing.T) {
		rec := testutil.GET(t, e, "/search?q="+urlQuery("Мерсдес"), testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code)

		_, found := groupByType(decodeSearch(t, rec.Body.String()), "cars")
		assert.True(t, found, "марка с опечаткой должна находиться: %s", rec.Body.String())
	})

	t.Run("точный номер по-прежнему находится", func(t *testing.T) {
		rec := testutil.GET(t, e, "/search?q="+urlQuery("В543НЕ654"), testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code)

		_, found := groupByType(decodeSearch(t, rec.Body.String()), "cars")
		assert.True(t, found, "%s", rec.Body.String())
	})
}

// Короткий фрагмент нечётко не сравнивается: на трёх символах похожим оказывается почти
// всё, и выдача превратилась бы в шум. Точное вхождение для них работает по-прежнему.
func TestSearch_Typos_ShortWordsExactOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "typo_short", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "typo_short")
	userID := userIDByName(t, db, "typo_short")
	seedSearchEmployee(t, db, "Роголев", userID, td.OrgID, "4510 666666")

	token, _ := testutil.LoginUser(t, e, "typo_short", "password123")

	t.Run("точный фрагмент находит", func(t *testing.T) {
		rec := testutil.GET(t, e, "/search?q="+urlQuery("рого"), testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code)

		_, found := groupByType(decodeSearch(t, rec.Body.String()), "employees")
		assert.True(t, found, "%s", rec.Body.String())
	})

	t.Run("три буквы с опечаткой не находят", func(t *testing.T) {
		rec := testutil.GET(t, e, "/search?q="+urlQuery("рга"), testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code)

		_, found := groupByType(decodeSearch(t, rec.Body.String()), "employees")
		assert.False(t, found, "короткий фрагмент нечётко не сравнивается: %s", rec.Body.String())
	})
}
