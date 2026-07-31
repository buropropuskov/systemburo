package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// CitizenshipService -- интерфейс бизнес-логики гражданств.
// Авторизация изменяющих операций (page.admin) -- на роут-middleware RequirePermissionV2.
type CitizenshipService interface {
	GetAll(ctx context.Context, includeArchived bool) ([]models.Citizenship, error)
	Create(ctx context.Context, userID int, req models.CreateCitizenshipRequest) (int, error)
	Update(ctx context.Context, userID, id int, req models.UpdateCitizenshipRequest) error
	Delete(ctx context.Context, userID, id int) error
	Restore(ctx context.Context, userID, id int) error
	BulkArchive(ctx context.Context, ids []int, userID int) (*BulkOpResult, error)
	BulkRestore(ctx context.Context, ids []int, userID int) (*BulkOpResult, error)
	GetHistory(ctx context.Context, id int) ([]models.CitizenshipHistoryItem, error)
	ClearDefaults(ctx context.Context) error
}

type citizenshipService struct {
	db       *gorm.DB
	recorder AuditRecorder
}

// NewCitizenshipService создаёт реализацию CitizenshipService.
func NewCitizenshipService(db *gorm.DB) CitizenshipService {
	return &citizenshipService{db: db, recorder: NewAuditRecorder(db)}
}

// GetAll возвращает список гражданств.
// По умолчанию только активные; includeArchived=true добавляет архивные.
func (s *citizenshipService) GetAll(ctx context.Context, includeArchived bool) ([]models.Citizenship, error) {
	citizenships := make([]models.Citizenship, 0)
	q := s.db.WithContext(ctx).Order("name")
	if !includeArchived {
		q = q.Where("is_active = ?", true)
	}
	if err := q.Find(&citizenships).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching citizenships")
	}
	return citizenships, nil
}

