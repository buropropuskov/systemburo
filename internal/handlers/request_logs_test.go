package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Раздел «Мониторинг запросов» гейтится ключом page.admin.monitoring (#2125) --
// тем же, по которому пункт показывает меню фронта. До этого группа висела на
// page.admin, и носитель ключа раздела получал 403 на каждом запросе экрана.

func TestRequestLogs_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "rluser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	for _, path := range []string{"/request-logs", "/request-logs/stats", "/request-logs/users", "/request-logs/export"} {
		rec := testutil.GET(t, e, path, h)
		assert.Equal(t, http.StatusForbidden, rec.Code, "non-admin must be forbidden on %s", path)
	}
}

func TestRequestLogs_Admin_Ok(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/request-logs", h)
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, "/request-logs/stats", h)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequestLogs_MonitoringKeyHolder_Ok(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "rlmonitor", "password123", 1, td.OrgID, td.CompanyID)
	testutil.GrantPermission(t, getUserID(t, db, "rlmonitor"), services.KeyPageAdminMonitoring)
	h := testutil.AuthHeader(token)

	paths := []string{
		"/request-logs", "/request-logs/stats", "/request-logs/users", "/request-logs/realtime",
		"/request-logs/timeline", "/request-logs/history", "/request-logs/export",
	}
	for _, path := range paths {
		rec := testutil.GET(t, e, path, h)
		assert.Equal(t, http.StatusOK, rec.Code, "носитель page.admin.monitoring должен проходить на %s", path)
	}
}

// page.admin сам по себе раздел больше не открывает: гейт сведён к одному ключу,
// иначе право «администрирование» тихо давало бы доступ к журналу с ПД в адресах.
func TestRequestLogs_PageAdminWithoutMonitoring_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "rlpageadmin", "password123", 1, td.OrgID, td.CompanyID)
	testutil.GrantPermission(t, getUserID(t, db, "rlpageadmin"), services.KeyPageAdmin)

	rec := testutil.GET(t, e, "/request-logs", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// Администратор проходит через adminAll, а личный deny-override раздел закрывает --
// до перевода гейта отзыв права мониторинга не влиял на API вообще.
func TestRequestLogs_AdminDenyOverride_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// RegisterManager -> is_admin=true, не супер (см. testutil/auth.go): у супера
	// deny-override не действует.
	token := testutil.RegisterManager(t, e, "rladmindeny", td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/request-logs", h)
	require.Equal(t, http.StatusOK, rec.Code, "администратор без deny должен проходить")

	testutil.DenyPermission(t, getUserID(t, db, "rladmindeny"), services.KeyPageAdminMonitoring)

	rec = testutil.GET(t, e, "/request-logs", h)
	assert.Equal(t, http.StatusForbidden, rec.Code, "deny-override должен закрывать раздел администратору")
}

func TestRequestLogs_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/request-logs", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// Длительности в шапке раздела считаются по микросекундной колонке и без учёта
// долгоживущих соединений. До #2125 быстрые ответы округлялись до нуля, а одна
// подписка на события длиной в двадцать секунд задирала среднее по всему журналу.
func TestRequestLogs_Stats_DurationMetrics(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	now := time.Now().UTC()
	insert := func(url string, durationUs int64, at time.Time) {
		require.NoError(t, db.Exec(
			`INSERT INTO request_logs (url, method, response_status, duration_ms, duration_us, created_at)
			 VALUES (?,?,?,?,?,?)`,
			url, "GET", 200, int(durationUs/1000), durationUs, at,
		).Error)
	}
	// Девять быстрых ответов быстрее миллисекунды и один медленный: медиана должна
	// остаться дробной, а p95 подняться к медленному.
	for i := 0; i < 9; i++ {
		insert("/api/fast-metrics-test", 300, now.Add(-time.Duration(i)*time.Minute))
	}
	insert("/api/slow-metrics-test", 200_000, now.Add(-10*time.Minute))
	// Подписка на события: в журнале лежит время жизни соединения.
	insert("/api/events?ticket=%2A%2A%2A", 20_000_000, now.Add(-11*time.Minute))

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/request-logs/stats", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Data struct {
			Total          int64   `json:"total"`
			Today          int64   `json:"today"`
			AvgDuration    float64 `json:"avg_duration"`
			MedianDuration float64 `json:"median_duration"`
			P95Duration    float64 `json:"p95_duration"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	assert.Equal(t, int64(11), body.Data.Total, "подписка остаётся в журнале, из счёта её не убирают")
	assert.InDelta(t, 0.3, body.Data.MedianDuration, 0.01, "быстрый ответ не должен округляться до нуля")
	assert.Greater(t, body.Data.P95Duration, body.Data.MedianDuration, "p95 обязан быть выше медианы")
	assert.Less(t, body.Data.AvgDuration, 1000.0, "двадцатисекундная подписка не должна задирать среднее")
}

// Сутки для показателя «сегодня» режутся по московской полуночи. По UTC день
// начинался в 03:00 МСК, и утренние обращения три часа не попадали в счётчик.
func TestRequestLogs_Stats_TodayInMoscowDay(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	loc := services.AnalyticsLocation()
	now := time.Now().In(loc)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	insert := func(at time.Time) {
		require.NoError(t, db.Exec(
			`INSERT INTO request_logs (url, method, response_status, duration_ms, duration_us, created_at)
			 VALUES (?,?,?,?,?,?)`,
			"/api/today-metrics-test", "GET", 200, 1, 1000, at.UTC(),
		).Error)
	}
	insert(dayStart.Add(time.Minute))  // первая минута московских суток
	insert(dayStart.Add(-time.Minute)) // последняя минута вчерашних

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/request-logs/stats", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Data struct {
			Total int64 `json:"total"`
			Today int64 `json:"today"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, int64(2), body.Data.Total)
	assert.Equal(t, int64(1), body.Data.Today, "во вчерашних сутках запись остаётся вчерашней")
}
