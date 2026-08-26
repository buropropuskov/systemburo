package handlers_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

// pngBytes -- минимальный контент с PNG-сигнатурой, чтобы http.DetectContentType
// определил image/png (валидация типа по magic bytes).
var pngBytes = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0, 0, 0, 0, 0}

func multipartPhoto(t *testing.T, field, filename string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, err = fw.Write(pngBytes)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	return &buf, mw.FormDataContentType()
}

// TestUnloadPlaces_UploadPhoto_Pipeline проверяет, что загрузка через поле photos
// (то, что реально шлёт фронт) сохраняет фото, отдаёт URL /api/uploads/... и что
// статика реально раздаёт файл. Ловит мисматч имени поля и неработающую раздачу
// статики -- два корня бага "успешно загружено, но ничего нет".
func TestUnloadPlaces_UploadPhoto_Pipeline(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/unload-places", `{"name":"Photo Pipeline"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	placeID := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Загрузка с правильным полем photos -> 200, фото привязано.
	body, ctype := multipartPhoto(t, "photos", "test.png")
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/unload-places/%d/photos", placeID), body)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("Authorization", "Bearer "+token)
	up := httptest.NewRecorder()
	e.ServeHTTP(up, req)
	require.Equal(t, http.StatusOK, up.Code, up.Body.String())

	// Фото видно в деталях и URL ведёт на /api/uploads/...
	rec = testutil.GET(t, e, fmt.Sprintf("/unload-places/%d", placeID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	details := testutil.ParseMap(t, rec)
	photos, ok := details["photos"].([]interface{})
	require.True(t, ok, "photos должно быть массивом")
	require.Len(t, photos, 1, "одно фото должно сохраниться")
	photoURL, _ := photos[0].(map[string]interface{})["photo_url"].(string)
	require.Contains(t, photoURL, "/api/uploads/unload_places/")

	// Без пропуска статика молчит: до #2133 файл забирал любой, кто знает адрес.
	anon := httptest.NewRecorder()
	e.ServeHTTP(anon, httptest.NewRequest(http.MethodGet, photoURL, nil))
	require.Equal(t, http.StatusUnauthorized, anon.Code, "статика /api/uploads не должна отвечать без входа в систему")

	// Статика реально отдаёт загруженный файл (без этого <img> ловит 404).
	sreq := httptest.NewRequest(http.MethodGet, photoURL, nil)
	sreq.Header.Set("Authorization", "Bearer "+token)
	srec := httptest.NewRecorder()
	e.ServeHTTP(srec, sreq)
	require.Equal(t, http.StatusOK, srec.Code, "статика /api/uploads должна раздавать файл")

	// Неверное имя поля (старый баг: фронт photos, бэк file) -> 400, не ложный success.
	body2, ctype2 := multipartPhoto(t, "file", "wrong.png")
	req2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/unload-places/%d/photos", placeID), body2)
	req2.Header.Set("Content-Type", ctype2)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusBadRequest, rec2.Code, "пустое поле photos должно давать 400, не ложный успех")
}

// TestSystemTables_UploadPhoto_Pipeline -- та же проверка для системных таблиц.
func TestSystemTables_UploadPhoto_Pipeline(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables", `{"name":"kpp_test","display_name":"КПП тест","table_type":"passage"}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	body, ctype := multipartPhoto(t, "photos", "kpp.png")
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/system-tables/%d/photos", tableID), body)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("Authorization", "Bearer "+token)
	up := httptest.NewRecorder()
	e.ServeHTTP(up, req)
	require.Equal(t, http.StatusOK, up.Code, up.Body.String())

	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	details := testutil.ParseMap(t, rec)
	photos, ok := details["photos"].([]interface{})
	require.True(t, ok)
	require.Len(t, photos, 1)
	photoURL, _ := photos[0].(map[string]interface{})["photo_url"].(string)
	require.Contains(t, photoURL, "/api/uploads/system_tables/")

	anon := httptest.NewRecorder()
	e.ServeHTTP(anon, httptest.NewRequest(http.MethodGet, photoURL, nil))
	require.Equal(t, http.StatusUnauthorized, anon.Code, "статика /api/uploads не должна отвечать без входа в систему")

	sreq := httptest.NewRequest(http.MethodGet, photoURL, nil)
	sreq.Header.Set("Authorization", "Bearer "+token)
	srec := httptest.NewRecorder()
	e.ServeHTTP(srec, sreq)
	require.Equal(t, http.StatusOK, srec.Code)
}
