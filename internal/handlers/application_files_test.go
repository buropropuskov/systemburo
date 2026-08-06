package handlers_test

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// pngBytes с PNG-сигнатурой объявлен в unload_place_photo_test.go того же пакета.

// uploadDraftFile загружает файл к будущей заявке и возвращает его id.
func uploadDraftFile(t *testing.T, e *echo.Echo, token, name string, content []byte) models.ApplicationFileItem {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("files", name)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/applications/files", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	items := testutil.ParseResponse[[]models.ApplicationFileItem](t, rec)
	require.Len(t, items, 1)
	require.Equal(t, name, items[0].FileName)
	return items[0]
}

// submitWithFiles подаёт заявку с приложенными файлами и возвращает ответ как есть:
// часть тестов ждёт отказа, поэтому код статуса проверяет вызывающий.
func submitWithFiles(t *testing.T, e *echo.Echo, db *gorm.DB, token, suffix string, fileIDs []int) *httptest.ResponseRecorder {
	t.Helper()

	uaID := seedUniqueAttachment(t, db, "cars", "cars_files_"+suffix, "Cars Files "+suffix)
	ids := "["
	for i, id := range fileIDs {
		if i > 0 {
			ids += ","
		}
		ids += fmt.Sprintf("%d", id)
	}
	ids += "]"

	body := fmt.Sprintf(`{
		"message": "заявка с файлами",
		"organization": "Test Organization",
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"file_ids": %s,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "cars_template",
			"attachment_display_name": "Cars Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": { "vehicles": [{ "car_number": "A700AA777", "car_brand": "Toyota" }] }
		}]
	}`, ids, uaID)

	return testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
}

