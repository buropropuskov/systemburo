package models

import (
	"encoding/json"
	"time"
)

// CompanyActionType - константы action-типов истории компании.
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
