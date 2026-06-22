package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RoleService управляет ролями и их default-группами.
type RoleService struct {
	db       *gorm.DB
	resolver *PermissionResolver
}

// NewRoleService конструирует сервис.
func NewRoleService(db *gorm.DB, resolver *PermissionResolver) *RoleService {
	return &RoleService{db: db, resolver: resolver}
}

// List возвращает все роли с дефолтными группами.
func (s *RoleService) List(ctx context.Context) ([]models.RoleResponse, error) {
	var roles []models.Role
	if err := s.db.WithContext(ctx).Order("id").Find(&roles).Error; err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}
	if len(roles) == 0 {
		return []models.RoleResponse{}, nil
	}

	roleIDs := make([]int, len(roles))
	for i, r := range roles {
		roleIDs[i] = r.ID
	}

	var defaults []models.RoleDefaultGroup
	if err := s.db.WithContext(ctx).Where("role_id IN ?", roleIDs).Find(&defaults).Error; err != nil {
		return nil, fmt.Errorf("failed to load role defaults: %w", err)
	}

	groupIDsByRole := make(map[int][]int)
	allGroupIDs := make(map[int]struct{})
	for _, d := range defaults {
		groupIDsByRole[d.RoleID] = append(groupIDsByRole[d.RoleID], d.GroupID)
		allGroupIDs[d.GroupID] = struct{}{}
	}

	groupResponses, err := s.fetchGroupResponses(ctx, allGroupIDs)
	if err != nil {
		return nil, err
	}

	var grants []models.RolePermissionGrant
	if err := s.db.WithContext(ctx).Where("role_id IN ? AND value = ?", roleIDs, "allow").
		Find(&grants).Error; err != nil {
		return nil, fmt.Errorf("failed to load role grants: %w", err)
	}
	grantsByRole := make(map[int][]string)
	for _, g := range grants {
		grantsByRole[g.RoleID] = append(grantsByRole[g.RoleID], g.PermissionKey)
	}

	result := make([]models.RoleResponse, len(roles))
	for i, r := range roles {
		groups := make([]models.PermissionGroupResponse, 0)
		for _, gid := range groupIDsByRole[r.ID] {
			if resp, ok := groupResponses[gid]; ok {
				groups = append(groups, resp)
			}
		}
		direct := grantsByRole[r.ID]
		if direct == nil {
			direct = []string{}
		}
		result[i] = models.RoleResponse{
			ID:            r.ID,
			Code:          r.Code,
			Name:          r.Name,
			Description:   r.Description,
			DefaultGroups: groups,
			DirectGrants:  direct,
		}
	}
	return result, nil
}

// Create создаёт новую роль со снимком grants базовой роли "Пользователь"
// на момент создания (фундамент-наследование из ТЗ). Дальше роль живёт независимо.
func (s *RoleService) Create(ctx context.Context, req models.CreateRoleRequest) (models.Role, error) {
	role := models.Role{Code: req.Code, Name: req.Name, Description: req.Description}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&role).Error; err != nil {
			return fmt.Errorf("failed to create role: %w", err)
		}
		var base models.Role
		if err := tx.Where("code = ? AND is_system = ?", "user", true).First(&base).Error; err != nil {
			// Базовой роли нет (например, до seed) -- создаём роль без снимка.
			return nil
		}
		if base.ID == role.ID {
			return nil
		}
		var baseGrants []models.RolePermissionGrant
		if err := tx.Where("role_id = ?", base.ID).Find(&baseGrants).Error; err != nil {
			return fmt.Errorf("failed to load base role grants: %w", err)
		}
		if len(baseGrants) == 0 {
			return nil
		}
		copied := make([]models.RolePermissionGrant, 0, len(baseGrants))
		for _, g := range baseGrants {
			copied = append(copied, models.RolePermissionGrant{
				RoleID:        role.ID,
				PermissionKey: g.PermissionKey,
				Value:         g.Value,
			})
		}
		if err := tx.Create(&copied).Error; err != nil {
			return fmt.Errorf("failed to copy base grants: %w", err)
		}
		return nil
	})
	if err != nil {
		return models.Role{}, err
	}
	return role, nil
}

// Update обновляет имя/описание роли. Code менять нельзя (это API-контракт).
func (s *RoleService) Update(ctx context.Context, roleID int, req models.UpdateRoleRequest) error {
	if err := s.db.WithContext(ctx).Model(&models.Role{}).Where("id = ?", roleID).
		Updates(map[string]any{"name": req.Name, "description": req.Description}).Error; err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}
	s.resolver.InvalidateAll()
	return nil
}

