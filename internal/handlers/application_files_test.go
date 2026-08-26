package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// realPNG -- настоящий PNG. Заглушка из одной сигнатуры (pngBytes соседнего
// теста) до диска не доедет: конвейер файлов заявки перекодирует изображения.
func realPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 30), G: uint8(y * 30), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

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
	file := uploadDraftFile(t, e, token, "разрешение.png", realPNG(t))

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
	require.Positive(t, items[0].FileSize)

	dlRec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/files/%d", appID, file.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, dlRec.Code)
	require.Equal(t, "image/png", dlRec.Header().Get("Content-Type"))
	// Побайтового равенства с исходником нет: изображение перекодировано, вместе с
	// этим срезаются метаданные. Проверяем, что отдалась картинка тех же размеров.
	cfg, format, err := image.DecodeConfig(bytes.NewReader(dlRec.Body.Bytes()))
	require.NoError(t, err)
	require.Equal(t, "png", format)
	require.Equal(t, 8, cfg.Width)
}

// TestApplicationFiles_ForeignUserGetsNoAccess: у файлов нет своего права, доступ
// равен доступу к заявке. Посторонний не получает ни списка, ни файла.
func TestApplicationFiles_ForeignUserGetsNoAccess(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	ownerToken := testutil.RegisterAndLogin(t, e, "filesowner", "pass123", 1, td.OrgID, td.CompanyID)
	file := uploadDraftFile(t, e, ownerToken, "скан.png", realPNG(t))
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
	file := uploadDraftFile(t, e, ownerToken, "чужой.png", realPNG(t))

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
	svc := services.NewApplicationFileService(db, dir, services.NewAuditRecorder(db))
	require.NoError(t, os.MkdirAll(svc.Dir(), 0o755))
	write := func(name string) string {
		require.NoError(t, os.WriteFile(filepath.Join(svc.Dir(), name), []byte("payload"), 0o600))
		return name
	}

	ownerToken := testutil.RegisterAndLogin(t, e, "sweepowner", "pass123", 1, td.OrgID, td.CompanyID)
	attached := uploadDraftFile(t, e, ownerToken, "приложенный.png", realPNG(t))
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
	_, err = part.Write(realPNG(t))
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
	file := uploadDraftFile(t, e, ownerToken, "черновик.png", realPNG(t))

	rec := testutil.DELETE(t, e, fmt.Sprintf("/applications/files/%d", file.ID), testutil.AuthHeader(strangerToken))
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	rec = testutil.DELETE(t, e, fmt.Sprintf("/applications/files/%d", file.ID), testutil.AuthHeader(ownerToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var count int64
	require.NoError(t, db.Model(&models.ApplicationFile{}).Where("id = ?", file.ID).Count(&count).Error)
	require.Zero(t, count)
}

// TestApplicationFiles_StoredEncryptedAndServedDecrypted: на диске лежит шифротекст,
// а скачивание отдаёт исходный документ. Смысл в резервных копиях: ключ в них
// намеренно не попадает, поэтому украденная копия не должна выдавать вложения.
func TestApplicationFiles_StoredEncryptedAndServedDecrypted(t *testing.T) {
	e, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	crypto.SetGlobalKey(key)
	defer crypto.SetGlobalKey(nil)

	token := testutil.RegisterAndLogin(t, e, "cryptosender", "pass123", 1, td.OrgID, td.CompanyID)
	pdf := append([]byte("%PDF-1.4\n"), []byte("паспортные данные внутри документа")...)
	file := uploadDraftFile(t, e, token, "разрешение.pdf", pdf)

	rec := submitWithFiles(t, e, db, token, "crypto", []int{file.ID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	appID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID

	var stored models.ApplicationFile
	require.NoError(t, db.First(&stored, file.ID).Error)
	require.True(t, stored.Encrypted)

	onDisk, err := os.ReadFile(filepath.Join(uploadDir, "application_files", stored.StoredName))
	require.NoError(t, err)
	require.NotContains(t, string(onDisk), "паспортные данные", "содержимое не должно лежать на диске открытым")

	dl := testutil.GET(t, e, fmt.Sprintf("/applications/%d/files/%d", appID, file.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, dl.Code)
	require.Equal(t, pdf, dl.Body.Bytes(), "скачивание отдаёт исходный документ")
}

// TestApplicationFiles_ImageShrunkAndExifDropped: снимок с телефона ужимается, а
// EXIF с координатами съёмки до диска не доезжает.
func TestApplicationFiles_ImageShrunkAndExifDropped(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "exifsender", "pass123", 1, td.OrgID, td.CompanyID)

	big := image.NewRGBA(image.Rect(0, 0, 3000, 1500))
	for y := 0; y < 1500; y++ {
		for x := 0; x < 3000; x++ {
			big.Set(x, y, color.RGBA{R: uint8(x % 251), G: uint8(y % 241), B: uint8((x * y) % 233), A: 255})
		}
	}
	var raw bytes.Buffer
	require.NoError(t, jpeg.Encode(&raw, big, &jpeg.Options{Quality: 95}))

	marker := []byte("GPSLatitudeRef")
	payload := append([]byte("Exif\x00\x00"), marker...)
	segment := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte((len(payload) + 2) & 0xFF)}
	withExif := append([]byte{}, raw.Bytes()[:2]...)
	withExif = append(withExif, segment...)
	withExif = append(withExif, payload...)
	withExif = append(withExif, raw.Bytes()[2:]...)

	file := uploadDraftFile(t, e, token, "снимок.jpg", withExif)
	require.Less(t, file.FileSize, int64(len(withExif)), "ужатый снимок занимает меньше исходного")

	rec := submitWithFiles(t, e, db, token, "exif", []int{file.ID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	appID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID

	dl := testutil.GET(t, e, fmt.Sprintf("/applications/%d/files/%d", appID, file.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, dl.Code)
	require.NotContains(t, dl.Body.String(), string(marker), "метаданные снимка не должны сохраняться")

	cfg, _, err := image.DecodeConfig(bytes.NewReader(dl.Body.Bytes()))
	require.NoError(t, err)
	require.Equal(t, 2000, cfg.Width, "длинная сторона ограничена")
}

// TestApplicationFiles_DeletedByAdminOnly: состав заявки после подачи неизменен -
// приложенный файл не убирает даже её автор. Удаление оставлено администратору как
// способ вычистить приложенное вопреки подписи поля (скан паспорта).
func TestApplicationFiles_DeletedByAdminOnly(t *testing.T) {
	e, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	ownerToken := testutil.RegisterAndLogin(t, e, "delattached", "pass123", 1, td.OrgID, td.CompanyID)
	strangerToken := testutil.RegisterAndLogin(t, e, "delattachedbad", "pass123", 1, 0, 0)
	file := uploadDraftFile(t, e, ownerToken, "лишний.png", realPNG(t))

	rec := submitWithFiles(t, e, db, ownerToken, "delatt", []int{file.ID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	appID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID

	path := fmt.Sprintf("/applications/%d/files/%d", appID, file.ID)

	// Посторонний не видит заявку - отказ ещё на доступе к ней.
	r := testutil.DELETE(t, e, path, testutil.AuthHeader(strangerToken))
	require.Equal(t, http.StatusForbidden, r.Code, r.Body.String())

	// Автор заявку видит, но состав менять не вправе: права администрирования у него нет.
	r = testutil.DELETE(t, e, path, testutil.AuthHeader(ownerToken))
	require.Equal(t, http.StatusForbidden, r.Code, r.Body.String())

	var stored models.ApplicationFile
	require.NoError(t, db.First(&stored, file.ID).Error)
	onDisk := filepath.Join(uploadDir, "application_files", stored.StoredName)
	require.FileExists(t, onDisk)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	r = testutil.DELETE(t, e, path, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, r.Code, r.Body.String())

	var left int64
	require.NoError(t, db.Model(&models.ApplicationFile{}).Where("id = ?", file.ID).Count(&left).Error)
	require.Zero(t, left)
	require.NoFileExists(t, onDisk, "файл должен уходить и с диска, а не только из базы")

	// Удаление документа попадает в журнал: снятый файл - это изменение состава
	// заявки, по которому выдают пропуск.
	var audits int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appID, "file_delete").
		Count(&audits).Error)
	require.EqualValues(t, 1, audits)
}

// TestApplicationFiles_DiskOrphanSwept: файл, потерявший строку (каскад от удалённой
// заявки), убирается с диска; свежий файл и файл с записью остаются.
func TestApplicationFiles_DiskOrphanSwept(t *testing.T) {
	e, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "orphandisk", "pass123", 1, td.OrgID, td.CompanyID)
	kept := uploadDraftFile(t, e, token, "живой.png", realPNG(t))

	svc := services.NewApplicationFileService(db, uploadDir, services.NewAuditRecorder(db))
	dir := svc.Dir()

	orphan := filepath.Join(dir, "orphan.png")
	require.NoError(t, os.WriteFile(orphan, []byte("payload"), 0o600))
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(orphan, old, old))

	fresh := filepath.Join(dir, "fresh-orphan.png")
	require.NoError(t, os.WriteFile(fresh, []byte("payload"), 0o600))

	removed, err := svc.SweepDiskOrphans(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, removed)

	require.NoFileExists(t, orphan)
	require.FileExists(t, fresh, "свежий файл может быть загрузкой в процессе")

	var stored models.ApplicationFile
	require.NoError(t, db.First(&stored, kept.ID).Error)
	require.FileExists(t, filepath.Join(dir, stored.StoredName))
}

// TestApplicationFiles_ListingFlagsApplicationWithFiles: в списке Центра заявка с
// приложенными файлами помечается признаком - по нему рисуется скрепка в строке.
// Заявка без файлов признак не получает, иначе скрепка висела бы у всех.
func TestApplicationFiles_ListingFlagsApplicationWithFiles(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "listflag", "pass123", 1, td.OrgID, td.CompanyID)

	withFile := uploadDraftFile(t, e, token, "разрешение.png", realPNG(t))
	rec := submitWithFiles(t, e, db, token, "flagged", []int{withFile.ID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	flaggedID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID

	rec = submitWithFiles(t, e, db, token, "plain", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	plainID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID

	listRec := testutil.GET(t, e, "/applications?limit=50", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, listRec.Code, listRec.Body.String())

	var payload struct {
		Data []struct {
			ID       int  `json:"id"`
			HasFiles bool `json:"has_files"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &payload))

	flags := map[int]bool{}
	for _, row := range payload.Data {
		flags[row.ID] = row.HasFiles
	}
	require.True(t, flags[flaggedID], "заявка с файлом должна быть помечена")
	require.False(t, flags[plainID], "заявка без файлов пометки не получает")
}
