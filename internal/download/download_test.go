package download

import (
	"archive/zip"
	"bytes"
	"io"
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

// readZip разбирает ответ StreamZip обратно и возвращает содержимое по имени записи.
func readZip(t *testing.T, body []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)

	out := make(map[string]string, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		out[f.Name] = string(content)
	}
	return out
}

func TestStreamZip_HeadersAndCyrillicNames(t *testing.T) {
	p1 := writeTemp(t, "a.xlsx", "первый файл")
	p2 := writeTemp(t, "b.xlsx", "второй файл")
	c, rec := newCtx()

	entries := []ZipEntry{
		{Path: p1, Name: "2026/Июль/Пропуск на людей (№1).xlsx"},
		{Path: p2, Name: "2026/Июль/Пропуск на людей (№2).xlsx"},
	}
	require.NoError(t, StreamZip(c, "архив.zip", entries))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/zip", rec.Header().Get(echo.HeaderContentType))
	assert.Contains(t, rec.Header().Get(echo.HeaderContentDisposition), "attachment")

	files := readZip(t, rec.Body.Bytes())
	require.Len(t, files, 2)
	assert.Equal(t, "первый файл", files["2026/Июль/Пропуск на людей (№1).xlsx"])
	assert.Equal(t, "второй файл", files["2026/Июль/Пропуск на людей (№2).xlsx"])
}

// Файл, пропавший с диска между отбором записей и потоковой отдачей, не должен
// оборвать весь архив - на его месте должна оказаться заметка об ошибке, а
// остальные записи дойти как обычно.
func TestStreamZip_MissingFileBecomesErrorNote(t *testing.T) {
	ok := writeTemp(t, "ok.xlsx", "содержимое")
	c, rec := newCtx()

	entries := []ZipEntry{
		{Path: "/no/such/file-xyz.xlsx", Name: "пропавший.xlsx"},
		{Path: ok, Name: "цел.xlsx"},
	}
	require.NoError(t, StreamZip(c, "архив.zip", entries))
	assert.Equal(t, http.StatusOK, rec.Code, "частичный сбой не должен менять статус - тело уже пошло в сеть")

	files := readZip(t, rec.Body.Bytes())
	require.Len(t, files, 2)
	assert.Equal(t, "содержимое", files["цел.xlsx"])
	require.Contains(t, files, "пропавший.xlsx"+zipErrorSuffix)
	assert.Contains(t, files["пропавший.xlsx"+zipErrorSuffix], "пропавший.xlsx")
}
