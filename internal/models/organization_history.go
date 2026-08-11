package models

import (
	"encoding/json"
	"time"
)

// OrganizationActionType - константы action-типов истории организации.
// renamed/retyped/updated различают, что именно изменилось при обновлении
// (только имя / только тип / и то и другое) - чтобы история не писала
// «переименована» при смене одного лишь типа.
const (
	OrganizationActionCreated  = "created"
	OrganizationActionRenamed  = "renamed"
	OrganizationActionRetyped  = "retyped"
	OrganizationActionUpdated  = "updated"
	OrganizationActionArchived = "archived"
	OrganizationActionRestored = "restored"
	// Групповые/одиночные изменения привязок пишут отдельные action с деталями
	// «было -> стало» (added/removed, для ответственных ещё approval_changed).
	// Разбор записи, заведённой из заявки (#1437): подтверждение и привязка к
	// существующей организации. Переименование при разборе пишется как renamed.
	OrganizationActionApproved = "moderation_approved"
	OrganizationActionMerged   = "moderation_merged"

	OrganizationActionResponsiblesChanged = "responsibles_changed"
	OrganizationActionUnloadPlacesChanged = "unload_places_changed"
	OrganizationActionTablesChanged       = "tables_changed"

	// OrganizationActionRetired / OrganizationActionRetireRestored - обратимый офбординг
	// через консольную команду entity retire/restore (internal/entityarchive), отдельно
	// от Archived/Restored выше: те архивируют ОДНУ организацию через админку и
	// блокируются активными пользователями, эти гасят организацию И её пользователей
	// одним действием и не блокируются ничем. Restore читает ПОСЛЕДНЮЮ запись с одним из
	// этих двух action - разными значениями, а не Archived/Restored, чтобы не подхватить
	// историю обычной архивации организации.
	OrganizationActionRetired        = "retired"
	OrganizationActionRetireRestored = "retire_restored"
)

// OrganizationHistoryItem - запись истории с именем актора для API (LEFT JOIN users).
type OrganizationHistoryItem struct {
	ID          int             `json:"id"`
	ActionType  string          `json:"action_type"`
	Details     json.RawMessage `json:"details,omitempty" swaggerignore:"true"`
	ActorUserID *int            `json:"actor_user_id,omitempty"`
	ActorName   string          `json:"actor_name"`
	CreatedAt   time.Time       `json:"created_at"`
}
