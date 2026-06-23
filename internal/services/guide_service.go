package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// GuideRoles -- порядок ролей в раскладке «Вкладки».
var GuideRoles = []string{"user", "guard", "admin"}

// GuideKeyForRole возвращает permission-ключ раздела для роли. Все три ключа
// (guide.user, guide.guard, guide.admin) описаны в каталоге прав
// (permission_catalog.go) -- гейтинг идёт через PermissionResolver как по любому
// ключу, ядро прав не затрагивается; код роль-агностичен.
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

// ErrGuideSectionNotFound -- раздел с такой ролью не заведён (хотя роль валидна).
var ErrGuideSectionNotFound = errors.New("guide section not found")

// GuideFileMeta -- метаданные PDF, сохранённого на диск (заполняет хендлер после Save).
type GuideFileMeta struct {
	StoredName string
	FileName   string
	Ext        string
	MimeType   string
	Size       int64
}

// GuideService отдаёт разделы руководства с учётом прав пользователя и админ-управление
// (правка текста + PDF-файл по разделу; авторизация page.admin -- на роут-middleware).
type GuideService interface {
	// ListForUser возвращает разделы, на которые у пользователя есть право guide.<role>.
	ListForUser(ctx context.Context, userID int) ([]models.GuideSectionResponse, error)
	// GetSectionForUser возвращает раздел роли. allowed=false -- нет права на раздел;
	// section=nil при allowed=true -- раздел ещё не заведён.
	GetSectionForUser(ctx context.Context, userID int, role string) (section *models.GuideSection, allowed bool, err error)

	// ListAll возвращает все разделы (для админ-управления), без фильтра по правам.
	ListAll(ctx context.Context) ([]models.GuideSectionResponse, error)
	// UpdateContent правит lead и items раздела. items сохраняются как jsonb.
	UpdateContent(ctx context.Context, role, lead string, items []string) (*models.GuideSectionResponse, error)
	// SetFile записывает метаданные нового PDF раздела. oldStored -- прежний storedName
	// (для удаления файла с диска вызывающим хендлером после успешной записи).
	SetFile(ctx context.Context, role string, meta GuideFileMeta) (resp *models.GuideSectionResponse, oldStored string, err error)
	// ClearFile очищает метаданные PDF раздела. oldStored -- удалённый storedName.
	ClearFile(ctx context.Context, role string) (resp *models.GuideSectionResponse, oldStored string, err error)
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

func (s *guideService) ListAll(ctx context.Context) ([]models.GuideSectionResponse, error) {
	var sections []models.GuideSection
	if err := s.db.WithContext(ctx).Order("sort_order, id").Find(&sections).Error; err != nil {
		return nil, fmt.Errorf("failed to load guide sections: %w", err)
	}
	result := make([]models.GuideSectionResponse, 0, len(sections))
	for i := range sections {
		resp, err := toGuideResponse(sections[i])
		if err != nil {
			return nil, err
		}
		result = append(result, resp)
	}
	return result, nil
}

func (s *guideService) UpdateContent(ctx context.Context, role, lead string, items []string) (*models.GuideSectionResponse, error) {
	sec, err := s.findSection(ctx, role)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal guide items: %w", err)
	}
	if err := s.db.WithContext(ctx).Model(sec).Updates(map[string]any{"lead": lead, "items": raw}).Error; err != nil {
		return nil, fmt.Errorf("failed to update guide section: %w", err)
	}
	sec.Lead = lead
	sec.Items = raw
	return responsePtr(*sec)
}

func (s *guideService) SetFile(ctx context.Context, role string, meta GuideFileMeta) (*models.GuideSectionResponse, string, error) {
	sec, err := s.findSection(ctx, role)
	if err != nil {
		return nil, "", err
	}
	oldStored := sec.StoredName

	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(sec).Updates(map[string]any{
		"file_name":       meta.FileName,
		"stored_name":     meta.StoredName,
		"file_ext":        meta.Ext,
		"mime_type":       meta.MimeType,
		"file_size":       meta.Size,
		"file_updated_at": now,
	}).Error; err != nil {
		return nil, "", fmt.Errorf("failed to save guide file meta: %w", err)
	}
	sec.FileName, sec.StoredName, sec.FileExt = meta.FileName, meta.StoredName, meta.Ext
	sec.MimeType, sec.FileSize, sec.FileUpdatedAt = meta.MimeType, meta.Size, &now
	resp, err := responsePtr(*sec)
	if err != nil {
		return nil, "", err
	}
	return resp, oldStored, nil
}

func (s *guideService) ClearFile(ctx context.Context, role string) (*models.GuideSectionResponse, string, error) {
	sec, err := s.findSection(ctx, role)
	if err != nil {
		return nil, "", err
	}
	oldStored := sec.StoredName
	if err := s.db.WithContext(ctx).Model(sec).Updates(map[string]any{
		"file_name":       "",
		"stored_name":     "",
		"file_ext":        "",
		"mime_type":       "",
		"file_size":       0,
		"file_updated_at": nil,
	}).Error; err != nil {
		return nil, "", fmt.Errorf("failed to clear guide file meta: %w", err)
	}
	sec.FileName, sec.StoredName, sec.FileExt = "", "", ""
	sec.MimeType, sec.FileSize, sec.FileUpdatedAt = "", 0, nil
	resp, err := responsePtr(*sec)
	if err != nil {
		return nil, "", err
	}
	return resp, oldStored, nil
}

// findSection грузит раздел по роли; ErrGuideSectionNotFound если роль валидна, но раздела нет.
func (s *guideService) findSection(ctx context.Context, role string) (*models.GuideSection, error) {
	if !IsGuideRole(role) {
		return nil, ErrGuideSectionNotFound
	}
	var sec models.GuideSection
	if err := s.db.WithContext(ctx).Where("role = ?", role).First(&sec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGuideSectionNotFound
		}
		return nil, fmt.Errorf("failed to load guide section: %w", err)
	}
	return &sec, nil
}

// responsePtr собирает ответ из раздела (уже с применёнными правками в памяти).
func responsePtr(sec models.GuideSection) (*models.GuideSectionResponse, error) {
	resp, err := toGuideResponse(sec)
	if err != nil {
		return nil, err
	}
	return &resp, nil
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
