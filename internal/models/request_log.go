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
