package models

import (
	"encoding/json"
	"time"
)

// CompanyHistory логирует действия над компанией: создание, переименование,
// архивацию и восстановление. Нужна для аудита (#412).
//
// CompanyID - над какой компанией действие, ActorUserID - кто его совершил.
// FK намеренно без constraint: аудит должен пережить любые изменения.
type CompanyHistory struct {
	ID          int             `json:"id"`
	CompanyID   int             `gorm:"index;not null" json:"company_id"`
	ActorUserID *int            `gorm:"index" json:"actor_user_id,omitempty"`
	ActionType  string          `gorm:"size:32;index" json:"action_type"`
	Details     json.RawMessage `gorm:"type:jsonb" json:"details,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// CompanyActionType - константы для CompanyHistory.ActionType.
const (
	CompanyActionCreated  = "created"
	CompanyActionRenamed  = "renamed"
	CompanyActionArchived = "archived"
	CompanyActionRestored = "restored"
)

// CompanyHistoryItem - запись истории с именем актора для API (LEFT JOIN users).
type CompanyHistoryItem struct {
	ID          int             `json:"id"`
	ActionType  string          `json:"action_type"`
	Details     json.RawMessage `json:"details,omitempty" swaggerignore:"true"`
	ActorUserID *int            `json:"actor_user_id,omitempty"`
	ActorName   string          `json:"actor_name"`
	CreatedAt   time.Time       `json:"created_at"`
}
