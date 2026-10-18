package middleware

import (
	"log/slog"
	"net/http"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// BannedReadWhitelist содержит только чтение кабинета. Заявки дополнительно
// проверяют отправителя и используют безопасную проекцию.
var BannedReadWhitelist = map[string]bool{
	"GET /api/users/me":                               true,
	"GET /api/users/current":                          true,
	"GET /api/user-data":                              true,
	"GET /api/users/me/theme":                         true,
	"GET /api/permissions/my":                         true,
	"GET /api/applications/user":                      true,
	"GET /api/applications/user/status-updates-count": true,
	"GET /api/applications/:id":                       true,
	"GET /api/applications/:id/details":               true,
}

// isSafeMethod используется также ограничением impersonation.
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func isLogout(c echo.Context) bool {
	return c.Request().Method == http.MethodPost && (c.Path() == "/api/logout" || c.Path() == "/api/logout-all")
}

func BanCheck(svc *services.BanCheckService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if isLogout(c) {
				return next(c)
			}
			userID, _ := c.Get("user_id").(int)
			if userID == 0 {
				return echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
			}
			banned, active, err := svc.Status(c.Request().Context(), userID)
			if err != nil {
				slog.Warn("ban_check: state lookup failed", "user_id", userID, "error", err)
				return echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка доступа временно недоступна")
			}
			if !active {
				return echo.NewHTTPError(http.StatusForbidden, "Учётная запись отключена")
			}
			if banned && !BannedReadWhitelist[c.Request().Method+" "+c.Path()] {
				return echo.NewHTTPError(http.StatusForbidden, "Учётная запись заблокирована")
			}
			return next(c)
		}
	}
}
