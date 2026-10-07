package handlers_test

// Выдача каждого раздела обрезана до нескольких строк, поэтому важно не только что
// найдено, но и что попало в эти строки. Проверяем два свойства: свежие записи идут
// раньше старых (иначе обрезка выбрасывает как раз актуальное) и запрос из нескольких
// слов находит запись, у которой слова лежат в разных колонках.

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every UNION branch must keep an old exact match before taking its local LIMIT.
func TestSearch_UnionBranchesExactBeforeLimit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	cases := []struct{ table, column, group, visibility string }{
		{"organizations", "name", "directories", "is_active"},
		{"companies", "name", "directories", "is_active"},
		{"unload_places", "name", "directories", "is_active"},
		{"system_tables", "name", "directories", "is_active"},
		{"marks", "name", "directories", "is_active"},
		{"citizenships", "name", "directories", "is_active"},
		{"license_plate_formats", "name", "directories", "is_active"},
		{"news", "title", "content", "is_active"},
		{"announcements", "title", "content", "is_active"},
		{"documents", "title", "content", "is_visible"},
		{"person_blacklists", "last_name", "blacklist", "is_active"},
		{"vehicle_blacklists", "car_number", "blacklist", "is_active"},
	}
	for index, tc := range cases {
		t.Run(tc.table, func(t *testing.T) {
			query := fmt.Sprintf("s6rank%02dexact", index)
			// CleanDB does not clear every directory. Remove only this fixture prefix.
			remove := func() {
				require.NoError(t, db.Exec("DELETE FROM "+tc.table+" WHERE "+tc.column+" LIKE ?", query+"%").Error)
			}
			remove()
			defer remove()
			insert := func(title string) {
				row := map[string]any{tc.column: title, tc.visibility: true}
				if tc.table == "system_tables" {
					row["table_type"] = "cars"
					row["display_name"] = title
				}
				if tc.table == "person_blacklists" {
					row["first_name"] = ""
					row["middle_name"] = ""
					row["reason"] = "synthetic"
				}
				if tc.table == "vehicle_blacklists" {
					row["reason"] = "synthetic"
					row["mark_id"] = 0
				}
				require.NoError(t, db.Table(tc.table).Create(row).Error)
			}
			insert(query)
			for i := 0; i < 10; i++ {
				insert(fmt.Sprintf("%s partial %02d", query, i))
			}
			rec := testutil.GET(t, e, "/search?q="+urlQuery(query)+"&limit=1&types="+tc.group, testutil.AuthHeader(token))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			resp := decodeSearch(t, rec.Body.String())
			found := false
			for _, group := range resp.Data.Groups {
				if group.Type != tc.group {
					continue
				}
				require.Len(t, group.Items, 1, rec.Body.String())
				assert.Equal(t, query, group.Items[0].Title, "old exact survives local branch LIMIT")
				found = true
			}
			require.True(t, found, rec.Body.String())
		})
	}
}

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
