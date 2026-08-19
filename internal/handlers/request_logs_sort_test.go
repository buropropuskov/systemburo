package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Живой журнал раздела «Мониторинг запросов» (#2125): порядок строк задаёт
// сервер, а поля даты понимают день с экрана. До этого клик по заголовку
// переставлял стрелку, но не строки, а фильтры «с» и «по» молча не применялись.

// insertSortedLog кладёт запись журнала с заданной длительностью; nil означает
// «длительности нет» -- так лежат записи, сделанные до перехода на микросекунды.
func insertSortedLog(t *testing.T, db *gorm.DB, url, method string, durationUs *int64, at time.Time) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO request_logs (url, method, response_status, duration_us, created_at)
		 VALUES (?,?,?,?,?)`,
		url, method, 200, durationUs, at.UTC(),
	).Error)
}

func logURLs(t *testing.T, body []byte) []string {
	t.Helper()
	var parsed struct {
		Data []struct {
			URL *string `json:"url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))

	urls := make([]string, 0, len(parsed.Data))
	for _, l := range parsed.Data {
		if l.URL != nil {
			urls = append(urls, *l.URL)
		}
	}
	return urls
}

func us(v int64) *int64 { return &v }

// Сортировка по длительности идёт в обе стороны и считает по микросекундной
// колонке, а записи без длительности уходят в конец в обоих направлениях.
func TestRequestLogs_SortByDuration(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	now := time.Now().UTC()
	insertSortedLog(t, db, "/api/slow", "GET", us(900_000), now.Add(-3*time.Minute))
	insertSortedLog(t, db, "/api/fast", "GET", us(120), now.Add(-2*time.Minute))
	insertSortedLog(t, db, "/api/medium", "GET", us(5_000), now.Add(-1*time.Minute))
	insertSortedLog(t, db, "/api/unknown-duration", "GET", nil, now)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/request-logs?sort=duration&order=asc", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t,
		[]string{"/api/fast", "/api/medium", "/api/slow", "/api/unknown-duration"},
		logURLs(t, rec.Body.Bytes()),
		"по возрастанию длительности, запись без неё последняя")

	rec = testutil.GET(t, e, "/request-logs?sort=duration&order=desc", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t,
		[]string{"/api/slow", "/api/medium", "/api/fast", "/api/unknown-duration"},
		logURLs(t, rec.Body.Bytes()),
		"по убыванию длительности, запись без неё снова последняя")
}

// Неизвестное поле сортировки возвращает журнал к привычному «сначала свежие»,
// а не роняет запрос и не уходит в SQL: в ORDER BY попадает выражение из
// белого списка, а не пришедшая строка.
func TestRequestLogs_SortRejectsUnknownField(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	now := time.Now().UTC()
	insertSortedLog(t, db, "/api/older", "GET", us(10), now.Add(-5*time.Minute))
	insertSortedLog(t, db, "/api/newer", "GET", us(20), now)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	for _, sort := range []string{"secret_column", "id) FROM request_logs; DROP TABLE users --", ""} {
		rec := testutil.GET(t, e, "/request-logs?sort="+url.QueryEscape(sort), testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "поле сортировки %q не должно ронять запрос", sort)
		assert.Equal(t, []string{"/api/newer", "/api/older"}, logURLs(t, rec.Body.Bytes()),
			"поле сортировки %q возвращает порядок по умолчанию", sort)
	}

	var users int64
	require.NoError(t, db.Table("users").Count(&users).Error)
	assert.Positive(t, users, "справочник учётных записей на месте")
}

// Страницы при сортировке по неуникальному полю не пересекаются: у метода и
// статуса значения повторяются тысячами, и без второго ключа соседние страницы
// показывали одни и те же строки, теряя другие.
func TestRequestLogs_SortPagesDoNotOverlap(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	now := time.Now().UTC()
	for _, name := range []string{"first", "second", "third", "fourth"} {
		insertSortedLog(t, db, "/api/"+name, "GET", us(1_000), now)
	}

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	seen := map[string]bool{}
	for page := 1; page <= 2; page++ {
		rec := testutil.GET(t, e,
			"/request-logs?sort=method&order=asc&per_page=2&page="+strconv.Itoa(page),
			testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code)

		urls := logURLs(t, rec.Body.Bytes())
		require.Len(t, urls, 2, "страница %d отдаёт две записи", page)
		for _, u := range urls {
			assert.False(t, seen[u], "запись %s пришла на двух страницах подряд", u)
			seen[u] = true
		}
	}
	assert.Len(t, seen, 4, "две страницы по две записи покрывают весь журнал")
}

// Поля «с» и «по» на экране отдают день без времени. День считается местными
// сутками: «за вчера» не должно захватывать ночь сегодняшнего дня, а конец
// выбранного дня обязан попасть в выборку.
func TestRequestLogs_FilterByCalendarDay(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	loc := services.AnalyticsLocation()
	nowLocal := time.Now().In(loc)
	todayStart := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
	yesterdayStart := todayStart.AddDate(0, 0, -1)

	insertSortedLog(t, db, "/api/yesterday-morning", "GET", us(100), yesterdayStart.Add(9*time.Hour))
	insertSortedLog(t, db, "/api/yesterday-late", "GET", us(100), yesterdayStart.Add(23*time.Hour+30*time.Minute))
	insertSortedLog(t, db, "/api/today-night", "GET", us(100), todayStart.Add(10*time.Minute))

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	day := yesterdayStart.Format("2006-01-02")

	rec := testutil.GET(t, e,
		"/request-logs?from_date="+day+"&to_date="+day+"&sort=created_at&order=asc",
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	assert.Equal(t,
		[]string{"/api/yesterday-morning", "/api/yesterday-late"},
		logURLs(t, rec.Body.Bytes()),
		"выбранные сутки берутся целиком и не захватывают ночь следующего дня")
}

// Выгрузка идёт в том же порядке, что и экран: список отсортирован по
// длительности, а в файле лежали последние по времени.
func TestRequestLogs_ExportKeepsChosenOrder(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	now := time.Now().UTC()
	insertSortedLog(t, db, "/api/export-slow", "GET", us(900_000), now.Add(-2*time.Minute))
	insertSortedLog(t, db, "/api/export-fast", "GET", us(120), now)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/request-logs/export?sort=duration&order=desc", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	// Выгрузка -- книга .xlsx (#2125), поэтому порядок читается по строкам листа,
	// а не поиском подстроки в теле ответа.
	order := make([]string, 0, 2)
	for _, row := range exportedSheet(t, rec.Body.Bytes()) {
		if len(row) > 2 && strings.HasPrefix(row[2], "/api/export-") {
			order = append(order, row[2])
		}
	}
	assert.Equal(t, []string{"/api/export-slow", "/api/export-fast"}, order,
		"самый медленный запрос стоит в выгрузке первым")
}
