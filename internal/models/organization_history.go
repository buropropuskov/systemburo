package models

import (
	"encoding/json"
	"time"
)

// OrganizationHistory логирует действия над организацией: создание,
// переименование, архивацию и восстановление. Нужна для аудита (#412).
//
// OrganizationID - над какой организацией действие, ActorUserID - кто его
// совершил. FK намеренно без constraint: аудит должен пережить любые изменения.
type OrganizationHistory struct {
	ID             int             `json:"id"`
	OrganizationID int             `gorm:"index;not null" json:"organization_id"`
	ActorUserID    *int            `gorm:"index" json:"actor_user_id,omitempty"`
	ActionType     string          `gorm:"size:32;index" json:"action_type"`
	Details        json.RawMessage `gorm:"type:jsonb" json:"details,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

// OrganizationActionType - константы для OrganizationHistory.ActionType.
const (
	OrganizationActionCreated  = "created"
	OrganizationActionRenamed  = "renamed"
	OrganizationActionArchived = "archived"
	OrganizationActionRestored = "restored"
)

// OrganizationHistoryItem - запись истории с именем актора для API (LEFT JOIN users).
type OrganizationHistoryItem struct {
	ID          int             `json:"id"`
	ActionType  string          `json:"action_type"`
	Details     json.RawMessage `json:"details,omitempty" swaggerignore:"true"`
	ActorUserID *int            `json:"actor_user_id,omitempty"`
	ActorName   string          `json:"actor_name"`
	CreatedAt   time.Time       `json:"created_at"`
}
