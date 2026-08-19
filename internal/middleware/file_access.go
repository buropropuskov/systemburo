package middleware

import (
	"net/http"
	"strings"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// FileAccess закрывает раздачу загруженных файлов от тех, кто не вошёл в систему.
//
// Обычное JWT-middleware здесь не годится: файлы подставляются в атрибут src
// изображения, а тег <img> заголовок Authorization не отправляет. Поэтому кроме
// Bearer принимается cookie продления сеанса - её браузер шлёт сам, а сценарии
// страницы не читают.
//
// Проверяются подпись и срок, в базу обращения нет: файлы запрашиваются пачками
// по десятку на страницу, и запрос к базе на каждую картинку стоил бы дороже,
// чем даёт. Отзыв маркера при выходе такая проверка не видит - до истечения
// срока прежняя cookie ещё открывает файлы, доступ ко всему остальному она уже
// не даёт.
func FileAccess(accessSecret, refreshSecret []byte) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if auth := c.Request().Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				if _, err := services.DecodeAccessToken(strings.TrimPrefix(auth, "Bearer "), accessSecret); err == nil {
					return next(c)
				}
				return echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
			}

			ck, err := c.Cookie(services.RefreshCookieName)
			if err != nil || ck.Value == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "Missing or invalid authorization header")
			}
			if _, err := services.DecodeRefreshToken(ck.Value, refreshSecret); err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
			}
			return next(c)
		}
	}
}
