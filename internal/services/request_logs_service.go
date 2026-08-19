package services

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RequestLogsService -- интерфейс бизнес-логики логов запросов.
// Весь раздел мониторинга admin-only (page.admin) -- авторизация на роут-middleware.
type RequestLogsService interface {
	GetLogs(ctx context.Context, q models.RequestLogsQuery) ([]models.RequestLogs, int64, error)
	GetUsers(ctx context.Context) ([]models.RequestLogsUser, error)
	GetStats(ctx context.Context) (*models.RequestLogsStats, error)
	GetRealtime(ctx context.Context) (*models.RealtimeStats, error)
	GetTimeline(ctx context.Context, q models.TimelineQuery) ([]models.TimelinePoint, error)
	GetHistory(ctx context.Context, q models.RequestLogsHistoryQuery) (*models.RequestLogsHistory, error)
	Export(ctx context.Context, q models.RequestLogsQuery) (string, error)
}

type requestLogsService struct {
	db    *gorm.DB
	stats *statsCache
}

// statsCacheTTL -- срок жизни снимка показателей шапки. Равен периоду, с которым
// их опрашивает экран: снимок гасит повторный расчёт от каждой открытой вкладки,
// но не показывает вчерашнее.
//
// Константа кода, а не параметр окружения: крутить оператору тут нечего, а каждая
// новая переменная тянет за собой проводку через compose, генератор .env и
// приложение Б документации заказчика.
const statsCacheTTL = 30 * time.Second

// RequestLogsOption настраивает сервис журнала при создании.
type RequestLogsOption func(*requestLogsService)

// WithRequestLogsStatsCache задаёт свой срок жизни снимка показателей. Нулевой
// (и отрицательный) оставляет расчёт на каждое обращение -- так проверяют сам
// расчёт, не разбираясь, чей ответ вернулся.
func WithRequestLogsStatsCache(ttl time.Duration) RequestLogsOption {
	return func(s *requestLogsService) {
		s.stats = &statsCache{ttl: ttl}
	}
}

