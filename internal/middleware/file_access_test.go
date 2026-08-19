package middleware_test

import (
	"net/http"
	"net/http/httptest"
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

// serveFileAccess прогоняет запрос через middleware и возвращает код ответа.
// Обработчик за middleware отвечает 200 - значит, любой другой код поставило оно.
func serveFileAccess(t *testing.T, req *http.Request) int {
	t.Helper()
	e := echo.New()
	h := mw.FileAccess(fileAccessSecret, fileRefreshSecret)(func(c echo.Context) error {
		return c.String(http.StatusOK, "file")
	})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h(c); err != nil {
		e.HTTPErrorHandler(err, c)
	}
	return rec.Code
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
