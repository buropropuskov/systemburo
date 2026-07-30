package middleware

import (
	"log/slog"
	"net/http"
	"sync/atomic"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// PDConsentWhitelist -- protected-роуты, доступные пользователю, у которого ещё нет
// согласия на обработку персональных данных (#1567). Ключ -- "МЕТОД ПУТЬ", путь
// совпадает с зарегистрированным в роутере (вместе с префиксом /api), потому что
// сверяется с c.Path().
//
// Список экспортирован ради гвард-теста: он сверяет каждый ключ с e.Routes(), иначе
// переименованный роут молча выпадет из исключений и запрёт сам механизм согласия.
//
// Обязательные: без них пользователь не сможет ни увидеть текст, ни согласиться,
// ни выйти - то есть окно согласия станет тупиком. /events/ticket в списке потому,
// что после согласия токен не меняется, real-time поток не переподключается сам, и
// без билета он молчал бы до перезагрузки страницы.
//
// Остальные держат окно согласия работоспособным: права (иначе фронт считает
// пользователя забаненным), тема оформления, настройки уведомлений и сам документ,
// который из окна предлагается скачать.
var PDConsentWhitelist = map[string]bool{
	"POST /api/consents/accept": true,
	"GET /api/consents/gate":    true,
	"POST /api/logout":          true,
	"POST /api/logout-all":      true,
	"POST /api/events/ticket":   true,

	"GET /api/permissions/my":                         true,
	"GET /api/users/me":                               true,
	"GET /api/users/me/theme":                         true,
	"GET /api/settings/notifications":                 true,
	"GET /api/settings/data-processing/document":      true,
	"GET /api/settings/data-processing/document/meta": true,
}

// PDConsentGate закрывает protected-API пользователю, который ещё не дал согласие
// на обработку персональных данных требуемой редакции (#1567).
//
// Должен стоять после JWTAuth (нужны user_id и is_super_admin) и после BanCheck:
// забаненный не может дать согласие (проверка бана режет POST), поэтому он должен
// видеть "учётная запись заблокирована", а не требование согласия.
//
// Супер-администратор проходит всегда - это аварийная дверь: с ошибочной настройкой
// согласия систему всё равно надо чинить через интерфейс.
//
// Отказ отдаём через c.JSON, а не echo.NewHTTPError: доп. поле consent_required
// через обработчик ошибок не пролезет, а фронту нужен маркер именно в теле ответа,
// чтобы отличить требование согласия от обычной нехватки прав и не показать стену
// тостов.
func PDConsentGate(svc *services.PDConsentGateService) echo.MiddlewareFunc {
	// Об ошибке настройки (тумблер включён, а текст пуст) сообщаем на ПЕРЕХОДЕ
	// состояния: писать её на каждый запрос значит утопить журнал.
	var misconfigured atomic.Bool

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, _ := c.Get("user_id").(int)
			if userID == 0 {
				return next(c)
			}
			if isSuper, _ := c.Get("is_super_admin").(bool); isSuper {
				return next(c)
			}
			if PDConsentWhitelist[c.Request().Method+" "+c.Path()] {
				return next(c)
			}

			req, err := svc.Requirement(c.Request().Context())
			if err != nil {
				// Сбой чтения настроек на горячем пути не должен класть весь API:
				// пропускаем и пишем в журнал (как BanCheck).
				slog.Warn("pd_consent: не удалось прочитать настройки, пропускаем", "user_id", userID, "error", err)
				return next(c)
			}
			if !req.Enabled {
				if req.Requested && !misconfigured.Swap(true) {
					slog.Error("pd_consent: запрос согласия включён, но текст пуст - гейт не работает")
				}
				return next(c)
			}
			misconfigured.Store(false)

			accepted, err := svc.AcceptedVersion(c.Request().Context(), userID)
			if err != nil {
				slog.Warn("pd_consent: не удалось прочитать согласие, пропускаем", "user_id", userID, "error", err)
				return next(c)
			}
			if accepted >= req.Version {
				return next(c)
			}

			c.Response().Header().Set("X-PD-Consent-Required", "1")
			return c.JSON(http.StatusForbidden, map[string]any{
				"success":          false,
				"error":            "Требуется согласие на обработку персональных данных",
				"consent_required": true,
			})
		}
	}
}
