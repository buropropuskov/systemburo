package models

import "time"

// Mark - справочник марок автомобилей. Заменяет hardcoded список в VehicleForm.
// Архивируется через is_active=false: новые машины марку не выбирают, но у
// существующих cars.mark_name сохраняется snapshot имени на момент присвоения,
// поэтому переименование/архивация не ломает историю.
type Mark struct {
	ID              int       `json:"id"`
	Name            string    `gorm:"size:100" json:"name"`
	IsActive        bool      `gorm:"default:true;index" json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	CreatedByUserID *int      `json:"created_by_user_id,omitempty"`
}

// MarkActionType - константы action-типов истории марок.
const (
	MarkActionCreated  = "created"
	MarkActionRenamed  = "renamed"
	MarkActionArchived = "archived"
	MarkActionRestored = "restored"
)

// MarkHistoryItem - элемент истории марки с именем пользователя для UI.
// UserName собирается через COALESCE(ФИО, username) при выборке (LEFT JOIN users),
// чтобы timeline показывал кто сделал действие, а не только user_id.
type MarkHistoryItem struct {
	ID         int       `json:"id"`
	MarkID     int       `json:"mark_id"`
	ActionType string    `json:"action_type"`
	OldValue   *string   `json:"old_value,omitempty"`
	NewValue   *string   `json:"new_value,omitempty"`
	UserID     *int      `json:"user_id,omitempty"`
	UserName   string    `json:"user_name"`
	Comment    *string   `json:"comment,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// CreateMarkRequest - запрос на создание марки.
type CreateMarkRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

// UpdateMarkRequest - запрос на переименование марки.
type UpdateMarkRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}
