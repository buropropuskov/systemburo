package models

import (
	"encoding/json"
	"time"
)

// UserTypeActionType - константы action-типов истории типов пользователя.
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
