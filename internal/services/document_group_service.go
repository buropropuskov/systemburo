package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// DocumentGroupService -- интерфейс управления группами документов.
type DocumentGroupService interface {
	List(ctx context.Context) ([]models.DocumentGroupWithCount, error)
	Create(ctx context.Context, userID int, req models.CreateDocumentGroupRequest) (*models.DocumentGroup, error)
	Update(ctx context.Context, userID int, id int, req models.UpdateDocumentGroupRequest) (*models.DocumentGroup, error)
	Delete(ctx context.Context, id int) error
	Reorder(ctx context.Context, req models.ReorderDocumentGroupsRequest) error
}

type documentGroupService struct {
	db *gorm.DB
}

// NewDocumentGroupService создаёт реализацию DocumentGroupService.
func NewDocumentGroupService(db *gorm.DB) DocumentGroupService {
	return &documentGroupService{db: db}
}

func (s *documentGroupService) List(ctx context.Context) ([]models.DocumentGroupWithCount, error) {
	results := make([]models.DocumentGroupWithCount, 0)
	err := s.db.WithContext(ctx).
		Table("document_groups dg").
		Select(`dg.id, dg.name, dg.sort_order, dg.created_at, dg.updated_at,
			dg.created_by, dg.updated_by,
			COUNT(d.id) AS count`).
		Joins("LEFT JOIN documents d ON d.group_id = dg.id").
		Group("dg.id").
		Order("dg.sort_order ASC, dg.id ASC").
		Scan(&results).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения групп документов")
	}
	return results, nil
}

func (s *documentGroupService) Create(ctx context.Context, userID int, req models.CreateDocumentGroupRequest) (*models.DocumentGroup, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Название группы не может быть пустым")
	}

	// Проверяем уникальность без учёта регистра
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.DocumentGroup{}).
		Where("LOWER(name) = LOWER(?)", name).
		Count(&count).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки уникальности")
	}
	if count > 0 {
		return nil, echo.NewHTTPError(http.StatusConflict, fmt.Sprintf("Группа с названием %q уже существует", name))
	}

	// Максимальный sort_order для новой группы в конце
	var maxOrder int
	s.db.WithContext(ctx).Model(&models.DocumentGroup{}).Select("COALESCE(MAX(sort_order), 0)").Scan(&maxOrder)

	now := time.Now().UTC()
	group := models.DocumentGroup{
		Name:      name,
		SortOrder: maxOrder + 1,
		CreatedBy: &userID,
		UpdatedBy: &userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&group).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка создания группы документов")
	}
	return &group, nil
}

func (s *documentGroupService) Update(ctx context.Context, userID int, id int, req models.UpdateDocumentGroupRequest) (*models.DocumentGroup, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Название группы не может быть пустым")
	}

	var group models.DocumentGroup
	if err := s.db.WithContext(ctx).First(&group, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Группа документов не найдена")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения группы")
	}

	// Уникальность нового имени среди других групп
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.DocumentGroup{}).
		Where("LOWER(name) = LOWER(?) AND id != ?", name, id).
		Count(&count).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки уникальности")
	}
	if count > 0 {
		return nil, echo.NewHTTPError(http.StatusConflict, fmt.Sprintf("Группа с названием %q уже существует", name))
	}

	if err := s.db.WithContext(ctx).Model(&group).Updates(map[string]interface{}{
		"name":       name,
		"updated_by": userID,
		"updated_at": time.Now().UTC(),
	}).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка обновления группы документов")
	}
	return &group, nil
}

func (s *documentGroupService) Delete(ctx context.Context, id int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.DocumentGroup{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки группы")
		}
		if count == 0 {
			return echo.NewHTTPError(http.StatusNotFound, "Группа документов не найдена")
		}

		// Документы группы переходят в «Прочее» (group_id = NULL)
		if err := tx.Model(&models.Document{}).
			Where("group_id = ?", id).
			Update("group_id", nil).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка переноса документов в группу «Прочее»")
		}

		if err := tx.Delete(&models.DocumentGroup{}, id).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка удаления группы документов")
		}
		return nil
	})
}

func (s *documentGroupService) Reorder(ctx context.Context, req models.ReorderDocumentGroupsRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range req.IDs {
			if err := tx.Model(&models.DocumentGroup{}).
				Where("id = ?", id).
				Update("sort_order", i).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка обновления порядка групп")
			}
		}
		return nil
	})
}
