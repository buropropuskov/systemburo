package models

import (
	"encoding/json"
	"time"
)

// CompanyActionType - константы action-типов истории компании.
// renamed/retyped/updated различают, что именно изменилось при обновлении
// (только имя / только тип / и то и другое).
const (
	CompanyActionCreated  = "created"
	CompanyActionRenamed  = "renamed"
	CompanyActionRetyped  = "retyped"
	CompanyActionUpdated  = "updated"
	CompanyActionArchived = "archived"
	CompanyActionRestored = "restored"
	// Групповые/одиночные изменения привязок пишут отдельные action с деталями
	// «было -> стало» (added/removed, для ответственных ещё approval_changed).
	// Разбор записи, заведённой из заявки (#1437), зеркально организациям.
	CompanyActionApproved = "moderation_approved"
	CompanyActionMerged   = "moderation_merged"

	CompanyActionResponsiblesChanged = "responsibles_changed"
	CompanyActionUnloadPlacesChanged = "unload_places_changed"
	CompanyActionTablesChanged       = "tables_changed"
)

// CompanyHistoryItem - запись истории с именем актора для API (LEFT JOIN users).
type CompanyHistoryItem struct {
	ID          int             `json:"id"`
	ActionType  string          `json:"action_type"`
	Details     json.RawMessage `json:"details,omitempty" swaggerignore:"true"`
	ActorUserID *int            `json:"actor_user_id,omitempty"`
	ActorName   string          `json:"actor_name"`
	CreatedAt   time.Time       `json:"created_at"`
}
