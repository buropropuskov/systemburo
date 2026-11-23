package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v5"
	"gorm.io/gorm"
)

// tableElementBindings - где искать посты элемента. Имена таблиц только из этого
// списка: в SQL они подставляются строкой.
var tableElementBindings = map[string]struct{ table, column string }{
	"cars":      {"car_target_tables", "car_id"},
	"employees": {"employee_target_tables", "employee_id"},
}

// RequireTableElementVerb проверяет возврат и восстановление по праву поста.
// Legacy deactivate меняет запись на всех постах и требует page.admin.
// Для адресного снятия используется отдельный RequireTableRemoval.
//
// Без этого гейта методы выполнял любой вошедший, в том числе для чужих элементов (#2600).
// Таблица берётся из привязок элемента, а не из тела запроса: FactTable шлёт снятие без
// table_id, а переданной в теле таблице без сверки с привязками верить нельзя.
func RequireTableElementVerb(db *gorm.DB, resolver *services.PermissionResolver, denialLog *services.AccessDenialService, element, verb string) echo.MiddlewareFunc {
	binding, ok := tableElementBindings[element]
	if !ok {
		panic(fmt.Sprintf("RequireTableElementVerb: неизвестный элемент %q", element))
	}
	query := fmt.Sprintf(`SELECT DISTINCT st.name FROM %s b JOIN system_tables st ON st.id = b.table_id WHERE b.%s = ?`,
		binding.table, binding.column)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			userID, ok := c.Get("user_id").(int)
			if !ok || userID == 0 {
				return echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
			}
			elementID, err := strconv.Atoi(c.Param("id"))
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
			}

			// Deactivation affects every post. A post-scoped delete permission must
			// use unbind-table instead; the body cannot narrow this legacy operation.
			if verb == "delete" && strings.HasSuffix(c.Path(), "/deactivate") {
				return RequirePermissionV2(resolver, denialLog, services.KeyPageAdmin)(next)(c)
			}
			set, err := resolver.ResolveFresh(c.Request().Context(), userID)
			if err != nil {
				return err
			}

			var names []string
			if err := db.WithContext(c.Request().Context()).Raw(query, elementID).Scan(&names).Error; err != nil {
				return echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка доступа временно недоступна").Wrap(err)
			}

			required := services.KeyPageAdmin
			if len(names) > 0 {
				required = fmt.Sprintf("table.%s.%s", names[0], verb)
			}
			for _, name := range names {
				if set.Has(fmt.Sprintf("table.%s.%s", name, verb)) {
					return next(c)
				}
			}
			if len(names) == 0 && set.Has(services.KeyPageAdmin) {
				return next(c)
			}

			uid := userID
			permKey := required
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
				"required_permission": required,
				"banned":              set.IsBanned(),
			})
		}
	}
}
