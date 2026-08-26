package models

import "time"

type PDAuditLog struct {
	ID         int       `json:"id"`
	UserID     *int      `gorm:"index" json:"user_id"`
	Username   string    `gorm:"size:100" json:"username"`
	Action     string    `gorm:"size:50;index" json:"action"`
	Resource   string    `gorm:"size:100;index" json:"resource"`
	ResourceID *int      `json:"resource_id"`
	IPAddress  string    `gorm:"size:45" json:"ip_address"`
	Method     string    `gorm:"size:10" json:"method"`
	Path       string    `gorm:"size:500" json:"path"`
	StatusCode int       `json:"status_code"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

func (PDAuditLog) TableName() string { return "pd_audit_logs" }

// PDAuditFilter -- фильтры страницы журнала доступа к персональным данным (#1472).
type PDAuditFilter struct {
	UserID     *int       `query:"user_id"`
	Username   *string    `query:"username"`
	Action     *string    `query:"action"`
	Resource   *string    `query:"resource"`
	OnlyDenied *bool      `query:"only_denied"`
	From       *time.Time `query:"from"`
	To         *time.Time `query:"to"`
	Page       int        `query:"page"`
	Limit      int        `query:"limit"`
}

// PDAuditResponse -- запись журнала с ФИО пользователя для экрана.
type PDAuditResponse struct {
	ID         int       `json:"id"`
	UserID     *int      `json:"user_id"`
	Username   string    `json:"username"`
	UserName   string    `json:"user_name,omitempty"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	IPAddress  string    `json:"ip_address"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	StatusCode int       `json:"status_code"`
	CreatedAt  time.Time `json:"created_at"`
}

// PDAuditPageResponse -- страница журнала.
type PDAuditPageResponse struct {
	Items []PDAuditResponse `json:"items"`
	Total int64             `json:"total"`
	Page  int               `json:"page"`
	Limit int               `json:"limit"`
}
