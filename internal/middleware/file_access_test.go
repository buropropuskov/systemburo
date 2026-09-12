package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mw "systemburo/internal/middleware"
	"systemburo/internal/services"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	fileAccessSecret    = []byte("access-secret-for-file-access-tests")
	fileRefreshSecret   = []byte("refresh-secret-for-file-access-tests")
	fileForeignSecret   = []byte("secret-of-somebody-else-entirely-xx")
	fileAccessTestClaim = "petrov"
)

// signToken собирает маркер того же вида, что выдаёт вход, чтобы тест проверял
// разбор настоящего маркера, а не подогнанную под middleware строку.
func signToken(t *testing.T, secret []byte, ttl time.Duration) string {
	t.Helper()
	claims := &services.Claims{
		UserID: 7,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fileAccessTestClaim,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	require.NoError(t, err)
	return signed
}

// scanGuardStub -- гейт принадлежности с заранее известным ответом. Настоящий
// ходит в базу, а здесь проверяется сама развилка middleware.
type scanGuardStub struct {
	allow bool
	asked string
}

func (g *scanGuardStub) CanAccessScan(_ context.Context, storedName, _ string) bool {
	g.asked = storedName
	return g.allow
}

// serveFileAccess прогоняет запрос через middleware и возвращает код ответа.
// Обработчик за middleware отвечает 200 - значит, любой другой код поставило оно.
func serveFileAccess(t *testing.T, req *http.Request) int {
	t.Helper()
	code, _ := serveFileAccessWith(t, req, &scanGuardStub{allow: true})
	return code
}

// serveFileAccessWith -- то же с заданным гейтом сканов. Параметр маршрута тут
// выставляется руками: в бою его кладёт маршрутизатор, а тест поднимает middleware
// без маршрута, и без параметра гейту нечего было бы разбирать.
func serveFileAccessWith(t *testing.T, req *http.Request, scans mw.ApplicationScanGuard) (int, string) {
	t.Helper()
	e := echo.New()
	h := mw.FileAccess(fileAccessSecret, fileRefreshSecret, scans)(func(c echo.Context) error {
		return c.String(http.StatusOK, "file")
	})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("*")
	c.SetParamValues(strings.TrimPrefix(req.URL.EscapedPath(), "/api/uploads"))
	if err := h(c); err != nil {
		e.HTTPErrorHandler(err, c)
	}
	return rec.Code, rec.Body.String()
}

// Раздача загруженных файлов до #2133 отвечала кому угодно: фото постов и схемы
// проезда забирал любой, кто знает адрес файла. Проверяем сам пропуск, а не то,
// что файл нашёлся.
func TestFileAccess_AnonymousRejected(t *testing.T) {
	code := serveFileAccess(t, httptest.NewRequest(http.MethodGet, "/api/uploads/unload_places/photo.png", nil))
	assert.Equal(t, http.StatusUnauthorized, code)
}

// Пропуск для браузера - cookie продления сеанса: тег <img> заголовок
// Authorization не отправляет, и без этой ветки картинки просто не загрузились бы.
func TestFileAccess_RefreshCookieAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/uploads/unload_places/photo.png", nil)
	req.AddCookie(&http.Cookie{Name: services.RefreshCookieName, Value: signToken(t, fileRefreshSecret, time.Hour)})
	assert.Equal(t, http.StatusOK, serveFileAccess(t, req))
}

// Bearer остаётся рабочим путём: им ходят тесты и обращения мимо браузера.
func TestFileAccess_BearerAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/uploads/system_tables/photo.png", nil)
	req.Header.Set("Authorization", "Bearer "+signToken(t, fileAccessSecret, time.Hour))
	assert.Equal(t, http.StatusOK, serveFileAccess(t, req))
}

