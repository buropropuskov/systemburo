package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// tablePassBody -- поля тела PUT /cars|employees/:id/territory-status, нужные для
// авторизации: направление прохода (territory_status) и таблица КПП (table_id).
type tablePassBody struct {
	TerritoryStatus int  `json:"territory_status"`
	TableID         *int `json:"table_id"`
}

// RequireTablePassVerb гейтит отметку прохода на КПП правом table.<name>.entry
// (въезд/вход, territory_status=1) или table.<name>.exit (выезд/выход, иначе).
// В отличие от RequireTableVerb имя таблицы и направление берутся из ТЕЛА запроса,
// а не из path - охранник отмечает конкретную машину/человека, указывая таблицу и
// направление в body. Тело читается в буфер и восстанавливается, чтобы хендлер
// прочитал его повторно. Право entry/exit уже есть в каталоге таблицы и
// раздаётся охранникам - здесь лишь требуем его на бэке (фронт-кнопку F12 обходит).
func RequireTablePassVerb(db *gorm.DB, resolver *services.PermissionResolver, denialLog *services.AccessDenialService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, ok := c.Get("user_id").(int)
			if !ok || userID == 0 {
				return echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
			}

			bodyBytes, err := io.ReadAll(c.Request().Body)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "не удалось прочитать тело запроса")
			}
			c.Request().Body = io.NopCloser(bytes.NewReader(bodyBytes))

			var body tablePassBody
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
			}
			if body.TableID == nil {
				return echo.NewHTTPError(http.StatusBadRequest, "table_id обязателен для отметки прохода")
			}

			var name string
			if err := db.WithContext(c.Request().Context()).
				Table("system_tables").
				Select("name").
				Where("id = ?", *body.TableID).
				Scan(&name).Error; err != nil {
				return fmt.Errorf("failed to resolve table name for pass gate: %w", err)
			}
			if name == "" {
				return echo.NewHTTPError(http.StatusNotFound, "Таблица не найдена")
			}

			verb := "exit"
			if body.TerritoryStatus == 1 {
				verb = "entry"
			}
			key := fmt.Sprintf("table.%s.%s", name, verb)

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
