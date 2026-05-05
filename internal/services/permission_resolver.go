package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// PermissionResolver вычисляет финальный набор прав пользователя на основе
// его роли (default-группы), явных групп и точечных override.
//
// Алгоритм (см. #187):
//  1. Если IsSuperAdmin = true -> возвращает специальный allowAll set (HasPermission всегда true).
//  2. Иначе union прав со всех групп: role.default_groups + user_groups.
//  3. Поверх UserPermissionOverride: deny здесь побеждает любое allow из групп.
//
// Кэш: in-memory sync.Map с TTL 30s, инвалидируется по Invalidate(userID).
type PermissionResolver struct {
	db    *gorm.DB
	cache sync.Map // userID -> *cacheEntry
	ttl   time.Duration
}

type cacheEntry struct {
	set       PermissionSet
	expiresAt time.Time
}

// PermissionSet -- финальный набор прав юзера.
// allowAll = true означает super-admin (HasPermission всегда true).
type PermissionSet struct {
	allowAll bool
	allows   map[string]struct{}
}

// NewPermissionResolver конструирует resolver с кешом по умолчанию (30s TTL).
func NewPermissionResolver(db *gorm.DB) *PermissionResolver {
	return &PermissionResolver{db: db, ttl: 30 * time.Second}
}

// Has проверяет наличие конкретного permission_key у юзера.
func (s *PermissionSet) Has(key string) bool {
	if s.allowAll {
		return true
	}
	_, ok := s.allows[key]
	return ok
}

// Keys возвращает отсортированный список разрешённых ключей.
// Для super-admin возвращает nil (всё разрешено, ключи перечислять нет смысла).
func (s *PermissionSet) Keys() []string {
	if s.allowAll {
		return nil
	}
	keys := make([]string, 0, len(s.allows))
	for k := range s.allows {
		keys = append(keys, k)
	}
	return keys
}

// IsSuperAdmin сообщает, что у юзера полный доступ.
func (s *PermissionSet) IsSuperAdmin() bool {
	return s.allowAll
}

// Resolve возвращает PermissionSet для юзера, используя кэш.
func (r *PermissionResolver) Resolve(ctx context.Context, userID int) (PermissionSet, error) {
	if v, ok := r.cache.Load(userID); ok {
		entry := v.(*cacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.set, nil
		}
	}

	set, err := r.computeSet(ctx, userID)
	if err != nil {
		return PermissionSet{}, fmt.Errorf("failed to compute permission set for user %d: %w", userID, err)
	}

	r.cache.Store(userID, &cacheEntry{
		set:       set,
		expiresAt: time.Now().Add(r.ttl),
	})
	return set, nil
}

// HasPermission -- удобный shortcut поверх Resolve.
func (r *PermissionResolver) HasPermission(ctx context.Context, userID int, key string) (bool, error) {
	set, err := r.Resolve(ctx, userID)
	if err != nil {
		return false, err
	}
	return set.Has(key), nil
}

// Invalidate сбрасывает кэш для конкретного юзера. Вызывается при изменении
// роли, групп или override.
func (r *PermissionResolver) Invalidate(userID int) {
	r.cache.Delete(userID)
}

// InvalidateAll сбрасывает кэш для всех юзеров. Вызывается при изменении
// permission_group_grants любой группы (broad invalidation -- проще, чем
// отслеживать какие юзеры в каких группах).
func (r *PermissionResolver) InvalidateAll() {
	r.cache.Range(func(k, _ any) bool {
		r.cache.Delete(k)
		return true
	})
}

// computeSet -- основная логика без кэша. Вынесена для тестируемости.
func (r *PermissionResolver) computeSet(ctx context.Context, userID int) (PermissionSet, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Select("id, role_id, is_super_admin").First(&user, userID).Error; err != nil {
		return PermissionSet{}, fmt.Errorf("user not found: %w", err)
	}

	if user.IsSuperAdmin {
		return PermissionSet{allowAll: true}, nil
	}

	allows := make(map[string]struct{})
	denies := make(map[string]struct{})

	// 1. Default-группы роли.
	if user.RoleID != nil {
		if err := r.collectGroupGrants(ctx, []int{*user.RoleID}, true, allows, denies); err != nil {
			return PermissionSet{}, err
		}
	}

	// 2. Явные группы юзера.
	var userGroupIDs []int
	if err := r.db.WithContext(ctx).
		Model(&models.UserGroup{}).
		Where("user_id = ?", userID).
		Pluck("group_id", &userGroupIDs).Error; err != nil {
		return PermissionSet{}, fmt.Errorf("failed to load user groups: %w", err)
	}
	if err := r.collectGroupGrants(ctx, userGroupIDs, false, allows, denies); err != nil {
		return PermissionSet{}, err
	}

	// 3. User-level overrides (deny приоритетнее всего).
	var overrides []models.UserPermissionOverride
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&overrides).Error; err != nil {
		return PermissionSet{}, fmt.Errorf("failed to load overrides: %w", err)
	}
	for _, o := range overrides {
		if o.Value == "deny" {
			denies[o.PermissionKey] = struct{}{}
			delete(allows, o.PermissionKey)
		} else if o.Value == "allow" {
			delete(denies, o.PermissionKey)
			allows[o.PermissionKey] = struct{}{}
		}
	}

	// Вычитаем deny из allows (на всякий случай, после групповых deny).
	for k := range denies {
		delete(allows, k)
	}

	return PermissionSet{allows: allows}, nil
}

// collectGroupGrants добавляет права из указанных групп.
// Если byRole = true, вместо groupIDs используются default-группы роли (groupIDs[0] = roleID).
func (r *PermissionResolver) collectGroupGrants(ctx context.Context, groupIDs []int, byRole bool, allows, denies map[string]struct{}) error {
	if len(groupIDs) == 0 {
		return nil
	}

	resolvedGroupIDs := groupIDs
	if byRole {
		var ids []int
		if err := r.db.WithContext(ctx).
			Model(&models.RoleDefaultGroup{}).
			Where("role_id IN ?", groupIDs).
			Pluck("group_id", &ids).Error; err != nil {
			return fmt.Errorf("failed to load role default groups: %w", err)
		}
		resolvedGroupIDs = ids
	}
	if len(resolvedGroupIDs) == 0 {
		return nil
	}

	var grants []models.PermissionGroupGrant
	if err := r.db.WithContext(ctx).
		Where("group_id IN ?", resolvedGroupIDs).
		Find(&grants).Error; err != nil {
		return fmt.Errorf("failed to load group grants: %w", err)
	}

	for _, g := range grants {
		switch g.Value {
		case "allow":
			if _, denied := denies[g.PermissionKey]; !denied {
				allows[g.PermissionKey] = struct{}{}
			}
		case "deny":
			denies[g.PermissionKey] = struct{}{}
			delete(allows, g.PermissionKey)
		}
	}
	return nil
}
