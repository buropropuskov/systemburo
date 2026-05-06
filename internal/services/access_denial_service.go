package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// AccessDenialService управляет журналом отказов в доступе.
// Поток записи: middleware -> Log() -> async insert через горутину
// (чтобы не блокировать отказ HTTP-ответа на ms ожидания БД).
// Поток чтения/архива: handler-ы для админов.
type AccessDenialService struct {
	db *gorm.DB
}

// NewAccessDenialService конструирует сервис.
func NewAccessDenialService(db *gorm.DB) *AccessDenialService {
	return &AccessDenialService{db: db}
}

// LogParams -- параметры события отказа.
type LogParams struct {
	UserID        *int
	Resource      string
	PermissionKey *string
	Reason        string
	IPAddress     *string
	UserAgent     *string
}

// Log пишет событие отказа в журнал.
// Запись идёт асинхронно через goroutine: если БД недоступна, отказ HTTP не должен висеть.
// Контекст для запроса берётся новый, чтобы не отменился вместе с HTTP-запросом.
func (s *AccessDenialService) Log(p LogParams) {
	denial := models.AccessDenial{
		UserID:        p.UserID,
		Resource:      p.Resource,
		PermissionKey: p.PermissionKey,
		Reason:        p.Reason,
		IPAddress:     p.IPAddress,
		UserAgent:     p.UserAgent,
		CreatedAt:     time.Now(),
	}
	go func(d models.AccessDenial) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.db.WithContext(ctx).Create(&d).Error; err != nil {
			slog.Error("failed to log access denial", "error", err, "resource", d.Resource)
		}
	}(denial)
}

// List возвращает страницу журнала с фильтрами + общее количество.
func (s *AccessDenialService) List(ctx context.Context, f models.AccessDenialFilter) (models.AccessDenialPageResponse, error) {
	q := s.applyFilters(s.db.WithContext(ctx).Model(&models.AccessDenial{}), f)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return models.AccessDenialPageResponse{}, fmt.Errorf("count denials: %w", err)
	}

	page, limit := normalizePage(f.Page, f.Limit)
	var denials []models.AccessDenial
	if err := q.Order("created_at DESC").
		Limit(limit).Offset((page - 1) * limit).
		Find(&denials).Error; err != nil {
		return models.AccessDenialPageResponse{}, fmt.Errorf("list denials: %w", err)
	}

	items, err := s.attachUserNames(ctx, denials)
	if err != nil {
		return models.AccessDenialPageResponse{}, err
	}
	return models.AccessDenialPageResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

