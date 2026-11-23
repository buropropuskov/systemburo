package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"systemburo/internal/services"

	"github.com/labstack/echo/v5"
	"gorm.io/gorm"
)

// RequireTableRemoval permits post-scoped removal without granting global
// deactivation. Multiple posts retain the existing administrative restriction.
func RequireTableRemoval(db *gorm.DB, resolver *services.PermissionResolver, denialLog *services.AccessDenialService, tableType string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			userID, ok := c.Get("user_id").(int)
			if !ok || userID <= 0 {
				return echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
			}
			body, err := io.ReadAll(c.Request().Body)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "Не удалось прочитать запрос")
			}
			c.Request().Body = io.NopCloser(bytes.NewReader(body))
			var req struct {
				IDs       []int `json:"ids"`
				TableID   int   `json:"table_id"`
				TableIDs  []int `json:"table_ids"`
				AllTables bool  `json:"all_tables"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "Некорректный запрос")
			}
			ids, err := services.TableRemovalTargets(req.TableID, req.TableIDs)
			if err != nil {
				return err
			}
			var tables []struct {
				ID   int
				Name string
			}
			if err := db.WithContext(c.Request().Context()).Table("system_tables").Select("id, name").Where("id IN ? AND table_type = ?", ids, tableType).Find(&tables).Error; err != nil {
				return echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка постов временно недоступна").Wrap(err)
			}
			if len(tables) != len(ids) {
				return echo.NewHTTPError(http.StatusBadRequest, "Некорректный набор постов")
			}
			set, err := resolver.ResolveFresh(c.Request().Context(), userID)
			if err != nil {
				return err
			}
			if !set.IsBanned() && set.Has(services.KeyPageAdmin) {
				return next(c)
			}
			key := services.KeyPageAdmin
			if len(tables) == 1 && !req.AllTables {
				key = fmt.Sprintf("table.%s.delete", tables[0].Name)
			}
			return RequirePermissionV2(resolver, denialLog, key)(func(c *echo.Context) error {
				// A post-scoped operator must not learn names of unrelated records
				// from per-record bulk error results.
				unique := make(map[int]struct{}, len(req.IDs))
				for _, id := range req.IDs {
					if id <= 0 {
						return echo.NewHTTPError(http.StatusBadRequest, "Некорректный набор записей")
					}
					unique[id] = struct{}{}
				}
				if len(unique) == 0 {
					return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны записи")
				}
				binding, column := "car_target_tables", "car_id"
				if tableType == "people" {
					binding, column = "employee_target_tables", "employee_id"
				}
				var count int64
				if err := db.WithContext(c.Request().Context()).Table(binding).Where("table_id = ? AND "+column+" IN ?", ids[0], req.IDs).Distinct(column).Count(&count).Error; err != nil {
					return echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка записей временно недоступна").Wrap(err)
				}
				if count != int64(len(unique)) {
					return echo.NewHTTPError(http.StatusForbidden, "Запись недоступна на этом посту")
				}
				return next(c)
			})(c)
		}
	}
}
