package services

import (
	"context"
	"fmt"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// PermissionGroupService управляет группами прав: CRUD + назначение юзерам + слияние.
type PermissionGroupService struct {
	db       *gorm.DB
	resolver *PermissionResolver
}

// NewPermissionGroupService конструирует сервис.
func NewPermissionGroupService(db *gorm.DB, resolver *PermissionResolver) *PermissionGroupService {
	return &PermissionGroupService{db: db, resolver: resolver}
}

// List возвращает все группы прав с их ключами.
func (s *PermissionGroupService) List(ctx context.Context) ([]models.PermissionGroupResponse, error) {
	var groups []models.PermissionGroup
	if err := s.db.WithContext(ctx).Order("id DESC").Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}
	if len(groups) == 0 {
		return []models.PermissionGroupResponse{}, nil
	}

	groupIDs := make([]int, len(groups))
	for i, g := range groups {
		groupIDs[i] = g.ID
	}
	var grants []models.PermissionGroupGrant
	if err := s.db.WithContext(ctx).Where("group_id IN ?", groupIDs).Find(&grants).Error; err != nil {
		return nil, fmt.Errorf("failed to load grants: %w", err)
	}

	keysByGroup := make(map[int][]string)
	for _, g := range grants {
		if g.Value == "allow" {
			keysByGroup[g.GroupID] = append(keysByGroup[g.GroupID], g.PermissionKey)
		}
	}

	result := make([]models.PermissionGroupResponse, len(groups))
	for i, g := range groups {
		keys := keysByGroup[g.ID]
		if keys == nil {
			keys = []string{}
		}
		result[i] = models.PermissionGroupResponse{
			ID:          g.ID,
			Name:        g.Name,
			Description: g.Description,
			Keys:        keys,
		}
	}
	return result, nil
}

// Get возвращает одну группу по ID.
func (s *PermissionGroupService) Get(ctx context.Context, groupID int) (models.PermissionGroupResponse, error) {
	var group models.PermissionGroup
	if err := s.db.WithContext(ctx).First(&group, groupID).Error; err != nil {
		return models.PermissionGroupResponse{}, fmt.Errorf("group not found: %w", err)
	}
	var grants []models.PermissionGroupGrant
	if err := s.db.WithContext(ctx).Where("group_id = ?", groupID).Find(&grants).Error; err != nil {
		return models.PermissionGroupResponse{}, fmt.Errorf("failed to load grants: %w", err)
	}
	keys := make([]string, 0, len(grants))
	for _, g := range grants {
		if g.Value == "allow" {
			keys = append(keys, g.PermissionKey)
		}
	}
	return models.PermissionGroupResponse{
		ID:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		Keys:        keys,
	}, nil
}

// Create создаёт группу с указанными ключами.
func (s *PermissionGroupService) Create(ctx context.Context, req models.CreatePermissionGroupRequest) (models.PermissionGroupResponse, error) {
	var resp models.PermissionGroupResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		group := models.PermissionGroup{Name: req.Name, Description: req.Description}
		if err := tx.Create(&group).Error; err != nil {
			return fmt.Errorf("failed to create group: %w", err)
		}
		grants := buildGrants(group.ID, req.Keys)
		if len(grants) > 0 {
			if err := tx.Create(&grants).Error; err != nil {
				return fmt.Errorf("failed to create grants: %w", err)
			}
		}
		resp = models.PermissionGroupResponse{
			ID:          group.ID,
			Name:        group.Name,
			Description: group.Description,
			Keys:        req.Keys,
		}
		return nil
	})
	if err == nil {
		s.resolver.InvalidateAll()
	}
	return resp, err
}

// Update обновляет имя/описание/ключи группы (полная замена ключей).
func (s *PermissionGroupService) Update(ctx context.Context, groupID int, req models.UpdatePermissionGroupRequest) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.PermissionGroup{}).Where("id = ?", groupID).
			Updates(map[string]any{"name": req.Name, "description": req.Description}).Error; err != nil {
			return fmt.Errorf("failed to update group: %w", err)
		}
		if err := tx.Where("group_id = ?", groupID).Delete(&models.PermissionGroupGrant{}).Error; err != nil {
			return fmt.Errorf("failed to clear grants: %w", err)
		}
		grants := buildGrants(groupID, req.Keys)
		if len(grants) > 0 {
			if err := tx.Create(&grants).Error; err != nil {
				return fmt.Errorf("failed to insert grants: %w", err)
			}
		}
		return nil
	})
	if err == nil {
		s.resolver.InvalidateAll()
	}
	return err
}

// Delete удаляет группу. Все связи в user_groups и role_default_groups удаляются каскадом.
func (s *PermissionGroupService) Delete(ctx context.Context, groupID int) error {
	if err := s.db.WithContext(ctx).Delete(&models.PermissionGroup{}, groupID).Error; err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}
	s.resolver.InvalidateAll()
	return nil
}

