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
	ID             int64     `json:"id"`
	UserID         *int      `gorm:"index" json:"user_id"`
	Username       *string   `gorm:"size:100" json:"username"`
	Method         *string   `gorm:"size:10" json:"method"`
	URL            *string   `gorm:"size:500" json:"url"`
	Headers        *string   `gorm:"type:jsonb" json:"headers"`
	RequestBody    *string   `gorm:"type:text" json:"request_body"`
	ResponseStatus *int      `json:"response_status"`
	ResponseBody   *string   `gorm:"type:text" json:"response_body"`
	DurationMs     *int      `json:"duration_ms"`
	CreatedAt      time.Time `json:"created_at"`
}

func (RequestLogs) TableName() string { return "request_logs" }

// RequestLogsQuery — параметры фильтрации и пагинации для списка логов.
type RequestLogsQuery struct {
	UserID  *int   `query:"user_id"`
	Method  string `query:"method"`
	Status  *int   `query:"status"`
	From    string `query:"from_date"`
	To      string `query:"to_date"`
	Search  string `query:"search"`
	Page    int    `query:"page"`
	PerPage int    `query:"per_page"`
}

// RequestLogsStats — агрегированная статистика по логам запросов.
type RequestLogsStats struct {
	Total             int64   `json:"total"`
	Today             int64   `json:"today"`
	AvgDuration       float64 `json:"avg_duration"`
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

// TimelinePoint — одна точка на графике таймлайна.
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
type RequestLogsUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

// RequestLogsHistoryQuery — период для агрегатов истории (вкладка «Аналитика»).
type RequestLogsHistoryQuery struct {
	From string `query:"from_date"`
	To   string `query:"to_date"`
}

// RequestLogsHistory — агрегаты логов за период из request_logs_daily: итоги,
// ряд по дням и топы. Это вкладка «Аналитика» поверх свёрнутых дневных данных.
type RequestLogsHistory struct {
	Totals       HistoryTotals       `json:"totals"`
	Daily        []HistoryDailyPoint `json:"daily"`
	TopEndpoints []HistoryEndpoint   `json:"top_endpoints"`
	TopUsers     []HistoryUser       `json:"top_users"`
}

// HistoryTotals — сводные показатели за период.
type HistoryTotals struct {
	Requests    int64   `json:"requests"`
	Errors      int64   `json:"errors"`
	ErrorRate   float64 `json:"error_rate"`
	AvgDuration int     `json:"avg_duration_ms"`
}

// HistoryDailyPoint — точка ряда «запросы/ошибки по дню».
type HistoryDailyPoint struct {
	Day      string `json:"day"`
	Requests int64  `json:"requests"`
	Errors   int64  `json:"errors"`
}

// HistoryEndpoint — строка топа эндпоинтов.
type HistoryEndpoint struct {
	Endpoint    string  `json:"endpoint"`
	Requests    int64   `json:"requests"`
	AvgDuration int     `json:"avg_duration_ms"`
	P95Duration int     `json:"p95_duration_ms"`
	ErrorRate   float64 `json:"error_rate"`
}

// HistoryUser — строка топа пользователей.
type HistoryUser struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Requests int64  `json:"requests"`
}
