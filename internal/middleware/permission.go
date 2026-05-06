package middleware

import (
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RequirePermission создаёт middleware для legacy PermissionService.
// Возвращает 403, если у юзера нет права со значением "allow".
//
// DEPRECATED: используйте RequirePermissionV2, которая работает через
// PermissionResolver (новая модель с ролями/группами/override) и логирует
// отказы в access_denials.
func RequirePermission(db *gorm.DB, permService services.PermissionService, key string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			username, _ := c.Get("username").(string)
			if username == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
			}

			var userID int
			err := db.Table("users").Select("id").Where("username = ?", username).Row().Scan(&userID)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "Пользователь не найден")
			}

			has, err := permService.HasPermission(c.Request().Context(), userID, key)
			if err != nil {
				return err
			}
			if !has {
				return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
			}

			return next(c)
		}
	}
}

// RequirePermissionV2 -- middleware на базе PermissionResolver (#229) с логированием
// отказов в access_denials (#230).
//
// Поведение при отказе:
//  1. Резолвит userID через user.id из контекста (положен JWT middleware).
//  2. Логирует событие через AccessDenialService.Log (асинхронно).
//  3. Возвращает 403 с JSON-телом {error, required_permission}.
//
// Если юзер не аутентифицирован -- 401 без логирования (это auth, а не denial).
func RequirePermissionV2(resolver *services.PermissionResolver, denialLog *services.AccessDenialService, key string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userIDAny := c.Get("user_id")
			userID, ok := userIDAny.(int)
			if !ok || userID == 0 {
				return echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
			}

			set, err := resolver.Resolve(c.Request().Context(), userID)
			if err != nil {
				return err
			}
			if set.Has(key) {
				return next(c)
			}

			uid := userID
			permKey := key
			ip := c.RealIP()
			ua := c.Request().UserAgent()
			reason := models.DenialReasonPermission
			errMsg := "Недостаточно прав"
			if set.IsBanned() {
				reason = models.DenialReasonBanned
				errMsg = "Учётная запись заблокирована"
			}
			denialLog.Log(services.LogParams{
				UserID:        &uid,
				Resource:      c.Request().Method + " " + c.Path(),
				PermissionKey: &permKey,
				Reason:        reason,
				IPAddress:     &ip,
				UserAgent:     &ua,
			})

			return c.JSON(http.StatusForbidden, map[string]any{
				"success":             false,
				"error":               errMsg,
				"required_permission": key,
				"banned":              set.IsBanned(),
			})
		}
	}
}
