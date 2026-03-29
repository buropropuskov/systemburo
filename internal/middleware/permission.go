package middleware

import (
	"net/http"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RequirePermission creates middleware that checks if the user has the specified permission.
// Returns 403 if the user does not have the permission with value "allow".
func RequirePermission(db *gorm.DB, permService services.PermissionService, key string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			username, _ := c.Get("username").(string)
			if username == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
			}

			// Get user ID by username
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
