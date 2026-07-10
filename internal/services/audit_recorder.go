package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// AuditRecorder - единая точка записи аудита (#870), заменяет ~15 копий
// *_history_service. Каждая сущность пишет свои действия сюда с своим entity_type;
// чтение - через единый reader с фильтром (entity_type, entity_id).
//
// КОНВЕНЦИЯ (итог #870): любую НОВУЮ историю/аудит-журнал вести ТОЛЬКО через этот
// recorder (Record/Log + новая константа models.AuditEntity*), НЕ заводя отдельную
// *_history таблицу/модель/сервис/endpoint. Это стережёт guard-тест
// database.TestAllModels_NoLegacyHistoryTables.
type AuditRecorder interface {
	// Record пишет запись аудита через exec и ВОЗВРАЩАЕТ ошибку. Для критичных
	// действий передавать exec=tx: провал записи откатит всю операцию (паттерн
	// blacklist Log). exec=nil -> пишет через db рекордера.
	Record(ctx context.Context, exec *gorm.DB, entityType string, entityID *int, action string, actorID *int, details interface{}) error
	// Log - как Record, но ошибку не возвращает (лог + проглот): для сущностей,
	// где аудит не должен ломать основное действие (паттерн citizenship Log).
	Log(ctx context.Context, exec *gorm.DB, entityType string, entityID *int, action string, actorID *int, details interface{})
}

type auditRecorder struct {
	db *gorm.DB
}

// NewAuditRecorder создаёт реализацию AuditRecorder.
func NewAuditRecorder(db *gorm.DB) AuditRecorder {
	return &auditRecorder{db: db}
}

func (r *auditRecorder) Record(ctx context.Context, exec *gorm.DB, entityType string, entityID *int, action string, actorID *int, details interface{}) error {
	if exec == nil {
		exec = r.db
	}
	var raw json.RawMessage
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			return fmt.Errorf("marshal audit details (%s/%s): %w", entityType, action, err)
		}
		raw = b
	}
	entry := models.AuditLog{
		EntityType:  entityType,
		EntityID:    entityID,
		Action:      action,
		ActorUserID: actorID,
		Details:     raw,
	}
	if err := exec.WithContext(ctx).Create(&entry).Error; err != nil {
		return fmt.Errorf("insert audit log (%s/%s): %w", entityType, action, err)
	}
	return nil
}

func (r *auditRecorder) Log(ctx context.Context, exec *gorm.DB, entityType string, entityID *int, action string, actorID *int, details interface{}) {
	if err := r.Record(ctx, exec, entityType, entityID, action, actorID, details); err != nil {
		slog.Error("audit: record failed", "entity_type", entityType, "entity_id", entityID, "action", action, "error", err)
	}
}

// recordAddedToTable пишет ОДНУ запись audit_log «Добавлен в таблицу проходной» (#1085) на пару
// (сущность, таблица): action=added_to_table, details.table_id=tableID -> reader резолвит table_name
// и работает фильтр «Место прохода» (как у entry/exit). entityType = models.AuditEntityCar |
// AuditEntityEmployee. exec обязан быть тем же tx, что и INSERT привязки в *_target_tables. Возвращает
// error (семантика Record): вызывающий выбирает - пробросить (откат, как соседний create-Record) или
// залогировать и проглотить (best-effort, как соседний create-Log в submit-complete).
//
// Comment намеренно не задаётся: FE рисует лейбл по action_type + отдельный блок .place-name с
// table_name; дублировать текст в details.comment (он бы отрисовался как .action-comment) не нужно.
func recordAddedToTable(ctx context.Context, r AuditRecorder, exec *gorm.DB, entityType string, entityID, tableID int, actorID *int) error {
	id, tid := entityID, tableID
	return r.Record(ctx, exec, entityType, &id, models.AuditActionAddedToTable, actorID, carAuditDetails{TableID: &tid})
}
