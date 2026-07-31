package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// AuditQuery - фильтры чтения журнала аудита (#870). Все поля опциональны кроме
// пагинации. EntityType+EntityID дают историю одной сущности (как старые per-id
// endpoint-ы); без них - сводный журнал с фильтрами.
type AuditQuery struct {
	EntityType  string
	EntityID    *int
	Action      string
	ActorUserID *int
	From        *time.Time // включительно (>=)
	To          *time.Time // исключительно (<), handler передаёт начало след. дня
	Page        int
	PerPage     int
}

// AuditReader - единое чтение audit_log с разрешением имени актора и пагинацией.
// Заменяет ~15 копий *_history_service.GetHistory.
type AuditReader interface {
	List(ctx context.Context, q AuditQuery) ([]models.AuditLogItem, int64, error)
}

type auditReader struct {
	db *gorm.DB
}

// NewAuditReader создаёт реализацию AuditReader.
func NewAuditReader(db *gorm.DB) AuditReader {
	return &auditReader{db: db}
}

func (r *auditReader) List(ctx context.Context, q AuditQuery) ([]models.AuditLogItem, int64, error) {
	// Условия применяем на общий билдер, затем клонируем сессией под count и data,
	// чтобы SELECT count(*) не протёк в выборку строк.
	cond := r.db.WithContext(ctx).Table("audit_log AS h")
	if q.EntityType != "" {
		cond = cond.Where("h.entity_type = ?", q.EntityType)
	}
	if q.EntityID != nil {
		cond = cond.Where("h.entity_id = ?", *q.EntityID)
	}
	if q.Action != "" {
		cond = cond.Where("h.action = ?", q.Action)
	}
	if q.ActorUserID != nil {
		cond = cond.Where("h.actor_user_id = ?", *q.ActorUserID)
	}
	if q.From != nil {
		cond = cond.Where("h.created_at >= ?", *q.From)
	}
	if q.To != nil {
		cond = cond.Where("h.created_at < ?", *q.To)
	}

	var total int64
	if err := cond.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count audit log: %w", err)
	}

	type row struct {
		ID          int             `gorm:"column:id"`
		EntityType  string          `gorm:"column:entity_type"`
		EntityID    *int            `gorm:"column:entity_id"`
		Action      string          `gorm:"column:action"`
		Details     json.RawMessage `gorm:"column:details"`
		ActorUserID *int            `gorm:"column:actor_user_id"`
		ActorName   string          `gorm:"column:actor_name"`
		CreatedAt   time.Time       `gorm:"column:created_at"`
	}

	dataQ := cond.Session(&gorm.Session{}).
		Select(`h.id, h.entity_type, h.entity_id, h.action, h.details, h.actor_user_id,
			COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '') AS actor_name,
			h.created_at`).
		Joins("LEFT JOIN users u ON u.id = h.actor_user_id").
		Order("h.created_at DESC, h.id DESC")
	if q.PerPage > 0 {
		dataQ = dataQ.Limit(q.PerPage).Offset((q.Page - 1) * q.PerPage)
	}

	var rows []row
	if err := dataQ.Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("scan audit log: %w", err)
	}

	// Логин вместо ФИО у акторов, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, r.db)
	items := make([]models.AuditLogItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, models.AuditLogItem{
			ID:          row.ID,
			EntityType:  row.EntityType,
			EntityID:    row.EntityID,
			Action:      row.Action,
			Details:     row.Details,
			ActorUserID: row.ActorUserID,
			ActorName:   maskName(masks, row.ActorUserID, row.ActorName),
			CreatedAt:   row.CreatedAt,
		})
	}
	return items, total, nil
}
