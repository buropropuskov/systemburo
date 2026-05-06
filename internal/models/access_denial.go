package models

import "time"

// AccessDenial -- запись в журнале отказов в доступе.
// Создаётся middleware при отказе в правах или попытке банёного юзера.
// Retention: 3 месяца в активной таблице, дальше переносится в access_denials_archive.
type AccessDenial struct {
	ID            int64     `gorm:"primaryKey" json:"id"`
	UserID        *int      `gorm:"index" json:"user_id"`
	User          *User     `gorm:"foreignKey:UserID;constraint:OnDelete:SET NULL" json:"user,omitempty"`
	Resource      string    `gorm:"size:255;index" json:"resource"`
	PermissionKey *string   `gorm:"size:255" json:"permission_key"`
	Reason        string    `gorm:"size:50;index" json:"reason"` // permission_denied | account_banned
	IPAddress     *string   `gorm:"size:45" json:"ip_address"`
	UserAgent     *string   `gorm:"size:500" json:"user_agent"`
	CreatedAt     time.Time `gorm:"index" json:"created_at"`
}

// AccessDenialArchive -- архивная таблица для записей старше retention.
// Та же схема что и AccessDenial, перенос делает cron.
type AccessDenialArchive struct {
	ID            int64     `gorm:"primaryKey" json:"id"`
	UserID        *int      `gorm:"index" json:"user_id"`
	Resource      string    `gorm:"size:255;index" json:"resource"`
	PermissionKey *string   `gorm:"size:255" json:"permission_key"`
	Reason        string    `gorm:"size:50;index" json:"reason"`
	IPAddress     *string   `gorm:"size:45" json:"ip_address"`
	UserAgent     *string   `gorm:"size:500" json:"user_agent"`
	CreatedAt     time.Time `gorm:"index" json:"created_at"`
	ArchivedAt    time.Time `json:"archived_at"`
}

// Reason константы для AccessDenial.Reason.
const (
	DenialReasonPermission = "permission_denied"
	DenialReasonBanned     = "account_banned"
)

// AccessDenialFilter -- фильтры запроса журнала.
type AccessDenialFilter struct {
	UserID   *int       `query:"user_id"`
	Resource *string    `query:"resource"`
	Reason   *string    `query:"reason"`
	From     *time.Time `query:"from"`
	To       *time.Time `query:"to"`
	Page     int        `query:"page"`
	Limit    int        `query:"limit"`
}

// AccessDenialResponse -- запись + ФИО юзера для UI.
type AccessDenialResponse struct {
	ID            int64     `json:"id"`
	UserID        *int      `json:"user_id"`
	UserName      string    `json:"user_name,omitempty"`
	Resource      string    `json:"resource"`
	PermissionKey *string   `json:"permission_key"`
	Reason        string    `json:"reason"`
	IPAddress     *string   `json:"ip_address"`
	UserAgent     *string   `json:"user_agent"`
	CreatedAt     time.Time `json:"created_at"`
}

// AccessDenialPageResponse -- ответ с пагинацией.
type AccessDenialPageResponse struct {
	Items []AccessDenialResponse `json:"items"`
	Total int64                  `json:"total"`
	Page  int                    `json:"page"`
	Limit int                    `json:"limit"`
}
