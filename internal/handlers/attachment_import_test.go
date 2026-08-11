package handlers_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

// templateIDOf - id активного шаблона вложения: нужен, чтобы дописать шаблону
// ещё одну списочную колонку уже после сидирования.
func templateIDOf(t *testing.T, db *gorm.DB, uaID int) int {
	t.Helper()
	var tpl models.AttachmentTemplate
	require.NoError(t, db.Where("unique_attachment_id = ? AND is_active = ?", uaID, true).First(&tpl).Error)
	return tpl.ID
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

	t.Run("у админа структурно корректный файл проходит гейт и разбирается построчно", func(t *testing.T) {
		// Шаблон мапит только колонку "Фамилия" - остальные обязательные поля
		// сотрудника (имя, гражданство, паспорт, должность) в файле не заполнены,
		// поэтому гейт файла пропускает загрузку, а построчный разбор (срез C3)
		// закономерно отклоняет все строки: 207, не 200 - "не мягче ручного ввода".
		data := buildImportUpload(t, tpl.startRow, tpl.listCol, tpl.headerText, 3, 0)
		rec := postImportFile(t, e, tpl.uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Equal(t, 3, result.Summary.Read)
		require.Zero(t, result.Summary.Accepted)
		require.Equal(t, 3, result.Summary.Rejected)
		require.Len(t, result.Rows, 3)
		for i, row := range result.Rows {
			require.Equal(t, tpl.startRow+i, row.RowNumber)
			require.NotNil(t, row.Employee)
			require.NotEmpty(t, row.Errors)
		}
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

	// Система выдаёт бланк с заранее пронумерованными строками (колонка «№ п/п»
	// размечена служебным полем row_number). Если считать такую строку
	// заполненной, скачанный и не тронутый бланк вернётся списком строк с
	// ошибками вместо честного отказа.
	t.Run("бланк с одной лишь нумерацией строк считается пустым", func(t *testing.T) {
		numbered := seedImportTemplate(t, db, "import_numbered", "people", 6, "B", "Фамилия")
		require.NoError(t, db.Create(&models.AttachmentTemplateMapping{
			TemplateID: templateIDOf(t, db, numbered.uaID), CellRef: "A6",
			FieldPath: "employee.row_number", IsListField: true,
		}).Error)

		f := excelize.NewFile()
		sheet := f.GetSheetName(0)
		require.NoError(t, f.SetCellStr(sheet, "B5", "Фамилия"))
		for i := 0; i < 15; i++ {
			require.NoError(t, f.SetCellStr(sheet, fmt.Sprintf("A%d", 6+i), fmt.Sprintf("%d", i+1)))
		}
		var buf bytes.Buffer
		_, err := f.WriteTo(&buf)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		rec := postImportFile(t, e, numbered.uaID, "list.xlsx", buf.Bytes(), admin)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), "нет ни одной заполненной строки")
	})

	// Ниже списка в бланке стоят подписи ("(контактный телефон)", "(дд.мм.гггг)"),
	// и попадают они как раз в колонку мест разгрузки. Мест в файле нет вовсе -
	// они задаются на сайте, - поэтому оформительская строка не должна выглядеть
	// участником.
	t.Run("подписи бланка ниже списка не считаются строками", func(t *testing.T) {
		signed := seedImportTemplate(t, db, "import_signed", "cars", 6, "B", "Номер ТС")
		require.NoError(t, db.Create(&models.AttachmentTemplateMapping{
			TemplateID: templateIDOf(t, db, signed.uaID), CellRef: "G6",
			FieldPath: "car.unload_places", IsListField: true,
		}).Error)

		f := excelize.NewFile()
		sheet := f.GetSheetName(0)
		require.NoError(t, f.SetCellStr(sheet, "B5", "Номер ТС"))
		require.NoError(t, f.SetCellStr(sheet, "G25", "(дд.мм.гггг)"))
		var buf bytes.Buffer
		_, err := f.WriteTo(&buf)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		rec := postImportFile(t, e, signed.uaID, "list.xlsx", buf.Bytes(), admin)
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

// importPersonRow - одна строка человека для построчных тестов разбора (C3). Пустое
// поле не пишется в ячейку вовсе - так же, как незаполненный столбец бланка.
// fullName - альтернатива last/first/middle: склеенное ФИО в одной колонке.
type importPersonRow struct {
	last, first, middle string
	citizenship         string
	passport            string
	patent              string
	permission          string
	position            string
	fullName            string
}

// importPeopleColumns - привязка полей сотрудника к колонкам общего шаблона
// построчных тестов (C3): каждое базовое поле в своей колонке, плюс J под склеенное
// employee.full_name.
var importPeopleColumns = map[string]string{
	"last_name": "B", "first_name": "C", "middle_name": "D", "citizenship": "E",
	"passport_series_number": "F", "patent_number": "G", "other_permission": "H",
	"position": "I", "full_name": "J",
}

var importPeopleHeaders = map[string]string{
	"B": "Фамилия", "C": "Имя", "D": "Отчество", "E": "Гражданство",
	"F": "Паспорт", "G": "Патент", "H": "Иное разрешение", "I": "Должность", "J": "ФИО",
}

// seedPeopleFieldsTemplate заводит тип вложения "people" с активным шаблоном, где
// КАЖДОЕ базовое поле сотрудника привязано к своей колонке (importPeopleColumns) - тот
// минимум, которого достаточно построчному разбору (срез C3), чтобы проверить
// обязательность/гражданство/патент/ФИО в комплексе, а не по одному полю.
func seedPeopleFieldsTemplate(t *testing.T, db *gorm.DB, name string, startRow int) int {
	t.Helper()
	nm := name
	ua := models.UniqueAttachment{AttachmentType: "people", Name: &nm, DisplayName: &nm, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	for col, text := range importPeopleHeaders {
		require.NoError(t, f.SetCellStr(sheet, fmt.Sprintf("%s%d", col, startRow-1), text))
	}
	path := filepath.Join(t.TempDir(), name+".xlsx")
	require.NoError(t, f.SaveAs(path))
	require.NoError(t, f.Close())

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path,
		OriginalFileName: name + ".xlsx",
		ListStartRow:     startRow, ListEndRow: startRow + 50, MaxListRows: 50,
	}
	require.NoError(t, db.Create(&tpl).Error)
	for field, col := range importPeopleColumns {
		require.NoError(t, db.Create(&models.AttachmentTemplateMapping{
			TemplateID: tpl.ID, CellRef: fmt.Sprintf("%s%d", col, startRow), FieldPath: "employee." + field, IsListField: true,
		}).Error)
	}
	return ua.ID
}

// buildPeopleRowsUpload собирает байты "заполненного" бланка с несколькими строками
// сотрудников по importPeopleColumns - тот же заголовок, что у шаблона (checkStructure
// сверяет их побайтово).
func buildPeopleRowsUpload(t *testing.T, startRow int, rows []importPersonRow) []byte {
	t.Helper()
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	for col, text := range importPeopleHeaders {
		require.NoError(t, f.SetCellStr(sheet, fmt.Sprintf("%s%d", col, startRow-1), text))
	}
	set := func(row int, col, val string) {
		if val == "" {
			return
		}
		require.NoError(t, f.SetCellStr(sheet, fmt.Sprintf("%s%d", col, row), val))
	}
	for i, r := range rows {
		row := startRow + i
		set(row, "B", r.last)
		set(row, "C", r.first)
		set(row, "D", r.middle)
		set(row, "E", r.citizenship)
		set(row, "F", r.passport)
		set(row, "G", r.patent)
		set(row, "H", r.permission)
		set(row, "I", r.position)
		set(row, "J", r.fullName)
	}
	var buf bytes.Buffer
	_, err := f.WriteTo(&buf)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return buf.Bytes()
}

// Построчный разбор и валидация загруженного бланка теми же правилами, что форма
// подачи (blank-import, срез C3): обязательность через MergeFieldConfig, патент по
// гражданству, длины полей под схему, омоглифы в ФИО, склеенное ФИО, дубли внутри
// файла, чёрный список пакетно, частичный успех.
func TestAttachmentImportListRows(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, 0)

	uaID := seedPeopleFieldsTemplate(t, db, "import_rows_people", 6)

	russia := models.Citizenship{Name: "Россия", IsActive: true, PatentRequired: false}
	require.NoError(t, db.Create(&russia).Error)
	uzbekistan := models.Citizenship{Name: "Узбекистан", IsActive: true, PatentRequired: true}
	require.NoError(t, db.Create(&uzbekistan).Error)

	validRow := func() importPersonRow {
		return importPersonRow{
			last: "Иванов", first: "Иван", middle: "Иванович",
			citizenship: "Россия", passport: "1234 567890", position: "Разнорабочий",
		}
	}

	t.Run("полностью валидная строка проходит без ошибок и предупреждений", func(t *testing.T) {
		data := buildPeopleRowsUpload(t, 6, []importPersonRow{validRow()})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Equal(t, 1, result.Summary.Accepted)
		require.Zero(t, result.Summary.Rejected)
		require.Len(t, result.Rows, 1)
		row := result.Rows[0]
		require.Empty(t, row.Errors)
		require.Empty(t, row.Warnings)
		require.NotNil(t, row.Employee)
		require.Equal(t, "Иванов", row.Employee.LastName)
		require.Equal(t, russia.ID, row.Employee.CitizenshipID)
	})

	t.Run("отсутствующее гражданство - ошибка строки с названием", func(t *testing.T) {
		r := validRow()
		r.citizenship = "Узбекистн" // опечатка - в справочнике нет такого
		data := buildPeopleRowsUpload(t, 6, []importPersonRow{r})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Equal(t, 1, result.Summary.Rejected)
		err := errorByCode(t, result.Rows[0].Errors, services.ImportErrCitizenshipUnknown)
		require.Equal(t, `Гражданство "Узбекистн" не найдено в справочнике`, err.Text)
	})

	t.Run("патент без разрешения при patent_required гражданства - блокирующая ошибка", func(t *testing.T) {
		r := validRow()
		r.citizenship = "Узбекистан"
		data := buildPeopleRowsUpload(t, 6, []importPersonRow{r})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Equal(t, 1, result.Summary.Rejected)
		require.Len(t, result.Rows[0].Errors, 1)
		require.Equal(t, services.ImportErrPatentRequired, result.Rows[0].Errors[0].Code)
		require.Contains(t, result.Rows[0].Errors[0].Text, "Узбекистан")
	})

	// Оверрайд "патент обязателен" делает поле обязательным ВСЕГДА, независимо от
	// гражданства - зеркало effectivePatentRequired из EmployeeForm.vue. Без этой
	// проверки импорт оказался бы мягче ручного ввода ровно там, где админ ужесточил
	// требования вручную.
	t.Run("патент обязателен по оверрайду даже при гражданстве без patent_required", func(t *testing.T) {
		require.NoError(t, db.Create(&models.AttachmentFieldConfig{
			UniqueAttachmentID: uaID, FieldKey: "patent", Visible: true, Required: true,
		}).Error)
		defer func() {
			require.NoError(t, db.Where("unique_attachment_id = ? AND field_key = ?", uaID, "patent").
				Delete(&models.AttachmentFieldConfig{}).Error)
		}()

		r := validRow()
		r.citizenship = "Россия"
		data := buildPeopleRowsUpload(t, 6, []importPersonRow{r})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Equal(t, 1, result.Summary.Rejected)
		require.Len(t, result.Rows[0].Errors, 1)
		require.Equal(t, services.ImportErrPatentRequired, result.Rows[0].Errors[0].Code)
	})

	t.Run("патент заполненный снимает ошибку по patent_required", func(t *testing.T) {
		r := validRow()
		r.citizenship = "Узбекистан"
		r.patent = "778899"
		data := buildPeopleRowsUpload(t, 6, []importPersonRow{r})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	t.Run("слишком длинное поле отклоняется", func(t *testing.T) {
		r := validRow()
		r.last = strings.Repeat("Ф", 101)
		data := buildPeopleRowsUpload(t, 6, []importPersonRow{r})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.NotEmpty(t, result.Rows[0].Errors)
		tooLong := errorByCode(t, result.Rows[0].Errors, services.ImportErrFieldTooLong)
		require.Equal(t, "last_name", tooLong.Field)
		require.Contains(t, tooLong.Text, "длиннее 100 символов")
	})

	t.Run("омоглиф в ФИО - предупреждение с исправленным вариантом, не ошибка", func(t *testing.T) {
		r := validRow()
		r.last = "Ивaнов" // латинская 'a' вместо кириллической
		data := buildPeopleRowsUpload(t, 6, []importPersonRow{r})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Empty(t, result.Rows[0].Errors)
		require.NotEmpty(t, result.Rows[0].Warnings)
		require.Contains(t, result.Rows[0].Warnings[0], `"Ивaнов"`)
		require.Contains(t, result.Rows[0].Warnings[0], `"Иванов"`)
		require.Equal(t, "Иванов", result.Rows[0].Employee.LastName)
	})

	t.Run("склеенное ФИО разбирается на три части с предупреждением", func(t *testing.T) {
		r := importPersonRow{
			fullName:    "Петров Пётр Петрович",
			citizenship: "Россия", passport: "1111 222233", position: "Монтажник",
		}
		data := buildPeopleRowsUpload(t, 6, []importPersonRow{r})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Empty(t, result.Rows[0].Errors)
		require.NotEmpty(t, result.Rows[0].Warnings)
		require.Contains(t, result.Rows[0].Warnings[0], "проверьте разбор")
		emp := result.Rows[0].Employee
		require.Equal(t, "Петров", emp.LastName)
		require.Equal(t, "Пётр", emp.FirstName)
	})

	t.Run("дубль внутри файла по паспорту - вторая строка отклоняется", func(t *testing.T) {
		r1 := validRow()
		r2 := validRow() // тот же паспорт
		data := buildPeopleRowsUpload(t, 6, []importPersonRow{r1, r2})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Len(t, result.Rows, 2)
		require.Empty(t, result.Rows[0].Errors)
		require.Len(t, result.Rows[1].Errors, 1)
		dup := errorByCode(t, result.Rows[1].Errors, services.ImportErrDuplicateInFile)
		require.Contains(t, dup.Text, "Дублирует строку 6")
		require.Contains(t, dup.Text, "паспорт")
	})

	t.Run("чёрный список - точное совпадение блокирует строку", func(t *testing.T) {
		bl := models.PersonBlacklist{LastName: "Сидоров", FirstName: "Сидор", Reason: "решение суда", IsActive: true}
		require.NoError(t, db.Create(&bl).Error)

		r := validRow()
		r.last, r.first, r.middle = "Сидоров", "Сидор", ""
		data := buildPeopleRowsUpload(t, 6, []importPersonRow{r})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Len(t, result.Rows[0].Errors, 1)
		blocked := errorByCode(t, result.Rows[0].Errors, services.ImportErrBlacklisted)
		require.Contains(t, blocked.Text, "в чёрном списке")
		require.Contains(t, blocked.Text, "решение суда")
	})

	t.Run("частичный успех - валидная и невалидная строка вместе отдают 207", func(t *testing.T) {
		ok := validRow()
		bad := validRow()
		bad.passport = "9999 000011"
		bad.citizenship = "Нигде"
		data := buildPeopleRowsUpload(t, 6, []importPersonRow{ok, bad})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Equal(t, 1, result.Summary.Accepted)
		require.Equal(t, 1, result.Summary.Rejected)
	})
}
