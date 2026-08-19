package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Вкладка «Аналитика» (#2125): свёрнутые сутки склеиваются с днями, которые ещё
// лежат в детальных партициях, средние взвешены по числу запросов, а фактический
// охват периода приходит в ответе.

const historyDayLayout = "2006-01-02"

// insertDailyAggregate кладёт строку суточного агрегата - то, во что превращается
// партиция журнала после свёртки.
func insertDailyAggregate(t *testing.T, db *gorm.DB, day time.Time, endpoint string,
	requests, errors, avgUs, p95Us int64) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO request_logs_daily
			(day, user_id, endpoint, method, status_class, request_count, error_count,
			 avg_duration_ms, p95_duration_ms, avg_duration_us, p95_duration_us)
		 VALUES (?,0,?,'GET',2,?,?,?,?,?,?)`,
		day.Format(historyDayLayout), endpoint, requests, errors,
		avgUs/1000, p95Us/1000, avgUs, p95Us,
	).Error)
}

// fetchHistory дёргает вкладку за период и разворачивает ответ.
func fetchHistory(t *testing.T, e *echo.Echo, token string, from, to time.Time) models.RequestLogsHistory {
	t.Helper()
	url := fmt.Sprintf("/request-logs/history?from_date=%s&to_date=%s",
		from.Format(historyDayLayout), to.Format(historyDayLayout))
	rec := testutil.GET(t, e, url, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Data models.RequestLogsHistory `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Data
}

// Свёрнутые сутки и свежие дни показываются вместе. До этого вкладка читала
// только агрегаты, и последний месяц - весь срок хранения подробностей - молча
// пропадал с экрана.
func TestRequestLogs_History_JoinsAggregatesAndFreshDays(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	rolled := today.AddDate(0, 0, -40)
	insertDailyAggregate(t, db, rolled, "/api/rolled", 100, 10, 200_000, 500_000)

	now := time.Now().UTC()
	insertRequestLog(t, db, "/api/fresh", now.Add(-2*time.Hour))
	insertRequestLog(t, db, "/api/fresh", now.Add(-1*time.Hour))

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	res := fetchHistory(t, e, token, today.AddDate(0, 0, -45), today)

	assert.Equal(t, int64(102), res.Totals.Requests, "итог складывает свёрнутые сутки и свежие записи")
	assert.Equal(t, "mixed", res.Coverage.Source)
	assert.Equal(t, rolled.Format(historyDayLayout), res.Coverage.AggregatedThrough)
	assert.Equal(t, rolled.Format(historyDayLayout), res.Coverage.From, "охват начинается с первого дня с записями")
	assert.Equal(t, today.Format(historyDayLayout), res.Coverage.To)
	assert.Equal(t, 2, res.Coverage.Days, "суток с данными два: свёрнутое и сегодняшнее")
	assert.False(t, res.Coverage.ExactP95, "в периоде есть свёрнутые сутки, точного перцентиля по ним нет")
	assert.Equal(t, today.AddDate(0, 0, -45).Format(historyDayLayout), res.Coverage.RequestedFrom)

	endpoints := map[string]int64{}
	for _, ep := range res.TopEndpoints {
		endpoints[ep.Endpoint] = ep.Requests
	}
	assert.Equal(t, int64(100), endpoints["/api/rolled"], "маршрут из агрегатов остаётся в топе")
	assert.Equal(t, int64(2), endpoints["/api/fresh"], "свежий маршрут читается из детальных партиций")
	require.Len(t, res.Daily, 2)
	assert.Equal(t, int64(2), res.Daily[1].Requests, "сегодняшние записи стоят последним днём ряда")
}

