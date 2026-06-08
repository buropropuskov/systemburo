package models

import (
	"encoding/json"
	"time"
)

// UniqueAttachmentHistory логирует действия над шаблоном вложения: создание,
// изменение, архивацию и восстановление. Нужна для аудита (#416): шаблоны вложений
// привязаны к Excel-бланкам и заявкам, важно знать кто и когда менял.
// FK (UniqueAttachmentID) намеренно без constraint: аудит должен пережить удаление шаблона.
type UniqueAttachmentHistory struct {
	ID                 int             `json:"id"`
	UniqueAttachmentID int             `gorm:"index;not null" json:"unique_attachment_id"`
	ActorUserID        *int            `gorm:"index" json:"actor_user_id,omitempty"`
	ActionType         string          `gorm:"size:32;index" json:"action_type"`
	Details            json.RawMessage `gorm:"type:jsonb" json:"details,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}

// UniqueAttachmentActionType - константы для UniqueAttachmentHistory.ActionType.
const (
	UniqueAttachmentActionCreated  = "created"
	UniqueAttachmentActionUpdated  = "updated"
	UniqueAttachmentActionArchived = "archived"
	UniqueAttachmentActionRestored = "restored"
)

// UniqueAttachmentHistoryItem - запись истории с именем актора для API (LEFT JOIN users).
type UniqueAttachmentHistoryItem struct {
	ID          int             `json:"id"`
	ActionType  string          `json:"action_type"`
	Details     json.RawMessage `json:"details,omitempty" swaggerignore:"true"`
	ActorUserID *int            `json:"actor_user_id,omitempty"`
	ActorName   string          `json:"actor_name"`
	CreatedAt   time.Time       `json:"created_at"`
}
