package models

import (
	"encoding/json"
	"time"
)

// UserTypeHistory логирует действия над типом пользователя: создание,
// переименование и удаление. Нужна для аудита: code типов используются в
// авторизации, и важно отследить кто и когда менял справочник типов.
//
// UserTypeID - над каким типом действие, ActorUserID - кто его совершил.
// FK на тип намеренно без constraint: аудит должен пережить удаление типа.
type UserTypeHistory struct {
	ID          int             `json:"id"`
	UserTypeID  int             `gorm:"index;not null" json:"user_type_id"`
	ActorUserID *int            `gorm:"index" json:"actor_user_id,omitempty"`
	ActionType  string          `gorm:"size:32;index" json:"action_type"`
	Details     json.RawMessage `gorm:"type:jsonb" json:"details,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// UserTypeActionType - константы для UserTypeHistory.ActionType.
const (
	UserTypeActionCreated = "created"
	UserTypeActionRenamed = "renamed"
	UserTypeActionDeleted = "deleted"
)

// UserTypeHistoryItem - запись истории с именем актора для API (LEFT JOIN users).
type UserTypeHistoryItem struct {
	ID          int             `json:"id"`
	ActionType  string          `json:"action_type"`
	Details     json.RawMessage `json:"details,omitempty" swaggerignore:"true"`
	ActorUserID *int            `json:"actor_user_id,omitempty"`
	ActorName   string          `json:"actor_name"`
	CreatedAt   time.Time       `json:"created_at"`
}
