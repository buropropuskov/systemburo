package middleware

import (
	"log/slog"
	"net/http"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// BanCheck блокирует забаненных И архивных пользователей на КАЖДОМ
// protected-запросе.
//
// Без этого middleware юзер с валидным access-токеном продолжает работать до
// истечения exp - это окно опасно для API-only клиентов (без фронт-полинга
// /permissions/my). Архив (is_active=false) трактуем как мгновенный офбординг
// наравне с баном: login/refresh уже блокируются по is_active, здесь закрываем
// окно живого access-токена.
//
// Должен стоять после JWTAuth (нужен user_id в context).
func BanCheck(svc *services.BanCheckService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, _ := c.Get("user_id").(int)
			if userID == 0 {
				return next(c)
			}
			banned, active, err := svc.Status(c.Request().Context(), userID)
			if err != nil {
				// Ошибка БД на горячем пути - пропускаем (fail-open),
				// иначе любой временный сбой положит весь API. Логируем.
				slog.Warn("ban_check: db lookup failed, fail-open", "user_id", userID, "error", err)
				return next(c)
			}
			if banned {
				return echo.NewHTTPError(http.StatusForbidden, "Учётная запись заблокирована")
			}
			if !active {
				return echo.NewHTTPError(http.StatusForbidden, "Учётная запись отключена")
			}
			return next(c)
		}
	}
}
