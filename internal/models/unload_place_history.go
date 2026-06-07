package models

import (
	"encoding/json"
	"time"
)

// UnloadPlaceHistory логирует действия над местом разгрузки: создание,
// переименование, архивацию и восстановление. Нужна для аудита (#413).
//
// UnloadPlaceID - над каким местом действие, ActorUserID - кто его совершил.
// FK намеренно без constraint: аудит должен пережить любые изменения.
type UnloadPlaceHistory struct {
	ID            int             `json:"id"`
	UnloadPlaceID int             `gorm:"index;not null" json:"unload_place_id"`
	ActorUserID   *int            `gorm:"index" json:"actor_user_id,omitempty"`
	ActionType    string          `gorm:"size:32;index" json:"action_type"`
	Details       json.RawMessage `gorm:"type:jsonb" json:"details,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// UnloadPlaceActionType - константы для UnloadPlaceHistory.ActionType.
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
