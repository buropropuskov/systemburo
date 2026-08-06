package models

import "time"

type Notification struct {
	ID     int     `json:"id"`
	UserID int     `gorm:"index;index:idx_notification_group,priority:1" json:"user_id"`
	User   User    `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Type   *string `gorm:"size:50;index:idx_notification_group,priority:2" json:"type"`
	Title  *string `gorm:"size:255" json:"title"`
	// GroupKey -- ключ схлопывания повторов одного типа в одну запись (#1748).
	// Заполняется агрегацией в следующем срезе; пустой -- уведомление не схлопывается.
	GroupKey *string `gorm:"size:120;index:idx_notification_group,priority:3" json:"group_key,omitempty"`
	Message  *string `gorm:"type:text" json:"message"`
	Data     *string `gorm:"type:jsonb" json:"data"`
	IsRead   bool    `gorm:"default:false;index;index:idx_notification_group,priority:4" json:"is_read"`
	// Count -- сколько событий схлопнуто в эту запись. 1 для обычного, не схлопнутого
	// уведомления.
	Count int `gorm:"default:1" json:"count"`
	// LastEventAt -- момент последнего схлопнутого события. Намеренно НЕ UpdatedAt:
	// gorm трогает UpdatedAt на любой Update (в т.ч. отметку "прочитано"), и уведомление
	// всплывало бы наверх ленты от простого прочтения, а не от нового события.
	LastEventAt *time.Time `json:"last_event_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
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
