package handlers_test

// Транслитерация поиска (#2414). Одно и то же название пишут буквами двух алфавитов, а
// человек ищет тем, которое знает: заведение «La Trattoria» знают как «Тратторию», а
// сотрудника, заведённого как «Sergey», ищут по «сергей».
//
// Важная часть - последние случаи в таблице: транслитерация обязана работать В ПАРЕ с
// нечётким сравнением. Опечатка в кириллице должна доходить и до кириллической записи,
// и до латинской, то есть похожесть считается ПОСЛЕ перевода в другой алфавит.

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearch_Translit_Employee(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "translit_user", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "translit_user")
	userID := userIDByName(t, db, "translit_user")
	seedSearchEmployee(t, db, "Sergeev", userID, td.OrgID, "4510 111111")

	token, _ := testutil.LoginUser(t, e, "translit_user", "password123")

	cases := []struct {
		name, query string
		want        bool
		почему      string
	}{
		{"латиницей как заведено", "Sergeev", true, ""},
		{"кириллицей на слух", "Сергеев", true,
			"человек знает сотрудника по-русски, а в базе он латиницей"},
		{"кириллицей с опечаткой", "Сергев", true,
			"похожесть обязана считаться после перевода в латиницу, а не только по оригиналу"},
		{"чужая фамилия не находится", "Кузнецов", false,
			"транслитерация не должна превращать поиск в выдачу всего подряд"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := testutil.GET(t, e, "/search?q="+urlQuery(tc.query), testutil.AuthHeader(token))
			require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

			_, found := groupByType(decodeSearch(t, rec.Body.String()), "employees")
			assert.Equal(t, tc.want, found, "запрос %q: %s. %s", tc.query, rec.Body.String(), tc.почему)
		})
	}
}

// Организация из issue: «La Trattoria» не находилась по «траттория» вовсе.
func TestSearch_Translit_Organization(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "translit_org", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "translit_org")
	// Организации ищутся в группе справочников и закрыты тем же правом, что их раздел.
	grantPermission(t, db, "translit_org", "page.admin.directories")
	token, _ := testutil.LoginUser(t, e, "translit_org", "password123")

	require.NoError(t, db.Create(&models.Organization{Name: "La Trattoria"}).Error)

	cases := []struct {
		name, query string
		want        bool
		почему      string
	}{
		{"как записано", "Trattoria", true, ""},
		{"по-русски", "Траттория", true, "ради этого случая задача и заводилась"},
		{"по-русски с потерянным удвоением", "Тратория", true,
			"удвоенные согласные теряют чаще всего"},
		{"чужое слово не находится", "Пиццерия", false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := testutil.GET(t, e, "/search?q="+urlQuery(tc.query), testutil.AuthHeader(token))
			require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

			_, found := groupByType(decodeSearch(t, rec.Body.String()), "directories")
			assert.Equal(t, tc.want, found, "запрос %q: %s. %s", tc.query, rec.Body.String(), tc.почему)
		})
	}
}
