package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// MarkService - бизнес-логика справочника марок автомобилей с историчностью.
type MarkService interface {
	GetAll(ctx context.Context, includeArchived bool) ([]models.Mark, error)
	GetByID(ctx context.Context, id int) (*models.Mark, error)
	Create(ctx context.Context, req models.CreateMarkRequest, userID int) (*models.Mark, error)
	Update(ctx context.Context, id int, req models.UpdateMarkRequest, userID int) error
	Archive(ctx context.Context, id int, userID int) error
	Restore(ctx context.Context, id int, userID int) error
	BulkArchive(ctx context.Context, ids []int, userID int) (*BulkOpResult, error)
	BulkRestore(ctx context.Context, ids []int, userID int) (*BulkOpResult, error)
	GetHistory(ctx context.Context, id int) ([]models.MarkHistoryItem, error)
}

type markService struct {
	db       *gorm.DB
	recorder AuditRecorder
}

// NewMarkService создаёт реализацию MarkService.
func NewMarkService(db *gorm.DB) MarkService {
	return &markService{db: db, recorder: NewAuditRecorder(db)}
}

func (s *markService) GetAll(ctx context.Context, includeArchived bool) ([]models.Mark, error) {
	marks := make([]models.Mark, 0)
	q := s.db.WithContext(ctx).Order("name")
	if !includeArchived {
		q = q.Where("is_active = ?", true)
	}
	if err := q.Find(&marks).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения марок")
	}
	return marks, nil
}

func (s *markService) GetByID(ctx context.Context, id int) (*models.Mark, error) {
	var m models.Mark
	if err := s.db.WithContext(ctx).First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Марка не найдена")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения марки")
	}
	return &m, nil
}

func (s *markService) Create(ctx context.Context, req models.CreateMarkRequest, userID int) (*models.Mark, error) {
	mark := models.Mark{
		Name:            req.Name,
		IsActive:        true,
		CreatedByUserID: &userID,
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&mark).Error; err != nil {
			return err
		}
		return s.recorder.Record(ctx, tx, models.AuditEntityMark, &mark.ID, models.MarkActionCreated, &userID,
			map[string]any{"new_value": req.Name})
	})
	if err != nil {
		// uniqueIndex на name дублирует - 409.
		return nil, echo.NewHTTPError(http.StatusConflict, "Марка с таким именем уже существует")
	}
	return &mark, nil
}

func (s *markService) Update(ctx context.Context, id int, req models.UpdateMarkRequest, userID int) error {
	var existing models.Mark
	if err := s.db.WithContext(ctx).First(&existing, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Марка не найдена")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения марки")
	}
	if existing.Name == req.Name {
		return nil
	}
	oldName := existing.Name
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&existing).Update("name", req.Name).Error; err != nil {
			return err
		}
		return s.recorder.Record(ctx, tx, models.AuditEntityMark, &id, models.MarkActionRenamed, &userID,
			map[string]any{"old_value": oldName, "new_value": req.Name})
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusConflict, "Марка с таким именем уже существует")
	}
	return nil
}

func (s *markService) Archive(ctx context.Context, id int, userID int) error {
	return s.setActive(ctx, id, false, userID, models.MarkActionArchived)
}

func (s *markService) Restore(ctx context.Context, id int, userID int) error {
	return s.setActive(ctx, id, true, userID, models.MarkActionRestored)
}

// BulkArchive архивирует набор марок через Archive. Несуществующие -> в Errors
// (частичный успех 207), не валят операцию. Дубли id дедуплицируются.
func (s *markService) BulkArchive(ctx context.Context, ids []int, userID int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		m, err := s.GetByID(ctx, id)
		if err != nil {
			res.addError(id, "", "Марка не найдена")
			continue
		}
		if err := s.Archive(ctx, id, userID); err != nil {
			res.addError(id, m.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkRestore восстанавливает набор марок через Restore.
func (s *markService) BulkRestore(ctx context.Context, ids []int, userID int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		m, err := s.GetByID(ctx, id)
		if err != nil {
			res.addError(id, "", "Марка не найдена")
			continue
		}
		if err := s.Restore(ctx, id, userID); err != nil {
			res.addError(id, m.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

func (s *markService) setActive(ctx context.Context, id int, active bool, userID int, action string) error {
	var existing models.Mark
	if err := s.db.WithContext(ctx).First(&existing, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Марка не найдена")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения марки")
	}
	if existing.IsActive == active {
		return nil // no-op
	}
	if active {
		// Partial-unique теперь только среди активных: при восстановлении проверяем,
		// что нет активной марки с тем же именем - иначе Update упал бы 500 вместо 409.
		var cnt int64
		if err := s.db.WithContext(ctx).Model(&models.Mark{}).
			Where("name = ? AND is_active = ? AND id <> ?", existing.Name, true, id).Count(&cnt).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки имени марки")
		}
		if cnt > 0 {
			return echo.NewHTTPError(http.StatusConflict, "Активная марка с таким именем уже существует")
		}
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&existing).Update("is_active", active).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка обновления марки")
		}
		return s.recorder.Record(ctx, tx, models.AuditEntityMark, &id, action, &userID, nil)
	})
}

// GetHistory возвращает историю изменений марки (новые сверху).
// Переходный период #870: запись уже идёт в audit_log, но старые строки лежат в
// замороженной mark_histories до финального backfill. Чтение объединяет обе
// таблицы в одинаковую форму ответа (форму стережёт TestMarkService_CRUD).
//
// Плоская схема mark_histories (old_value/new_value) маппится в детали аудит-лога
// при записи; обратно из audit_log извлекаем через ->> оператор jsonb.
func (s *markService) GetHistory(ctx context.Context, id int) ([]models.MarkHistoryItem, error) {
	const actorName = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`
	// Read-switch #870 (F.3): до-cutover строки mark_histories подняты в audit_log
	// разовым backfill'ом (old_value/new_value свёрнуты в details), читаем только
	// audit_log. comment приложением никогда не писался (плоская колонка всегда NULL) -
	// сохраняем NULL. Форму стережёт TestMarks_History_BackfillLegacyIntoAudit.
	sql := `
		SELECT a.id AS id,
			COALESCE(a.entity_id, 0) AS mark_id,
			a.action AS action_type,
			a.details->>'old_value' AS old_value,
			a.details->>'new_value' AS new_value,
			a.actor_user_id AS user_id,
			` + actorName + ` AS user_name,
			NULL::text AS comment,
			a.created_at AS created_at
		FROM audit_log a LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE a.entity_type = ? AND a.entity_id = ?
		ORDER BY a.created_at DESC, a.id DESC`

	type row struct {
		ID         int       `gorm:"column:id"`
		MarkID     int       `gorm:"column:mark_id"`
		ActionType string    `gorm:"column:action_type"`
		OldValue   *string   `gorm:"column:old_value"`
		NewValue   *string   `gorm:"column:new_value"`
		UserID     *int      `gorm:"column:user_id"`
		UserName   string    `gorm:"column:user_name"`
		Comment    *string   `gorm:"column:comment"`
		CreatedAt  time.Time `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityMark, id).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения истории")
	}

	// Логин вместо ФИО у акторов, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, s.db)
	items := make([]models.MarkHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.MarkHistoryItem{
			ID:         r.ID,
			MarkID:     r.MarkID,
			ActionType: r.ActionType,
			OldValue:   r.OldValue,
			NewValue:   r.NewValue,
			UserID:     r.UserID,
			UserName:   maskName(masks, r.UserID, r.UserName),
			Comment:    r.Comment,
			CreatedAt:  r.CreatedAt,
		})
	}
	return items, nil
}
