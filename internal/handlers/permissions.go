package handlers

import (
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// PermissionHandler -- HTTP-обработчики разрешений.
type PermissionHandler struct {
	service  services.PermissionService
	resolver *services.PermissionResolver
}

// NewPermissionHandler создаёт новый экземпляр обработчика разрешений.
// resolver используется для GetMyPermissions (новая система прав #187),
// service остаётся для /catalog, /user/:id и auto-generate.
func NewPermissionHandler(service services.PermissionService, resolver *services.PermissionResolver) *PermissionHandler {
	return &PermissionHandler{service: service, resolver: resolver}
}

// GetMyPermissions возвращает эффективные права текущего пользователя.
// Формат {mode, permissions[{key,value,source}], denied, banned, ban_reason}:
//   - mode=super  -> всё разрешено (permissions пуст, фронт включает всё readonly);
//   - mode=admin  -> всё кроме super-only и denied (личных deny-override);
//   - mode=normal -> permissions = allow-список с источником (роль/группа/override);
//   - mode=banned -> прав нет.
//
// Источник данных -- PermissionResolver (роли + группы + grants + overrides).
func (h *PermissionHandler) GetMyPermissions(c echo.Context) error {
	set, err := h.resolver.Resolve(c.Request().Context(), GetUserID(c))
	if err != nil {
		return err
	}
	return RespondSuccess(c, buildPermissionsResponse(set))
}

// GetUserEffectivePermissions возвращает эффективные права указанного пользователя
// с источником (роль/группа/override) -- для админ-экрана настройки доступа (#187 Фаза 3).
// Доступ - permission.audit.manage (super + admin), гейтится route-middleware.
// Формат идентичен GetMyPermissions, но считается для целевого юзера, чтобы в правом
// столбце модалки прав показать бейдж источника каждого права и унаследованные от
// роли/групп права (а не только личные override из /user/:id).
func (h *PermissionHandler) GetUserEffectivePermissions(c echo.Context) error {
	userID, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	set, err := h.resolver.Resolve(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, buildPermissionsResponse(set))
}

// buildPermissionsResponse собирает ответ {mode, permissions[{key,value,source}],
// denied, banned, ban_reason} из вычисленного набора прав. Общий код для
// GetMyPermissions (текущий юзер) и GetUserEffectivePermissions (целевой юзер).
func buildPermissionsResponse(set services.PermissionSet) models.MyPermissionsResponse {
	keys := set.Keys()
	perms := make([]models.MyPermissionItem, 0, len(keys))
	for _, k := range keys {
		perms = append(perms, models.MyPermissionItem{
			Key:    k,
			Value:  "allow",
			Source: set.Source(k),
		})
	}
	denied := set.Denies()
	if set.Mode() == "admin" {
		// PermissionSet.Has режет super-only ключи для всех, кроме супер-админа,
		// но Denies() отдаёт только личные deny-override (#1997) - фронтовый стор
		// в admin-режиме считает ключ выданным, если его нет в denied, поэтому
		// без явного добавления интерфейс показывал бы доступным то, что сервер
		// на сохранении отклонит.
		denied = append(append([]string{}, denied...), services.SuperOnlyKeys()...)
	}
	return models.MyPermissionsResponse{
		Mode:        set.Mode(),
		Permissions: perms,
		Denied:      denied,
		Banned:      set.IsBanned(),
		BanReason:   set.BanReason(),
	}
}

// GetUserPermissions возвращает индивидуальные override указанного пользователя.
// Доступ - permission.audit.manage (super + admin), гейтится route-middleware.
func (h *PermissionHandler) GetUserPermissions(c echo.Context) error {
	userID, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	perms, err := h.service.GetUserPermissions(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, perms)
}

// UpdateUserPermissions обновляет индивидуальные override указанного пользователя.
// Доступ - permission.audit.manage (super + admin). Флаг isSuperAdmin прокидывается
// в сервис: не-супер не может выдать override на super-only ключи.
func (h *PermissionHandler) UpdateUserPermissions(c echo.Context) error {
	userID, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	var req models.UpdatePermissionsRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateUserPermissions(c.Request().Context(), IsSuperAdmin(c), GetUserID(c), userID, req); err != nil {
		return err
	}
	// Сбрасываем кэш резолвера (TTL 30s), чтобы выданные права применились сразу.
	h.resolver.Invalidate(userID)
	return RespondMessage(c, "ok")
}

// GetCatalog возвращает каталог прав (статика + динамические table.*) для UI настройки.
func (h *PermissionHandler) GetCatalog(c echo.Context) error {
	catalog, err := h.service.GetCatalog(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, catalog)
}

// AutoGenerate создаёт разрешения для таблицы (только admin).
func (h *PermissionHandler) AutoGenerate(c echo.Context) error {
	var req models.AutoGenerateRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.AutoGenerateForTable(c.Request().Context(), req.TableID, req.TableName); err != nil {
		return err
	}
	return RespondMessage(c, "ok")
}