// ListArchive возвращает страницу архива с теми же фильтрами.
func (s *AccessDenialService) ListArchive(ctx context.Context, f models.AccessDenialFilter) (models.AccessDenialPageResponse, error) {
	q := s.applyFiltersArchive(s.db.WithContext(ctx).Model(&models.AccessDenialArchive{}), f)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return models.AccessDenialPageResponse{}, fmt.Errorf("count archive: %w", err)
	}

	page, limit := normalizePage(f.Page, f.Limit)
	var rows []models.AccessDenialArchive
	if err := q.Order("created_at DESC").
		Limit(limit).Offset((page - 1) * limit).
		Find(&rows).Error; err != nil {
		return models.AccessDenialPageResponse{}, fmt.Errorf("list archive: %w", err)
	}

	denials := make([]models.AccessDenial, len(rows))
	for i, r := range rows {
		denials[i] = models.AccessDenial{
			ID: r.ID, UserID: r.UserID, Resource: r.Resource,
			PermissionKey: r.PermissionKey, Reason: r.Reason,
			IPAddress: r.IPAddress, UserAgent: r.UserAgent, CreatedAt: r.CreatedAt,
		}
	}
	items, err := s.attachUserNames(ctx, denials)
	if err != nil {
		return models.AccessDenialPageResponse{}, err
	}
	return models.AccessDenialPageResponse{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

// DeleteByFilter удаляет записи активной таблицы по фильтрам.
// Архив не трогает -- архив только append-only.
func (s *AccessDenialService) DeleteByFilter(ctx context.Context, f models.AccessDenialFilter) (int64, error) {
	q := s.applyFilters(s.db.WithContext(ctx).Model(&models.AccessDenial{}), f)
	res := q.Delete(&models.AccessDenial{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete denials: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// ArchiveOlderThan переносит записи старше cutoff в архивную таблицу.
// Возвращает количество перемещённых записей.
// Используется и cron-ом (раз в сутки) и ручным endpoint-ом (за период).
func (s *AccessDenialService) ArchiveOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	var moved int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []models.AccessDenial
		if err := tx.Where("created_at < ?", cutoff).Find(&rows).Error; err != nil {
			return fmt.Errorf("select for archive: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}

		archives := make([]models.AccessDenialArchive, len(rows))
		now := time.Now()
		for i, r := range rows {
			archives[i] = models.AccessDenialArchive{
				ID: r.ID, UserID: r.UserID, Resource: r.Resource,
				PermissionKey: r.PermissionKey, Reason: r.Reason,
				IPAddress: r.IPAddress, UserAgent: r.UserAgent,
				CreatedAt: r.CreatedAt, ArchivedAt: now,
			}
		}
		// Insert игнорирует конфликты по PK (если archive уже содержит запись).
		if err := tx.Clauses().Create(&archives).Error; err != nil {
			return fmt.Errorf("insert archive: %w", err)
		}
		if err := tx.Where("created_at < ?", cutoff).Delete(&models.AccessDenial{}).Error; err != nil {
			return fmt.Errorf("delete moved: %w", err)
		}
		moved = int64(len(rows))
		return nil
	})
	return moved, err
}

func (s *AccessDenialService) applyFilters(q *gorm.DB, f models.AccessDenialFilter) *gorm.DB {
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.Resource != nil && *f.Resource != "" {
		q = q.Where("resource ILIKE ?", "%"+*f.Resource+"%")
	}
	if f.Reason != nil && *f.Reason != "" {
		q = q.Where("reason = ?", *f.Reason)
	}
	if f.From != nil {
		q = q.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("created_at <= ?", *f.To)
	}
	return q
}

func (s *AccessDenialService) applyFiltersArchive(q *gorm.DB, f models.AccessDenialFilter) *gorm.DB {
	return s.applyFilters(q, f)
}

func (s *AccessDenialService) attachUserNames(ctx context.Context, denials []models.AccessDenial) ([]models.AccessDenialResponse, error) {
	if len(denials) == 0 {
		return []models.AccessDenialResponse{}, nil
	}
	userIDs := make(map[int]struct{})
	for _, d := range denials {
		if d.UserID != nil {
			userIDs[*d.UserID] = struct{}{}
		}
	}
	names := make(map[int]string, len(userIDs))
	if len(userIDs) > 0 {
		ids := make([]int, 0, len(userIDs))
		for id := range userIDs {
			ids = append(ids, id)
		}
		var users []models.User
		if err := s.db.WithContext(ctx).Select("id, username, last_name, first_name, middle_name").
			Where("id IN ?", ids).Find(&users).Error; err != nil {
			return nil, fmt.Errorf("load user names: %w", err)
		}
		for _, u := range users {
			names[u.ID] = formatShortName(u.LastName, u.FirstName, u.MiddleName)
		}
	}
	result := make([]models.AccessDenialResponse, len(denials))
	for i, d := range denials {
		var name string
		if d.UserID != nil {
			name = names[*d.UserID]
		}
		result[i] = models.AccessDenialResponse{
			ID:            d.ID,
			UserID:        d.UserID,
			UserName:      name,
			Resource:      d.Resource,
			PermissionKey: d.PermissionKey,
			Reason:        d.Reason,
			IPAddress:     d.IPAddress,
			UserAgent:     d.UserAgent,
			CreatedAt:     d.CreatedAt,
		}
	}
	return result, nil
}

func normalizePage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}
	return page, limit
}
