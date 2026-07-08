package services

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/realtime"

	"gorm.io/gorm"
)

// permissionRealtimePublisher - узкий контракт публикации сигнала смены прав
// (#840). Реализуется *realtime.Hub. Опционален: без него инвалидация молчит.
type permissionRealtimePublisher interface {
	Publish(userID int, ev realtime.Event)
	PublishToEachConnected(mk func(userID int) realtime.Event)
}

// PermissionResolver вычисляет финальный набор прав пользователя.
//
// Уровни (приоритет сверху вниз):
//  1. banned -> прав нет (Has всегда false).
//  2. is_super_admin -> allowAll (Has всегда true, включая super-only).
//  3. is_admin -> adminAll: всё, КРОМЕ super-only ключей и личных deny-override.
//  4. обычный -> union(role_permission_grants + role.default_groups + user_groups),
//     затем UserPermissionOverride (deny побеждает allow). Super-only режутся для всех,
//     кроме super-admin.
//
// Кэш: in-memory sync.Map с TTL 30s, инвалидируется по Invalidate(userID).
type PermissionResolver struct {
	db                *gorm.DB
	cache             sync.Map // userID -> *cacheEntry
	ttl               time.Duration
	realtimePublisher permissionRealtimePublisher
}

type cacheEntry struct {
	set       PermissionSet
	expiresAt time.Time
}

// Источники права (для отображения в UI настройки).
const (
	SourceRole     = "role"
	SourceGroup    = "group"
	SourceOverride = "override"
)

// PermissionSet -- финальный набор прав юзера.
type PermissionSet struct {
	allowAll  bool                // super-admin: всё разрешено
	adminAll  bool                // admin: всё, кроме super-only и личных deny
	banned    bool                // забанен: ничего
	banReason string              // причина бана (для забаненного)
	allows    map[string]string   // key -> источник (role|group|override) -- для обычного юзера
	denies    map[string]struct{} // явные deny (личные для admin; вычтенные для обычного)
}

// NewPermissionResolver конструирует resolver с кешом по умолчанию (30s TTL).
func NewPermissionResolver(db *gorm.DB) *PermissionResolver {
	return &PermissionResolver{db: db, ttl: 30 * time.Second}
}

// Has проверяет наличие конкретного permission_key у юзера.
func (s *PermissionSet) Has(key string) bool {
	if s.banned {
		return false
	}
	if s.allowAll {
		return true
	}
	// Super-only ключи доступны только супер-админу (выше).
	if IsSuperOnly(key) {
		return false
	}
	if s.adminAll {
		_, denied := s.denies[key]
		return !denied
	}
	_, ok := s.allows[key]
	return ok
}

// IsBanned сообщает, что юзер заблокирован.
func (s *PermissionSet) IsBanned() bool { return s.banned }

// BanReason возвращает причину блокировки (пусто, если не забанен).
func (s *PermissionSet) BanReason() string { return s.banReason }

// IsSuperAdmin сообщает, что у юзера полный доступ.
func (s *PermissionSet) IsSuperAdmin() bool { return s.allowAll }

// IsAdmin сообщает, что юзер -- администратор (всё кроме super-only и личных deny).
func (s *PermissionSet) IsAdmin() bool { return s.adminAll }

// Mode возвращает режим набора: banned|super|admin|normal.
func (s *PermissionSet) Mode() string {
	switch {
	case s.banned:
		return "banned"
	case s.allowAll:
		return "super"
	case s.adminAll:
		return "admin"
	default:
		return "normal"
	}
}

// Source возвращает источник права (role|group|override) для обычного юзера.
func (s *PermissionSet) Source(key string) string { return s.allows[key] }

