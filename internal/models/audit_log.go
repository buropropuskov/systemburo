package models

import (
	"encoding/json"
	"time"
)

// AuditLog - единый журнал аудита (#870): сводит ~21 отдельную *_history таблицу
// в одно место. Одна строка = одно действие над сущностью. Снаружи у каждой
// сущности своя история через фильтр (entity_type, entity_id) - не общая "свалка".
//
// EntityID/ActorUserID намеренно без FK constraint: аудит должен пережить удаление
// родителя или пользователя (как в legacy *History-моделях).
// Details (jsonb) - надмножество всех старых схем: новый паттерн пишет details как
// есть; плоский field_name/old/new/comment/metadata и snapshot-поля маппятся внутрь.
type AuditLog struct {
	ID          int             `json:"id"`
	EntityType  string          `gorm:"size:64;not null;index:idx_audit_entity,priority:1" json:"entity_type"`
	EntityID    *int            `gorm:"index:idx_audit_entity,priority:2" json:"entity_id,omitempty"`
	Action      string          `gorm:"size:64;index" json:"action"`
	ActorUserID *int            `gorm:"index" json:"actor_user_id,omitempty"`
	Details     json.RawMessage `gorm:"type:jsonb" json:"details,omitempty"`
	CreatedAt   time.Time       `gorm:"index:idx_audit_entity,priority:3" json:"created_at"`
}

// TableName задаёт имя таблицы явно (singular per #870), без gorm-плюрализации.
func (AuditLog) TableName() string { return "audit_log" }

// AuditEntity* - значения AuditLog.EntityType. Добавляются по мере переноса
// сущностей на audit_log (#870).
const (
	AuditEntityCitizenship        = "citizenship"
	AuditEntityCompany            = "company"
	AuditEntityOrganization       = "organization"
	AuditEntityUserType           = "user_type"
	AuditEntityLicensePlateFormat = "license_plate_format"
	AuditEntityUnloadPlace        = "unload_place"
	AuditEntityUniqueAttachment   = "unique_attachment"
	AuditEntityUser               = "user"
	AuditEntityApprover           = "approver"
	AuditEntityPersonBlacklist    = "person_blacklist"
	AuditEntityVehicleBlacklist   = "vehicle_blacklist"
	AuditEntitySystemTable        = "system_table"
	AuditEntitySystemTableTrash   = "system_table_trash"
	AuditEntityMark               = "mark"
	AuditEntityCar                = "car"
	AuditEntityUniqueCar          = "unique_car"
	AuditEntityEmployee           = "employee"
	AuditEntityUniqueEmployee     = "unique_employee"
	AuditEntityApplication        = "application"
)

// AuditAction* - значения AuditLog.Action, вынесенные в константы там, где значение
// используется в нескольких местах записи/чтения (иначе дрейф литерала). Большинство
// действий car/employee остаются строковыми литералами в своих сервисах.
const (
	// AuditActionAddedToTable - машина/сотрудник внесены в таблицу проходной
	// (car_target_tables/employee_target_tables, #1036). Пишется по одной записи на
	// таблицу, details.table_id хранит пост - reader резолвит table_name как у entry/exit.
	AuditActionAddedToTable = "added_to_table"
	// AuditActionUnboundFromTable - машина/сотрудник сняты с таблицы проходной групповой
	// операцией «Убрать» (#1194), но у сущности осталась хотя бы одна другая привязка
	// (иначе см. deactivate/#951). details.table_id - снятая таблица.
	AuditActionUnboundFromTable = "unbound_from_table"
	// AuditActionMovedBetweenTables - машина/сотрудник перенесены групповой операцией
	// «Перенести» (#1194) из одной таблицы проходной в другую(ие) одним действием.
	// details.table_id - таблица-источник (там же появляется событие для фильтра
	// «Место прохода»); таблицы назначения - только в человекочитаемом comment
	// (их может быть несколько, details.table_id - одно поле).
	AuditActionMovedBetweenTables = "moved_between_tables"
	// AuditActionForwarded - сводная запись о пересылке заявки: ОДНА на действие
	// (не на получателя, тех пишут assigned_responsible/assigned_viewer). Читают
	// ветка пересылок в истории заявки (#680) и метрика avg_forwards (#1240).
	AuditActionForwarded = "forwarded"
)

// AuditLogItem - запись аудита для API с разрезолвленным именем актора
// (LEFT JOIN users). Унифицирует поле актора: старые модели отдавали то actor_name,
// то user_name - generic-ответ всегда actor_name.
type AuditLogItem struct {
	ID          int             `json:"id"`
	EntityType  string          `json:"entity_type"`
	EntityID    *int            `json:"entity_id,omitempty"`
	Action      string          `json:"action"`
	Details     json.RawMessage `json:"details,omitempty" swaggerignore:"true"`
	ActorUserID *int            `json:"actor_user_id,omitempty"`
	ActorName   string          `json:"actor_name"`
	CreatedAt   time.Time       `json:"created_at"`
}
