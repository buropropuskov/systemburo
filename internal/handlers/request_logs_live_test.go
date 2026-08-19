package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Живая лента раздела мониторинга (#2125): быстрый отбор ошибок и медленных
// ответов, а крупный шаг графика читается вместе со свёрткой -- иначе «месяц» и
// «год» показывали срок хранения подробностей, а не историю.

// insertLiveLog кладёт запись журнала с заданными кодом ответа и длительностью.
func insertLiveLog(t *testing.T, db *gorm.DB, url string, status int, durationUs int64, at time.Time) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO request_logs (url, method, response_status, duration_us, created_at)
		 VALUES (?,'GET',?,?,?)`,
		url, status, durationUs, at.UTC(),
	).Error)
}

// timelinePoints дёргает график и разворачивает точки.
func timelinePoints(t *testing.T, e *echo.Echo, token, query string) []struct {
	Timestamp   string  `json:"timestamp"`
	Count       int64   `json:"count"`
	AvgDuration float64 `json:"avg_duration"`
} {
	t.Helper()
	rec := testutil.GET(t, e, "/request-logs/timeline?"+query, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Data []struct {
			Timestamp   string  `json:"timestamp"`
			Count       int64   `json:"count"`
			AvgDuration float64 `json:"avg_duration"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Data
}

// Отбор «только ошибки» берёт весь диапазон кодов, а не один выбранный: ошибок в
// журнале десяток разных, и перебирать их по одной оператор не станет.
func TestRequestLogs_FilterByStatusRange(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	now := time.Now().UTC()
	insertLiveLog(t, db, "/api/ok", 200, 1_000, now.Add(-4*time.Minute))
	insertLiveLog(t, db, "/api/not-found", 404, 1_000, now.Add(-3*time.Minute))
	insertLiveLog(t, db, "/api/forbidden", 403, 1_000, now.Add(-2*time.Minute))
	insertLiveLog(t, db, "/api/broken", 500, 1_000, now.Add(-time.Minute))

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/request-logs?status_min=400&sort=created_at&order=asc", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []string{"/api/not-found", "/api/forbidden", "/api/broken"}, logURLs(t, rec.Body.Bytes()),
		"нижняя граница кода отбирает все ошибки")

	rec = testutil.GET(t, e, "/request-logs?status_min=400&status_max=499", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.ElementsMatch(t, []string{"/api/not-found", "/api/forbidden"}, logURLs(t, rec.Body.Bytes()),
		"верхняя граница оставляет один класс статусов")
}

// Отбор «медленнее секунды» ищет затыки, поэтому долгоживущие подписки в него не
// попадают: у подписки на события в журнале записано время жизни соединения, и
// она заняла бы собой весь список.
func TestRequestLogs_FilterByMinDuration(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	now := time.Now().UTC()
	insertLiveLog(t, db, "/api/fast", 200, 40_000, now.Add(-3*time.Minute))
	insertLiveLog(t, db, "/api/slow", 200, 1_500_000, now.Add(-2*time.Minute))
	insertLiveLog(t, db, "/api/events", 200, 900_000_000, now.Add(-time.Minute))

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/request-logs?min_duration_ms=1000", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []string{"/api/slow"}, logURLs(t, rec.Body.Bytes()),
		"в отборе остаются медленные ответы, а не подписка длиной в четверть часа")
}

// Суточный шаг графика читает свёртку вместе с ещё не свёрнутыми сутками.
// Подробные записи живут срок хранения партиций, и «месяц» по ним обрывался
// молча -- как будто запросов не было.
func TestRequestLogs_TimelineDailyUsesAggregates(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	rolled := today.AddDate(0, 0, -10)
	insertDailyAggregate(t, db, rolled, "/api/rolled", 100, 5, 200_000, 500_000)
	// Сегодняшние сутки уже свёрнуты, но партиция ещё на месте: подробные записи
	// того же дня не должны прибавиться к столбику второй раз.
	insertDailyAggregate(t, db, today, "/api/rolled-today", 7, 0, 3_000, 9_000)
	insertRequestLog(t, db, "/api/fresh", time.Now().UTC().Add(-time.Hour))

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	points := timelinePoints(t, e, token, "interval=86400&limit=30")

	counts := map[string]int64{}
	durations := map[string]float64{}
	for _, p := range points {
		ts, err := time.Parse(time.RFC3339, p.Timestamp)
		require.NoError(t, err, "точка графика приходит временем по RFC 3339")
		day := ts.UTC().Format("2006-01-02")
		counts[day] += p.Count
		durations[day] = p.AvgDuration
	}

	assert.Equal(t, int64(100), counts[rolled.Format("2006-01-02")],
		"свёрнутые сутки видны на графике за месяц")
	assert.Equal(t, int64(7), counts[today.Format("2006-01-02")],
		"свёрнутый день берётся из свёртки один раз, подробные записи того же дня не удваивают столбик")
	assert.InDelta(t, 200.0, durations[rolled.Format("2006-01-02")], 0.01,
		"длительность точки считается по свёрнутым микросекундам: 200 000 мкс это 200 мс")
}

// Недельный шаг («Год») тоже идёт по свёртке: до этого график показывал только
// последние недели, укладывающиеся в срок хранения подробностей.
func TestRequestLogs_TimelineWeeklyReachesOldAggregates(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	old := today.AddDate(0, 0, -200)
	insertDailyAggregate(t, db, old, "/api/old", 42, 0, 5_000, 9_000)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	points := timelinePoints(t, e, token, "interval=604800&limit=52")

	// Считается столбик той недели, в которую попал свёрнутый день: обращения
	// самих тестов ложатся в текущую неделю, и сумма по графику ловила бы их.
	const week = int64(7 * 24 * 3600)
	bucket := time.Unix(old.Unix()/week*week, 0).UTC().Format(time.RFC3339)

	var found int64
	for _, p := range points {
		if p.Timestamp == bucket {
			found = p.Count
		}
	}
	assert.Equal(t, int64(42), found, "сутки двухсотдневной давности попадают в годовой график")
}
