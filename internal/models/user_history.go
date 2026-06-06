package models

import (
	"encoding/json"
	"time"
)

// UserHistory логирует действия над учётной записью пользователя: создание,
// изменение данных/типа/организации/компании, сброс пароля, архивацию и
// восстановление. Нужна для аудита ("кто и что сделал с учёткой").
//
// TargetUserID - чья учётка изменена, ActorUserID - кто совершил действие.
// FK на пользователей намеренно без constraint: аудит должен пережить удаление.
type UserHistory struct {
	ID           int             `json:"id"`
	TargetUserID int             `gorm:"index;not null" json:"target_user_id"`
	ActorUserID  *int            `gorm:"index" json:"actor_user_id,omitempty"`
	ActionType   string          `gorm:"size:32;index" json:"action_type"`
	Details      json.RawMessage `gorm:"type:jsonb" json:"details,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

// UserActionType - константы для UserHistory.ActionType.
const (
	UserActionCreated        = "created"
	UserActionUpdated        = "updated"
	UserActionTypeChanged    = "type_changed"
	UserActionOrgChanged     = "org_changed"
	UserActionCompanyChanged = "company_changed"
	UserActionPasswordReset  = "password_reset"
	UserActionArchived       = "archived"
	UserActionRestored       = "restored"
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
