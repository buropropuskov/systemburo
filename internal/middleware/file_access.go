package middleware

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"strings"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// ApplicationScanGuard решает, открыт ли вошедшему пользователю скан заявки,
// найденный по имени файла на диске. Реализация - services.ApplicationScanAccess.
type ApplicationScanGuard interface {
	CanAccessScan(ctx context.Context, storedName, username string) bool
}

// FileAccess закрывает раздачу загруженных файлов от тех, кто не вошёл в систему,
// а сканы, приложенные к заявкам, - ещё и от тех, кому не открыта сама заявка.
//
// Обычное JWT-middleware здесь не годится: файлы подставляются в атрибут src
// изображения, а тег <img> заголовок Authorization не отправляет. Поэтому кроме
// Bearer принимается cookie продления сеанса - её браузер шлёт сам, а сценарии
// страницы не читают.
//
// Проверяются подпись и срок, в базу за самим входом обращения нет: файлы
// запрашиваются пачками по десятку на страницу, и запрос к базе на каждую
// картинку стоил бы дороже, чем даёт. Отзыв маркера при выходе такая проверка не
// видит - до истечения срока прежняя cookie ещё открывает файлы, доступ ко всему
// остальному она уже не даёт.
//
// Принадлежность проверяется только у каталога сканов (#2465): фото мест
// разгрузки, системных таблиц и шаблоны персональных данных не содержат, и лишний
// запрос к базе на каждую их картинку как раз и был бы той ценой, которой здесь
// избегают.
func FileAccess(accessSecret, refreshSecret []byte, scans ApplicationScanGuard) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, err := fileRequester(c, accessSecret, refreshSecret)
			if err != nil {
				return err
			}
			username, _ := claims.GetSubject()
			if err := guardApplicationScan(c, username, scans); err != nil {
				return err
			}
			return next(c)
		}
	}
}

// fileRequester опознаёт запросившего файл: заголовком Authorization либо cookie
// продления сеанса. У маркера продления в полезной нагрузке только subject, поэтому
// наружу отдаются сами разобранные права доступа, а не поля маркера.
func fileRequester(c echo.Context, accessSecret, refreshSecret []byte) (*services.Claims, error) {
	if auth := c.Request().Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		claims, err := services.DecodeAccessToken(strings.TrimPrefix(auth, "Bearer "), accessSecret)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
		}
		return claims, nil
	}

	ck, err := c.Cookie(services.RefreshCookieName)
	if err != nil || ck.Value == "" {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Missing or invalid authorization header")
	}
	claims, err := services.DecodeRefreshToken(ck.Value, refreshSecret)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
	}
	return claims, nil
}

// guardApplicationScan пускает к скану заявки только того, кому открыта сама заявка.
//
// Чужой и несуществующий файл отвечают одинаково: 403 подтверждал бы, что файл с
// таким именем есть, и превращал бы раздачу в оракул подбора имён.
func guardApplicationScan(c echo.Context, username string, scans ApplicationScanGuard) error {
	name, ok := staticFileName(c)
	if !ok {
		return nil
	}
	prefix := services.ApplicationFilesDir + "/"
	if !strings.HasPrefix(name, prefix) {
		return nil
	}
	// Гейт не настроен - сканы не раздаём вовсе: отдать их без проверки хуже, чем
	// не отдать (раздаче они и не нужны, файл скачивается по идентификатору строки).
	if scans == nil {
		return echo.ErrNotFound
	}
	stored := strings.TrimPrefix(name, prefix)
	// Вложенных каталогов у сканов нет: имя со слэшем не откроется и без гейта.
	if stored == "" || strings.Contains(stored, "/") {
		return echo.ErrNotFound
	}
	if !scans.CanAccessScan(c.Request().Context(), stored, username) {
		return echo.ErrNotFound
	}
	return nil
}

// staticFileName повторяет разбор пути из echo.StaticDirectoryHandler: маршрутизатор
// кладёт в параметр сырой путь, а раздача снимает процентное кодирование сама, уже
// после выбора маршрута. Разбирать путь иначе - значит проверять не тот файл,
// который потом откроется с диска: %2f и %2e%2e доехали бы до раздачи мимо гейта
// (GHSA-vfp3-v2gw-7wfq).
func staticFileName(c echo.Context) (string, bool) {
	p, err := url.PathUnescape(c.Param("*"))
	if err != nil {
		// Раздача споткнётся о ту же ошибку и файла не отдаст.
		return "", false
	}
	return path.Clean(strings.TrimPrefix(p, "/")), true
}
