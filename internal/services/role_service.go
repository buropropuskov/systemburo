package services

import (
	"context"
	"fmt"

	"systemburo/internal/models"

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

	result := make([]models.RoleResponse, len(roles))
	for i, r := range roles {
		groups := make([]models.PermissionGroupResponse, 0)
		for _, gid := range groupIDsByRole[r.ID] {
			if resp, ok := groupResponses[gid]; ok {
				groups = append(groups, resp)
			}
		}
		result[i] = models.RoleResponse{
			ID:            r.ID,
			Code:          r.Code,
			Name:          r.Name,
			Description:   r.Description,
			DefaultGroups: groups,
		}
	}
	return result, nil
}

// Create создаёт новую роль.
func (s *RoleService) Create(ctx context.Context, req models.CreateRoleRequest) (models.Role, error) {
	role := models.Role{Code: req.Code, Name: req.Name, Description: req.Description}
	if err := s.db.WithContext(ctx).Create(&role).Error; err != nil {
		return models.Role{}, fmt.Errorf("failed to create role: %w", err)
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

// Delete удаляет роль. Запрещено если есть юзеры с этой ролью.
func (s *RoleService) Delete(ctx context.Context, roleID int) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("role_id = ?", roleID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete role: %d users still assigned", count)
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