// Delete удаляет роль. Запрещено для системной роли и если есть юзеры с этой ролью.
func (s *RoleService) Delete(ctx context.Context, roleID int) error {
	var role models.Role
	if err := s.db.WithContext(ctx).First(&role, roleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Роль не найдена")
		}
		return fmt.Errorf("failed to load role: %w", err)
	}
	if role.IsSystem {
		return echo.NewHTTPError(http.StatusConflict, "Нельзя удалить системную роль")
	}

	// Считаем всех пользователей с ролью, включая архивных: у архивных role_id
	// сохраняется, и удаление роли осиротит ссылку. Сначала переназначить.
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("role_id = ?", roleID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}
	if count > 0 {
		return echo.NewHTTPError(http.StatusConflict,
			fmt.Sprintf("Роль назначена пользователям (%d, включая архивных). Сначала переназначьте их на другую роль.", count))
	}
	if err := s.db.WithContext(ctx).Delete(&models.Role{}, roleID).Error; err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}
	return nil
}

// SetDefaultGroups полностью заменяет набор default-групп для роли.
func (s *RoleService) SetDefaultGroups(ctx context.Context, roleID int, groupIDs []int) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&models.RoleDefaultGroup{}).Error; err != nil {
			return fmt.Errorf("failed to clear role defaults: %w", err)
		}
		if len(groupIDs) == 0 {
			return nil
		}
		records := make([]models.RoleDefaultGroup, 0, len(groupIDs))
		seen := make(map[int]struct{})
		for _, gid := range groupIDs {
			if _, dup := seen[gid]; dup {
				continue
			}
			seen[gid] = struct{}{}
			records = append(records, models.RoleDefaultGroup{RoleID: roleID, GroupID: gid})
		}
		if err := tx.Create(&records).Error; err != nil {
			return fmt.Errorf("failed to insert role defaults: %w", err)
		}
		return nil
	})
	if err == nil {
		s.resolver.InvalidateAll()
	}
	return err
}

// SetPermissions полностью заменяет набор прямых (allow) грантов роли по ключам.
// Ключи валидируются по каталогу; super-only выдавать через роль нельзя (резолвер
// их всё равно игнорирует у не-супера -- отклоняем явно, чтобы не плодить мёртвые гранты).
func (s *RoleService) SetPermissions(ctx context.Context, roleID int, keys []string) error {
	var exists int64
	if err := s.db.WithContext(ctx).Model(&models.Role{}).Where("id = ?", roleID).Count(&exists).Error; err != nil {
		return fmt.Errorf("failed to check role: %w", err)
	}
	if exists == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Роль не найдена")
	}

	seen := make(map[string]struct{}, len(keys))
	unique := make([]string, 0, len(keys))
	for _, k := range keys {
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		if !IsValidKey(k) {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Неизвестный ключ права: %s", k))
		}
		if IsSuperOnly(k) {
			return echo.NewHTTPError(http.StatusForbidden, "Эти права нельзя выдать через роль")
		}
		unique = append(unique, k)
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&models.RolePermissionGrant{}).Error; err != nil {
			return fmt.Errorf("failed to clear role grants: %w", err)
		}
		if len(unique) == 0 {
			return nil
		}
		records := make([]models.RolePermissionGrant, 0, len(unique))
		for _, k := range unique {
			records = append(records, models.RolePermissionGrant{RoleID: roleID, PermissionKey: k, Value: "allow"})
		}
		if err := tx.Create(&records).Error; err != nil {
			return fmt.Errorf("failed to insert role grants: %w", err)
		}
		return nil
	})
	if err == nil {
		s.resolver.InvalidateAll()
	}
	return err
}

func (s *RoleService) fetchGroupResponses(ctx context.Context, ids map[int]struct{}) (map[int]models.PermissionGroupResponse, error) {
	if len(ids) == 0 {
		return map[int]models.PermissionGroupResponse{}, nil
	}
	idList := make([]int, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	var groups []models.PermissionGroup
	if err := s.db.WithContext(ctx).Where("id IN ?", idList).Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to load groups: %w", err)
	}
	var grants []models.PermissionGroupGrant
	if err := s.db.WithContext(ctx).Where("group_id IN ? AND value = ?", idList, "allow").
		Find(&grants).Error; err != nil {
		return nil, fmt.Errorf("failed to load grants: %w", err)
	}
	keysByGroup := make(map[int][]string)
	for _, g := range grants {
		keysByGroup[g.GroupID] = append(keysByGroup[g.GroupID], g.PermissionKey)
	}
	result := make(map[int]models.PermissionGroupResponse, len(groups))
	for _, g := range groups {
		keys := keysByGroup[g.ID]
		if keys == nil {
			keys = []string{}
		}
		result[g.ID] = models.PermissionGroupResponse{
			ID:          g.ID,
			Name:        g.Name,
			Description: g.Description,
			Keys:        keys,
		}
	}
	return result, nil
}
