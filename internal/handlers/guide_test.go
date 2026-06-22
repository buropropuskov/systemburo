package handlers_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// guideStoredName читает stored_name раздела из БД (для проверки файла на диске).
func guideStoredName(t *testing.T, db *gorm.DB, role string) string {
	t.Helper()
	var sec models.GuideSection
	require.NoError(t, db.Where("role = ?", role).First(&sec).Error)
	return sec.StoredName
}

// uploadGuideFile грузит PDF раздела руководства multipart-запросом (PUT) от имени token.
func uploadGuideFile(t *testing.T, e *echo.Echo, token, role, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := buildMultipartDoc(t, filename, content, nil)
	req := httptest.NewRequest(http.MethodPut, "/api/guide/admin/sections/"+role+"/file", body)
	req.Header.Set("Content-Type", ct)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// Супер-админ видит все засеянные разделы руководства, items развёрнуты в массив,
// file == nil пока PDF не загружен.
func TestGuideSections_SuperAdminSeesAll(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	// Ключи раздела совпадают с каталогом прав (защита от молчаливого рассинхрона
	// permission_catalog.go). guide.guard в каталоге пока нет (заводит perm-gating).
	require.Equal(t, services.KeyGuideUser, services.GuideKeyForRole("user"))
	require.Equal(t, services.KeyGuideAdmin, services.GuideKeyForRole("admin"))

	token := testutil.RegisterAdmin(t, e, 0, 0)
	rec := testutil.GET(t, e, "/guide/sections", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	sections := testutil.ParseResponse[[]models.GuideSectionResponse](t, rec)
	require.Len(t, sections, 3)

	// Порядок по sort_order: user -> guard -> admin.
	assert.Equal(t, "user", sections[0].Role)
	assert.Equal(t, "guard", sections[1].Role)
	assert.Equal(t, "admin", sections[2].Role)

	assert.Equal(t, "Руководство пользователя", sections[0].Title)
	assert.NotEmpty(t, sections[0].Lead)
	require.NotEmpty(t, sections[0].Items, "items должны разворачиваться из jsonb в массив строк")
	assert.Nil(t, sections[0].File, "file == nil пока PDF не загружен")
}

// Обычный пользователь с правом только на guide.user видит лишь свой раздел;
// download гейтит та же проверка.
func TestGuideSections_GatedByPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	const username, password = "guideuser", "Password123!"
	testutil.RegisterUser(t, e, username, password, 1, 0, 0)

	var u models.User
	require.NoError(t, db.Where("username = ?", username).First(&u).Error)
	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID:        u.ID,
		PermissionKey: services.KeyGuideUser,
		Value:         "allow",
		GrantedAt:     time.Now().UTC(),
	}).Error)

	token, _ := testutil.LoginUser(t, e, username, password)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/guide/sections", h)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	sections := testutil.ParseResponse[[]models.GuideSectionResponse](t, rec)
	require.Len(t, sections, 1, "виден только раздел с выданным правом")
	assert.Equal(t, "user", sections[0].Role)

	// Разрешённый раздел без загруженного файла -> 404.
	rec = testutil.GET(t, e, "/guide/sections/user/download", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// Запрещённый раздел -> 403 (нет права guide.admin).
	rec = testutil.GET(t, e, "/guide/sections/admin/download", h)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// Неизвестная роль -> 404.
	rec = testutil.GET(t, e, "/guide/sections/bogus/download", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// minimalGuidePDF -- корректная PDF magic-сигнатура (%PDF) для тестов загрузки.
var minimalGuidePDF = []byte("%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF\n")

// Админ (page.admin) видит все 3 раздела через admin-листинг без фильтра по правам.
func TestGuide_AdminList_AllSections(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	token := testutil.RegisterAdmin(t, e, 0, 0)
	rec := testutil.GET(t, e, "/guide/admin/sections", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	sections := testutil.ParseResponse[[]models.GuideSectionResponse](t, rec)
	require.Len(t, sections, 3)
	assert.Equal(t, "user", sections[0].Role)
	assert.Equal(t, "guard", sections[1].Role)
	assert.Equal(t, "admin", sections[2].Role)
	assert.Nil(t, sections[0].File, "file == nil пока PDF не загружен")
}

// Правка lead+items раздела сохраняется и видна в последующем ответе; пустые элементы
// отбрасываются.
func TestGuide_AdminUpdateContent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	token := testutil.RegisterAdmin(t, e, 0, 0)
	h := testutil.AuthHeader(token)

	body := `{"lead":"Новый лид охранника","items":["Первый пункт","   ","Второй пункт"]}`
	rec := testutil.PUT(t, e, "/guide/admin/sections/guard", body, h)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	updated := testutil.ParseResponse[models.GuideSectionResponse](t, rec)
	assert.Equal(t, "guard", updated.Role)
	assert.Equal(t, "Новый лид охранника", updated.Lead)
	require.Equal(t, []string{"Первый пункт", "Второй пункт"}, updated.Items, "пустые элементы отброшены")

	// Повторное чтение из admin-листинга отражает правку.
	list := testutil.ParseResponse[[]models.GuideSectionResponse](t,
		testutil.GET(t, e, "/guide/admin/sections", h))
	require.Len(t, list, 3)
	assert.Equal(t, "Новый лид охранника", list[1].Lead)
}

// Невалидная роль раздела -> 404.
func TestGuide_AdminUpdateContent_InvalidRole(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	token := testutil.RegisterAdmin(t, e, 0, 0)
	rec := testutil.PUT(t, e, "/guide/admin/sections/bogus", `{"lead":"x","items":[]}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// Загрузка PDF: метаданные появляются в ответе и в admin-листинге, файл скачивается.
func TestGuide_AdminUploadFile_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	token := testutil.RegisterAdmin(t, e, 0, 0)
	h := testutil.AuthHeader(token)

	rec := uploadGuideFile(t, e, token, "user", "Инструкция.pdf", minimalGuidePDF)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	resp := testutil.ParseResponse[models.GuideSectionResponse](t, rec)
	require.NotNil(t, resp.File)
	assert.Equal(t, "Инструкция.pdf", resp.File.Name)
	assert.Equal(t, ".pdf", resp.File.Ext)
	assert.Equal(t, "application/pdf", resp.File.MimeType)
	assert.Greater(t, resp.File.Size, int64(0))
	assert.Equal(t, "/api/guide/sections/user/download", resp.File.DownloadURL)

	// Admin-листинг показывает загруженный файл.
	list := testutil.ParseResponse[[]models.GuideSectionResponse](t,
		testutil.GET(t, e, "/guide/admin/sections", h))
	require.NotNil(t, list[0].File)

	// Реальное скачивание отдаёт PDF.
	dl := testutil.GET(t, e, "/guide/sections/user/download", h)
	require.Equal(t, http.StatusOK, dl.Code)
	assert.True(t, bytes.HasPrefix(dl.Body.Bytes(), []byte("%PDF")), "тело -- PDF")
	assert.Contains(t, dl.Header().Get("Content-Disposition"), "attachment")
}

// Замена файла: повторная загрузка успешна, старый файл удаляется с диска,
// скачивание отдаёт новый.
func TestGuide_AdminUploadFile_Replace(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	token := testutil.RegisterAdmin(t, e, 0, 0)
	require.Equal(t, http.StatusOK, uploadGuideFile(t, e, token, "user", "v1.pdf", minimalGuidePDF).Code)
	oldStored := guideStoredName(t, db, "user")
	guideDir := filepath.Join("uploads", "guide")
	_, statErr := os.Stat(filepath.Join(guideDir, oldStored))
	require.NoError(t, statErr, "первый файл должен лежать на диске")

	rec := uploadGuideFile(t, e, token, "user", "v2.pdf", minimalGuidePDF)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	resp := testutil.ParseResponse[models.GuideSectionResponse](t, rec)
	require.NotNil(t, resp.File)
	assert.Equal(t, "v2.pdf", resp.File.Name)

	newStored := guideStoredName(t, db, "user")
	require.NotEqual(t, oldStored, newStored, "storedName должен смениться")
	_, statErr = os.Stat(filepath.Join(guideDir, oldStored))
	assert.True(t, os.IsNotExist(statErr), "старый файл удалён с диска (без orphan)")
	_, statErr = os.Stat(filepath.Join(guideDir, newStored))
	assert.NoError(t, statErr, "новый файл лежит на диске")

	dl := testutil.GET(t, e, "/guide/sections/user/download", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, dl.Code)
}

// Не-PDF расширение -> 400 (guide принимает только PDF).
func TestGuide_AdminUploadFile_NonPDFExt(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	token := testutil.RegisterAdmin(t, e, 0, 0)
	rec := uploadGuideFile(t, e, token, "user", "doc.docx", []byte("PK\x03\x04zzz"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// .pdf по имени, но содержимое не PDF -> 400 (magic-проверка DocumentFileService).
func TestGuide_AdminUploadFile_WrongMagic(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	token := testutil.RegisterAdmin(t, e, 0, 0)
	content := append([]byte{0x4D, 0x5A}, bytes.Repeat([]byte("x"), 100)...)
	rec := uploadGuideFile(t, e, token, "user", "fake.pdf", content)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// Удаление файла очищает метаданные; скачивание после удаления -> 404.
func TestGuide_AdminDeleteFile(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	token := testutil.RegisterAdmin(t, e, 0, 0)
	h := testutil.AuthHeader(token)
	require.Equal(t, http.StatusOK, uploadGuideFile(t, e, token, "user", "g.pdf", minimalGuidePDF).Code)

	del := testutil.DELETE(t, e, "/guide/admin/sections/user/file", h)
	require.Equal(t, http.StatusOK, del.Code, "body: %s", del.Body.String())
	resp := testutil.ParseResponse[models.GuideSectionResponse](t, del)
	assert.Nil(t, resp.File, "file == nil после удаления")

	dl := testutil.GET(t, e, "/guide/sections/user/download", h)
	assert.Equal(t, http.StatusNotFound, dl.Code)
}

// Не-админ получает 403 на всех admin-эндпоинтах руководства (гейт page.admin).
func TestGuide_Admin_NonAdmin_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	user := testutil.RegisterAndLogin(t, e, "guide_nonadmin", "password123456789012345678901234", 1, 0, 0)
	h := testutil.AuthHeader(user)

	assert.Equal(t, http.StatusForbidden, testutil.GET(t, e, "/guide/admin/sections", h).Code)
	assert.Equal(t, http.StatusForbidden, testutil.PUT(t, e, "/guide/admin/sections/user", `{"lead":"x","items":[]}`, h).Code)
	assert.Equal(t, http.StatusForbidden, uploadGuideFile(t, e, user, "user", "g.pdf", minimalGuidePDF).Code)
	assert.Equal(t, http.StatusForbidden, testutil.DELETE(t, e, "/guide/admin/sections/user/file", h).Code)
}

// Без токена admin-эндпоинты руководства отвечают 401.
func TestGuide_Admin_Unauthenticated(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	assert.Equal(t, http.StatusUnauthorized, testutil.GET(t, e, "/guide/admin/sections", nil).Code)
	assert.Equal(t, http.StatusUnauthorized, testutil.PUT(t, e, "/guide/admin/sections/user", `{"lead":"x","items":[]}`, nil).Code)
	assert.Equal(t, http.StatusUnauthorized, uploadGuideFile(t, e, "", "user", "g.pdf", minimalGuidePDF).Code)
	assert.Equal(t, http.StatusUnauthorized, testutil.DELETE(t, e, "/guide/admin/sections/user/file", nil).Code)
}
