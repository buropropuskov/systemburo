package models

import "time"

// Mark - справочник марок автомобилей. Заменяет hardcoded список в VehicleForm.
// Архивируется через is_active=false: новые машины марку не выбирают, но у
// существующих cars.mark_name сохраняется snapshot имени на момент присвоения,
// поэтому переименование/архивация не ломает историю.
type Mark struct {
	ID              int       `json:"id"`
	Name            string    `gorm:"size:100;uniqueIndex" json:"name"`
	IsActive        bool      `gorm:"default:true;index" json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	CreatedByUserID *int      `json:"created_by_user_id,omitempty"`
}

// MarkHistory логирует изменения марки: создание, переименование, архивация,
// разархивация. История нужна для аудита и отображения в UI справочника.
type MarkHistory struct {
	ID         int       `json:"id"`
	MarkID     int       `gorm:"index" json:"mark_id"`
	Mark       *Mark     `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	ActionType string    `gorm:"size:50;index" json:"action_type"` // created|renamed|archived|restored
	OldValue   *string   `gorm:"type:text" json:"old_value,omitempty"`
	NewValue   *string   `gorm:"type:text" json:"new_value,omitempty"`
	UserID     *int      `gorm:"index" json:"user_id,omitempty"`
	User       *User     `gorm:"foreignKey:UserID" json:"-"`
	Comment    *string   `gorm:"type:text" json:"comment,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// MarkActionType - константы для MarkHistory.ActionType.
const (
	MarkActionCreated  = "created"
	MarkActionRenamed  = "renamed"
	MarkActionArchived = "archived"
	MarkActionRestored = "restored"
)

// CreateMarkRequest - запрос на создание марки.
type CreateMarkRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

// UpdateMarkRequest - запрос на переименование марки.
type UpdateMarkRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}
