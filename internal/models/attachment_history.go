package models

import (
	"encoding/json"
	"time"
)

// UniqueAttachmentActionType - константы action-типов истории шаблонов вложений.
const (
	UniqueAttachmentActionCreated  = "created"
	UniqueAttachmentActionUpdated  = "updated"
	UniqueAttachmentActionArchived = "archived"
	UniqueAttachmentActionRestored = "restored"
	// UniqueAttachmentActionListImported - загружен заполненный Excel-бланк для массового
	// ввода списка (blank-import, срез C3). details несёт имя файла и сводку разбора
	// (сколько строк прочитано/принято/отклонено) - кто, когда и что именно загрузил.
	UniqueAttachmentActionListImported = "list_imported"
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
