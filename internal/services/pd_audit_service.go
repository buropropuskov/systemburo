package services

import (
	"context"
	"fmt"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// PDAuditService -- чтение журнала доступа к персональным данным (152-ФЗ, #1472).
// Пишет журнал middleware (internal/middleware/pd_audit.go), здесь только выборка:
// записи по закону удалять нельзя, сроком хранения занимаются партиции.
type PDAuditService struct {
	db *gorm.DB
}

// NewPDAuditService конструирует сервис.
func NewPDAuditService(db *gorm.DB) *PDAuditService {
	return &PDAuditService{db: db}
}

// List возвращает страницу журнала с фильтрами и общее количество.
func (s *PDAuditService) List(ctx context.Context, f models.PDAuditFilter) (models.PDAuditPageResponse, error) {
	q := s.applyFilters(s.db.WithContext(ctx).Model(&models.PDAuditLog{}), f)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return models.PDAuditPageResponse{}, fmt.Errorf("count pd audit: %w", err)
	}

	page, limit := normalizePage(f.Page, f.Limit)
	rows := make([]models.PDAuditLog, 0, limit)
	if err := q.Order("created_at DESC, id DESC").
		Limit(limit).Offset((page - 1) * limit).
		Find(&rows).Error; err != nil {
		return models.PDAuditPageResponse{}, fmt.Errorf("list pd audit: %w", err)
	}

	items, err := s.attachUserNames(ctx, rows)
	if err != nil {
		return models.PDAuditPageResponse{}, err
	}
	return models.PDAuditPageResponse{Items: items, Total: total, Page: page, Limit: limit}, nil
}

// applyFilters накладывает фильтры страницы: период, пользователь, действие, ресурс.
func (s *PDAuditService) applyFilters(q *gorm.DB, f models.PDAuditFilter) *gorm.DB {
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.Username != nil && *f.Username != "" {
		q = q.Where("username ILIKE ?", "%"+*f.Username+"%")
	}
	if f.Action != nil && *f.Action != "" {
		q = q.Where("action = ?", *f.Action)
	}
	if f.Resource != nil && *f.Resource != "" {
		q = q.Where("resource = ?", *f.Resource)
	}
	// Отдельный фильтр по отказам: для проверки чаще всего нужны именно они.
	if f.OnlyDenied != nil && *f.OnlyDenied {
		q = q.Where("status_code >= ?", 400)
	}
	if f.From != nil {
		q = q.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("created_at <= ?", *f.To)
	}
	return q
}

// attachUserNames добавляет ФИО к записям: журнал хранит логин, а на экране нужен человек.
// Имя берётся из users по user_id; у записей до появления колонки его нет, там остаётся логин.
func (s *PDAuditService) attachUserNames(ctx context.Context, rows []models.PDAuditLog) ([]models.PDAuditResponse, error) {
	items := make([]models.PDAuditResponse, 0, len(rows))
	ids := make([]int, 0, len(rows))
	for _, r := range rows {
		if r.UserID != nil {
			ids = append(ids, *r.UserID)
		}
	}

	names := map[int]string{}
	if len(ids) > 0 {
		var users []struct {
			ID       int
			FullName string
		}
		if err := s.db.WithContext(ctx).Raw(`
			SELECT id, TRIM(CONCAT_WS(' ', last_name, first_name, middle_name)) AS full_name
			FROM users WHERE id IN ?
		`, ids).Scan(&users).Error; err != nil {
			return nil, fmt.Errorf("attach user names: %w", err)
		}
		for _, u := range users {
			names[u.ID] = u.FullName
		}
	}

	for _, r := range rows {
		item := models.PDAuditResponse{
			ID:         r.ID,
			UserID:     r.UserID,
			Username:   r.Username,
			Action:     r.Action,
			Resource:   r.Resource,
			IPAddress:  r.IPAddress,
			Method:     r.Method,
			Path:       r.Path,
			StatusCode: r.StatusCode,
			CreatedAt:  r.CreatedAt,
		}
		if r.UserID != nil {
			item.UserName = names[*r.UserID]
		}
		items = append(items, item)
	}
	return items, nil
}
