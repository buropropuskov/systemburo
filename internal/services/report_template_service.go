package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// validTemplateConfig — config должен быть непустым JSON-объектом (снимок формы
// конструктора). json.Valid пропускает скаляры (`null`, числа, строки), поэтому
// дополнительно требуем ведущую `{` — иначе фронт получит не-объект и сломает гид.
func validTemplateConfig(c json.RawMessage) bool {
	t := bytes.TrimSpace(c)
	return len(t) > 0 && json.Valid(t) && t[0] == '{'
}

// Ошибки управления шаблонами отчётов (#632).
var (
	// ErrTemplateNotFound — шаблон не существует.
	ErrTemplateNotFound = errors.New("report template not found")
	// ErrTemplateForbidden — шаблон принадлежит другому пользователю.
	ErrTemplateForbidden = errors.New("report template belongs to another user")
	// ErrTemplateSystem — системный пресет нельзя менять или удалять.
	ErrTemplateSystem = errors.New("system report template is read-only")
	// ErrTemplateInvalidConfig — пустой или невалидный JSON конфигурации.
	ErrTemplateInvalidConfig = errors.New("report template config must be valid non-empty JSON")
)

// ListReportTemplates возвращает доступные пользователю шаблоны: системные пресеты,
// собственные личные и расшаренные чужие. Системные идут первыми, затем по имени.
// owner_user_id расшаренных чужих шаблонов отдаётся намеренно — фронт по нему
// понимает, что шаблон не свой (только применить, без правки/удаления).
func (s *statisticsService) ListReportTemplates(ctx context.Context, userID int) ([]models.ReportTemplate, error) {
	var templates []models.ReportTemplate
	err := s.db.WithContext(ctx).
		Where("is_system = ? OR owner_user_id = ? OR is_shared = ?", true, userID, true).
		Order("is_system DESC, name ASC").
		Find(&templates).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list report templates: %w", err)
	}
	return templates, nil
}

// CreateReportTemplate сохраняет личный шаблон текущего пользователя.
func (s *statisticsService) CreateReportTemplate(ctx context.Context, userID int, req models.SaveReportTemplateRequest) (*models.ReportTemplate, error) {
	if !validTemplateConfig(req.Config) {
		return nil, ErrTemplateInvalidConfig
	}
	tpl := models.ReportTemplate{
		Name:        req.Name,
		Description: req.Description,
		Config:      req.Config,
		IsShared:    req.IsShared != nil && *req.IsShared,
		OwnerUserID: &userID,
	}
	if err := s.db.WithContext(ctx).Create(&tpl).Error; err != nil {
		return nil, fmt.Errorf("failed to create report template: %w", err)
	}
	return &tpl, nil
}

// UpdateReportTemplate обновляет личный шаблон владельца. Системные пресеты и чужие
// шаблоны защищены.
func (s *statisticsService) UpdateReportTemplate(ctx context.Context, userID, id int, req models.SaveReportTemplateRequest) (*models.ReportTemplate, error) {
	if !validTemplateConfig(req.Config) {
		return nil, ErrTemplateInvalidConfig
	}
	tpl, err := s.loadOwnedTemplate(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	tpl.Name = req.Name
	tpl.Description = req.Description
	tpl.Config = req.Config
	if req.IsShared != nil {
		tpl.IsShared = *req.IsShared
	}
	if err := s.db.WithContext(ctx).Save(tpl).Error; err != nil {
		return nil, fmt.Errorf("failed to update report template: %w", err)
	}
	return tpl, nil
}

// DeleteReportTemplate удаляет личный шаблон владельца. Системные пресеты и чужие
// шаблоны защищены.
func (s *statisticsService) DeleteReportTemplate(ctx context.Context, userID, id int) error {
	tpl, err := s.loadOwnedTemplate(ctx, userID, id)
	if err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Delete(tpl).Error; err != nil {
		return fmt.Errorf("failed to delete report template: %w", err)
	}
	return nil
}

// loadOwnedTemplate загружает шаблон и проверяет, что текущий пользователь вправе
// его менять: существует, не системный, принадлежит ему.
func (s *statisticsService) loadOwnedTemplate(ctx context.Context, userID, id int) (*models.ReportTemplate, error) {
	var tpl models.ReportTemplate
	err := s.db.WithContext(ctx).First(&tpl, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load report template: %w", err)
	}
	if tpl.IsSystem {
		return nil, ErrTemplateSystem
	}
	if tpl.OwnerUserID == nil || *tpl.OwnerUserID != userID {
		return nil, ErrTemplateForbidden
	}
	return &tpl, nil
}
