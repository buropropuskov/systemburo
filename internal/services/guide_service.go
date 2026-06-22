package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// GuideRoles -- порядок ролей в раскладке «Вкладки».
var GuideRoles = []string{"user", "guard", "admin"}

// GuideKeyForRole возвращает permission-ключ раздела для роли. guide.user и
// guide.admin уже описаны в каталоге прав (permission_catalog.go: KeyGuideUser,
// KeyGuideAdmin) -- гейтинг идёт через PermissionResolver как по любому ключу,
// ядро прав не затрагивается. guide.guard пока не заведён в каталоге (добавляет
// perm-gating отдельным срезом), поэтому раздел «Охранник» до этого виден только
// супер-админу; код роль-агностичен и подхватит ключ автоматически.
func GuideKeyForRole(role string) string {
	return "guide." + role
}

// IsGuideRole сообщает, известна ли роль руководства.
func IsGuideRole(role string) bool {
	for _, r := range GuideRoles {
		if r == role {
			return true
		}
	}
	return false
}

// GuideService отдаёт разделы руководства с учётом прав пользователя.
type GuideService interface {
	// ListForUser возвращает разделы, на которые у пользователя есть право guide.<role>.
	ListForUser(ctx context.Context, userID int) ([]models.GuideSectionResponse, error)
	// GetSectionForUser возвращает раздел роли. allowed=false -- нет права на раздел;
	// section=nil при allowed=true -- раздел ещё не заведён.
	GetSectionForUser(ctx context.Context, userID int, role string) (section *models.GuideSection, allowed bool, err error)
}

type guideService struct {
	db       *gorm.DB
	resolver *PermissionResolver
}

// NewGuideService конструирует GuideService поверх PermissionResolver.
func NewGuideService(db *gorm.DB, resolver *PermissionResolver) GuideService {
	return &guideService{db: db, resolver: resolver}
}

func (s *guideService) ListForUser(ctx context.Context, userID int) ([]models.GuideSectionResponse, error) {
	set, err := s.resolver.Resolve(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve permissions: %w", err)
	}

	var sections []models.GuideSection
	if err := s.db.WithContext(ctx).Order("sort_order, id").Find(&sections).Error; err != nil {
		return nil, fmt.Errorf("failed to load guide sections: %w", err)
	}

	result := make([]models.GuideSectionResponse, 0, len(sections))
	for i := range sections {
		sec := sections[i]
		if !set.Has(GuideKeyForRole(sec.Role)) {
			continue
		}
		resp, err := toGuideResponse(sec)
		if err != nil {
			return nil, err
		}
		result = append(result, resp)
	}
	return result, nil
}

func (s *guideService) GetSectionForUser(ctx context.Context, userID int, role string) (*models.GuideSection, bool, error) {
	allowed, err := s.resolver.HasPermission(ctx, userID, GuideKeyForRole(role))
	if err != nil {
		return nil, false, fmt.Errorf("failed to check guide permission: %w", err)
	}
	if !allowed {
		return nil, false, nil
	}

	var sec models.GuideSection
	if err := s.db.WithContext(ctx).Where("role = ?", role).First(&sec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, true, nil
		}
		return nil, true, fmt.Errorf("failed to load guide section: %w", err)
	}
	return &sec, true, nil
}

// toGuideResponse разворачивает jsonb items в []string и собирает file-блок
// (nil пока PDF не загружен).
func toGuideResponse(sec models.GuideSection) (models.GuideSectionResponse, error) {
	items := []string{}
	if len(sec.Items) > 0 {
		if err := json.Unmarshal(sec.Items, &items); err != nil {
			return models.GuideSectionResponse{}, fmt.Errorf("failed to parse items for guide section %q: %w", sec.Role, err)
		}
	}

	resp := models.GuideSectionResponse{
		Role:  sec.Role,
		Title: sec.Title,
		Lead:  sec.Lead,
		Items: items,
	}

	if sec.StoredName != "" {
		updated := sec.UpdatedAt
		if sec.FileUpdatedAt != nil {
			updated = *sec.FileUpdatedAt
		}
		resp.File = &models.GuideFileInfo{
			Name:        sec.FileName,
			Ext:         sec.FileExt,
			MimeType:    sec.MimeType,
			Size:        sec.FileSize,
			UpdatedAt:   updated,
			DownloadURL: "/api/guide/sections/" + sec.Role + "/download",
		}
	}
	return resp, nil
}
