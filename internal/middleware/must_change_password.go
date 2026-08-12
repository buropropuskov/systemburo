package middleware

import (
	"log/slog"
	"net/http"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// PasswordChangeRequiredCode - машинный код отказа. По нему интерфейс отличает
// "надо сменить пароль" от обычной нехватки прав и уводит человека на форму, а не
// показывает пустой экран с отказом.
const PasswordChangeRequiredCode = "PASSWORD_CHANGE_REQUIRED"

// MustChangePasswordWhitelist - protected-роуты, доступные пользователю с поднятым
// users.must_change_password (#1911). Ключ - "МЕТОД ПУТЬ", путь совпадает с
// зарегистрированным в роутере (вместе с префиксом /api), потому что сверяется
// с c.Path().
//
// Список экспортирован ради гвард-тестов: расширение исключений - это расширение
// дыры, и оно должно быть осознанным, а не побочным эффектом правки соседнего кода.
//
// Обязательные: смена своего пароля (ради неё гейт и стоит), требования политики
// для чеклиста в окне и выход - без них форма стала бы тупиком. Права нужны, иначе
// фронт по пустому ответу считает пользователя заблокированным и показывает плашку
// блокировки вместо формы; профиль и тема держат окно похожим на систему, а не на
// белый лист.
//
// Билета real-time потока здесь намеренно нет, хотя гейт согласия его пропускает:
// там после согласия маркер не меняется и поток надо переподключить на месте, а
// здесь смена пароля отзывает все сессии - человек входит заново, и поток
// поднимается с нуля.
//
// Роуты окна согласия (гейт, принятие, документ и его описание, настройки
// уведомлений) пропускаются, хотя к паролю отношения не имеют. Причина в порядке
// двух гейтов: согласие человек даёт первым, и пока его окно на экране, ни один
// запрос оттуда не должен упереться в требование сменить пароль. Без этого
// работник с поднятым признаком смены и без согласия запирался снаружи наглухо -
// смену пароля закрывал гейт согласия, принятие согласия закрывал этот гейт.
// Инвариант «всё, что пропускает гейт согласия, пропускает и этот» держит
// TestGates_ConsentWhitelistIsSubsetOfPasswordWhitelist.
var MustChangePasswordWhitelist = map[string]bool{
	"PUT /api/users/me/password": true,
	"POST /api/logout":           true,
	"POST /api/logout-all":       true,

	"GET /api/settings/password-policy": true,
	"GET /api/permissions/my":           true,
	"GET /api/users/me":                 true,
	"GET /api/users/me/theme":           true,

	"GET /api/consents/gate":                          true,
	"POST /api/consents/accept":                       true,
	"GET /api/settings/data-processing/document":      true,
	"GET /api/settings/data-processing/document/meta": true,
	"GET /api/settings/notifications":                 true,
}

// MustChangePassword закрывает protected-API пользователю, которому система обязала
// сменить пароль (#1911). Пароль из письма лежит в почтовом ящике открытым текстом;
// пока он в силе, перехваченное письмо равно захваченной учётной записи, поэтому
// работа в системе начинается только после того, как человек задал свой.
//
// Должен стоять после JWTAuth (нужен user_id) и после проверки блокировки:
// заблокированному менять пароль всё равно нечего - ему показывается блокировка.
//
// Супер-администратор НЕ исключается, в отличие от гейта согласия. Там исключение -
// аварийная дверь на случай битой настройки, здесь ломаться нечему: флаг ставится
// пользователю поимённо, а путь к его снятию открыт белым списком всегда.
//
// Отказ отдаём через c.JSON, а не echo.NewHTTPError: доп. поле code через
// обработчик ошибок не пролезет, а фронту нужен маркер именно в теле ответа.
func MustChangePassword(svc *services.PasswordChangeGateService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, _ := c.Get("user_id").(int)
			if userID == 0 {
				return next(c)
			}
			if MustChangePasswordWhitelist[c.Request().Method+" "+c.Path()] {
				return next(c)
			}

			required, err := svc.Required(c.Request().Context(), userID)
			if err != nil {
				// Гейт висит на каждом protected-запросе: сбой базы на горячем пути
				// пропускаем и пишем в журнал, иначе временная недоступность положит
				// весь API. Поведение то же, что у проверки блокировки.
				slog.Warn("must_change_password: не удалось прочитать флаг, пропускаем", "user_id", userID, "error", err)
				return next(c)
			}
			if !required {
				return next(c)
			}

			c.Response().Header().Set("X-Password-Change-Required", "1")
			return c.JSON(http.StatusForbidden, map[string]any{
				"success": false,
				"error":   "Требуется сменить пароль",
				"code":    PasswordChangeRequiredCode,
			})
		}
	}
}