// TestApplicationFiles_AttachedAtSubmitAndServed: файл загружен до подачи, привязан
// подачей, виден в списке и скачивается тем же содержимым.
func TestApplicationFiles_AttachedAtSubmitAndServed(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "filesender", "pass123", 1, td.OrgID, td.CompanyID)
	file := uploadDraftFile(t, e, token, "разрешение.png", pngBytes)

	rec := submitWithFiles(t, e, db, token, "ok", []int{file.ID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	appID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID

	listRec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/files", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, listRec.Code, listRec.Body.String())
	items := testutil.ParseResponse[[]models.ApplicationFileItem](t, listRec)
	require.Len(t, items, 1)
	require.Equal(t, "разрешение.png", items[0].FileName)
	// Тип определён по magic bytes, а не взят из Content-Type формы.
	require.Equal(t, "image/png", items[0].MimeType)
	require.Equal(t, int64(len(pngBytes)), items[0].FileSize)

	dlRec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/files/%d", appID, file.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, dlRec.Code)
	require.Equal(t, pngBytes, dlRec.Body.Bytes())
	require.Equal(t, "image/png", dlRec.Header().Get("Content-Type"))
}

// TestApplicationFiles_ForeignUserGetsNoAccess: у файлов нет своего права, доступ
// равен доступу к заявке. Посторонний не получает ни списка, ни файла.
func TestApplicationFiles_ForeignUserGetsNoAccess(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	ownerToken := testutil.RegisterAndLogin(t, e, "filesowner", "pass123", 1, td.OrgID, td.CompanyID)
	file := uploadDraftFile(t, e, ownerToken, "скан.png", pngBytes)
	rec := submitWithFiles(t, e, db, ownerToken, "foreign", []int{file.ID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	appID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID

	strangerToken := testutil.RegisterAndLogin(t, e, "filesstranger", "pass123", 1, 0, 0)

	for _, path := range []string{
		fmt.Sprintf("/applications/%d/files", appID),
		fmt.Sprintf("/applications/%d/files/%d", appID, file.ID),
	} {
		r := testutil.GET(t, e, path, testutil.AuthHeader(strangerToken))
		require.Equal(t, http.StatusForbidden, r.Code, path+": "+r.Body.String())
	}
}

// TestApplicationFiles_ForeignDraftRejectsSubmit: чужой черновик нельзя приложить к
// своей заявке, и подача откатывается целиком - иначе заявитель получил бы заявку
// без документа, считая его приложенным.
func TestApplicationFiles_ForeignDraftRejectsSubmit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	ownerToken := testutil.RegisterAndLogin(t, e, "draftowner", "pass123", 1, td.OrgID, td.CompanyID)
	file := uploadDraftFile(t, e, ownerToken, "чужой.png", pngBytes)

	thiefToken := testutil.RegisterAndLogin(t, e, "draftthief", "pass123", 1, td.OrgID, td.CompanyID)
	rec := submitWithFiles(t, e, db, thiefToken, "thief", []int{file.ID})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	var apps int64
	require.NoError(t, db.Model(&models.Application{}).Count(&apps).Error)
	require.Zero(t, apps, "подача с чужим файлом не должна создавать заявку")

	// Файл остался черновиком владельца и не привязался ни к чему.
	var stored models.ApplicationFile
	require.NoError(t, db.First(&stored, file.ID).Error)
	require.Nil(t, stored.ApplicationID)
}

// TestApplicationFiles_SweepOrphansKeepsAttachedAndFresh: уборщик снимает только
// старые черновики. Привязанный к заявке файл и свежий черновик он трогать не смеет -
// иначе заявка теряет документ, а заполняемая форма теряет уже выбранное вложение.
func TestApplicationFiles_SweepOrphansKeepsAttachedAndFresh(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	dir := t.TempDir()
	svc := services.NewApplicationFileService(db, dir)
	require.NoError(t, os.MkdirAll(svc.Dir(), 0o755))
	write := func(name string) string {
		require.NoError(t, os.WriteFile(filepath.Join(svc.Dir(), name), []byte("payload"), 0o600))
		return name
	}

	ownerToken := testutil.RegisterAndLogin(t, e, "sweepowner", "pass123", 1, td.OrgID, td.CompanyID)
	attached := uploadDraftFile(t, e, ownerToken, "приложенный.png", pngBytes)
	rec := submitWithFiles(t, e, db, ownerToken, "sweep", []int{attached.ID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	old := time.Now().Add(-48 * time.Hour)
	rows := []models.ApplicationFile{
		{FileName: "старый.png", StoredName: write("old.png"), MimeType: "image/png", FileSize: 7, UploadedBy: 1, CreatedAt: old},
		{FileName: "свежий.png", StoredName: write("fresh.png"), MimeType: "image/png", FileSize: 7, UploadedBy: 1},
	}
	require.NoError(t, db.Create(&rows).Error)
	// Привязанному файлу возраст сироты не помогает: уборщик смотрит на application_id.
	require.NoError(t, db.Model(&models.ApplicationFile{}).Where("id = ?", attached.ID).
		Update("created_at", old).Error)

	removed, err := svc.SweepOrphans(context.Background(), 24*time.Hour)
	require.NoError(t, err)
	require.Equal(t, 1, removed)

	var left int64
	require.NoError(t, db.Model(&models.ApplicationFile{}).Count(&left).Error)
	require.EqualValues(t, 2, left, "остаются свежий черновик и приложенный к заявке файл")

	_, err = os.Stat(filepath.Join(svc.Dir(), "old.png"))
	require.True(t, os.IsNotExist(err), "файл старого черновика должен быть убран с диска")
	require.FileExists(t, filepath.Join(svc.Dir(), "fresh.png"))
}

// TestApplicationFiles_StoresDetectedMimeNotFormHeader: в базу идёт тип из magic
// bytes, а не Content-Type формы - его задаёт клиент, и text/html в нём превратил бы
// скачивание картинки в исполняемую страницу.
func TestApplicationFiles_StoresDetectedMimeNotFormHeader(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "mimesender", "pass123", 1, td.OrgID, td.CompanyID)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="files"; filename="фото.png"`)
	header.Set("Content-Type", "text/html")
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write(pngBytes)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/applications/files", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	items := testutil.ParseResponse[[]models.ApplicationFileItem](t, rec)
	require.Len(t, items, 1)
	require.Equal(t, "image/png", items[0].MimeType)
}

// TestApplicationFiles_DraftDeletedByOwnerOnly: черновик удаляет только тот, кто его
// загрузил.
func TestApplicationFiles_DraftDeletedByOwnerOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	ownerToken := testutil.RegisterAndLogin(t, e, "delowner", "pass123", 1, td.OrgID, td.CompanyID)
	strangerToken := testutil.RegisterAndLogin(t, e, "delstranger", "pass123", 1, td.OrgID, td.CompanyID)
	file := uploadDraftFile(t, e, ownerToken, "черновик.png", pngBytes)

	rec := testutil.DELETE(t, e, fmt.Sprintf("/applications/files/%d", file.ID), testutil.AuthHeader(strangerToken))
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	rec = testutil.DELETE(t, e, fmt.Sprintf("/applications/files/%d", file.ID), testutil.AuthHeader(ownerToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var count int64
	require.NoError(t, db.Model(&models.ApplicationFile{}).Where("id = ?", file.ID).Count(&count).Error)
	require.Zero(t, count)
}
