package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// AttachmentTemplateService - CRUD шаблонов Excel-бланков (#183).
// Генерация .xlsx вынесена в отдельный сервис AttachmentBlankService
// (будет реализована в следующем PR).
type AttachmentTemplateService interface {
	Get(ctx context.Context, uniqueAttachmentID int) (*models.AttachmentTemplate, error)
	Upload(ctx context.Context, uniqueAttachmentID int, file *multipart.FileHeader, req models.CreateTemplateRequest, userID int) (*models.AttachmentTemplate, error)
	UpdateMappings(ctx context.Context, uniqueAttachmentID int, req models.UpdateMappingsRequest) error
	Delete(ctx context.Context, uniqueAttachmentID int) error
	// Custom fields
	ListCustomFields(ctx context.Context, uniqueAttachmentID int) ([]models.AttachmentCustomField, error)
	CreateCustomField(ctx context.Context, uniqueAttachmentID int, req models.CreateCustomFieldRequest) (*models.AttachmentCustomField, error)
	UpdateCustomField(ctx context.Context, id int, req models.CreateCustomFieldRequest) error
	DeleteCustomField(ctx context.Context, id int) error
}

type attachmentTemplateService struct {
	db         *gorm.DB
	uploadPath string // базовый путь, обычно cfg.UploadPath. Шаблоны в <uploadPath>/templates/
}

// NewAttachmentTemplateService создаёт сервис.
func NewAttachmentTemplateService(db *gorm.DB, uploadPath string) AttachmentTemplateService {
	return &attachmentTemplateService{db: db, uploadPath: uploadPath}
}

func (s *attachmentTemplateService) Get(ctx context.Context, uaID int) (*models.AttachmentTemplate, error) {
	var t models.AttachmentTemplate
	err := s.db.WithContext(ctx).
		Preload("Mappings").
		Where("unique_attachment_id = ?", uaID).
		First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Шаблон не настроен")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения шаблона")
	}
	return &t, nil
}

func (s *attachmentTemplateService) Upload(ctx context.Context, uaID int, file *multipart.FileHeader, req models.CreateTemplateRequest, userID int) (*models.AttachmentTemplate, error) {
	var ua models.UniqueAttachment
	if err := s.db.WithContext(ctx).First(&ua, uaID).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Вложение не найдено")
	}

	dir := filepath.Join(s.uploadPath, "templates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Не удалось создать директорию шаблонов")
	}
	dst := filepath.Join(dir, strconv.Itoa(uaID)+".xlsx")
	if err := saveMultipartFile(file, dst); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Не удалось сохранить файл")
	}

	t := models.AttachmentTemplate{
		UniqueAttachmentID: uaID,
		FilePath:           dst,
		OriginalFileName:   file.Filename,
		ListStartRow:       req.ListStartRow,
		ListEndRow:         req.ListEndRow,
		MaxListRows:        req.MaxListRows,
		UploadedByUserID:   &userID,
	}
	if t.MaxListRows == 0 && t.ListStartRow > 0 && t.ListEndRow >= t.ListStartRow {
		t.MaxListRows = t.ListEndRow - t.ListStartRow + 1
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("template_id IN (SELECT id FROM attachment_templates WHERE unique_attachment_id = ?)", uaID).
			Delete(&models.AttachmentTemplateMapping{}).Error; err != nil {
			return fmt.Errorf("failed to delete old mappings: %w", err)
		}
		if err := tx.Where("unique_attachment_id = ?", uaID).
			Delete(&models.AttachmentTemplate{}).Error; err != nil {
			return fmt.Errorf("failed to delete old template: %w", err)
		}
		return tx.Create(&t).Error
	})
	if err != nil {
		_ = os.Remove(dst)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Не удалось сохранить шаблон")
	}
	return &t, nil
}

