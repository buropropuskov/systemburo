package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AttachmentFieldConfigService - оверрайды видимости/обязательности базовых
// полей вложения per-UniqueAttachment (feedback-0608-H / #529).
// Реестр полей - в коде (attachment_fields_registry.go), сервис мержит его с
// оверрайдами из БД и принимает bulk-upsert оверрайдов.
type AttachmentFieldConfigService interface {
	// GetMerged возвращает базовые поля типа вложения, смерженные с оверрайдами.
	GetMerged(ctx context.Context, uniqueAttachmentID int) ([]models.MergedField, error)
	// Save делает bulk-upsert оверрайдов. Ключи не из реестра типа отклоняются.
	Save(ctx context.Context, uniqueAttachmentID int, items []models.FieldConfigItem) error
}

type attachmentFieldConfigService struct {
	db *gorm.DB
}

// NewAttachmentFieldConfigService создаёт сервис.
func NewAttachmentFieldConfigService(db *gorm.DB) AttachmentFieldConfigService {
	return &attachmentFieldConfigService{db: db}
}

// UnknownFieldKeys возвращает ключи, которых нет в реестре указанного типа
// вложения. Пустой результат = все ключи валидны. Чистая функция (без БД).
func UnknownFieldKeys(attachmentType string, keys []string) []string {
	valid := fieldDefByKey(attachmentType)
	var unknown []string
	for _, k := range keys {
		if _, ok := valid[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	return unknown
}

// buildFieldConfigRows готовит строки для bulk-upsert: дедуп по ключу
// (last-wins) и форс required=false для не-Requirable полей. Чистая функция.
//
// Дедуп нужен против PG "ON CONFLICT cannot affect row a second time": один и
// тот же ключ дважды в одном INSERT иначе падает. PUT идемпотентен -
// побеждает последнее значение ключа.
func buildFieldConfigRows(attType string, uaID int, items []models.FieldConfigItem) []models.AttachmentFieldConfig {
	defs := fieldDefByKey(attType)
	byKey := make(map[string]models.FieldConfigItem, len(items))
	order := make([]string, 0, len(items))
	for _, it := range items {
		if _, seen := byKey[it.Key]; !seen {
			order = append(order, it.Key)
		}
		byKey[it.Key] = it
	}

	rows := make([]models.AttachmentFieldConfig, 0, len(order))
	for _, key := range order {
		it := byKey[key]
		required := it.Required
		if d := defs[key]; !d.Requirable {
			required = false
		}
		rows = append(rows, models.AttachmentFieldConfig{
			UniqueAttachmentID: uaID,
			FieldKey:           key,
			Visible:            it.Visible,
			Required:           required,
		})
	}
	return rows
}

func (s *attachmentFieldConfigService) attachmentType(ctx context.Context, uaID int) (string, error) {
	var ua models.UniqueAttachment
	err := s.db.WithContext(ctx).Select("attachment_type").First(&ua, uaID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", echo.NewHTTPError(http.StatusNotFound, "Вложение не найдено")
		}
		return "", echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения вложения")
	}
	return ua.AttachmentType, nil
}

func (s *attachmentFieldConfigService) GetMerged(ctx context.Context, uaID int) ([]models.MergedField, error) {
	attType, err := s.attachmentType(ctx, uaID)
	if err != nil {
		return nil, err
	}

	var overrides []models.AttachmentFieldConfig
	if err := s.db.WithContext(ctx).
		Where("unique_attachment_id = ?", uaID).
		Find(&overrides).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения настройки полей")
	}

	return MergeFieldConfig(attType, overrides), nil
}

func (s *attachmentFieldConfigService) Save(ctx context.Context, uaID int, items []models.FieldConfigItem) error {
	attType, err := s.attachmentType(ctx, uaID)
	if err != nil {
		return err
	}

	keys := make([]string, len(items))
	for i, it := range items {
		keys[i] = it.Key
	}
	if unknown := UnknownFieldKeys(attType, keys); len(unknown) > 0 {
		return echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("Неизвестные поля для типа %q: %v", attType, unknown))
	}

	if len(items) == 0 {
		return nil
	}

	rows := buildFieldConfigRows(attType, uaID, items)

	err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "unique_attachment_id"}, {Name: "field_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"visible", "required", "updated_at"}),
	}).Create(&rows).Error
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка сохранения настройки полей")
	}
	return nil
}
