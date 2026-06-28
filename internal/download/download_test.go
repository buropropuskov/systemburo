package download

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCtx() (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func httpCode(err error) int {
	if he, ok := err.(*echo.HTTPError); ok {
		return he.Code
	}
	return 0
}

func TestServe_NotFoundOnEmptyPath(t *testing.T) {
	c, _ := newCtx()
	assert.Equal(t, http.StatusNotFound, httpCode(Serve(c, File{Path: ""})))
}

func TestServe_NotFoundOnMissingFile(t *testing.T) {
	c, _ := newCtx()
	assert.Equal(t, http.StatusNotFound, httpCode(Serve(c, File{Path: "/no/such/file-xyz.bin"})))
}

func TestServe_NotFoundOnDir(t *testing.T) {
	c, _ := newCtx()
	assert.Equal(t, http.StatusNotFound, httpCode(Serve(c, File{Path: t.TempDir()})))
}

func TestServe_AttachmentHeaders(t *testing.T) {
	p := writeTemp(t, "f.pdf", "%PDF-1.4 test")
	c, rec := newCtx()

	require.NoError(t, Serve(c, File{Path: p, Name: `от"чёт.pdf`, Mime: "application/pdf"}))
	assert.Equal(t, http.StatusOK, rec.Code)
	cd := rec.Header().Get(echo.HeaderContentDisposition)
	assert.Contains(t, cd, "attachment")
	// кавычка экранирована, имя не разорвало заголовок
	assert.Contains(t, cd, `от\"чёт.pdf`)
	assert.Equal(t, "application/pdf", rec.Header().Get(echo.HeaderContentType))
	assert.Equal(t, "%PDF-1.4 test", rec.Body.String())
}

func TestServe_Inline(t *testing.T) {
	p := writeTemp(t, "f.pdf", "x")
	c, rec := newCtx()

	require.NoError(t, Serve(c, File{Path: p, Name: "f.pdf", Inline: true}))
	assert.Contains(t, rec.Header().Get(echo.HeaderContentDisposition), "inline")
}

func TestServe_NoNameNoDisposition(t *testing.T) {
	p := writeTemp(t, "tpl.xlsx", "data")
	c, rec := newCtx()

	require.NoError(t, Serve(c, File{Path: p}))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get(echo.HeaderContentDisposition))
}

func TestSanitizeName(t *testing.T) {
	assert.Equal(t, `ab\"c`, sanitizeName("a\r\nb\"c"))
	assert.Equal(t, "normal.pdf", sanitizeName("normal.pdf"))
}