// Merge сливает несколько групп юзера в новую simple-группу.
// Старые группы остаются в системе, у юзера в user_groups заменяются на новую.
func (s *PermissionGroupService) Merge(ctx context.Context, req models.MergePermissionGroupsRequest, grantedBy int) (models.PermissionGroupResponse, error) {
	var resp models.PermissionGroupResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existingGrants []models.PermissionGroupGrant
		if err := tx.Where("group_id IN ? AND value = ?", req.SourceGroupIDs, "allow").
			Find(&existingGrants).Error; err != nil {
			return fmt.Errorf("failed to load source grants: %w", err)
		}
		uniqKeys := uniqueKeys(existingGrants)

		newGroup := models.PermissionGroup{Name: req.NewGroupName}
		if err := tx.Create(&newGroup).Error; err != nil {
			return fmt.Errorf("failed to create merged group: %w", err)
		}
		grants := buildGrants(newGroup.ID, uniqKeys)
		if len(grants) > 0 {
			if err := tx.Create(&grants).Error; err != nil {
				return fmt.Errorf("failed to create merged grants: %w", err)
			}
		}

		// Заменяем у юзера: убираем source-группы, добавляем новую.
		if err := tx.Where("user_id = ? AND group_id IN ?", req.UserID, req.SourceGroupIDs).
			Delete(&models.UserGroup{}).Error; err != nil {
			return fmt.Errorf("failed to remove source user_groups: %w", err)
		}
		ug := models.UserGroup{UserID: req.UserID, GroupID: newGroup.ID, GrantedBy: &grantedBy}
		if err := tx.Create(&ug).Error; err != nil {
			return fmt.Errorf("failed to assign merged group: %w", err)
		}

		resp = models.PermissionGroupResponse{
			ID:   newGroup.ID,
			Name: newGroup.Name,
			Keys: uniqKeys,
		}
		return nil
	})
	if err == nil {
		s.resolver.Invalidate(req.UserID)
	}
	return resp, err
}

// AssignToUser добавляет группу юзеру (или ничего не делает, если уже назначена).
func (s *PermissionGroupService) AssignToUser(ctx context.Context, userID, groupID int, grantedBy int) error {
	ug := models.UserGroup{UserID: userID, GroupID: groupID, GrantedBy: &grantedBy}
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND group_id = ?", userID, groupID).
		FirstOrCreate(&ug).Error; err != nil {
		return fmt.Errorf("failed to assign group: %w", err)
	}
	s.resolver.Invalidate(userID)
	return nil
}

// ListForUser возвращает группы прав, явно назначенные пользователю.
// Не включает дефолтные группы роли (они вычисляются resolver-ом).
func (s *PermissionGroupService) ListForUser(ctx context.Context, userID int) ([]models.PermissionGroupResponse, error) {
	var groupIDs []int
	if err := s.db.WithContext(ctx).
		Model(&models.UserGroup{}).
		Where("user_id = ?", userID).
		Pluck("group_id", &groupIDs).Error; err != nil {
		return nil, fmt.Errorf("failed to load user groups: %w", err)
	}
	if len(groupIDs) == 0 {
		return []models.PermissionGroupResponse{}, nil
	}
	var groups []models.PermissionGroup
	if err := s.db.WithContext(ctx).Where("id IN ?", groupIDs).Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to load groups: %w", err)
	}
	var grants []models.PermissionGroupGrant
	if err := s.db.WithContext(ctx).
		Where("group_id IN ? AND value = ?", groupIDs, "allow").
		Find(&grants).Error; err != nil {
		return nil, fmt.Errorf("failed to load grants: %w", err)
	}
	keysByGroup := make(map[int][]string)
	for _, g := range grants {
		keysByGroup[g.GroupID] = append(keysByGroup[g.GroupID], g.PermissionKey)
	}
	result := make([]models.PermissionGroupResponse, len(groups))
	for i, g := range groups {
		keys := keysByGroup[g.ID]
		if keys == nil {
			keys = []string{}
		}
		result[i] = models.PermissionGroupResponse{
			ID:          g.ID,
			Name:        g.Name,
			Description: g.Description,
			Keys:        keys,
		}
	}
	return result, nil
}

// SetUserRole присваивает роль пользователю (nil очищает).
func (s *PermissionGroupService) SetUserRole(ctx context.Context, userID int, roleID *int) error {
	updates := map[string]any{"role_id": roleID}
	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to set role: %w", err)
	}
	s.resolver.Invalidate(userID)
	return nil
}

// SetUserAdmin переключает флаг администратора у пользователя (выдача/снятие админки).
func (s *PermissionGroupService) SetUserAdmin(ctx context.Context, userID int, isAdmin bool) error {
	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).
		Update("is_admin", isAdmin).Error; err != nil {
		return fmt.Errorf("failed to set admin flag: %w", err)
	}
	s.resolver.Invalidate(userID)
	return nil
}

// UnassignFromUser убирает группу у юзера.
func (s *PermissionGroupService) UnassignFromUser(ctx context.Context, userID, groupID int) error {
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND group_id = ?", userID, groupID).
		Delete(&models.UserGroup{}).Error; err != nil {
		return fmt.Errorf("failed to unassign group: %w", err)
	}
	s.resolver.Invalidate(userID)
	return nil
}

// buildGrants строит slice grants из набора ключей (все со значением allow).
func buildGrants(groupID int, keys []string) []models.PermissionGroupGrant {
	seen := make(map[string]struct{}, len(keys))
	grants := make([]models.PermissionGroupGrant, 0, len(keys))
	for _, k := range keys {
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		grants = append(grants, models.PermissionGroupGrant{
			GroupID: groupID, PermissionKey: k, Value: "allow",
		})
	}
	return grants
}

func uniqueKeys(grants []models.PermissionGroupGrant) []string {
	seen := make(map[string]struct{}, len(grants))
	keys := make([]string, 0, len(grants))
	for _, g := range grants {
		if _, ok := seen[g.PermissionKey]; ok {
			continue
		}
		seen[g.PermissionKey] = struct{}{}
		keys = append(keys, g.PermissionKey)
	}
	return keys
}
