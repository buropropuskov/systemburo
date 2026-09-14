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
			if err := guardUploadDir(c); err != nil {
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

// staticUploadDirs - каталоги uploads, которые раздаются по адресу файла. Перечень
// закрытый: всё, чего в нём нет, не отдаётся вовсе (#2498).
//
// До этого раздавалось содержимое всего каталога загрузок, и вошедшему в систему был
// открыт по прямому адресу любой файл - в том числе Excel-бланки массового ввода
// (imports), где строки людей с ФИО и документами, и документы с материалами
// руководства, у которых есть собственные ручки скачивания с проверкой прав.
// Неугадываемое имя файла защитой не считается: адрес оседает в журнале запросов
// администратора, в резервной копии и в пакете выгрузки организации.
//
// Перечень закрытый, а не список запретов, ровно по той причине, по которой эта
// задача и появилась: каталог заводят под новую возможность, про раздачу при этом не
// вспоминают, и он молча оказывается открытым. Теперь наоборот - молча закрытым.
var staticUploadDirs = map[string]bool{
	// Фото мест разгрузки и системных таблиц: показываются тегом img по ссылке из
	// ответа API, персональных данных не несут.
	"unload_places": true,
	"system_tables": true,
	// Excel-шаблоны бланков: file_path у шаблона указывает прямо сюда.
	"templates": true,
	// Сканы заявок остаются доступными по адресу, но под проверкой принадлежности
	// (#2465): фронт качает их своей ручкой по идентификатору строки, а прямые
	// ссылки могли разойтись раньше.
	services.ApplicationFilesDir: true,
}

// guardUploadDir отсекает каталоги, не предназначенные для раздачи по адресу файла.
// Ответ тот же, что у несуществующего файла: перечень каталогов - не секрет, но и
// подсказывать, какие из них существуют, ни к чему.
func guardUploadDir(c echo.Context) error {
	name, ok := staticFileName(c)
	if !ok {
		return nil
	}
	dir, _, found := strings.Cut(name, "/")
	// Файл в корне каталога загрузок не раздаётся: там лежит служебное, а не то, на
	// что система выдаёт ссылки.
	if !found || !staticUploadDirs[dir] {
		return echo.ErrNotFound
	}
	return nil
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