func (s *attachmentTemplateService) UpdateMappings(ctx context.Context, uaID int, req models.UpdateMappingsRequest) error {
	var t models.AttachmentTemplate
	if err := s.db.WithContext(ctx).Where("unique_attachment_id = ?", uaID).First(&t).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Шаблон не настроен")
	}

	// Валидация: все field_path должны быть либо встроенными, либо custom-полями этого UA.
	customIDs := map[string]struct{}{}
	var customFields []models.AttachmentCustomField
	s.db.WithContext(ctx).Where("unique_attachment_id = ?", uaID).Find(&customFields)
	for _, cf := range customFields {
		customIDs[fmt.Sprintf("custom.%d", cf.ID)] = struct{}{}
	}
	for _, m := range req.Mappings {
		if !IsValidFieldPath(m.FieldPath) {
			if _, ok := customIDs[m.FieldPath]; !ok {
				return echo.NewHTTPError(http.StatusBadRequest,
					fmt.Sprintf("Неизвестное поле: %s", m.FieldPath))
			}
		}
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("template_id = ?", t.ID).Delete(&models.AttachmentTemplateMapping{}).Error; err != nil {
			return err
		}
		if len(req.Mappings) == 0 {
			return nil
		}
		rows := make([]models.AttachmentTemplateMapping, 0, len(req.Mappings))
		for _, m := range req.Mappings {
			rows = append(rows, models.AttachmentTemplateMapping{
				TemplateID:  t.ID,
				CellRef:     m.CellRef,
				FieldPath:   m.FieldPath,
				IsListField: m.IsListField,
			})
		}
		return tx.Create(&rows).Error
	})
}

func (s *attachmentTemplateService) Delete(ctx context.Context, uaID int) error {
	var t models.AttachmentTemplate
	if err := s.db.WithContext(ctx).Where("unique_attachment_id = ?", uaID).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка")
	}
	_ = os.Remove(t.FilePath)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("template_id = ?", t.ID).Delete(&models.AttachmentTemplateMapping{}).Error; err != nil {
			return err
		}
		return tx.Delete(&t).Error
	})
}

func (s *attachmentTemplateService) ListCustomFields(ctx context.Context, uaID int) ([]models.AttachmentCustomField, error) {
	fields := make([]models.AttachmentCustomField, 0)
	err := s.db.WithContext(ctx).
		Where("unique_attachment_id = ? AND is_active = ?", uaID, true).
		Order("sort_order, id").
		Find(&fields).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения полей")
	}
	return fields, nil
}

func (s *attachmentTemplateService) CreateCustomField(ctx context.Context, uaID int, req models.CreateCustomFieldRequest) (*models.AttachmentCustomField, error) {
	cf := models.AttachmentCustomField{
		UniqueAttachmentID: uaID,
		Label:              req.Label,
		Placeholder:        req.Placeholder,
		SortOrder:          req.SortOrder,
		IsActive:           true,
	}
	if err := s.db.WithContext(ctx).Create(&cf).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Не удалось создать поле")
	}
	return &cf, nil
}

func (s *attachmentTemplateService) UpdateCustomField(ctx context.Context, id int, req models.CreateCustomFieldRequest) error {
	updates := map[string]any{
		"label":       req.Label,
		"placeholder": req.Placeholder,
		"sort_order":  req.SortOrder,
	}
	res := s.db.WithContext(ctx).Model(&models.AttachmentCustomField{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось обновить поле")
	}
	if res.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Поле не найдено")
	}
	return nil
}

func (s *attachmentTemplateService) DeleteCustomField(ctx context.Context, id int) error {
	// Soft delete: is_active=false. Старые значения для уже поданных заявок остаются.
	res := s.db.WithContext(ctx).Model(&models.AttachmentCustomField{}).Where("id = ?", id).Update("is_active", false)
	if res.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Поле не найдено")
	}
	return nil
}

// saveMultipartFile - helper для сохранения multipart-файла на диск.
func saveMultipartFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}