// Create создаёт новое гражданство с опциональной установкой по умолчанию.
func (s *citizenshipService) Create(ctx context.Context, userID int, req models.CreateCitizenshipRequest) (int, error) {
	isDefault := req.IsDefault != nil && *req.IsDefault
	patentRequired := req.PatentRequired != nil && *req.PatentRequired

	citizenship := models.Citizenship{
		Name:           req.Name,
		Icon:           req.Icon,
		IsActive:       true,
		IsDefault:      isDefault,
		PatentRequired: patentRequired,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Если выбран как гражданство по умолчанию, снимаем флаг у остальных
		if isDefault {
			if err := tx.Model(&models.Citizenship{}).
				Where("is_default = ?", true).
				Update("is_default", false).Error; err != nil {
				slog.Error("не удалось сбросить гражданства по умолчанию", "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error clearing default citizenships")
			}
		}

		if err := tx.Create(&citizenship).Error; err != nil {
			slog.Error("не удалось создать гражданство", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error creating citizenship")
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	slog.Info("гражданство создано", "id", citizenship.ID)
	s.recorder.Log(ctx, nil, models.AuditEntityCitizenship, &citizenship.ID, models.CitizenshipActionCreated, &userID, map[string]any{"name": req.Name})
	return citizenship.ID, nil
}

// Update обновляет гражданство по ID. is_active не трогает - архивацией/восстановлением
// управляют Delete/Restore (отдельные действия в истории).
func (s *citizenshipService) Update(ctx context.Context, userID, id int, req models.UpdateCitizenshipRequest) error {
	isDefault := req.IsDefault != nil && *req.IsDefault
	patentRequired := req.PatentRequired != nil && *req.PatentRequired

	var details map[string]any
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Снимок до изменений - для diff в истории. First даёт чистый 404, если нет.
		var prev models.Citizenship
		if err := tx.First(&prev, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "Гражданство не найдено")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching citizenship")
		}

		// Если выбран как гражданство по умолчанию, снимаем флаг у остальных
		if isDefault {
			if err := tx.Model(&models.Citizenship{}).
				Where("is_default = ? AND id != ?", true, id).
				Update("is_default", false).Error; err != nil {
				slog.Error("не удалось сбросить гражданства по умолчанию", "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error clearing default citizenships")
			}
		}

		if err := tx.Model(&models.Citizenship{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"name":            req.Name,
				"icon":            req.Icon,
				"is_default":      isDefault,
				"patent_required": patentRequired,
			}).Error; err != nil {
			slog.Error("не удалось обновить гражданство", "id", id, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating citizenship")
		}
		slog.Info("гражданство обновлено", "id", id)
		details = buildCitizenshipUpdateDetails(prev, req.Name, req.Icon, isDefault, patentRequired)
		return nil
	})
	if err != nil {
		return err
	}

	// Логируем только если что-то реально изменилось - иначе спам "Изменены данные".
	if len(details) > 0 {
		s.recorder.Log(ctx, nil, models.AuditEntityCitizenship, &id, models.CitizenshipActionUpdated, &userID, details)
	}
	return nil
}

// Delete архивирует гражданство (soft-delete через is_active=false).
// Гражданство по умолчанию архивировать нельзя - сначала нужно назначить другое.
// Гражданство, используемое сотрудниками (employees.citizenship_id), архивировать
// можно: сотрудники уже созданы, архивное гражданство лишь скрывается из выбора новых.
func (s *citizenshipService) Delete(ctx context.Context, userID, id int) error {
	var citizenship models.Citizenship
	if err := s.db.WithContext(ctx).First(&citizenship, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Гражданство не найдено")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching citizenship")
	}
	if !citizenship.IsActive {
		return nil // уже в архиве - идемпотентно; проверяем до is_default, иначе архивный дефолт залип бы на 409
	}
	if citizenship.IsDefault {
		return echo.NewHTTPError(http.StatusConflict, "Нельзя архивировать гражданство по умолчанию. Сначала назначьте другое гражданство по умолчанию")
	}
	if err := s.db.WithContext(ctx).Model(&citizenship).Update("is_active", false).Error; err != nil {
		slog.Error("не удалось архивировать гражданство", "id", id, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error archiving citizenship")
	}
	slog.Info("гражданство архивировано", "id", id)
	s.recorder.Log(ctx, nil, models.AuditEntityCitizenship, &id, models.CitizenshipActionArchived, &userID, nil)
	return nil
}

// Restore восстанавливает гражданство из архива (is_active=true).
func (s *citizenshipService) Restore(ctx context.Context, userID, id int) error {
	var citizenship models.Citizenship
	if err := s.db.WithContext(ctx).First(&citizenship, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Гражданство не найдено")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching citizenship")
	}
	if citizenship.IsActive {
		return nil // уже активно - идемпотентно
	}
	// У гражданств нет уникального индекса по name, конфликт имени при восстановлении невозможен.
	if err := s.db.WithContext(ctx).Model(&citizenship).Update("is_active", true).Error; err != nil {
		slog.Error("не удалось восстановить гражданство", "id", id, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error restoring citizenship")
	}
	slog.Info("гражданство восстановлено", "id", id)
	s.recorder.Log(ctx, nil, models.AuditEntityCitizenship, &id, models.CitizenshipActionRestored, &userID, nil)
	return nil
}

// loadCitizenship — вспомогательная выборка гражданства для bulk-операций (нужно
// имя для BulkItemError). ok=false, если гражданство не найдено.
func (s *citizenshipService) loadCitizenship(ctx context.Context, id int) (models.Citizenship, bool) {
	var citizenship models.Citizenship
	if err := s.db.WithContext(ctx).First(&citizenship, id).Error; err != nil {
		return citizenship, false
	}
	return citizenship, true
}

// BulkArchive архивирует набор гражданств через Delete. Несуществующие -> в Errors
// (частичный успех 207), не валят операцию. Дубли id дедуплицируются.
func (s *citizenshipService) BulkArchive(ctx context.Context, ids []int, userID int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		c, ok := s.loadCitizenship(ctx, id)
		if !ok {
			res.addError(id, "", "Гражданство не найдено")
			continue
		}
		if err := s.Delete(ctx, userID, id); err != nil {
			res.addError(id, c.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkRestore восстанавливает набор гражданств через Restore.
func (s *citizenshipService) BulkRestore(ctx context.Context, ids []int, userID int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		c, ok := s.loadCitizenship(ctx, id)
		if !ok {
			res.addError(id, "", "Гражданство не найдено")
			continue
		}
		if err := s.Restore(ctx, userID, id); err != nil {
			res.addError(id, c.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// GetHistory возвращает историю изменений гражданства (новые сверху).
// #870, финал F.1: запись и до-cutover строки живут в общем audit_log (старые
// перенесены backfill'ом BackfillAuditFromLegacy), поэтому чтение идёт только из
// audit_log. Замороженная citizenship_histories дропнута в дроп-sweep (F.8).
// Форму ответа стережёт TestCitizenships_History.
func (s *citizenshipService) GetHistory(ctx context.Context, id int) ([]models.CitizenshipHistoryItem, error) {
	const actorName = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`
	sql := `
		SELECT a.id AS id, a.action AS action_type, a.details AS details,
			a.actor_user_id AS actor_user_id, ` + actorName + ` AS actor_name, a.created_at AS created_at
		FROM audit_log a LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE a.entity_type = ? AND a.entity_id = ?
		ORDER BY a.created_at DESC, a.id DESC`

	type row struct {
		ID          int             `gorm:"column:id"`
		ActionType  string          `gorm:"column:action_type"`
		Details     json.RawMessage `gorm:"column:details"`
		ActorUserID *int            `gorm:"column:actor_user_id"`
		ActorName   string          `gorm:"column:actor_name"`
		CreatedAt   time.Time       `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityCitizenship, id).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching citizenship history")
	}

	// Логин вместо ФИО у акторов, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, s.db)
	items := make([]models.CitizenshipHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.CitizenshipHistoryItem{
			ID:          r.ID,
			ActionType:  r.ActionType,
			Details:     r.Details,
			ActorUserID: r.ActorUserID,
			ActorName:   maskName(masks, r.ActorUserID, r.ActorName),
			CreatedAt:   r.CreatedAt,
		})
	}
	return items, nil
}

// ClearDefaults сбрасывает флаг «по умолчанию» у всех гражданств.
func (s *citizenshipService) ClearDefaults(ctx context.Context) error {
	if err := s.db.WithContext(ctx).
		Model(&models.Citizenship{}).
		Where("is_default = ?", true).
		Update("is_default", false).Error; err != nil {
		slog.Error("не удалось сбросить гражданства по умолчанию", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error clearing default citizenships")
	}
	slog.Info("гражданства по умолчанию сброшены")
	return nil
}

// buildCitizenshipUpdateDetails собирает diff изменённых полей гражданства как {old, new}.
// В результат попадают только реально изменившиеся поля - иначе история засоряется
// "пустыми" записями (см. ui-history: фильтр неизменённого обязателен). strPtrVal
// определён в license_format_service.go (тот же пакет).
func buildCitizenshipUpdateDetails(prev models.Citizenship, name string, icon *string, isDefault, patentRequired bool) map[string]any {
	details := map[string]any{}
	if name != prev.Name {
		details["name"] = map[string]any{"old": prev.Name, "new": name}
	}
	if strPtrVal(prev.Icon) != strPtrVal(icon) {
		details["icon"] = map[string]any{"old": strPtrVal(prev.Icon), "new": strPtrVal(icon)}
	}
	if isDefault != prev.IsDefault {
		details["is_default"] = map[string]any{"old": prev.IsDefault, "new": isDefault}
	}
	if patentRequired != prev.PatentRequired {
		details["patent_required"] = map[string]any{"old": prev.PatentRequired, "new": patentRequired}
	}
	return details
}
