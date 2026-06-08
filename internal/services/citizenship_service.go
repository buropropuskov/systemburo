package services

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// CitizenshipService -- интерфейс бизнес-логики гражданств.
type CitizenshipService interface {
	GetAll(ctx context.Context, includeArchived bool) ([]models.Citizenship, error)
	Create(ctx context.Context, typeID, userID int, req models.CreateCitizenshipRequest) (int, error)
	Update(ctx context.Context, typeID, userID, id int, req models.UpdateCitizenshipRequest) error
	Delete(ctx context.Context, typeID, userID, id int) error
	Restore(ctx context.Context, typeID, userID, id int) error
	GetHistory(ctx context.Context, id int) ([]models.CitizenshipHistoryItem, error)
	ClearDefaults(ctx context.Context, typeID int) error
}

type citizenshipService struct {
	db      *gorm.DB
	history CitizenshipHistoryService
}

// NewCitizenshipService создаёт реализацию CitizenshipService.
func NewCitizenshipService(db *gorm.DB) CitizenshipService {
	return &citizenshipService{db: db, history: NewCitizenshipHistoryService(db)}
}

// checkAdmin проверяет, что пользователь с данным type_id является администратором
// (код типа "manager" или "buropropuskov").
func (s *citizenshipService) checkAdmin(ctx context.Context, typeID int) error {
	var code string
	err := s.db.WithContext(ctx).
		Table("user_types").
		Select("code").
		Where("id = ?", typeID).
		Row().
		Scan(&code)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}
	if code != "manager" && code != "buropropuskov" {
		return echo.NewHTTPError(http.StatusForbidden, "Insufficient permissions")
	}
	return nil
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
func (s *citizenshipService) Create(ctx context.Context, typeID, userID int, req models.CreateCitizenshipRequest) (int, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return 0, err
	}

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
	s.history.Log(ctx, citizenship.ID, &userID, models.CitizenshipActionCreated, map[string]any{"name": req.Name})
	return citizenship.ID, nil
}

// Update обновляет гражданство по ID. is_active не трогает - архивацией/восстановлением
// управляют Delete/Restore (отдельные действия в истории).
func (s *citizenshipService) Update(ctx context.Context, typeID, userID, id int, req models.UpdateCitizenshipRequest) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

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
		s.history.Log(ctx, id, &userID, models.CitizenshipActionUpdated, details)
	}
	return nil
}

// Delete архивирует гражданство (soft-delete через is_active=false).
// Гражданство по умолчанию архивировать нельзя - сначала нужно назначить другое.
// Гражданство, используемое сотрудниками (employees.citizenship_id), архивировать
// можно: сотрудники уже созданы, архивное гражданство лишь скрывается из выбора новых.
func (s *citizenshipService) Delete(ctx context.Context, typeID, userID, id int) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

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
	s.history.Log(ctx, id, &userID, models.CitizenshipActionArchived, nil)
	return nil
}

// Restore восстанавливает гражданство из архива (is_active=true).
func (s *citizenshipService) Restore(ctx context.Context, typeID, userID, id int) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

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
	s.history.Log(ctx, id, &userID, models.CitizenshipActionRestored, nil)
	return nil
}

// GetHistory возвращает историю изменений гражданства (новые сверху).
func (s *citizenshipService) GetHistory(ctx context.Context, id int) ([]models.CitizenshipHistoryItem, error) {
	return s.history.GetHistory(ctx, id)
}

// ClearDefaults сбрасывает флаг «по умолчанию» у всех гражданств.
func (s *citizenshipService) ClearDefaults(ctx context.Context, typeID int) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

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
