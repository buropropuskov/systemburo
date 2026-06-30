package models

import (
	"encoding/json"
	"time"
)

// OrganizationActionType - константы action-типов истории организации.
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