// NewRequestLogsService создаёт реализацию RequestLogsService.
func NewRequestLogsService(db *gorm.DB, opts ...RequestLogsOption) RequestLogsService {
	s := &requestLogsService{db: db, stats: &statsCache{ttl: statsCacheTTL}}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// statsCache -- снимок показателей шапки на срок ttl. Экран опрашивает их раз в
// полминуты из каждой открытой вкладки, а расчёт берёт перцентили по журналу
// целиком: без общего снимка каждая вкладка платит собственным сканом партиций.
type statsCache struct {
	mu    sync.Mutex
	ttl   time.Duration
	value *models.RequestLogsStats
	at    time.Time
}

// get отдаёт снимок, пока он свежий, и считает под замком, когда протух:
// одновременные читатели ждут один расчёт вместо того, чтобы запускать
// одинаковый скан каждый за себя. Отказ базы не кэшируется -- иначе одна
// неудача держала бы экран пустым весь срок жизни снимка.
func (c *statsCache) get(ctx context.Context, now time.Time,
	compute func(context.Context) (*models.RequestLogsStats, error)) (*models.RequestLogsStats, error) {
	if c == nil || c.ttl <= 0 {
		return compute(ctx)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.value != nil && now.Sub(c.at) < c.ttl {
		fresh := *c.value
		return &fresh, nil
	}

	value, err := compute(ctx)
	if err != nil {
		return nil, err
	}
	c.value, c.at = value, now

	fresh := *value
	return &fresh, nil
}

// durationUsExpr -- длительность запроса в микросекундах. Записи, сделанные до
// перехода на микросекунды, читаются из миллисекундной колонки (#2125).
const durationUsExpr = "COALESCE(duration_us, duration_ms * 1000)"

// streamingLogPaths -- пути, где записанная длительность это время жизни
// соединения, а не время ответа. Подписка на события живёт минутами, и одна
// такая запись перевешивала десятки тысяч обычных: среднее по журналу
// показывало 5 секунд при реальных ответах в десятки миллисекунд.
//
// Из журнала такие обращения не исключаются - факт подключения виден в ленте.
// Их не берут только расчёты длительности.
var streamingLogPaths = []string{"/api/events"}

// notStreamingSQL -- условие «это обычный запрос, а не долгоживущее соединение».
// Пути берутся из константы кода, не из пользовательского ввода, поэтому
// подставляются литералами.
//
// Разделитель адреса и query задан как chr(63), а не знаком вопроса в кавычках:
// gorm считает вопросительный знак местом подстановки даже внутри строковой
// константы, и аргументы запроса разъезжаются - в split_part уезжает дата.
func notStreamingSQL() string {
	if len(streamingLogPaths) == 0 {
		return "TRUE"
	}
	quoted := make([]string, 0, len(streamingLogPaths))
	for _, p := range streamingLogPaths {
		quoted = append(quoted, "'"+p+"'")
	}
	return "split_part(COALESCE(url, ''), chr(63), 1) NOT IN (" + strings.Join(quoted, ", ") + ")"
}

func (s *requestLogsService) applyFilters(tx *gorm.DB, q models.RequestLogsQuery) *gorm.DB {
	if q.UserID != nil {
		tx = tx.Where("user_id = ?", *q.UserID)
	}
	if q.Method != "" {
		tx = tx.Where("method = ?", strings.ToUpper(q.Method))
	}
	if q.Status != nil {
		tx = tx.Where("response_status = ?", *q.Status)
	}
	if q.From != "" {
		if t, err := time.Parse(time.RFC3339, q.From); err == nil {
			tx = tx.Where("created_at >= ?", t)
		}
	}
	if q.To != "" {
		if t, err := time.Parse(time.RFC3339, q.To); err == nil {
			tx = tx.Where("created_at <= ?", t)
		}
	}
	if q.Search != "" {
		pattern := "%" + q.Search + "%"
		tx = tx.Where("(url ILIKE ? OR username ILIKE ?)", pattern, pattern)
	}
	return tx
}

// GetLogs возвращает логи с пагинацией и фильтрацией.
func (s *requestLogsService) GetLogs(ctx context.Context, q models.RequestLogsQuery) ([]models.RequestLogs, int64, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PerPage < 1 {
		q.PerPage = 20
	}
	if q.PerPage > 100 {
		q.PerPage = 100
	}

	tx := s.db.WithContext(ctx).Table("request_logs")
	tx = s.applyFilters(tx, q)

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "failed to count request logs")
	}

	logs := make([]models.RequestLogs, 0)
	offset := (q.Page - 1) * q.PerPage
	if err := tx.Order("created_at DESC").Offset(offset).Limit(q.PerPage).Find(&logs).Error; err != nil {
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch request logs")
	}

	return logs, total, nil
}

