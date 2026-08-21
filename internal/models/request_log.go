package models

import "time"

type RequestLog struct {
	ID              int       `json:"id"`
	Timestamp       time.Time `gorm:"index" json:"timestamp"`
	Method          *string   `gorm:"size:10" json:"method"`
	Path            *string   `gorm:"size:500" json:"path"`
	QueryParams     *string   `gorm:"type:text" json:"query_params"`
	UserID          *int      `gorm:"index" json:"user_id"`
	Username        *string   `gorm:"size:100" json:"username"`
	IPAddress       *string   `gorm:"size:45" json:"ip_address"`
	UserAgent       *string   `gorm:"type:text" json:"user_agent"`
	RequestBody     *string   `gorm:"type:text" json:"request_body"`
	RequestHeaders  *string   `gorm:"type:text" json:"request_headers"`
	ResponseStatus  *int      `json:"response_status"`
	ResponseBody    *string   `gorm:"type:text" json:"response_body"`
	ResponseHeaders *string   `gorm:"type:text" json:"response_headers"`
	DurationMs      *int      `json:"duration_ms"`
	ErrorMessage    *string   `gorm:"type:text" json:"error_message"`
}

func (RequestLog) TableName() string { return "request_log" }

type RequestLogs struct {
	ID             int64   `json:"id"`
	UserID         *int    `gorm:"index" json:"user_id"`
	Username       *string `gorm:"size:100" json:"username"`
	Method         *string `gorm:"size:10" json:"method"`
	URL            *string `gorm:"size:500" json:"url"`
	Headers        *string `gorm:"type:jsonb" json:"headers"`
	RequestBody    *string `gorm:"type:text" json:"request_body"`
	ResponseStatus *int    `json:"response_status"`
	ResponseBody   *string `gorm:"type:text" json:"response_body"`
	DurationMs     *int    `json:"duration_ms"`
	// DurationUs -- длительность в микросекундах. Миллисекунды округлены вниз, и
	// треть запросов отвечает быстрее миллисекунды: по ним перцентили вырождались
	// в ноль (#2125). Указатель, потому что у записей до перехода значения нет.
	DurationUs *int64    `json:"duration_us"`
	CreatedAt  time.Time `json:"created_at"`
}

func (RequestLogs) TableName() string { return "request_logs" }

// RequestLogsQuery — параметры фильтрации, сортировки и пагинации для списка
// логов. Sort и Order приходят из адресной строки: порядок строк должен
// переживать обновление страницы и пересылку ссылки.
type RequestLogsQuery struct {
	UserID *int   `query:"user_id"`
	Method string `query:"method"`
	Status *int   `query:"status"`
	// StatusMin и StatusMax -- границы кода ответа для быстрого отбора «только
	// ошибки» и разбора отдельного класса статусов. Точный Status для этого не
	// годится: ошибок в журнале десяток разных кодов, и перебирать их вручную
	// оператор не станет.
	StatusMin *int `query:"status_min"`
	StatusMax *int `query:"status_max"`
	// MinDurationMs -- нижняя граница времени ответа в миллисекундах: отбор
	// «медленнее секунды» ищет затыки, а не листает журнал целиком.
	MinDurationMs *int   `query:"min_duration_ms"`
	From          string `query:"from_date"`
	To            string `query:"to_date"`
	Search        string `query:"search"`
	Sort          string `query:"sort"`
	Order         string `query:"order"`
	Page          int    `query:"page"`
	PerPage       int    `query:"per_page"`
}

// RequestLogsExport -- выборка журнала под выгрузку файлом вместе с числами
// охвата. Total и Truncated нужны на экране и в самом файле: до #2125 выгрузка
// молча отдавала первые 10 000 записей, и человек получал файл, по которому
// считал итоги за период, не зная, что период в него не поместился.
type RequestLogsExport struct {
	Rows      []RequestLogs
	Total     int64
	Limit     int
	Truncated bool
}

// RequestLogsStats — агрегированная статистика по логам запросов. Длительности
// в миллисекундах с дробной частью: считаются по микросекундной колонке и не
// округляются до нуля на быстрых ответах.
//
// Долгоживущие соединения (подписка на события) в длительности не учитываются:
// у них в журнале записано время жизни соединения, и одно такое перевешивало
// десятки тысяч обычных ответов, задирая среднее до секунд.
type RequestLogsStats struct {
	Total             int64   `json:"total"`
	Today             int64   `json:"today"`
	AvgDuration       float64 `json:"avg_duration"`
	MedianDuration    float64 `json:"median_duration"`
	P95Duration       float64 `json:"p95_duration"`
	ErrorRate         float64 `json:"error_rate"`
	RequestsPerMinute float64 `json:"requests_per_minute"`
}

