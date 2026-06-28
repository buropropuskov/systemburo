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
// родителя или пользователя (как в новом паттерне *History, см. CitizenshipHistory).
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
	AuditEntityCitizenship = "citizenship"
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
