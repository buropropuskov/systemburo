package models

import (
	"encoding/json"
	"time"
)

// LicensePlateFormatActionType - константы action-типов истории формата номеров.
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
