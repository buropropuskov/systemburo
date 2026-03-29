package services

import (
	"context"
	"net/http"
	"strings"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// AttachmentService -- интерфейс бизнес-логики шаблонов вложений (unique_attachments).
type AttachmentService interface {
	// GetActive возвращает все активные шаблоны вложений.
	GetActive(ctx context.Context) ([]models.UniqueAttachment, error)
	// GetAll возвращает все шаблоны вложений (активные и архивные).
	GetAll(ctx context.Context) ([]models.UniqueAttachment, error)
	// GetByID возвращает активный шаблон вложения по ID.
	GetByID(ctx context.Context, id int) (*models.UniqueAttachment, error)
	// Create создаёт новый шаблон вложения.
	Create(ctx context.Context, req models.CreateUniqueAttachmentRequest) (*models.CreateUniqueAttachmentResponse, error)
	// Update обновляет существующий шаблон вложения.
	Update(ctx context.Context, id int, req models.UpdateUniqueAttachmentRequest) error
	// Delete выполняет мягкое удаление шаблона вложения.
	Delete(ctx context.Context, id int) error
	// Restore восстанавливает ранее удалённый шаблон вложения.
	Restore(ctx context.Context, id int) error
}

type attachmentService struct {
	db *gorm.DB
}

// NewAttachmentService создаёт новый экземпляр AttachmentService.
func NewAttachmentService(db *gorm.DB) AttachmentService {
	return &attachmentService{db: db}
}

func (s *attachmentService) GetActive(ctx context.Context) ([]models.UniqueAttachment, error) {
	attachments := make([]models.UniqueAttachment, 0)
	err := s.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("title, display_name").
		Find(&attachments).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachments")
	}
	return attachments, nil
}

func (s *attachmentService) GetAll(ctx context.Context) ([]models.UniqueAttachment, error) {
	attachments := make([]models.UniqueAttachment, 0)
	err := s.db.WithContext(ctx).
		Order("is_active DESC, title, display_name").
		Find(&attachments).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachments")
	}
	return attachments, nil
}

func (s *attachmentService) GetByID(ctx context.Context, id int) (*models.UniqueAttachment, error) {
	var attachment models.UniqueAttachment
	err := s.db.WithContext(ctx).
		Where("id = ? AND is_active = ?", id, true).
		First(&attachment).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Attachment not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachment")
	}
	return &attachment, nil
}

func (s *attachmentService) Create(ctx context.Context, req models.CreateUniqueAttachmentRequest) (*models.CreateUniqueAttachmentResponse, error) {
	// Проверяем уникальность имени среди активных
	var count int64
	err := s.db.WithContext(ctx).
		Model(&models.UniqueAttachment{}).
		Where("name = ? AND is_active = ?", req.Name, true).
		Count(&count).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking attachment existence")
	}
	if count > 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Attachment with this name already exists")
	}

	titleUpper := strings.ToUpper(req.Title)
	attachment := models.UniqueAttachment{
		AttachmentType: req.AttachmentType,
		Name:           &req.Name,
		DisplayName:    &req.DisplayName,
		Title:          &titleUpper,
		Instruction:    req.Instruction,
		IsActive:       true,
	}

	if err := s.db.WithContext(ctx).Create(&attachment).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return &models.CreateUniqueAttachmentResponse{
		ID:      attachment.ID,
		Message: "Вложение успешно создано",
	}, nil
}

func (s *attachmentService) Update(ctx context.Context, id int, req models.UpdateUniqueAttachmentRequest) error {
	// Проверяем существование среди активных
	var count int64
	err := s.db.WithContext(ctx).
		Model(&models.UniqueAttachment{}).
		Where("id = ? AND is_active = ?", id, true).
		Count(&count).Error
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking attachment existence")
	}
	if count == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Attachment not found")
	}

	titleUpper := strings.ToUpper(req.Title)
	result := s.db.WithContext(ctx).
		Model(&models.UniqueAttachment{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"attachment_type": req.AttachmentType,
			"name":           req.Name,
			"display_name":   req.DisplayName,
			"title":          titleUpper,
			"instruction":    req.Instruction,
		})
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating attachment")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Вложение не найдено")
	}
	return nil
}

func (s *attachmentService) Delete(ctx context.Context, id int) error {
	result := s.db.WithContext(ctx).
		Model(&models.UniqueAttachment{}).
		Where("id = ?", id).
		Update("is_active", false)
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting attachment")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Вложение не найдено")
	}
	return nil
}

func (s *attachmentService) Restore(ctx context.Context, id int) error {
	result := s.db.WithContext(ctx).
		Model(&models.UniqueAttachment{}).
		Where("id = ?", id).
		Update("is_active", true)
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error restoring attachment")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Вложение не найдено")
	}
	return nil
}