// Keys возвращает отсортированный список разрешённых ключей обычного юзера.
// Для super/admin возвращает nil (перечислять смысла нет -- всё разрешено, кроме denied).
func (s *PermissionSet) Keys() []string {
	if s.allowAll || s.adminAll {
		return nil
	}
	keys := make([]string, 0, len(s.allows))
	for k := range s.allows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Denies возвращает отсортированный список явных deny (личные исключения админа).
func (s *PermissionSet) Denies() []string {
	keys := make([]string, 0, len(s.denies))
	for k := range s.denies {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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

// SetRealtimePublisher включает real-time сигнал смены прав (#840): при
// инвалидации кэша прав юзера (смена роли/группы/override/бан) затронутому
// шлётся user.permissions (scope user:<id>), его App.vue сразу перезапрашивает
// права - без ожидания 30с-опроса. Опционально.
func (r *PermissionResolver) SetRealtimePublisher(p permissionRealtimePublisher) {
	r.realtimePublisher = p
}

// Invalidate сбрасывает кэш для конкретного юзера. Вызывается при изменении
// роли, групп, override или флага is_admin/бана.
func (r *PermissionResolver) Invalidate(userID int) {
	r.cache.Delete(userID)
	// Адресный сигнал перезапросить права затронутому (тот же scope user:<id>,
	// что слушает App.vue для бана). Best-effort, nil-safe. После сброса кэша,
	// чтобы перезапрос увидел свежие права. Примечание: при бане это сработает
	// вдобавок к user.banned от UserBanService (тот тоже зовёт Invalidate) -
	// забаненный получит два сигнала, оба -> fetchPermissions(true); дубль
	// безвреден (идемпотентный refetch), бан - редкая операция.
	if r.realtimePublisher != nil {
		r.realtimePublisher.Publish(userID, permissionsChangedEvent(userID))
	}
}

// InvalidateAll сбрасывает кэш для всех юзеров. Вызывается при изменении grants
// любой роли/группы (broad invalidation -- проще, чем отслеживать носителей).
func (r *PermissionResolver) InvalidateAll() {
	r.cache.Range(func(k, _ any) bool {
		r.cache.Delete(k)
		return true
	})
	// Гранты роли/группы могли изменить права любого носителя - шлём каждому
	// подключённому сигнал перезапросить своё (у каждого свой scope user:<id>;
	// для незатронутых это no-op refetch). Best-effort, nil-safe.
	if r.realtimePublisher != nil {
		r.realtimePublisher.PublishToEachConnected(permissionsChangedEvent)
	}
}

// permissionsChangedEvent - сигнал "перезапроси права" для юзера userID.
func permissionsChangedEvent(userID int) realtime.Event {
	return realtime.Event{Type: "user.permissions", Scope: fmt.Sprintf("user:%d", userID)}
}

// computeSet -- основная логика без кэша. Вынесена для тестируемости.
func (r *PermissionResolver) computeSet(ctx context.Context, userID int) (PermissionSet, error) {
	var user models.User
	if err := r.db.WithContext(ctx).
		Select("id, role_id, is_super_admin, is_admin, is_banned, ban_reason").
		First(&user, userID).Error; err != nil {
		return PermissionSet{}, fmt.Errorf("user not found: %w", err)
	}

	// Бан проверяется до super-admin: бан сильнее (защита от случайной самоблокировки).
	if user.IsBanned {
		reason := ""
		if user.BanReason != nil {
			reason = *user.BanReason
		}
		return PermissionSet{banned: true, banReason: reason}, nil
	}
	if user.IsSuperAdmin {
		return PermissionSet{allowAll: true}, nil
	}

	// Администратор: всё кроме super-only и личных deny-override. Роль/группы не нужны.
	if user.IsAdmin {
		denies, err := r.loadDenyOverrides(ctx, userID)
		if err != nil {
			return PermissionSet{}, err
		}
		return PermissionSet{adminAll: true, denies: denies}, nil
	}

	allows := make(map[string]string)
	denies := make(map[string]struct{})

	// 1a. Собственные точечные grants роли.
	if user.RoleID != nil {
		if err := r.collectRoleGrants(ctx, *user.RoleID, allows, denies); err != nil {
			return PermissionSet{}, err
		}
		// 1b. Default-группы роли.
		if err := r.collectGroupGrants(ctx, []int{*user.RoleID}, true, SourceRole, allows, denies); err != nil {
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
	if err := r.collectGroupGrants(ctx, userGroupIDs, false, SourceGroup, allows, denies); err != nil {
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
		switch o.Value {
		case "deny":
			denies[o.PermissionKey] = struct{}{}
			delete(allows, o.PermissionKey)
		case "allow":
			delete(denies, o.PermissionKey)
			allows[o.PermissionKey] = SourceOverride
		}
	}

	// Вычитаем deny из allows (на всякий случай, после групповых deny).
	for k := range denies {
		delete(allows, k)
	}

	return PermissionSet{allows: allows, denies: denies}, nil
}

// loadDenyOverrides загружает личные deny-override юзера (для admin-режима).
func (r *PermissionResolver) loadDenyOverrides(ctx context.Context, userID int) (map[string]struct{}, error) {
	var overrides []models.UserPermissionOverride
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND value = ?", userID, "deny").
		Find(&overrides).Error; err != nil {
		return nil, fmt.Errorf("failed to load deny overrides: %w", err)
	}
	denies := make(map[string]struct{}, len(overrides))
	for _, o := range overrides {
		denies[o.PermissionKey] = struct{}{}
	}
	return denies, nil
}

// collectRoleGrants добавляет собственные точечные grants роли (source=role).
func (r *PermissionResolver) collectRoleGrants(ctx context.Context, roleID int, allows map[string]string, denies map[string]struct{}) error {
	var grants []models.RolePermissionGrant
	if err := r.db.WithContext(ctx).
		Where("role_id = ?", roleID).
		Find(&grants).Error; err != nil {
		return fmt.Errorf("failed to load role grants: %w", err)
	}
	for _, g := range grants {
		applyGrant(g.PermissionKey, g.Value, SourceRole, allows, denies)
	}
	return nil
}

// collectGroupGrants добавляет права из указанных групп с заданным источником.
// Если byRole = true, вместо groupIDs используются default-группы роли (groupIDs = roleIDs).
func (r *PermissionResolver) collectGroupGrants(ctx context.Context, groupIDs []int, byRole bool, source string, allows map[string]string, denies map[string]struct{}) error {
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
		applyGrant(g.PermissionKey, g.Value, source, allows, denies)
	}
	return nil
}

// applyGrant применяет одну запись grant (allow/deny) к наборам, обновляя источник.
func applyGrant(key, value, source string, allows map[string]string, denies map[string]struct{}) {
	switch value {
	case "allow":
		if _, denied := denies[key]; !denied {
			allows[key] = source
		}
	case "deny":
		denies[key] = struct{}{}
		delete(allows, key)
	}
}
