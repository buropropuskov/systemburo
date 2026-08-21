package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"systemburo/internal/database"
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
	Export(ctx context.Context, q models.RequestLogsQuery) (models.RequestLogsExport, error)
}

type requestLogsService struct {
	db          *gorm.DB
	stats       *statsCache
	exportLimit int
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

// WithRequestLogsExportLimit задаёт свой потолок строк в выгрузке. Нужен тестам:
// проверять обрезку на настоящих десяти тысячах записей дороже, чем на двух.
func WithRequestLogsExportLimit(limit int) RequestLogsOption {
	return func(s *requestLogsService) {
		if limit > 0 {
			s.exportLimit = limit
		}
	}
}

// NewRequestLogsService создаёт реализацию RequestLogsService.
func NewRequestLogsService(db *gorm.DB, opts ...RequestLogsOption) RequestLogsService {
	s := &requestLogsService{db: db, stats: &statsCache{ttl: statsCacheTTL}, exportLimit: exportMaxRows}
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

// logDayLayout -- день без времени, как его шлёт поле даты на экране.
const logDayLayout = "2006-01-02"

// parseLogBound разбирает границу периода и говорит, пришёл ли день целиком.
// С экрана приходит день (2026-08-19), из присланных ссылок -- момент по
// RFC 3339.
//
// День считается московскими сутками: created_at хранится с зоной, и «за
// 19 августа» для оператора это местные сутки, а не UTC-шные. Раньше день
// вообще не разбирался -- фильтры «с» и «по» молча не применялись, список
// оставался за всё время.
func parseLogBound(value string) (moment time.Time, wholeDay bool, ok bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false, false
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, false, true
	}
	if d, err := time.ParseInLocation(logDayLayout, value, AnalyticsLocation()); err == nil {
		return d, true, true
	}
	return time.Time{}, false, false
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
	if q.StatusMin != nil {
		tx = tx.Where("response_status >= ?", *q.StatusMin)
	}
	if q.StatusMax != nil {
		tx = tx.Where("response_status <= ?", *q.StatusMax)
	}
	if q.MinDurationMs != nil && *q.MinDurationMs > 0 {
		// Долгоживущие соединения из отбора «медленнее секунды» исключаются: у
		// подписки на события записано время жизни соединения, и она заняла бы
		// собой весь список, вытеснив настоящие затыки.
		tx = tx.Where(durationUsExpr+" >= ?", int64(*q.MinDurationMs)*1000).
			Where(notStreamingSQL())
	}
	if from, _, ok := parseLogBound(q.From); ok {
		tx = tx.Where("created_at >= ?", from)
	}
	if to, wholeDay, ok := parseLogBound(q.To); ok {
		// Выбранный день включается целиком, поэтому сравнение идёт с началом
		// следующих суток: «по 19 августа» с обычным <= отсекало всё после
		// полуночи выбранного дня.
		if wholeDay {
			tx = tx.Where("created_at < ?", to.AddDate(0, 0, 1))
		} else {
			tx = tx.Where("created_at <= ?", to)
		}
	}
	if q.Search != "" {
		pattern := "%" + q.Search + "%"
		tx = tx.Where("(url ILIKE ? OR username ILIKE ?)", pattern, pattern)
	}
	return tx
}

// logSortColumns -- поля, по которым журнал разрешено упорядочивать, и то, что
// уходит в ORDER BY. Ключ приходит из адресной строки, поэтому в запрос
// попадает только выражение из этой карты, а не пришедшая строка.
var logSortColumns = map[string]string{
	"created_at": "created_at",
	"method":     "method",
	"url":        "url",
	"status":     "response_status",
	"username":   "username",
	"duration":   durationUsExpr,
}

// applySort добавляет порядок строк. Неизвестное поле и пустой запрос дают
// привычный «сначала свежие».
func applySort(tx *gorm.DB, q models.RequestLogsQuery) *gorm.DB {
	column, ok := logSortColumns[strings.ToLower(strings.TrimSpace(q.Sort))]
	if !ok {
		column = "created_at"
	}

	direction := "DESC"
	if strings.EqualFold(strings.TrimSpace(q.Order), "asc") {
		direction = "ASC"
	}

	// Пустые значения (имя у неавторизованного обращения, статус у оборванного
	// соединения) уходят в конец в обе стороны, иначе первая страница сортировки
	// по имени состоит из прочерков.
	//
	// Идентификатор вторым ключом: у метода и статуса значения повторяются
	// тысячами, и без устойчивого порядка соседние страницы показывают одни и те
	// же строки, теряя другие.
	return tx.Order(column + " " + direction + " NULLS LAST").Order("id DESC")
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
	if err := applySort(tx, q).Offset(offset).Limit(q.PerPage).Find(&logs).Error; err != nil {
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
// Архивные учётные записи из списка не убираются, а уходят в его конец (#2191):
// обращения уволенного в журнале остаются, и разбор происшествия с его участием -
// ровно тот случай, ради которого журнал держат. Пока их прятали, выбрать такого
// работника кликом было нельзя, оставался поиск по имени, а имя надо знать заранее.
// Соединения с журналом здесь нет намеренно: в системе сотня учётных записей, а
// проверка «обращался ли он хоть раз» стоила бы скана всех партиций.
func (s *requestLogsService) GetUsers(ctx context.Context) ([]models.RequestLogsUser, error) {
	users := make([]models.RequestLogsUser, 0)
	err := s.db.WithContext(ctx).
		Table("users").
		Select("id, username, is_active").
		Order("is_active DESC, username").
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

// timelineAggregateInterval -- шаг, начиная с которого график читается вместе со
// свёрткой. Подробные записи живут REQUEST_LOG_DETAIL_DAYS суток (по умолчанию
// тридцать), поэтому «месяц» упирался в срок хранения, а «год» показывал четыре
// последние недели и пустоту до них -- молча, как будто запросов не было (#2125).
const timelineAggregateInterval = 24 * 3600

// timelineBucketSQL -- выражение начала интервала, в который попадает момент.
// Шаг приходит числом из уже нормализованного запроса, не строкой пользователя.
func timelineBucketSQL(column string, interval int) string {
	return fmt.Sprintf("to_timestamp(floor(EXTRACT(EPOCH FROM %s) / %d) * %d)", column, interval, interval)
}

// timelineStamp -- метка точки в ответе. Соединение работает в UTC
// (database.EnsureUTCTimezone), поэтому суффикс зоны проставляется литералом.
const timelineStamp = `to_char(%s, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')`

// GetTimeline возвращает точки для графика, сгруппированные по интервалу.
//
// Шаг мельче суток читается по подробным записям, шаг от суток -- по свёртке
// вместе с ещё не свёрнутыми сутками: иначе крупные периоды показывают не
// историю, а срок хранения подробностей.
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

	// Нижняя граница есть всегда: заданная вызовом либо окно под запрошенные точки.
	from := timelineWindow(q, time.Now())
	if t, perr := time.Parse(time.RFC3339, q.From); perr == nil {
		from = t
	}
	to, hasTo := time.Time{}, false
	if t, err := time.Parse(time.RFC3339, q.To); err == nil {
		to, hasTo = t, true
	}

	if q.Interval >= timelineAggregateInterval {
		if !hasTo {
			to = time.Now()
		}
		return s.timelineWithAggregates(ctx, q, from, to)
	}
	return s.timelineDetailed(ctx, q, from, to, hasTo)
}

// timelineDetailed строит график по подробным записям журнала.
func (s *requestLogsService) timelineDetailed(
	ctx context.Context, q models.TimelineQuery, from, to time.Time, hasTo bool,
) ([]models.TimelinePoint, error) {
	tx := s.db.WithContext(ctx).Table("request_logs").Where("created_at >= ?", from)
	if hasTo {
		tx = tx.Where("created_at <= ?", to)
	}

	bucketExpr := timelineBucketSQL("created_at", q.Interval)

	points := make([]models.TimelinePoint, 0)
	err := tx.
		Select(fmt.Sprintf(
			timelineStamp+" AS timestamp, COUNT(*) AS count, "+
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

	return timelineChronological(points), nil
}

// timelineWithAggregates строит график по суточной свёртке и добавляет к ней
// сутки, которые свернуться ещё не успели.
//
// Сутки, уже попавшие в свёртку, из подробной части выбрасываются
// (notRolledUpSQL): свёрнутая, но не удалённая партиция иначе удвоила бы столбик.
//
// Средняя длительность точки взвешена по числу запросов: среднее суточных
// средних приписывает тихой ночи вес рабочего дня. Приведение к numeric
// обязательно -- деление bigint на bigint в Postgres целочисленное.
func (s *requestLogsService) timelineWithAggregates(
	ctx context.Context, q models.TimelineQuery, from, to time.Time,
) ([]models.TimelinePoint, error) {
	notStreamAgg := notStreamingEndpointSQL()
	aggSQL := `
		SELECT ` + timelineBucketSQL("day", q.Interval) + ` AS bucket,
			SUM(request_count) AS requests,
			COALESCE(SUM(request_count) FILTER (WHERE ` + notStreamAgg + `), 0) AS dur_weight,
			COALESCE(SUM(avg_duration_us * request_count) FILTER (WHERE ` + notStreamAgg + `), 0) AS dur_sum_us
		FROM request_logs_daily WHERE day >= ? AND day <= ? GROUP BY 1`

	notStream := notStreamingSQL()
	detSQL := `
		SELECT ` + timelineBucketSQL("created_at", q.Interval) + ` AS bucket,
			COUNT(*) AS requests,
			COUNT(*) FILTER (WHERE ` + notStream + `) AS dur_weight,
			COALESCE(SUM(` + durationUsExpr + `) FILTER (WHERE ` + notStream + `), 0) AS dur_sum_us
		FROM request_logs WHERE created_at >= ? AND created_at <= ? AND ` + notRolledUpSQL + ` GROUP BY 1`

	query := fmt.Sprintf(`SELECT `+timelineStamp+` AS timestamp,
			SUM(requests) AS count,
			CASE WHEN SUM(dur_weight) > 0
				THEN SUM(dur_sum_us)::numeric / SUM(dur_weight) / 1000.0
				ELSE 0 END AS avg_duration
		FROM (%s
		UNION ALL%s
		) t GROUP BY bucket ORDER BY bucket DESC LIMIT %d`, "bucket", aggSQL, detSQL, q.Limit)

	args := []any{
		from.Format(dayLayout), to.Format(dayLayout),
		from, to,
		from.Format(dayLayout),
	}

	points := make([]models.TimelinePoint, 0)
	if err := s.db.WithContext(ctx).Raw(query, args...).Scan(&points).Error; err != nil {
		slog.Error("request logs timeline aggregates", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch timeline")
	}

	return timelineChronological(points), nil
}

// timelineChronological разворачивает точки в порядок «сначала старые»: запрос
// берёт последние интервалы, а график рисует слева направо.
func timelineChronological(points []models.TimelinePoint) []models.TimelinePoint {
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}
	return points
}

// exportMaxRows -- потолок строк в одной выгрузке. Файл собирается в памяти
// целиком (excelize), а журнал на стенде -- треть миллиона записей за месяц:
// без потолка одна кнопка кладёт процесс. Потолок остаётся, но перестаёт быть
// тихим -- сколько записей отсечено, видно и в файле, и на экране.
const exportMaxRows = 10000

// Export отдаёт выборку журнала под выгрузку: те же фильтры и тот же порядок,
// что на экране, плюс полное число подходящих записей. Рендер файла -- дело
// обработчика, здесь только данные.
func (s *requestLogsService) Export(ctx context.Context, q models.RequestLogsQuery) (models.RequestLogsExport, error) {
	q.Page = 1
	q.PerPage = s.exportLimit

	tx := s.db.WithContext(ctx).Table("request_logs")
	tx = s.applyFilters(tx, q)

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return models.RequestLogsExport{}, echo.NewHTTPError(http.StatusInternalServerError, "failed to count request logs")
	}

	logs := make([]models.RequestLogs, 0)
	if err := applySort(tx, q).Limit(q.PerPage).Find(&logs).Error; err != nil {
		return models.RequestLogsExport{}, echo.NewHTTPError(http.StatusInternalServerError, "failed to export request logs")
	}

	return models.RequestLogsExport{
		Rows:      logs,
		Total:     total,
		Limit:     s.exportLimit,
		Truncated: total > int64(len(logs)),
	}, nil
}

// dayLayout -- формат суток в параметрах и ответах истории.
const dayLayout = "2006-01-02"

// notStreamingEndpointSQL -- то же условие «это обычный запрос», что и
// notStreamingSQL, но по свёрнутому маршруту: в агрегатах адрес уже нормализован
// и хранится без query-строки.
func notStreamingEndpointSQL() string {
	if len(streamingLogPaths) == 0 {
		return "TRUE"
	}
	quoted := make([]string, 0, len(streamingLogPaths))
	for _, p := range streamingLogPaths {
		quoted = append(quoted, "'"+p+"'")
	}
	return "endpoint NOT IN (" + strings.Join(quoted, ", ") + ")"
}

// notRolledUpSQL -- «эти сутки ещё не свёрнуты». Детальная выборка отбрасывает
// дни, по которым уже есть агрегат, поэтому свёрнутая, но не удалённая партиция
// не удваивает итог. Отбор идёт по факту наличия дня в свёртке, а не по её
// верхней границе: партиции сворачиваются в порядке, который база не обещает, и
// сорвавшаяся старая партиция при успешной новой оставляла бы день, которого нет
// ни в агрегатах, ни в детальной части.
//
// Проверка идёт по первичному ключу агрегатов, где day стоит первым столбцом.
const notRolledUpSQL = `NOT EXISTS (
			SELECT 1 FROM request_logs_daily d
			WHERE d.day = request_logs.created_at::date AND d.day >= ?)`

// historyUnion склеивает выборку по свёрнутым суткам с выборкой по детальным
// партициям и собирает аргументы в порядке плейсхолдеров: границы агрегатов
// (сутками), окно детальной части (правая граница исключающая) и нижняя граница
// проверки на свёртку -- она держит сравнение в пределах периода.
func historyUnion(from, to time.Time, aggSQL, detSQL string) (string, []any) {
	return aggSQL + "\nUNION ALL\n" + detSQL,
		[]any{from.Format(dayLayout), to.Format(dayLayout), from, to.AddDate(0, 0, 1), from.Format(dayLayout)}
}

// GetHistory собирает показатели журнала за период для вкладки «Аналитика».
//
// Свёрнутые сутки читаются из request_logs_daily, сутки без свёртки -- из
// детальных партиций. До этого вкладка знала только агрегаты, и последний месяц
// пропадал с неё молча: свёртка отстаёт на срок хранения подробностей (#2125).
//
// Средние взвешены по числу запросов. Среднее суточных средних приписывало
// выходным тот же вес, что рабочему дню, и итог периода не сходился ни с одним
// днём ряда.
func (s *requestLogsService) GetHistory(ctx context.Context, q models.RequestLogsHistoryQuery) (*models.RequestLogsHistory, error) {
	from, to := historyRange(q.From, q.To)
	res := &models.RequestLogsHistory{
		Daily:        []models.HistoryDailyPoint{},
		TopEndpoints: []models.HistoryEndpoint{},
		TopUsers:     []models.HistoryUser{},
		Coverage: models.HistoryCoverage{
			RequestedFrom: from.Format(dayLayout),
			RequestedTo:   to.Format(dayLayout),
			Source:        "empty",
			ExactP95:      true,
		},
	}

	aggThrough, err := s.aggregatedThrough(ctx)
	if err != nil {
		return nil, err
	}
	if aggThrough != nil {
		res.Coverage.AggregatedThrough = aggThrough.Format(dayLayout)
	}

	if err := s.historyDaily(ctx, from, to, res); err != nil {
		return nil, err
	}
	if err := s.historyEndpoints(ctx, from, to, res); err != nil {
		return nil, err
	}
	if err := s.historyUsers(ctx, from, to, res); err != nil {
		return nil, err
	}
	return res, nil
}

// aggregatedThrough возвращает последний свёрнутый день. nil -- журнал ещё ни
// разу не сворачивался, и весь период читается из детальных партиций.
func (s *requestLogsService) aggregatedThrough(ctx context.Context) (*time.Time, error) {
	// NullTime, а не указатель: на стенде без свёрток MAX отдаёт NULL, и обычный
	// time.Time на нём падает разбором, роняя весь раздел пятисотой.
	var row struct{ Day sql.NullTime }
	if err := s.db.WithContext(ctx).Table("request_logs_daily").
		Select("MAX(day) AS day").Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("history aggregated through: %w", err)
	}
	if !row.Day.Valid {
		return nil, nil
	}
	utc := row.Day.Time.UTC()
	utc = time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	return &utc, nil
}

// historyDaily читает ряд по суткам и сводит из него итоги периода и охват.
func (s *requestLogsService) historyDaily(ctx context.Context, from, to time.Time, res *models.RequestLogsHistory) error {
	aggSQL := `
		SELECT day::text AS day, 'aggregates' AS source,
			SUM(request_count) AS requests,
			SUM(error_count)   AS errors,
			COALESCE(SUM(request_count) FILTER (WHERE ` + notStreamingEndpointSQL() + `), 0) AS dur_weight,
			COALESCE(SUM(avg_duration_us * request_count) FILTER (WHERE ` + notStreamingEndpointSQL() + `), 0) AS dur_sum_us
		FROM request_logs_daily WHERE day BETWEEN ? AND ? GROUP BY day`
	detSQL := `
		SELECT created_at::date::text AS day, 'detailed' AS source,
			COUNT(*) AS requests,
			COUNT(*) FILTER (WHERE response_status >= 400) AS errors,
			COUNT(*) FILTER (WHERE ` + notStreamingSQL() + `) AS dur_weight,
			COALESCE(SUM(` + durationUsExpr + `) FILTER (WHERE ` + notStreamingSQL() + `), 0) AS dur_sum_us
		FROM request_logs WHERE created_at >= ? AND created_at < ? AND ` + notRolledUpSQL + ` GROUP BY 1`

	union, args := historyUnion(from, to, aggSQL, detSQL)

	var rows []struct {
		Day       string
		Source    string
		Requests  int64
		Errors    int64
		DurWeight int64
		DurSumUs  float64
	}
	if err := s.db.WithContext(ctx).Raw(
		"SELECT * FROM (\n"+union+"\n) t ORDER BY day", args...).Scan(&rows).Error; err != nil {
		return fmt.Errorf("history daily: %w", err)
	}

	var totals models.HistoryTotals
	var durWeight, durSum float64
	var aggRequests, detRequests int64

	for _, r := range rows {
		if n := len(res.Daily); n > 0 && res.Daily[n-1].Day == r.Day {
			res.Daily[n-1].Requests += r.Requests
			res.Daily[n-1].Errors += r.Errors
		} else {
			res.Daily = append(res.Daily, models.HistoryDailyPoint{
				Day: r.Day, Requests: r.Requests, Errors: r.Errors,
			})
		}
		totals.Requests += r.Requests
		totals.Errors += r.Errors
		durWeight += float64(r.DurWeight)
		durSum += r.DurSumUs
		if r.Source == "aggregates" {
			aggRequests += r.Requests
		} else {
			detRequests += r.Requests
		}
	}

	if totals.Requests > 0 {
		totals.ErrorRate = float64(totals.Errors) / float64(totals.Requests) * 100
	}
	if durWeight > 0 {
		totals.AvgDuration = usToMs(durSum / durWeight)
	}
	res.Totals = totals

	res.Coverage.Days = len(res.Daily)
	res.Coverage.Source = historySource(aggRequests, detRequests)
	res.Coverage.ExactP95 = aggRequests == 0
	if len(res.Daily) > 0 {
		res.Coverage.From = res.Daily[0].Day
		res.Coverage.To = res.Daily[len(res.Daily)-1].Day
	}
	return nil
}

// historySource называет, откуда пришли числа ответа.
func historySource(aggRequests, detRequests int64) string {
	switch {
	case aggRequests > 0 && detRequests > 0:
		return "mixed"
	case aggRequests > 0:
		return "aggregates"
	case detRequests > 0:
		return "detailed"
	default:
		return "empty"
	}
}

// historyEndpoints собирает топ маршрутов. Свёрнутые сутки и детальные записи
// объединяются ДО группировки: иначе один маршрут пришёл бы на экран двумя
// строками -- одной из агрегатов, другой из свежих суток.
//
// Перцентиль по свёрнутым суткам честно не считается: отдельных длительностей
// там уже нет, остаётся наибольшее суточное значение. Что именно показано,
// экран берёт из coverage.exact_p95.
func (s *requestLogsService) historyEndpoints(ctx context.Context, from, to time.Time, res *models.RequestLogsHistory) error {
	aggSQL := `
		SELECT endpoint, request_count AS requests, error_count AS errors,
			avg_duration_us, p95_duration_us
		FROM request_logs_daily WHERE day BETWEEN ? AND ?`
	detSQL := `
		SELECT ` + database.LogEndpointExpr + ` AS endpoint,
			COUNT(*) AS requests,
			COUNT(*) FILTER (WHERE response_status >= 400) AS errors,
			COALESCE(AVG(` + durationUsExpr + `), 0)::bigint AS avg_duration_us,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY ` + durationUsExpr + `), 0)::bigint AS p95_duration_us
		FROM request_logs WHERE created_at >= ? AND created_at < ? AND ` + notRolledUpSQL + ` GROUP BY 1`

	union, args := historyUnion(from, to, aggSQL, detSQL)

	var rows []struct {
		Endpoint string
		Requests int64
		Errors   int64
		AvgUs    float64
		P95Us    float64
	}
	query := `
		SELECT endpoint,
			SUM(requests) AS requests,
			SUM(errors)   AS errors,
			CASE WHEN SUM(requests) > 0
				THEN SUM(avg_duration_us * requests)::numeric / SUM(requests) ELSE 0 END AS avg_us,
			MAX(p95_duration_us) AS p95_us
		FROM (
` + union + `
		) t GROUP BY endpoint ORDER BY requests DESC LIMIT 10`
	if err := s.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return fmt.Errorf("history top endpoints: %w", err)
	}

	for _, r := range rows {
		e := models.HistoryEndpoint{
			Endpoint:    r.Endpoint,
			Requests:    r.Requests,
			AvgDuration: usToMs(r.AvgUs),
			P95Duration: usToMs(r.P95Us),
		}
		if r.Requests > 0 {
			e.ErrorRate = math.Round(float64(r.Errors)/float64(r.Requests)*1000) / 10
		}
		res.TopEndpoints = append(res.TopEndpoints, e)
	}
	return nil
}

// historyUsers собирает топ учётных записей за период.
func (s *requestLogsService) historyUsers(ctx context.Context, from, to time.Time, res *models.RequestLogsHistory) error {
	aggSQL := `
		SELECT user_id, request_count AS requests
		FROM request_logs_daily WHERE day BETWEEN ? AND ?`
	detSQL := `
		SELECT COALESCE(user_id, 0) AS user_id, COUNT(*) AS requests
		FROM request_logs WHERE created_at >= ? AND created_at < ? AND ` + notRolledUpSQL + ` GROUP BY 1`

	union, args := historyUnion(from, to, aggSQL, detSQL)

	query := `
		SELECT t.user_id AS user_id, COALESCE(u.username, '-') AS username, SUM(t.requests) AS requests
		FROM (
` + union + `
		) t LEFT JOIN users u ON u.id = t.user_id
		GROUP BY t.user_id, u.username ORDER BY requests DESC LIMIT 10`
	if err := s.db.WithContext(ctx).Raw(query, args...).Scan(&res.TopUsers).Error; err != nil {
		return fmt.Errorf("history top users: %w", err)
	}
	return nil
}

// historyRange парсит период истории; по умолчанию последние 90 дней. Период,
// заданный наоборот, разворачивается: пустой ответ на осмысленный запрос
// читается как поломка раздела.
func historyRange(fromStr, toStr string) (time.Time, time.Time) {
	to := time.Now().UTC().Truncate(24 * time.Hour)
	if t, err := time.Parse(dayLayout, toStr); err == nil {
		to = t.UTC()
	}

	from := to.AddDate(0, 0, -90)
	if f, err := time.Parse(dayLayout, fromStr); err == nil {
		from = f.UTC()
	}
	if from.After(to) {
		from, to = to, from
	}
	return from, to
}
