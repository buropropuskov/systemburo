package models

import (
	"encoding/json"
	"time"
)

// SystemTableActionType - константы action-типов истории системных таблиц.
const (
	SystemTableActionCreated         = "created"
	SystemTableActionUpdated         = "updated"
	SystemTableActionArchived        = "archived"
	SystemTableActionRestored        = "restored"
	SystemTableActionColumnsUpdated  = "columns_updated"
	SystemTableActionAppearanceUpdated = "appearance_updated"
)

// SystemTableHistoryItem - запись истории с именем пользователя для API.
type SystemTableHistoryItem struct {
	ID         int             `json:"id"`
	ActionType string          `json:"action_type"`
	Details    json.RawMessage `json:"details,omitempty" swaggerignore:"true"`
	UserID     *int            `json:"user_id,omitempty"`
	UserName   string          `json:"user_name,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
