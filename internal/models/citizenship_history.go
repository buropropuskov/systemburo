package models

import (
	"encoding/json"
	"time"
)

// CitizenshipActionType - константы action-типов истории гражданства.
const (
	CitizenshipActionCreated  = "created"
	CitizenshipActionUpdated  = "updated"
	CitizenshipActionArchived = "archived"
	CitizenshipActionRestored = "restored"
)

// CitizenshipHistoryItem - запись истории с именем актора для API (LEFT JOIN users).
type CitizenshipHistoryItem struct {
	ID          int             `json:"id"`
	ActionType  string          `json:"action_type"`
	Details     json.RawMessage `json:"details,omitempty" swaggerignore:"true"`
	ActorUserID *int            `json:"actor_user_id,omitempty"`
	ActorName   string          `json:"actor_name"`
	CreatedAt   time.Time       `json:"created_at"`
}
