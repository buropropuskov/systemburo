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
