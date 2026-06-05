package models

import (
	"encoding/json"
	"time"
)

// SystemTableHistory логирует CRUD-действия над системной таблицей (#345).
// Используется для отображения "Истории" в конструкторе таблиц.
// Поля и слоты пишутся в Details как diff/snapshot - формат зависит от ActionType.
type SystemTableHistory struct {
	ID            int             `json:"id"`
	SystemTableID int             `gorm:"index" json:"system_table_id"`
	ActionType    string          `gorm:"size:30;index" json:"action_type"`
	Details       json.RawMessage `gorm:"type:jsonb" json:"details,omitempty"`
	UserID        *int            `gorm:"index" json:"user_id,omitempty"`
	User          *User           `gorm:"foreignKey:UserID" json:"-"`
	CreatedAt     time.Time       `json:"created_at"`
}

// SystemTableActionType - константы для SystemTableHistory.ActionType.
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
