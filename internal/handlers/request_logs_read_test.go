package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Стоимость чтения раздела «Мониторинг запросов» (#2125): график читает окно, а
// не весь журнал, список фильтра берётся из справочника, отказ базы доходит до
// экрана вместо уверенных нулей.

func insertRequestLog(t *testing.T, db *gorm.DB, url string, at time.Time) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO request_logs (url, method, response_status, duration_ms, duration_us, created_at)
		 VALUES (?,?,?,?,?,?)`,
		url, "GET", 200, 1, 1500, at.UTC(),
	).Error)
}

// График строится по запрошенному числу интервалов, и запись старше окна в него
// не попадает. До окна запрос группировал журнал целиком и отбрасывал лишние
// точки уже после чтения всех партиций.
func TestRequestLogs_Timeline_KeepsRequestedWindow(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	now := time.Now().UTC()
	insertRequestLog(t, db, "/api/timeline-recent", now.Add(-30*time.Second))
	insertRequestLog(t, db, "/api/timeline-old", now.Add(-45*time.Minute))

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	// Двадцать четыре минутных интервала - окно шириной 24 минуты.
	rec := testutil.GET(t, e, "/request-logs/timeline?interval=60&limit=24", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Data []struct {
			Timestamp string `json:"timestamp"`
			Count     int64  `json:"count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	windowStart := now.Add(-24 * time.Minute)
	var total int64
	for _, p := range body.Data {
		ts, err := time.Parse(time.RFC3339, p.Timestamp)
		require.NoError(t, err, "точка графика приходит временем по RFC 3339")
		assert.False(t, ts.Before(windowStart), "точка %s старше окна графика", p.Timestamp)
		total += p.Count
	}
	assert.Equal(t, int64(1), total, "в окно попадает только свежая запись")
}

// Список фильтра «пользователь» собирается из справочника учётных записей.
// Раньше он строился DISTINCT-ом по журналу: полный скан партиций ради сотни
// имён, да ещё и с именами, которых в системе уже нет.
func TestRequestLogs_Users_FromDirectory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	require.NoError(t, db.Exec(
		`INSERT INTO request_logs (url, method, response_status, duration_us, user_id, username, created_at)
		 VALUES (?,?,?,?,?,?,?)`,
		"/api/ghost", "GET", 200, 1500, 999999, "ghost-from-journal", time.Now().UTC(),
	).Error)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "rl-archived", "password123", 1, td.OrgID, td.CompanyID)
	require.NoError(t, db.Exec(`UPDATE users SET is_active = false WHERE username = ?`, "rl-archived").Error)

	rec := testutil.GET(t, e, "/request-logs/users", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Data []models.RequestLogsUser `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	names := make([]string, 0, len(body.Data))
	archived := false
	for _, u := range body.Data {
		names = append(names, u.Username)
		if u.Username == "rl-archived" {
			archived = true
			assert.False(t, u.IsActive, "уволенный должен приходить с признаком архива")
		}
	}
	assert.Contains(t, names, "testadmin", "действующая учётная запись обязана быть в фильтре")
	assert.NotContains(t, names, "ghost-from-journal", "имя из журнала не заводит запись в справочнике")
	// Прежде здесь стояло обратное утверждение (#2125, S3): архивных из фильтра
	// убирали. Решение отменено в #2191 - обращения уволенного в журнале остаются, и
	// разбор происшествия с его участием ровно тот случай, ради которого журнал держат.
	// Порядок (активные, следом архивные) закрыт отдельным тестом.
	assert.True(t, archived, "архивная учётная запись остаётся в фильтре: её обращения в журнале никуда не делись")
}

// errJournalDown - причина, которую подставляем вместо ответа базы.
var errJournalDown = errors.New("pq: canceling statement due to statement timeout")

// breakJournalQueries поднимает поверх рабочей тест-БД второе gorm-соединение, у
// которого чтение журнала падает по флагу. Пул общий (postgres.Config.Conn), новая
// только цепочка обработчиков, поэтому подмена не задевает соседние тесты.
func breakJournalQueries(t *testing.T, base *gorm.DB) (*gorm.DB, *bool) {
	t.Helper()

	sqlDB, err := base.DB()
	require.NoError(t, err)

	db, err := gorm.Open(
		postgres.New(postgres.Config{Conn: sqlDB}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)

	broken := false
	// Показатели шапки идут сырым запросом (обработчик row), остальное - обычной
	// выборкой (обработчик query), поэтому подмена вешается на оба.
	stop := func(tx *gorm.DB) {
		if !broken {
			return
		}
		if tx.Statement.Table == "request_logs" || strings.Contains(tx.Statement.SQL.String(), "request_logs") {
			tx.AddError(errJournalDown)
		}
	}
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:break_journal", stop))
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register("test:break_journal_row", stop))

	return db, &broken
}

// Недоступный журнал обязан выглядеть как авария, а не как тишина. Счётчики ленты
// складывались из двух Count, чьи ошибки не проверялись: при отказе базы раздел
// показывал ноль запросов в секунду и ноль в минуту.
func TestRequestLogs_DbFailure_ReachesCaller(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	broken, fail := breakJournalQueries(t, db)
	svc := services.NewRequestLogsService(broken, services.WithRequestLogsStatsCache(0))
	ctx := context.Background()

	calls := []struct {
		name string
		call func() error
	}{
		{"лента", func() error { _, err := svc.GetRealtime(ctx); return err }},
		{"показатели шапки", func() error { _, err := svc.GetStats(ctx); return err }},
		{"график", func() error {
			_, err := svc.GetTimeline(ctx, models.TimelineQuery{Interval: 60, Limit: 24})
			return err
		}},
		{"список записей", func() error { _, _, err := svc.GetLogs(ctx, models.RequestLogsQuery{}); return err }},
		{"аналитика", func() error {
			_, err := svc.GetHistory(ctx, models.RequestLogsHistoryQuery{})
			return err
		}},
	}

	// Сначала на исправной базе: доказывает, что ломает флаг, а не сама обёртка.
	for _, c := range calls {
		require.NoError(t, c.call(), "%s: на исправной базе ошибок быть не должно", c.name)
	}

	*fail = true
	for _, c := range calls {
		assert.Error(t, c.call(), "%s: отказ базы не должен превращаться в пустой ответ", c.name)
	}
}

// Счётчики ленты считаются одним проходом по минутному окну: секунда лежит внутри
// минуты и берётся фильтром, а не отдельным запросом. Проверяем сами числа - в
// один проход легко ошибиться границей и показать минуту вместо секунды.
func TestRequestLogs_Realtime_CountsWindows(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Токен берётся до вставок: регистрация считает пароль Argon2id, это сотни
	// миллисекунд, а последняя запись должна попасть в секундное окно.
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	now := time.Now().UTC()
	insertRequestLog(t, db, "/api/realtime-old", now.Add(-90*time.Second))
	insertRequestLog(t, db, "/api/realtime-minute", now.Add(-30*time.Second))
	insertRequestLog(t, db, "/api/realtime-now", time.Now().UTC())

	rec := testutil.GET(t, e, "/request-logs/realtime", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Data models.RealtimeStats `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	assert.Equal(t, int64(2), body.Data.LastMinuteCount, "полуторная минута в минутное окно не входит")
	assert.Equal(t, int64(1), body.Data.LastSecondCount, "в секундном окне остаётся только последняя запись")
}
