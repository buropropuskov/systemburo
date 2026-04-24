package middleware

import (
	"net/http"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// MaintenanceBlock - после JWTAuth блокирует 503-м любой protected-запрос
// если включён режим техработ. Super-admin (type_id=6) проходит всегда.
// Для производительности использует GetStatusCached (10-сек in-memory).
func MaintenanceBlock(svc services.MaintenanceService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			typeID, _ := c.Get("type_id").(int)
			// Super-admin проходит всегда - иначе как включать/выключать
			// maintenance и тестировать систему.
			if typeID == 6 {
				return next(c)
			}
			st := svc.GetStatusCached(c.Request().Context())
			if st != nil && st.Enabled {
				return echo.NewHTTPError(http.StatusServiceUnavailable,
					"Сервис временно недоступен: технические работы")
			}
			return next(c)
		}
	}
}
