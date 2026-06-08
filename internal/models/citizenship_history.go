package models

import (
	"encoding/json"
	"time"
)

// CitizenshipHistory логирует действия над гражданством: создание, изменение,
// архивацию и восстановление. Нужна для аудита (#415): гражданство влияет на
// требование патента у сотрудников, важно знать кто и когда менял.
// FK (CitizenshipID) намеренно без constraint: аудит должен пережить удаление гражданства.
type CitizenshipHistory struct {
	ID            int             `json:"id"`
	CitizenshipID int             `gorm:"index;not null" json:"citizenship_id"`
	ActorUserID   *int            `gorm:"index" json:"actor_user_id,omitempty"`
	ActionType    string          `gorm:"size:32;index" json:"action_type"`
	Details       json.RawMessage `gorm:"type:jsonb" json:"details,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// CitizenshipActionType - константы для CitizenshipHistory.ActionType.
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
