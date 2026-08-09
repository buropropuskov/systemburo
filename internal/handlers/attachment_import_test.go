package handlers_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

const importTestPassword = "importpass_long_enough_for_login"

// importTemplate - тип вложения с активным шаблоном на диске: подпись колонки
// над списком (headerText) и сам список, начинающийся со startRow.
type importTemplate struct {
	uaID       int
	startRow   int
	listCol    string
	headerText string
}

// seedImportTemplate кладёт на диск .xlsx с заголовком списка и заводит под него
// активный шаблон с одной списочной привязкой - тот минимум, которого достаточно
// гейту файла (структура + отпечаток), без разметки заявочных полей.
func seedImportTemplate(t *testing.T, db *gorm.DB, name, attachmentType string, startRow int, listCol, headerText string) importTemplate {
	t.Helper()
	nm := name
	ua := models.UniqueAttachment{AttachmentType: attachmentType, Name: &nm, DisplayName: &nm, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	require.NoError(t, f.SetCellStr(sheet, fmt.Sprintf("%s%d", listCol, startRow-1), headerText))
	path := filepath.Join(t.TempDir(), name+".xlsx")
	require.NoError(t, f.SaveAs(path))
	require.NoError(t, f.Close())

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path,
		OriginalFileName: name + ".xlsx",
		ListStartRow:     startRow, ListEndRow: startRow + 20, MaxListRows: 20,
	}
	require.NoError(t, db.Create(&tpl).Error)

	listField := "employee.last_name"
	if attachmentType == "cars" {
		listField = "car.state_number"
	}
	require.NoError(t, db.Create(&models.AttachmentTemplateMapping{
		TemplateID: tpl.ID, CellRef: fmt.Sprintf("%s%d", listCol, startRow), FieldPath: listField, IsListField: true,
	}).Error)

	return importTemplate{uaID: ua.ID, startRow: startRow, listCol: listCol, headerText: headerText}
}

// buildImportUpload собирает байты «заполненного» бланка: тот же заголовок, что у
// шаблона, и rowsCount непустых строк списка. stampForUA > 0 - подмешать отпечаток
// другого типа вложения (тест на чужой бланк); 0 - без отпечатка вовсе (структура
// - единственная защита, как у бланка, пересобранного руками).
func buildImportUpload(t *testing.T, startRow int, listCol, headerText string, rowsCount, stampForUA int) []byte {
	t.Helper()
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	require.NoError(t, f.SetCellStr(sheet, fmt.Sprintf("%s%d", listCol, startRow-1), headerText))
	for i := 0; i < rowsCount; i++ {
		require.NoError(t, f.SetCellStr(sheet, fmt.Sprintf("%s%d", listCol, startRow+i), fmt.Sprintf("Строка %d", i+1)))
	}
	if stampForUA > 0 {
		require.NoError(t, services.StampBlankFingerprint(f, services.BlankFingerprint{
			UniqueAttachmentID: stampForUA, TemplateID: 1, ListStartRow: startRow,
		}))
	}
	var buf bytes.Buffer
	_, err := f.WriteTo(&buf)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return buf.Bytes()
}

// postImportFile шлёт multipart-запрос на приём бланка - handler ждёт файл в поле "file".
func postImportFile(t *testing.T, e *echo.Echo, uaID int, filename string, data []byte, token string) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/attachments/%d/import-list", uaID), body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// Гейт файла массового импорта (blank-import, C1C2): кривой бланк отлетает целиком,
// до разбора отдельных строк, понятным русским текстом.
func TestAttachmentImportList(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userTypeID := secUserTypeIDByCode(t, db, "user")
	plain := testutil.RegisterAndLogin(t, e, "importplain", importTestPassword, userTypeID, td.OrgID, 0)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, 0)

	tpl := seedImportTemplate(t, db, "import_people", "people", 6, "B", "Фамилия")
	other := seedImportTemplate(t, db, "import_other", "people", 5, "B", "Фамилия")

	t.Run("без права action.import.list - 403", func(t *testing.T) {
		data := buildImportUpload(t, tpl.startRow, tpl.listCol, tpl.headerText, 3, 0)
		rec := postImportFile(t, e, tpl.uaID, "list.xlsx", data, plain)
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("у админа корректный файл проходит", func(t *testing.T) {
		data := buildImportUpload(t, tpl.startRow, tpl.listCol, tpl.headerText, 3, 0)
		rec := postImportFile(t, e, tpl.uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Equal(t, 3, result.Summary.Read)
		require.Zero(t, result.Summary.Accepted, "построчный разбор - срез C3")
		require.Zero(t, result.Summary.Rejected)
		require.Empty(t, result.Rows)
	})

	t.Run("не xlsx отлетает", func(t *testing.T) {
		rec := postImportFile(t, e, tpl.uaID, "list.xlsx", []byte("это не excel файл, а обычный текст"), admin)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), "не похож на .xlsx")
	})

	t.Run("бланк другого типа вложения отлетает по отпечатку", func(t *testing.T) {
		data := buildImportUpload(t, tpl.startRow, tpl.listCol, tpl.headerText, 3, other.uaID)
		rec := postImportFile(t, e, tpl.uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), "бланк другого вида пропуска")
	})

	t.Run("изменённая структура колонок отлетает", func(t *testing.T) {
		data := buildImportUpload(t, tpl.startRow, tpl.listCol, "Совсем другая подпись", 3, 0)
		rec := postImportFile(t, e, tpl.uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), "колонки не на своих местах")
	})

	t.Run("пустой список отлетает", func(t *testing.T) {
		data := buildImportUpload(t, tpl.startRow, tpl.listCol, tpl.headerText, 0, 0)
		rec := postImportFile(t, e, tpl.uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), "нет ни одной заполненной строки")
	})

	t.Run("2001 строка отлетает потолком", func(t *testing.T) {
		data := buildImportUpload(t, tpl.startRow, tpl.listCol, tpl.headerText, 2001, 0)
		rec := postImportFile(t, e, tpl.uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), "2001 строк")
		require.Contains(t, rec.Body.String(), "максимум 2000")
	})

	t.Run("без авторизации - 401", func(t *testing.T) {
		data := buildImportUpload(t, tpl.startRow, tpl.listCol, tpl.headerText, 3, 0)
		rec := postImportFile(t, e, tpl.uaID, "list.xlsx", data, "")
		require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	})

	t.Run("скачивание пустого бланка гейтится тем же правом", func(t *testing.T) {
		rec := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/blank-template", tpl.uaID), testutil.AuthHeader(plain))
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

		rec = testutil.GET(t, e, fmt.Sprintf("/attachments/%d/blank-template", tpl.uaID), testutil.AuthHeader(admin))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})
}
