package handlers_test

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// Пустой бланк для заполнения списка участников: тот же файл, которым система
// заполняет бланки, плюс отпечаток - по нему загруженный обратно файл узнаётся как
// бланк именно этого типа вложения, а не «какой-то xlsx».
//
// Гейт скачивания - action.import.list (blank-import, C1C2), то же право, что и у
// приёма заполненного файла: скачавший без права загрузить не смог бы. Сценарии
// ниже проверяются под admin (право есть через adminAll); отдельный подтест
// проверяет отказ обычному пользователю.
func blankTemplateDownloadSection(t *testing.T, w blankWorld) {
	db := w.h.w.db
	admin := testutil.AuthHeader(w.h.adminToken)

	peopleUA, peopleTpl := seedListTemplate(t, db, "import_people", "people", 6, 20)
	carsUA, carsTpl := seedListTemplate(t, db, "import_cars", "cars", 4, 30)

	t.Run("без права action.import.list - 403", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, fmt.Sprintf("/attachments/%d/blank-template", peopleUA),
			testutil.AuthHeader(w.h.userToken))
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("отпечаток читается обратно", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, fmt.Sprintf("/attachments/%d/blank-template", peopleUA), admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Contains(t, rec.Header().Get("Content-Disposition"), "filename*=UTF-8''",
			"кириллица в имени файла требует RFC 5987")

		fp := readFingerprint(t, rec.Body.Bytes())
		require.Equal(t, services.BlankFingerprint{
			UniqueAttachmentID: peopleUA, TemplateID: peopleTpl, ListStartRow: 6,
		}, fp)
	})

	t.Run("бланк другого типа вложения не совпадает", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, fmt.Sprintf("/attachments/%d/blank-template", carsUA), admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		fp := readFingerprint(t, rec.Body.Bytes())
		require.Equal(t, services.BlankFingerprint{
			UniqueAttachmentID: carsUA, TemplateID: carsTpl, ListStartRow: 4,
		}, fp)
		require.NotEqual(t, peopleUA, fp.UniqueAttachmentID,
			"бланк машин обязан отличаться от бланка людей отпечатком")
	})

	t.Run("тип вложения без активного шаблона", func(t *testing.T) {
		uaID, tplID := seedListTemplate(t, db, "import_no_active", "people", 5, 9)
		require.NoError(t, db.Model(&models.AttachmentTemplate{}).Where("id = ?", tplID).
			Update("is_active", false).Error)

		rec := testutil.GET(t, w.h.e, fmt.Sprintf("/attachments/%d/blank-template", uaID), admin)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	})

	t.Run("шаблон без разметки списка", func(t *testing.T) {
		uaID, tplID := copySeedTemplate(t, db, "import_no_list", "people", 5, 9)
		require.NoError(t, db.Create(&models.AttachmentTemplateMapping{
			TemplateID: tplID, CellRef: "A1", FieldPath: "application.application_number",
		}).Error)

		rec := testutil.GET(t, w.h.e, fmt.Sprintf("/attachments/%d/blank-template", uaID), admin)
		require.Equal(t, http.StatusNotFound, rec.Code, "заполнять в таком бланке нечего")
	})

	t.Run("архивный тип вложения", func(t *testing.T) {
		uaID, _ := seedListTemplate(t, db, "import_archived", "people", 5, 9)
		require.NoError(t, db.Model(&models.UniqueAttachment{}).Where("id = ?", uaID).
			Update("is_active", false).Error)

		rec := testutil.GET(t, w.h.e, fmt.Sprintf("/attachments/%d/blank-template", uaID), admin)
		require.Equal(t, http.StatusNotFound, rec.Code, "по архивному типу заявку не подать")
	})

	t.Run("несуществующий тип вложения", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, "/attachments/999999/blank-template", admin)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	})

	t.Run("без авторизации", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, fmt.Sprintf("/attachments/%d/blank-template", peopleUA), nil)
		require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	})
}

// seedListTemplate - тип вложения с активным шаблоном и размеченной списочной частью.
func seedListTemplate(t *testing.T, db *gorm.DB, name, attachmentType string, start, end int) (int, int) {
	t.Helper()
	uaID, tplID := copySeedTemplate(t, db, name, attachmentType, start, end)
	listField := "employee.last_name"
	if attachmentType == "cars" {
		listField = "car.state_number"
	}
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tplID, CellRef: "A1", FieldPath: "application.application_number"},
		{TemplateID: tplID, CellRef: fmt.Sprintf("B%d", start), FieldPath: listField, IsListField: true},
	}).Error)
	return uaID, tplID
}

// readFingerprint открывает отданный файл как .xlsx и достаёт отпечаток - то же
// самое, что будет делать разбор загруженного бланка.
func readFingerprint(t *testing.T, body []byte) services.BlankFingerprint {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err, "ответ не открывается как .xlsx")
	defer func() { require.NoError(t, f.Close()) }()

	fp, ok := services.ReadBlankFingerprint(f)
	require.True(t, ok, "в выданном бланке нет отпечатка")
	return fp
}