// Период целиком в пределах хранения подробностей считается по самим записям, и
// перцентиль в нём честный.
func TestRequestLogs_History_DetailedOnlyPeriodIsExact(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	now := time.Now().UTC()
	insertRequestLog(t, db, "/api/fresh", now.Add(-30*time.Minute))

	today := now.Truncate(24 * time.Hour)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	res := fetchHistory(t, e, token, today.AddDate(0, 0, -2), today)

	assert.Equal(t, "detailed", res.Coverage.Source)
	assert.True(t, res.Coverage.ExactP95, "свёрнутых суток в периоде нет - перцентиль считается по записям")
	assert.Empty(t, res.Coverage.AggregatedThrough)
	assert.Equal(t, int64(1), res.Totals.Requests)
}

// Средняя длительность периода взвешена по числу запросов. Среднее суточных
// средних давало тихой ночи тот же вес, что рабочему дню: сутки с одним медленным
// запросом перевешивали тысячу быстрых.
func TestRequestLogs_History_AverageIsWeighted(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	insertDailyAggregate(t, db, today.AddDate(0, 0, -41), "/api/slow-night", 1, 0, 1_000_000, 1_000_000)
	insertDailyAggregate(t, db, today.AddDate(0, 0, -40), "/api/busy-day", 999, 0, 1_000, 2_000)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	res := fetchHistory(t, e, token, today.AddDate(0, 0, -45), today)

	assert.Equal(t, int64(1000), res.Totals.Requests)
	assert.InDelta(t, 2.0, res.Totals.AvgDuration, 0.1,
		"взвешенная средняя близка к длительности массовых запросов, а не к среднему двух суток")
	assert.Equal(t, "aggregates", res.Coverage.Source)
}

// Долгоживущие подписки не попадают в среднюю периода: в журнале у них записано
// время жизни соединения. В своей строке топа они остаются как есть - это её
// собственная длительность, а не смесь.
func TestRequestLogs_History_StreamingOutOfAverage(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	day := today.AddDate(0, 0, -40)
	insertDailyAggregate(t, db, day, "/api/events", 10, 0, 20_000_000, 21_000_000)
	insertDailyAggregate(t, db, day, "/api/applications", 10, 0, 10_000, 20_000)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	res := fetchHistory(t, e, token, today.AddDate(0, 0, -45), today)

	assert.InDelta(t, 10.0, res.Totals.AvgDuration, 0.1,
		"подписка на события не должна задирать среднюю периода до секунд")

	var events models.HistoryEndpoint
	for _, ep := range res.TopEndpoints {
		if ep.Endpoint == "/api/events" {
			events = ep
		}
	}
	assert.InDelta(t, 20_000.0, events.AvgDuration, 1, "в своей строке подписка показывает собственное время")
}

// Свёрнутая, но не удалённая партиция не удваивает итог: детальная часть читается
// строго со следующего дня после последнего свёрнутого.
func TestRequestLogs_History_RolledDayCountedOnce(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)
	insertRequestLog(t, db, "/api/twice", now.Add(-3*time.Hour))
	insertDailyAggregate(t, db, today, "/api/twice", 1, 0, 1_500, 1_500)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	res := fetchHistory(t, e, token, today.AddDate(0, 0, -1), today)

	assert.Equal(t, int64(1), res.Totals.Requests, "запись, уже попавшая в агрегат, второй раз не считается")
	assert.Equal(t, "aggregates", res.Coverage.Source)
}

// Пустой период объясняется словами, а не показывается нулями без пояснений:
// экран берёт из охвата, что именно спрашивали и что нашлось.
func TestRequestLogs_History_EmptyPeriodReportsCoverage(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	res := fetchHistory(t, e, token, today.AddDate(0, 0, -3), today.AddDate(0, 0, -2))

	assert.Equal(t, "empty", res.Coverage.Source)
	assert.Zero(t, res.Coverage.Days)
	assert.Empty(t, res.Coverage.From)
	assert.Equal(t, today.AddDate(0, 0, -3).Format(historyDayLayout), res.Coverage.RequestedFrom)
	assert.Equal(t, today.AddDate(0, 0, -2).Format(historyDayLayout), res.Coverage.RequestedTo)
	assert.Empty(t, res.Daily)
}
