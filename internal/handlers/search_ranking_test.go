package handlers_test

// Выдача каждого раздела обрезана до нескольких строк, поэтому важно не только что
// найдено, но и что попало в эти строки. Проверяем два свойства: свежие записи идут
// раньше старых (иначе обрезка выбрасывает как раз актуальное) и запрос из нескольких
// слов находит запись, у которой слова лежат в разных колонках.

import (
	"net/http"
	"net/url"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// urlQuery кодирует строку запроса: в тестовых запросах есть пробелы и кириллица.
func urlQuery(q string) string { return url.QueryEscape(q) }

// Заявок с одной машиной бывает много, а показываем мы пять. Свежая заявка нужнее
// прошлогодней, поэтому порядок идёт от новых к старым.
func TestSearch_Applications_NewestFirst(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "ord_user", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "ord_user")
	userID := userIDByName(t, db, "ord_user")

	token := searchDirToken("Ордтест")
	var ids []int
	for i := 0; i < 3; i++ {
		ids = append(ids, seedSearchApplication(t, db, token+"/"+string(rune('a'+i)), userID, td.OrgID))
	}

	authToken, _ := testutil.LoginUser(t, e, "ord_user", "password123")
	rec := testutil.GET(t, e, "/search?q="+token, testutil.AuthHeader(authToken))
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	resp := decodeSearch(t, rec.Body.String())
	var got []int
	for _, g := range resp.Data.Groups {
		if g.Type == "applications" {
			for _, it := range g.Items {
				got = append(got, it.ID)
			}
		}
	}
	require.Len(t, got, 3, "должны найтись все три заявки: %s", rec.Body.String())
	assert.Equal(t, []int{ids[2], ids[1], ids[0]}, got, "свежие заявки идут первыми")
}

// Человек вводит номер вместе с маркой, а лежат они в разных колонках. Поиск обязан
// понимать такой запрос: искать каждое слово отдельно и требовать, чтобы нашлись все.
func TestSearch_MultiWordQueryAcrossColumns(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "mw_user", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "mw_user")
	userID := userIDByName(t, db, "mw_user")

	number := "В 543 НЕ 654"
	require.NoError(t, db.Create(&models.UniqueCar{
		Number:         searchStrPtr(number),
		Mark:           searchStrPtr("Мерседес"),
		UserID:         &userID,
		OrganizationID: &td.OrgID,
	}).Error)

	authToken, _ := testutil.LoginUser(t, e, "mw_user", "password123")

	cases := []struct {
		name, query string
	}{
		{"только номер", number},
		{"номер и марка вместе", number + " Мерседес"},
		{"марка перед номером", "Мерседес " + number},
		{"часть номера и марка", "543 Мерседес"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := testutil.GET(t, e, "/search?q="+urlQuery(tc.query), testutil.AuthHeader(authToken))
			require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

			count, found := groupByType(decodeSearch(t, rec.Body.String()), "cars")
			require.True(t, found, "машина должна находиться по запросу %q: %s", tc.query, rec.Body.String())
			assert.Equal(t, 1, count)
		})
	}
}

// Слова, которых нет ни в одной колонке записи, не должны её находить: иначе поиск из
// нескольких слов превратится в поиск по любому из них и вернёт полреестра.
func TestSearch_MultiWordRequiresAllWords(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "mw_strict", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "mw_strict")
	userID := userIDByName(t, db, "mw_strict")

	require.NoError(t, db.Create(&models.UniqueCar{
		Number:         searchStrPtr("Т 111 УУ 777"),
		Mark:           searchStrPtr("Мерседес"),
		UserID:         &userID,
		OrganizationID: &td.OrgID,
	}).Error)

	authToken, _ := testutil.LoginUser(t, e, "mw_strict", "password123")
	rec := testutil.GET(t, e, "/search?q="+urlQuery("Мерседес Запорожец"), testutil.AuthHeader(authToken))
	require.Equal(t, http.StatusOK, rec.Code)

	_, found := groupByType(decodeSearch(t, rec.Body.String()), "cars")
	assert.False(t, found, "второе слово не встречается у записи -- находиться она не должна: %s", rec.Body.String())
}
