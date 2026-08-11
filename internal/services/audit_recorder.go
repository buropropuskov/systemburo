package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

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
	entry, err := buildAuditLogEntry(ctx, entityType, entityID, action, actorID, details)
	if err != nil {
		return err
	}
	if err := exec.WithContext(ctx).Create(&entry).Error; err != nil {
		return fmt.Errorf("insert audit log (%s/%s): %w", entityType, action, err)
	}
	return nil
}

// buildAuditLogEntry строит запись audit_log без записи в БД: общая точка сборки для
// одиночных Record/Log и пакетных путей (массовая подача, срез A2A3 blank-import),
// которые копят строки в срез и вставляют одним CreateInBatches вместо N отдельных Create.
//
// ctx нужен ради отметки инициатора режима «войти как пользователь» (#1912): она
// ставится здесь, в единственной точке сборки, - на пакетном пути её иначе не было бы.
func buildAuditLogEntry(ctx context.Context, entityType string, entityID *int, action string, actorID *int, details interface{}) (models.AuditLog, error) {
	var raw json.RawMessage
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			return models.AuditLog{}, fmt.Errorf("marshal audit details (%s/%s): %w", entityType, action, err)
		}
		raw = b
	}
	return models.AuditLog{
		EntityType:  entityType,
		EntityID:    entityID,
		Action:      action,
		ActorUserID: actorID,
		Details:     withImpersonatorDetails(ctx, raw),
	}, nil
}

// withImpersonatorDetails дописывает в details инициатора режима «войти как
// пользователя» (#1912). Отметка ставится здесь, а не в местах записи: иначе
// «действия внутри режима отличимы» пришлось бы поддерживать в полутора сотнях
// вызовов, и первый же новый забыл бы про неё. actor_user_id при этом остаётся
// тем, от чьего имени работают, - подменять его инициатором нельзя, иначе история
// сущности перестанет отвечать на вопрос «под какой учётной записью это сделано».
//
// Details не объект (контракт этого не запрещает) - оставляем запись как есть:
// потерять действие ради отметки хуже, чем потерять отметку.
func withImpersonatorDetails(ctx context.Context, raw json.RawMessage) json.RawMessage {
	actorUserID, ok := ImpersonatorFromContext(ctx)
	if !ok {
		return raw
	}
	fields := map[string]json.RawMessage{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &fields); err != nil {
			return raw
		}
	}
	fields["impersonated_by"] = json.RawMessage(strconv.Itoa(actorUserID))
	merged, err := json.Marshal(fields)
	if err != nil {
		return raw
	}
	return merged
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

// recordUnboundFromTable пишет ОДНУ запись audit_log «Снят с таблицы проходной»
// (#1194) на пару (сущность, таблица): action=unbound_from_table, details.table_id=
// tableID - зеркало recordAddedToTable для противоположного события групповой
// операции «Убрать». exec обязан быть тем же tx, что и DELETE привязки.
func recordUnboundFromTable(ctx context.Context, r AuditRecorder, exec *gorm.DB, entityType string, entityID, tableID int, actorID *int) error {
	id, tid := entityID, tableID
	return r.Record(ctx, exec, entityType, &id, models.AuditActionUnboundFromTable, actorID, carAuditDetails{TableID: &tid})
}