// TimelineQuery — параметры для построения таймлайна.
type TimelineQuery struct {
	Interval int    `query:"interval"`
	Limit    int    `query:"limit"`
	From     string `query:"from_date"`
	To       string `query:"to_date"`
}

// TimelinePoint — одна точка на графике таймлайна. Длительность в миллисекундах
// с дробной частью: считается по микросекундной колонке и без долгоживущих
// соединений, как и показатели шапки (#2125).
type TimelinePoint struct {
	Timestamp   string  `json:"timestamp"`
	Count       int64   `json:"count"`
	AvgDuration float64 `json:"avg_duration"`
}

// RealtimeStats — статистика в реальном времени.
type RealtimeStats struct {
	LastSecondCount int64 `json:"last_second_count"`
	LastMinuteCount int64 `json:"last_minute_count"`
}

// RequestLogsUser — пользователь для фильтра.
//
// IsActive нужен экрану, чтобы отделить уволенных: их обращения в журнале
// остаются, и выбрать такого работника кликом должно быть можно.
type RequestLogsUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

// RequestLogsHistoryQuery — период для агрегатов истории (вкладка «Аналитика»).
type RequestLogsHistoryQuery struct {
	From string `query:"from_date"`
	To   string `query:"to_date"`
}

// RequestLogsHistory — показатели журнала за период для вкладки «Аналитика».
// Свёрнутые сутки берутся из request_logs_daily, дни новее последней свёртки —
// из детальных партиций request_logs, поэтому последний месяц больше не
// пропадает с экрана молча (#2125).
type RequestLogsHistory struct {
	Totals       HistoryTotals       `json:"totals"`
	Coverage     HistoryCoverage     `json:"coverage"`
	Daily        []HistoryDailyPoint `json:"daily"`
	TopEndpoints []HistoryEndpoint   `json:"top_endpoints"`
	TopUsers     []HistoryUser       `json:"top_users"`
}

// HistoryCoverage — фактический охват ответа: какие сутки реально попали в
// расчёт и откуда взяты числа. Запрошенный период и охват расходятся почти
// всегда: журнал моложе периода, свёртка отстаёт, часть суток пустая.
type HistoryCoverage struct {
	RequestedFrom string `json:"requested_from"`
	RequestedTo   string `json:"requested_to"`
	// From и To — первый и последний день с данными; пустые, когда данных нет.
	From string `json:"from"`
	To   string `json:"to"`
	Days int    `json:"days"`
	// Source — откуда пришли числа: empty, aggregates, detailed или mixed.
	Source string `json:"source"`
	// AggregatedThrough — последний свёрнутый день. Всё, что новее, читается из
	// детальных партиций.
	AggregatedThrough string `json:"aggregated_through"`
	// ExactP95 — перцентиль посчитан по самим записям. У свёрнутых суток
	// отдельных длительностей уже нет, и перцентиль периода по ним честно не
	// считается: показывается наибольшее суточное значение.
	ExactP95 bool `json:"exact_p95"`
}

// HistoryTotals — сводные показатели за период. Средняя длительность взвешена
// по числу запросов: среднее суточных средних приписывало тихой ночи тот же вес,
// что рабочему дню. Долгоживущие соединения в неё не входят, как и в шапке.
type HistoryTotals struct {
	Requests    int64   `json:"requests"`
	Errors      int64   `json:"errors"`
	ErrorRate   float64 `json:"error_rate"`
	AvgDuration float64 `json:"avg_duration_ms"`
}

// HistoryDailyPoint — точка ряда «запросы/ошибки по дню».
type HistoryDailyPoint struct {
	Day      string `json:"day"`
	Requests int64  `json:"requests"`
	Errors   int64  `json:"errors"`
}

// HistoryEndpoint — строка топа эндпоинтов. Длительности в миллисекундах с
// дробной частью: ответы быстрее миллисекунды не должны выглядеть нулевыми.
type HistoryEndpoint struct {
	Endpoint    string  `json:"endpoint"`
	Requests    int64   `json:"requests"`
	AvgDuration float64 `json:"avg_duration_ms"`
	P95Duration float64 `json:"p95_duration_ms"`
	ErrorRate   float64 `json:"error_rate"`
}

// HistoryUser — строка топа пользователей.
type HistoryUser struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Requests int64  `json:"requests"`
}
