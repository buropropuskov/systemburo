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
//
// Заблокированному/архивному оставляем доступ ТОЛЬКО на чтение (безопасные методы
// GET/HEAD/OPTIONS): личный кабинет грузит свои данные (ФИО, заявки, уведомления,
// статус блокировки из /permissions/my) под неснимаемой плашкой, но любая мутация
// (POST/PUT/PATCH/DELETE) -- 403. Чужие/админские данные закрыты и на чтении:
// permission-гейтнутые ручки всё равно 403 (резолвер: banned > admin, права пусты).
// Раньше резалось всё подряд - юзер видел пустой кабинет + спам "недостаточно прав".
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

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
			if banned || !active {
				// Чтение (GET/HEAD/OPTIONS) пропускаем: кабинет показывается read-only.
				if isSafeMethod(c.Request().Method) {
					return next(c)
				}
				if banned {
					return echo.NewHTTPError(http.StatusForbidden, "Учётная запись заблокирована")
				}
				return echo.NewHTTPError(http.StatusForbidden, "Учётная запись отключена")
			}
			return next(c)
		}
	}
}