// Подпись чужим ключом и просроченный маркер - главное, ради чего middleware
// разбирает маркер, а не проверяет наличие cookie.
func TestFileAccess_ForgedAndExpiredRejected(t *testing.T) {
	cases := []struct {
		name   string
		cookie string
	}{
		{"подписан чужим ключом", signToken(t, fileForeignSecret, time.Hour)},
		{"срок истёк", signToken(t, fileRefreshSecret, -time.Minute)},
		{"не маркер вовсе", "ok"},
		{"пустое значение", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/uploads/unload_places/photo.png", nil)
			req.AddCookie(&http.Cookie{Name: services.RefreshCookieName, Value: tc.cookie})
			assert.Equal(t, http.StatusUnauthorized, serveFileAccess(t, req))
		})
	}
}

// Маркер доступа в cookie и маркер продления в заголовке подписаны разными
// ключами: перепутанные местами они не должны открывать файлы.
func TestFileAccess_SecretsNotInterchangeable(t *testing.T) {
	byCookie := httptest.NewRequest(http.MethodGet, "/api/uploads/unload_places/photo.png", nil)
	byCookie.AddCookie(&http.Cookie{Name: services.RefreshCookieName, Value: signToken(t, fileAccessSecret, time.Hour)})
	assert.Equal(t, http.StatusUnauthorized, serveFileAccess(t, byCookie))

	byHeader := httptest.NewRequest(http.MethodGet, "/api/uploads/unload_places/photo.png", nil)
	byHeader.Header.Set("Authorization", "Bearer "+signToken(t, fileRefreshSecret, time.Hour))
	assert.Equal(t, http.StatusUnauthorized, serveFileAccess(t, byHeader))
}

// Сканы, приложенные к заявке, -- персональные данные, и одного входа в систему
// для них мало (#2465): имя файла на диске оседает в журнале запросов, в резервной
// копии и в пакете выгрузки организации, а до гейта знание имени заменяло проверку
// доступа к заявке.
func TestFileAccess_ApplicationScanChecked(t *testing.T) {
	const stored = "9f1c2f1e-0000-4000-8000-abcdefabcdef.png"

	request := func(path string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+signToken(t, fileAccessSecret, time.Hour))
		return req
	}

	t.Run("свой скан отдаётся", func(t *testing.T) {
		guard := &scanGuardStub{allow: true}
		code, _ := serveFileAccessWith(t, request("/api/uploads/application_files/"+stored), guard)
		assert.Equal(t, http.StatusOK, code)
		assert.Equal(t, stored, guard.asked, "гейту должно достаться имя файла на диске")
	})

	t.Run("чужой скан неотличим от несуществующего", func(t *testing.T) {
		foreign, foreignBody := serveFileAccessWith(t,
			request("/api/uploads/application_files/"+stored), &scanGuardStub{allow: false})
		assert.Equal(t, http.StatusNotFound, foreign)
		// 403 подтвердил бы, что файл с таким именем есть.
		assert.Contains(t, foreignBody, "Not Found")
	})

	t.Run("процентное кодирование не проносит путь мимо гейта", func(t *testing.T) {
		// Раздача echo снимает кодировку сама, уже после выбора маршрута
		// (GHSA-vfp3-v2gw-7wfq): гейт обязан разбирать путь так же.
		guard := &scanGuardStub{allow: false}
		code, _ := serveFileAccessWith(t,
			request("/api/uploads/unload_places/%2e%2e/application%5ffiles/"+stored), guard)
		assert.Equal(t, http.StatusNotFound, code)
		assert.Equal(t, stored, guard.asked, "гейт должен был разобрать закодированный путь")
	})

	t.Run("без гейта каталог сканов не раздаётся вовсе", func(t *testing.T) {
		code, _ := serveFileAccessWith(t, request("/api/uploads/application_files/"+stored), nil)
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("прочие каталоги проверкой принадлежности не закрыты", func(t *testing.T) {
		guard := &scanGuardStub{allow: false}
		for _, path := range []string{
			"/api/uploads/unload_places/photo.png",
			"/api/uploads/system_tables/photo.png",
			"/api/uploads/templates/blank.xlsx",
		} {
			code, _ := serveFileAccessWith(t, request(path), guard)
			assert.Equal(t, http.StatusOK, code, path)
		}
		assert.Empty(t, guard.asked, "гейт не должен трогаться ради фото и шаблонов")
	})
}
