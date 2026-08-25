package handlers_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dpDocPath = "/api/settings/data-processing/document"

// minimalPDF -- содержимое с корректной PDF magic-сигнатурой (%PDF).
var minimalPDF = []byte("%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF\n")

// minimalOOXML -- содержимое с magic-сигнатурой zip (PK), общей для docx/xlsx/pptx.
var minimalOOXML = append([]byte{0x50, 0x4B, 0x03, 0x04}, bytes.Repeat([]byte("x"), 64)...)

// uploadDPDoc загружает документ согласия multipart-запросом от имени token.
func uploadDPDoc(t *testing.T, e *echo.Echo, token, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := buildMultipartDoc(t, filename, content, nil)
	req := httptest.NewRequest(http.MethodPost, dpDocPath, body)
	req.Header.Set("Content-Type", ct)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestDataProcessing_Upload_Admin_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := uploadDPDoc(t, e, token, "Согласие.pdf", minimalPDF)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	meta := testutil.ParseResponse[*models.DataProcessingDocument](t, rec)
	require.NotNil(t, meta)
	assert.Equal(t, "Согласие.pdf", meta.FileName)
	assert.Equal(t, "application/pdf", meta.MimeType)
	assert.Equal(t, ".pdf", meta.Ext)
	assert.NotEmpty(t, meta.StoredName)
	assert.NotEmpty(t, meta.UploadedAt)
}

func TestDataProcessing_GetMeta_AnyAuthUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, uploadDPDoc(t, e, admin, "consent.pdf", minimalPDF).Code)

	// Обычный пользователь видит метаданные (документ показывается при подаче заявки).
	user := testutil.RegisterAndLogin(t, e, "dp_reader", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/settings/data-processing/document/meta", testutil.AuthHeader(user))
	require.Equal(t, http.StatusOK, rec.Code)
	meta := testutil.ParseResponse[*models.DataProcessingDocument](t, rec)
	require.NotNil(t, meta)
	assert.Equal(t, "consent.pdf", meta.FileName)
}

func TestDataProcessing_GetMeta_Empty_ReturnsNull(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	user := testutil.RegisterAndLogin(t, e, "dp_empty", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/settings/data-processing/document/meta", testutil.AuthHeader(user))
	require.Equal(t, http.StatusOK, rec.Code)
	meta := testutil.ParseResponse[*models.DataProcessingDocument](t, rec)
	assert.Nil(t, meta, "без загруженного документа data должна быть null")
}

func TestDataProcessing_Serve_InlineAndDownload(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, uploadDPDoc(t, e, admin, "consent.pdf", minimalPDF).Code)

	user := testutil.RegisterAndLogin(t, e, "dp_viewer", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	// Просмотр: inline + реальное содержимое PDF.
	inline := testutil.GET(t, e, "/settings/data-processing/document", testutil.AuthHeader(user))
	require.Equal(t, http.StatusOK, inline.Code)
	assert.Contains(t, inline.Header().Get("Content-Disposition"), "inline")
	assert.Equal(t, "application/pdf", inline.Header().Get("Content-Type"))
	assert.True(t, bytes.HasPrefix(inline.Body.Bytes(), []byte("%PDF")), "тело должно быть отдан PDF")

	// Скачивание: attachment.
	dl := testutil.GET(t, e, "/settings/data-processing/document?download=1", testutil.AuthHeader(user))
	require.Equal(t, http.StatusOK, dl.Code)
	assert.Contains(t, dl.Header().Get("Content-Disposition"), "attachment")
}

func TestDataProcessing_Serve_Empty_404(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	user := testutil.RegisterAndLogin(t, e, "dp_none", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/settings/data-processing/document", testutil.AuthHeader(user))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDataProcessing_Delete_Admin_RemovesDoc(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, uploadDPDoc(t, e, admin, "consent.pdf", minimalPDF).Code)

	del := testutil.DELETE(t, e, "/settings/data-processing/document", testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, del.Code, del.Body.String())

	// После удаления meta пуста, файл не отдаётся.
	meta := testutil.ParseResponse[*models.DataProcessingDocument](t,
		testutil.GET(t, e, "/settings/data-processing/document/meta", testutil.AuthHeader(admin)))
	assert.Nil(t, meta)
	serve := testutil.GET(t, e, "/settings/data-processing/document", testutil.AuthHeader(admin))
	assert.Equal(t, http.StatusNotFound, serve.Code)
}

func TestDataProcessing_Upload_NonAdmin_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	user := testutil.RegisterAndLogin(t, e, "dp_nonadmin", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := uploadDPDoc(t, e, user, "consent.pdf", minimalPDF)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestDataProcessing_Delete_NonAdmin_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	user := testutil.RegisterAndLogin(t, e, "dp_nonadmin_del", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.DELETE(t, e, "/settings/data-processing/document", testutil.AuthHeader(user))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestDataProcessing_Upload_Xlsx_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := uploadDPDoc(t, e, admin, "Перечень данных.xlsx", minimalOOXML)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	meta := testutil.ParseResponse[*models.DataProcessingDocument](t, rec)
	require.NotNil(t, meta)
	assert.Equal(t, ".xlsx", meta.Ext)
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", meta.MimeType)
}

// Хранилище документов принимает и pptx, но документом согласия он быть не может:
// извлечения текста для него нет. Белый список хендлера уже хранилища.
func TestDataProcessing_Upload_Pptx_Rejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := uploadDPDoc(t, e, admin, "Презентация.pptx", minimalOOXML)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDataProcessing_Upload_InvalidExt(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := uploadDPDoc(t, e, admin, "evil.exe", []byte("MZ"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDataProcessing_Upload_WrongMagic(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// .pdf по имени, но содержимое не PDF.
	content := append([]byte{0x4D, 0x5A}, bytes.Repeat([]byte("x"), 100)...)
	rec := uploadDPDoc(t, e, admin, "fake.pdf", content)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
