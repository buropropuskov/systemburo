package models

import "time"

// ApplicationRead фиксирует прочтение заявки пользователем.
type ApplicationRead struct {
	ID            int       `json:"id"`
	ApplicationID int       `gorm:"uniqueIndex:idx_app_user_read" json:"application_id"`
	UserID        int       `gorm:"uniqueIndex:idx_app_user_read" json:"user_id"`
	ReadAt        time.Time `gorm:"autoCreateTime" json:"read_at"`
}

// ApplicationReadResponse ответ с информацией о прочтении.
type ApplicationReadResponse struct {
	UserID    int       `json:"user_id"`
	Username  string    `json:"username"`
	LastName  *string   `json:"last_name"`
	FirstName *string   `json:"first_name"`
	ReadAt    time.Time `json:"read_at"`
}

// UnreadCountResponse количество непрочитанных заявок.
type UnreadCountResponse struct {
	Count int `json:"count"`
}
