package models

import (
	"encoding/json"
	"time"
)

// UserActionType - константы action-типов истории учётных записей.
const (
	UserActionCreated        = "created"
	UserActionUpdated        = "updated"
	UserActionTypeChanged    = "type_changed"
	UserActionOrgChanged     = "org_changed"
	UserActionCompanyChanged = "company_changed"
	UserActionPasswordReset  = "password_reset"
	UserActionArchived       = "archived"
	UserActionRestored       = "restored"
	UserActionBanned         = "banned"
	UserActionUnbanned       = "unbanned"
)

// UserHistoryItem - запись истории с именем актора для API (LEFT JOIN users).
type UserHistoryItem struct {
	ID          int             `json:"id"`
	ActionType  string          `json:"action_type"`
	Details     json.RawMessage `json:"details,omitempty" swaggerignore:"true"`
	ActorUserID *int            `json:"actor_user_id,omitempty"`
	ActorName   string          `json:"actor_name"`
	CreatedAt   time.Time       `json:"created_at"`
}
