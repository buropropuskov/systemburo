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

	// OrganizationActionAnonymized - необратимое обезличивание персональных полей
	// сотрудников и пользователей организации через консольную команду entity anonymize
	// (internal/entityarchive, срез 5). В отличие от Retired/RetireRestored выше действие
	// НЕ обратимо и не имеет своей restore-команды: запись в истории хранит только факт и
	// счётчики затронутых строк по таблицам, без исходных значений.
	OrganizationActionAnonymized = "anonymized"

	// OrganizationActionExported / OrganizationActionPurged / OrganizationActionImported -
	// три следа жизненного цикла пакета entity export/purge/import (internal/entityarchive,
	// срезы 6-8): снятие копии графа организации, её физический снос по проверенному
	// пакету и разворот пакета на другом стенде. Exported и Purged пишет одна и та же
	// установка (снимающая и сносящая), Imported - принимающая сторона другого обмена;
	// три разных action нужны, чтобы отличать эти следы друг от друга в журнале.
	OrganizationActionExported = "exported"
	OrganizationActionPurged   = "purged"
	OrganizationActionImported = "imported"
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
