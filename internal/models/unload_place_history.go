package models

import (
	"encoding/json"
	"time"
)

// UnloadPlaceActionType - константы action-типов истории мест разгрузки.
const (
	UnloadPlaceActionCreated  = "created"
	UnloadPlaceActionRenamed  = "renamed"
	UnloadPlaceActionArchived = "archived"
	UnloadPlaceActionRestored = "restored"
)

// UnloadPlaceHistoryItem - запись истории с именем актора для API (LEFT JOIN users).
type UnloadPlaceHistoryItem struct {
	ID          int             `json:"id"`
	ActionType  string          `json:"action_type"`
	Details     json.RawMessage `json:"details,omitempty" swaggerignore:"true"`
	ActorUserID *int            `json:"actor_user_id,omitempty"`
	ActorName   string          `json:"actor_name"`
	CreatedAt   time.Time       `json:"created_at"`
}
