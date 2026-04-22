package models

import "time"

type Notification struct {
	ID        int       `json:"id"`
	UserID    int       `gorm:"index" json:"user_id"`
	User      User      `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Type      *string   `gorm:"size:50" json:"type"`
	Title     *string   `gorm:"size:255" json:"title"`
	Message   *string   `gorm:"type:text" json:"message"`
	Data      *string   `gorm:"type:jsonb" json:"data"`
	IsRead    bool      `gorm:"default:false;index" json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

// MarkNotificationReadRequest -- тело запроса на изменение статуса прочтения уведомления.
type MarkNotificationReadRequest struct {
	IsRead bool `json:"is_read"`
}

// CreateNotificationRequest -- тело запроса на создание уведомления (admin-only).
type CreateNotificationRequest struct {
	UserID  int     `json:"user_id" validate:"required,min=1"`
	Type    *string `json:"type"`
	Title   *string `json:"title" validate:"required,max=255"`
	Message *string `json:"message"`
	Data    *string `json:"data"`
}
