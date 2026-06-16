package handlers_test

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/config"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Document Groups ---

func TestDocumentGroups_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/document-groups", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDocumentGroups_CRUD(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Пустой список
	rec := testutil.GET(t, e, "/document-groups", h)
	require.Equal(t, http.StatusOK, rec.Code)
	list := testutil.ParseSlice(t, rec)
	assert.Empty(t, list)

	// Создание группы
	rec = testutil.POST(t, e, "/document-groups", `{"name":"Приказы"}`, h)
	require.Equal(t, http.StatusCreated, rec.Code)
	data := testutil.ParseMap(t, rec)
	groupID := int(data["id"].(float64))
	assert.Equal(t, "Приказы", data["name"])

	// Список после создания
	rec = testutil.GET(t, e, "/document-groups", h)
	require.Equal(t, http.StatusOK, rec.Code)
	list = testutil.ParseSlice(t, rec)
	require.Len(t, list, 1)
	assert.Equal(t, "Приказы", list[0]["name"])
	// count должен быть 0 (нет документов)
	assert.EqualValues(t, 0, list[0]["count"])

	// Дубль -- конфликт (без учёта регистра)
	rec = testutil.POST(t, e, "/document-groups", `{"name":"ПРИКАЗЫ"}`, h)
	assert.Equal(t, http.StatusConflict, rec.Code)

	// Переименование
	rec = testutil.PUT(t, e, fmt.Sprintf("/document-groups/%d", groupID), `{"name":"Распоряжения"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	data = testutil.ParseMap(t, rec)
	assert.Equal(t, "Распоряжения", data["name"])

	// Создаём вторую группу
	rec = testutil.POST(t, e, "/document-groups", `{"name":"Инструкции"}`, h)
	require.Equal(t, http.StatusCreated, rec.Code)
	data2 := testutil.ParseMap(t, rec)
	groupID2 := int(data2["id"].(float64))

	// Переупорядочивание
	reorderBody := fmt.Sprintf(`{"ids":[%d,%d]}`, groupID2, groupID)
	rec = testutil.PUT(t, e, "/document-groups/reorder", reorderBody, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Удаление группы
	rec = testutil.DELETE(t, e, fmt.Sprintf("/document-groups/%d", groupID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Список после удаления -- только вторая группа
	rec = testutil.GET(t, e, "/document-groups", h)
	require.Equal(t, http.StatusOK, rec.Code)
	list = testutil.ParseSlice(t, rec)
	require.Len(t, list, 1)
	assert.EqualValues(t, groupID2, list[0]["id"])
}

func TestDocumentGroups_Delete_MovesDocsToOther(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Создаём группу
	rec := testutil.POST(t, e, "/document-groups", `{"name":"Группа для теста"}`, h)
	require.Equal(t, http.StatusCreated, rec.Code)
	gData := testutil.ParseMap(t, rec)
	groupID := int(gData["id"].(float64))

	// Вставляем документ напрямую в БД
	doc := models.Document{
		GroupID:    &groupID,
		Title:      "Тестовый документ",
		FileName:   "test.pdf",
		StoredName: "test-uuid.pdf",
		FileExt:    ".pdf",
		MimeType:   "application/pdf",
		FileSize:   1024,
		IsVisible:  true,
	}
	require.NoError(t, db.Create(&doc).Error)

	// Удаляем группу
	rec = testutil.DELETE(t, e, fmt.Sprintf("/document-groups/%d", groupID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Документ должен быть в «Прочее» (group_id = NULL)
	var updated models.Document
	require.NoError(t, db.First(&updated, doc.ID).Error)
	assert.Nil(t, updated.GroupID, "ожидается group_id = NULL после удаления группы")
}

// --- Public endpoint ---

func TestDocuments_GetPublic_OnlyVisible(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	visible := models.Document{
		Title: "Видимый", FileName: "v.pdf", StoredName: "v.pdf",
		FileExt: ".pdf", MimeType: "application/pdf", FileSize: 100, IsVisible: true,
	}
	hidden := models.Document{
		Title: "Скрытый", FileName: "h.pdf", StoredName: "h.pdf",
		FileExt: ".pdf", MimeType: "application/pdf", FileSize: 100, IsVisible: false,
	}
	require.NoError(t, db.Create(&visible).Error)
	require.NoError(t, db.Create(&hidden).Error)

	token := testutil.RegisterAndLogin(t, e, "pubuser1", "password123!ABCabc", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/public/documents", h)
	require.Equal(t, http.StatusOK, rec.Code)

	groups := testutil.ParseSlice(t, rec)
	// Одна виртуальная группа «Прочее» с одним видимым документом
	require.Len(t, groups, 1)
	docsRaw := groups[0]["documents"].([]interface{})
	require.Len(t, docsRaw, 1)
	doc := docsRaw[0].(map[string]interface{})
	assert.Equal(t, "Видимый", doc["title"])
}

func TestDocuments_GetPublic_Grouped(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	group := models.DocumentGroup{Name: "Приказы", SortOrder: 0}
	require.NoError(t, db.Create(&group).Error)

	docInGroup := models.Document{
		GroupID: &group.ID, Title: "В группе", FileName: "g.pdf",
		StoredName: "g.pdf", FileExt: ".pdf", MimeType: "application/pdf",
		FileSize: 100, IsVisible: true, SortOrder: 0,
	}
	docNoGroup := models.Document{
		Title: "Прочее", FileName: "o.pdf",
		StoredName: "o.pdf", FileExt: ".pdf", MimeType: "application/pdf",
		FileSize: 100, IsVisible: true, SortOrder: 0,
	}
	require.NoError(t, db.Create(&docInGroup).Error)
	require.NoError(t, db.Create(&docNoGroup).Error)

	token := testutil.RegisterAndLogin(t, e, "pubuser2", "password123!ABCabc", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/public/documents", h)
	require.Equal(t, http.StatusOK, rec.Code)

	groups := testutil.ParseSlice(t, rec)
	// 2 группы: «Приказы» и «Прочее»
	require.Len(t, groups, 2)
	assert.Equal(t, "Приказы", groups[0]["name"])
	assert.Equal(t, "Прочее", groups[1]["name"])
}

func TestDocuments_GetPublic_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/public/documents", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDocuments_GetPublic_NoOther(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	group := models.DocumentGroup{Name: "Группа", SortOrder: 0}
	require.NoError(t, db.Create(&group).Error)
	doc := models.Document{
		GroupID: &group.ID, Title: "Документ", FileName: "d.pdf",
		StoredName: "d.pdf", FileExt: ".pdf", MimeType: "application/pdf",
		FileSize: 100, IsVisible: true,
	}
	require.NoError(t, db.Create(&doc).Error)

	token := testutil.RegisterAndLogin(t, e, "pubuser3", "password123!ABCabc", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/public/documents", h)
	require.Equal(t, http.StatusOK, rec.Code)

	groups := testutil.ParseSlice(t, rec)
	require.Len(t, groups, 1)
	assert.Equal(t, "Группа", groups[0]["name"])
	// Нет группы «Прочее» когда все документы в группах
}

// --- Download ---

func TestDocuments_Download_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "dluser", "password123!ABCabc", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/documents/99999/download", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- Upload validation ---

func TestDocuments_Upload_InvalidExt(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	body, ct := buildMultipartDoc(t, "test.exe", []byte("MZ"), map[string]string{"title": "Тест"})
	req := httptest.NewRequest(http.MethodPost, "/api/documents", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDocuments_Upload_WrongMagic(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// .pdf расширение, но содержимое не PDF (EXE magic bytes)
	content := append([]byte{0x4D, 0x5A}, bytes.Repeat([]byte("x"), 100)...)
	body, ct := buildMultipartDoc(t, "fake.pdf", content, map[string]string{"title": "Тест"})
	req := httptest.NewRequest(http.MethodPost, "/api/documents", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Admin-only access ---

func TestDocuments_List_AdminOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "regularuser4docs", "password123!ABCabc", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/documents", h)
	// 403 из-за page.admin permission
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// --- Reorder (service-level) ---

func TestDocuments_Reorder(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	d1 := models.Document{
		Title: "A", FileName: "a.pdf", StoredName: "a.pdf",
		FileExt: ".pdf", MimeType: "application/pdf", SortOrder: 0, IsVisible: true,
	}
	d2 := models.Document{
		Title: "B", FileName: "b.pdf", StoredName: "b.pdf",
		FileExt: ".pdf", MimeType: "application/pdf", SortOrder: 1, IsVisible: true,
	}
	require.NoError(t, db.Create(&d1).Error)
	require.NoError(t, db.Create(&d2).Error)

	ctx := context.Background()
	fileSvc := services.NewDocumentFileService(t.TempDir())
	settingsSvc := services.NewSettingsService(db, &config.Config{
		UploadMaxFileSize: 10 * 1024 * 1024,
	})
	svc := services.NewDocumentService(db, fileSvc, settingsSvc)

	err := svc.Reorder(ctx, models.ReorderDocumentsRequest{IDs: []int{d2.ID, d1.ID}})
	require.NoError(t, err)

	var updated1, updated2 models.Document
	require.NoError(t, db.First(&updated1, d1.ID).Error)
	require.NoError(t, db.First(&updated2, d2.ID).Error)
	// d2 -> sort_order=0, d1 -> sort_order=1
	assert.Equal(t, 0, updated2.SortOrder)
	assert.Equal(t, 1, updated1.SortOrder)
}

// buildMultipartDoc создаёт multipart-тело с файлом и полями формы.
func buildMultipartDoc(t *testing.T, filename string, fileContent []byte, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = fw.Write(fileContent)
	require.NoError(t, err)
	for k, v := range fields {
		require.NoError(t, w.WriteField(k, v))
	}
	w.Close()
	return &buf, w.FormDataContentType()
}
