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

// LicensePlateFormatService определяет интерфейс бизнес-логики форматов номерных знаков.
type LicensePlateFormatService interface {
	GetAll(ctx context.Context, includeArchived bool) ([]models.LicensePlateFormatWithCells, error)
	Create(ctx context.Context, req models.CreateLicensePlateFormatRequest) (int, error)
	Update(ctx context.Context, id int, req models.UpdateLicensePlateFormatRequest) error
	Delete(ctx context.Context, id int) error
	Restore(ctx context.Context, id int) error
}

type licensePlateFormatService struct {
	db *gorm.DB
}

// NewLicensePlateFormatService создаёт сервис для управления форматами номерных знаков.
func NewLicensePlateFormatService(db *gorm.DB) LicensePlateFormatService {
	return &licensePlateFormatService{db: db}
}

// GetAll возвращает форматы номерных знаков с ячейками.
// По умолчанию только активные; includeArchived=true добавляет архивные.
func (s *licensePlateFormatService) GetAll(ctx context.Context, includeArchived bool) ([]models.LicensePlateFormatWithCells, error) {
	formats := make([]models.LicensePlateFormat, 0)
	q := s.db.WithContext(ctx).Order("name")
	if !includeArchived {
		q = q.Where("is_active = ?", true)
	}
	if err := q.Find(&formats).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching license plate formats")
	}

	result := make([]models.LicensePlateFormatWithCells, 0, len(formats))
	for _, f := range formats {
		cells := make([]models.LicensePlateFormatCell, 0)
		if err := s.db.WithContext(ctx).
			Where("format_id = ?", f.ID).
			Order("cell_order").
			Find(&cells).Error; err != nil {
			continue
		}
		result = append(result, models.LicensePlateFormatWithCells{
			Format: f,
			Cells:  cells,
		})
	}

	return result, nil
}

// Create создаёт формат номерного знака с ячейками в транзакции.
func (s *licensePlateFormatService) Create(ctx context.Context, req models.CreateLicensePlateFormatRequest) (int, error) {
	var formatID int

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if req.IsDefault != nil && *req.IsDefault {
			if err := tx.Model(&models.LicensePlateFormat{}).
				Where("is_default = ?", true).
				Update("is_default", false).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error clearing default formats")
			}
		}

		format := models.LicensePlateFormat{
			Name:        req.Name,
			CountryCode: req.CountryCode,
			Icon:        req.Icon,
			IsActive:    true,
			IsDefault:   req.IsDefault != nil && *req.IsDefault,
		}
		if err := tx.Create(&format).Error; err != nil {
			slog.Error("не удалось создать формат номеров", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка при создании формата номеров")
		}
		formatID = format.ID

		for _, c := range req.Cells {
			cell := models.LicensePlateFormatCell{
				FormatID:       formatID,
				CellOrder:      c.CellOrder,
				CellType:       c.CellType,
				MinLength:      c.MinLength,
				MaxLength:      c.MaxLength,
				AllowedLetters: c.AllowedLetters,
				AlphabetType:   c.AlphabetType,
				Language:       c.Language,
				PaddingChar:    ptrOrDefault(c.PaddingChar, "0"),
				PaddingSide:    ptrOrDefault(c.PaddingSide, "left"),
			}
			if err := tx.Create(&cell).Error; err != nil {
				slog.Error("не удалось создать ячейку формата номеров", "format_id", formatID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка при создании ячейки формата номеров")
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}
	slog.Info("формат номеров создан", "id", formatID)
	return formatID, nil
}

// Update обновляет формат номерного знака и пересоздаёт ячейки.
func (s *licensePlateFormatService) Update(ctx context.Context, id int, req models.UpdateLicensePlateFormatRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if req.IsDefault != nil && *req.IsDefault {
			if err := tx.Model(&models.LicensePlateFormat{}).
				Where("is_default = ? AND id != ?", true, id).
				Update("is_default", false).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error clearing default formats")
			}
		}

		isDefault := req.IsDefault != nil && *req.IsDefault
		result := tx.Model(&models.LicensePlateFormat{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"name":         req.Name,
				"country_code": req.CountryCode,
				"icon":         req.Icon,
				"is_default":   isDefault,
			})
		if result.Error != nil {
			slog.Error("не удалось обновить формат номеров", "id", id, "error", result.Error)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating license plate format")
		}
		if result.RowsAffected == 0 {
			return echo.NewHTTPError(http.StatusNotFound, "Формат номеров не найден")
		}
		slog.Info("формат номеров обновлён", "id", id)

		if err := tx.Where("format_id = ?", id).Delete(&models.LicensePlateFormatCell{}).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating format cells")
		}

		for _, c := range req.Cells {
			cell := models.LicensePlateFormatCell{
				FormatID:       id,
				CellOrder:      c.CellOrder,
				CellType:       c.CellType,
				MinLength:      c.MinLength,
				MaxLength:      c.MaxLength,
				AllowedLetters: c.AllowedLetters,
				AlphabetType:   c.AlphabetType,
				Language:       c.Language,
				PaddingChar:    ptrOrDefault(c.PaddingChar, "0"),
				PaddingSide:    ptrOrDefault(c.PaddingSide, "left"),
			}
			if err := tx.Create(&cell).Error; err != nil {
				slog.Error("не удалось создать ячейку формата номеров при обновлении", "format_id", id, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка при создании ячейки формата номеров")
			}
		}

		return nil
	})
}

// Delete архивирует формат номерного знака (soft-delete через is_active=false).
// Формат по умолчанию архивировать нельзя - сначала нужно назначить другой дефолтный.
// Формат, используемый машинами (unique_cars.format_id), архивировать можно: машины уже
// созданы и валидацию прошли, архивный формат лишь скрывается из выбора новых.
func (s *licensePlateFormatService) Delete(ctx context.Context, id int) error {
	var format models.LicensePlateFormat
	if err := s.db.WithContext(ctx).First(&format, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Формат номеров не найден")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения формата номеров")
	}
	if !format.IsActive {
		return nil // уже в архиве - идемпотентно; проверяем до is_default, иначе архивный дефолт залип бы на 409
	}
	if format.IsDefault {
		return echo.NewHTTPError(http.StatusConflict, "Нельзя архивировать формат по умолчанию. Сначала назначьте другой формат по умолчанию")
	}
	if err := s.db.WithContext(ctx).Model(&format).Update("is_active", false).Error; err != nil {
		slog.Error("не удалось архивировать формат номеров", "id", id, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка архивации формата номеров")
	}
	slog.Info("формат номеров архивирован", "id", id)
	return nil
}

// Restore восстанавливает формат номерного знака из архива (is_active=true).
func (s *licensePlateFormatService) Restore(ctx context.Context, id int) error {
	var format models.LicensePlateFormat
	if err := s.db.WithContext(ctx).First(&format, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Формат номеров не найден")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения формата номеров")
	}
	if format.IsActive {
		return nil // уже активен - идемпотентно
	}
	// У форматов нет уникального индекса по name (в отличие от marks/organizations),
	// поэтому при восстановлении проверка конфликта имени не нужна.
	if err := s.db.WithContext(ctx).Model(&format).Update("is_active", true).Error; err != nil {
		slog.Error("не удалось восстановить формат номеров", "id", id, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка восстановления формата номеров")
	}
	slog.Info("формат номеров восстановлен", "id", id)
	return nil
}

// ptrOrDefault возвращает указатель на значение или значение по умолчанию.
func ptrOrDefault(p *string, def string) *string {
	if p != nil {
		return p
	}
	return &def
}
