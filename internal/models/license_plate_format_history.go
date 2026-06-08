package models

import (
	"encoding/json"
	"time"
)

// LicensePlateFormatHistory логирует действия над форматом номеров: создание,
// изменение, архивацию и восстановление. Нужна для аудита (#414): изменение
// ячеек меняет правила валидации номеров, важно знать кто и когда.
//
// FormatID - над каким форматом действие, ActorUserID - кто его совершил.
// FK намеренно без constraint: аудит должен пережить архивацию/удаление формата.
type LicensePlateFormatHistory struct {
	ID          int             `json:"id"`
	FormatID    int             `gorm:"index;not null" json:"format_id"`
	ActorUserID *int            `gorm:"index" json:"actor_user_id,omitempty"`
	ActionType  string          `gorm:"size:32;index" json:"action_type"`
	Details     json.RawMessage `gorm:"type:jsonb" json:"details,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// LicensePlateFormatActionType - константы для LicensePlateFormatHistory.ActionType.
const (
	LicensePlateFormatActionCreated  = "created"
	LicensePlateFormatActionUpdated  = "updated"
	LicensePlateFormatActionArchived = "archived"
	LicensePlateFormatActionRestored = "restored"
)

// LicensePlateFormatHistoryItem - запись истории с именем актора для API (LEFT JOIN users).
type LicensePlateFormatHistoryItem struct {
	ID          int             `json:"id"`
	ActionType  string          `json:"action_type"`
	Details     json.RawMessage `json:"details,omitempty" swaggerignore:"true"`
	ActorUserID *int            `json:"actor_user_id,omitempty"`
	ActorName   string          `json:"actor_name"`
	CreatedAt   time.Time       `json:"created_at"`
}