// GetUsers возвращает список учётных записей для фильтра «пользователь».
//
// Читается справочник, а не DISTINCT по журналу: уникальные значения из журнала
// стоят полного скана всех партиций (сотни тысяч строк) ради сотни имён, которые
// уже лежат в users.
//
// Учётные записи в архиве в список не попадают: фильтр отвечает на вопрос «за кем
// посмотреть», а не «кто когда-либо обращался». Обращения уволенного в журнале
// остаются и находятся поиском по имени.
func (s *requestLogsService) GetUsers(ctx context.Context) ([]models.RequestLogsUser, error) {
	users := make([]models.RequestLogsUser, 0)
	err := s.db.WithContext(ctx).
		Table("users").
		Select("id, username").
		Where("is_active = ?", true).
		Order("username").
		Scan(&users).Error
	if err != nil {
		slog.Error("request logs users", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch users")
	}

	return users, nil
}

// GetStats возвращает агрегированную статистику для шапки раздела.
//
// Считается одним проходом по таблице: раздельные COUNT/AVG давали пять
// последовательных сканов всех партиций на каждое обновление экрана.
//
// Длительности берутся за последний час, а не за всю историю: шапка живого
// журнала отвечает на вопрос «как система отвечает сейчас», а перцентиль по
// всем тридцати суткам ещё и требовал бы сортировки миллионов строк каждые
// полминуты.
func (s *requestLogsService) GetStats(ctx context.Context) (*models.RequestLogsStats, error) {
	return s.stats.get(ctx, time.Now(), s.computeStats)
}

// computeStats считает показатели шапки одним запросом.
func (s *requestLogsService) computeStats(ctx context.Context) (*models.RequestLogsStats, error) {
	now := time.Now().In(AnalyticsLocation())
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, AnalyticsLocation())
	hourAgo := time.Now().UTC().Add(-1 * time.Hour)

	var row struct {
		Total    int64
		Today    int64
		Errors   int64
		LastHour int64
		AvgUs    float64
		MedianUs float64
		P95Us    float64
	}

	// Окно длительностей: последний час и без долгоживущих подписок. Час задаётся
	// параметром, поэтому подставляется третьим аргументом запроса.
	recent := "created_at >= ? AND " + notStreamingSQL()
	query := `
		SELECT
			COUNT(*)                                       AS total,
			COUNT(*) FILTER (WHERE created_at >= ?)        AS today,
			COUNT(*) FILTER (WHERE response_status >= 400) AS errors,
			COUNT(*) FILTER (WHERE created_at >= ?)        AS last_hour,
			COALESCE(AVG(` + durationUsExpr + `) FILTER (WHERE ` + recent + `), 0) AS avg_us,
			COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY ` + durationUsExpr + `)
				FILTER (WHERE ` + recent + `), 0) AS median_us,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY ` + durationUsExpr + `)
				FILTER (WHERE ` + recent + `), 0) AS p95_us
		FROM request_logs`

	if err := s.db.WithContext(ctx).Raw(query, dayStart, hourAgo, hourAgo, hourAgo, hourAgo).Scan(&row).Error; err != nil {
		slog.Error("request logs stats", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch stats")
	}

	stats := models.RequestLogsStats{
		Total:             row.Total,
		Today:             row.Today,
		AvgDuration:       usToMs(row.AvgUs),
		MedianDuration:    usToMs(row.MedianUs),
		P95Duration:       usToMs(row.P95Us),
		RequestsPerMinute: float64(row.LastHour) / 60.0,
	}
	if row.Total > 0 {
		stats.ErrorRate = float64(row.Errors) / float64(row.Total) * 100
	}

	return &stats, nil
}

// usToMs переводит микросекунды в миллисекунды, оставляя один знак после запятой:
// ответы быстрее миллисекунды не должны выглядеть как нулевые.
func usToMs(us float64) float64 {
	return math.Round(us/100) / 10
}

// GetRealtime возвращает количество запросов за последнюю секунду и минуту.
//
// Оба счётчика берутся одним проходом по минутному окну: вторая секунда лежит
// внутри первой минуты, поэтому отдельный запрос за ней был лишним сканом.
//
// Отказ базы возвращается наверх. Раньше ошибки Count терялись, и лента
// показывала честные нули вместо сообщения о том, что журнал недоступен.
func (s *requestLogsService) GetRealtime(ctx context.Context) (*models.RealtimeStats, error) {
	now := time.Now().UTC()
	var stats models.RealtimeStats

	err := s.db.WithContext(ctx).Table("request_logs").
		Select("COUNT(*) FILTER (WHERE created_at >= ?) AS last_second_count, COUNT(*) AS last_minute_count",
			now.Add(-1*time.Second)).
		Where("created_at >= ?", now.Add(-1*time.Minute)).
		Scan(&stats).Error
	if err != nil {
		slog.Error("request logs realtime", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch realtime stats")
	}

	return &stats, nil
}

// Потолки параметров графика. Оба приходят из адресной строки: без них запрос на
// сто тысяч точек по столетнему шагу растягивает окно за пределы разумного (и за
// границы типа при умножении), а журнал снова читается целиком.
//
// Экран просит не больше шестидесяти точек с шагом до недели.
const (
	timelineMaxPoints   = 500
	timelineMaxInterval = 31 * 24 * 3600
)

// timelineWindow возвращает начало окна графика -- ровно те q.Limit интервалов,
// которые попадут в ответ.
//
// Окно обязательное. Без него запрос группировал весь журнал и отбрасывал всё,
// кроме последних точек, уже после чтения: на стенде это Parallel Seq Scan по
// тридцати восьми партициям каждые тридцать секунд ради двадцати четырёх чисел.
//
// Отсчёт идёт от границы интервала, а не от «сейчас»: точки лежат по границам
// интервалов, и окно должно накрывать целиком в том числе начатую сейчас.
func timelineWindow(q models.TimelineQuery, now time.Time) time.Time {
	base := now
	if t, err := time.Parse(time.RFC3339, q.To); err == nil {
		base = t
	}

	interval := int64(q.Interval)
	bucket := base.Unix() / interval * interval

	return time.Unix(bucket-int64(q.Limit-1)*interval, 0).UTC()
}

// GetTimeline возвращает точки для графика, сгруппированные по интервалу.
func (s *requestLogsService) GetTimeline(ctx context.Context, q models.TimelineQuery) ([]models.TimelinePoint, error) {
	if q.Interval < 1 {
		q.Interval = 60
	}
	if q.Interval > timelineMaxInterval {
		q.Interval = timelineMaxInterval
	}
	if q.Limit < 1 {
		q.Limit = 24
	}
	if q.Limit > timelineMaxPoints {
		q.Limit = timelineMaxPoints
	}

	tx := s.db.WithContext(ctx).Table("request_logs")

	// Нижняя граница есть всегда: заданная вызовом либо окно под запрошенные точки.
	from := timelineWindow(q, time.Now())
	if t, perr := time.Parse(time.RFC3339, q.From); perr == nil {
		from = t
	}
	tx = tx.Where("created_at >= ?", from)

	if q.To != "" {
		if t, err := time.Parse(time.RFC3339, q.To); err == nil {
			tx = tx.Where("created_at <= ?", t)
		}
	}

	bucketExpr := fmt.Sprintf(
		"to_timestamp(floor(EXTRACT(EPOCH FROM created_at) / %d) * %d)",
		q.Interval, q.Interval,
	)

	points := make([]models.TimelinePoint, 0)
	err := tx.
		Select(fmt.Sprintf(
			"to_char(%s, 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') AS timestamp, COUNT(*) AS count, "+
				"COALESCE(AVG(%s) FILTER (WHERE %s), 0) / 1000.0 AS avg_duration",
			bucketExpr, durationUsExpr, notStreamingSQL(),
		)).
		Group(bucketExpr).
		Order(bucketExpr + " DESC").
		Limit(q.Limit).
		Scan(&points).Error
	if err != nil {
		slog.Error("request logs timeline", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch timeline")
	}

	// Reverse to chronological order
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}

	return points, nil
}

// Export экспортирует логи в текстовый формат.
func (s *requestLogsService) Export(ctx context.Context, q models.RequestLogsQuery) (string, error) {
	// Снимаем лимит пагинации для экспорта, но ограничиваем 10000 записей
	q.Page = 1
	q.PerPage = 10000

	tx := s.db.WithContext(ctx).Table("request_logs")
	tx = s.applyFilters(tx, q)

	logs := make([]models.RequestLogs, 0)
	if err := tx.Order("created_at DESC").Limit(q.PerPage).Find(&logs).Error; err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, "failed to export request logs")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Request Logs Export (%d records)\n", len(logs)))
	sb.WriteString(strings.Repeat("=", 80) + "\n\n")

	for _, l := range logs {
		sb.WriteString(fmt.Sprintf("[%s] ", l.CreatedAt.Format("2006-01-02 15:04:05")))

		if l.Method != nil {
			sb.WriteString(*l.Method + " ")
		}
		if l.URL != nil {
			sb.WriteString(*l.URL)
		}
		sb.WriteString(" -> ")
		if l.ResponseStatus != nil {
			sb.WriteString(fmt.Sprintf("%d", *l.ResponseStatus))
		}
		if l.DurationMs != nil {
			sb.WriteString(fmt.Sprintf(" (%dms)", *l.DurationMs))
		}
		if l.Username != nil {
			sb.WriteString(fmt.Sprintf(" [%s]", *l.Username))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// GetHistory собирает агрегаты логов за период из request_logs_daily для вкладки
// «Аналитика»: итоги, ряд по дням, топ эндпоинтов и топ пользователей.
func (s *requestLogsService) GetHistory(ctx context.Context, q models.RequestLogsHistoryQuery) (*models.RequestLogsHistory, error) {
	from, to := historyRange(q.From, q.To)
	res := &models.RequestLogsHistory{
		Daily:        []models.HistoryDailyPoint{},
		TopEndpoints: []models.HistoryEndpoint{},
		TopUsers:     []models.HistoryUser{},
	}

	var tot struct {
		Requests int64
		Errors   int64
		AvgDur   float64
	}
	if err := s.db.WithContext(ctx).Table("request_logs_daily").
		Where("day BETWEEN ? AND ?", from, to).
		Select("COALESCE(SUM(request_count),0) AS requests, COALESCE(SUM(error_count),0) AS errors, COALESCE(AVG(avg_duration_ms),0) AS avg_dur").
		Scan(&tot).Error; err != nil {
		return nil, fmt.Errorf("history totals: %w", err)
	}
	res.Totals = models.HistoryTotals{Requests: tot.Requests, Errors: tot.Errors, AvgDuration: int(tot.AvgDur)}
	if tot.Requests > 0 {
		res.Totals.ErrorRate = float64(tot.Errors) / float64(tot.Requests) * 100
	}

	if err := s.db.WithContext(ctx).Table("request_logs_daily").
		Where("day BETWEEN ? AND ?", from, to).
		Select("day::text AS day, SUM(request_count) AS requests, SUM(error_count) AS errors").
		Group("day").Order("day").Scan(&res.Daily).Error; err != nil {
		return nil, fmt.Errorf("history daily: %w", err)
	}

	if err := s.db.WithContext(ctx).Table("request_logs_daily").
		Where("day BETWEEN ? AND ?", from, to).
		Select("endpoint, SUM(request_count) AS requests, ROUND(AVG(avg_duration_ms))::int AS avg_duration_ms, " +
			"MAX(p95_duration_ms) AS p95_duration_ms, " +
			"CASE WHEN SUM(request_count)>0 THEN ROUND(SUM(error_count)::numeric/SUM(request_count)*100,1) ELSE 0 END AS error_rate").
		Group("endpoint").Order("requests DESC").Limit(10).Scan(&res.TopEndpoints).Error; err != nil {
		return nil, fmt.Errorf("history top endpoints: %w", err)
	}

	if err := s.db.WithContext(ctx).Table("request_logs_daily d").
		Joins("LEFT JOIN users u ON u.id = d.user_id").
		Where("d.day BETWEEN ? AND ?", from, to).
		Select("d.user_id AS user_id, COALESCE(u.username, '-') AS username, SUM(d.request_count) AS requests").
		Group("d.user_id, u.username").Order("requests DESC").Limit(10).Scan(&res.TopUsers).Error; err != nil {
		return nil, fmt.Errorf("history top users: %w", err)
	}

	return res, nil
}

// historyRange парсит период истории; по умолчанию последние 90 дней.
func historyRange(fromStr, toStr string) (string, string) {
	const layout = "2006-01-02"
	to := time.Now().UTC()
	from := to.AddDate(0, 0, -90)
	if t, err := time.Parse(layout, toStr); err == nil {
		to = t
	}
	if f, err := time.Parse(layout, fromStr); err == nil {
		from = f
	}
	return from.Format(layout), to.Format(layout)
}
