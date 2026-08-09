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

// importCarRow - одна строка машины для построчных тестов разбора номера
// (blank-import-ux U2): пустое поле не пишется в ячейку вовсе, как незаполненный
// столбец бланка.
type importCarRow struct {
	number, mark string
}

var importCarsColumns = map[string]string{"car.car_number": "B", "car.mark_name": "C"}
var importCarsHeaders = map[string]string{"B": "Номер ТС", "C": "Марка"}

// seedCarsFieldsTemplate заводит тип вложения "cars" с активным шаблоном, где номер и
// марка привязаны каждый к своей колонке - зеркало seedPeopleFieldsTemplate для машин.
func seedCarsFieldsTemplate(t *testing.T, db *gorm.DB, name string, startRow int) int {
	t.Helper()
	nm := name
	ua := models.UniqueAttachment{AttachmentType: "cars", Name: &nm, DisplayName: &nm, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	for col, text := range importCarsHeaders {
		require.NoError(t, f.SetCellStr(sheet, fmt.Sprintf("%s%d", col, startRow-1), text))
	}
	path := t.TempDir() + "/" + name + ".xlsx"
	require.NoError(t, f.SaveAs(path))
	require.NoError(t, f.Close())

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path,
		OriginalFileName: name + ".xlsx",
		ListStartRow:     startRow, ListEndRow: startRow + 50, MaxListRows: 50,
	}
	require.NoError(t, db.Create(&tpl).Error)
	for field, col := range importCarsColumns {
		require.NoError(t, db.Create(&models.AttachmentTemplateMapping{
			TemplateID: tpl.ID, CellRef: fmt.Sprintf("%s%d", col, startRow), FieldPath: field, IsListField: true,
		}).Error)
	}
	return ua.ID
}

// buildCarsRowsUpload собирает байты "заполненного" бланка с несколькими строками машин -
// тот же заголовок, что у шаблона (checkStructure сверяет их побайтово).
func buildCarsRowsUpload(t *testing.T, startRow int, rows []importCarRow) []byte {
	t.Helper()
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	for col, text := range importCarsHeaders {
		require.NoError(t, f.SetCellStr(sheet, fmt.Sprintf("%s%d", col, startRow-1), text))
	}
	for i, r := range rows {
		row := startRow + i
		if r.number != "" {
			require.NoError(t, f.SetCellStr(sheet, fmt.Sprintf("B%d", row), r.number))
		}
		if r.mark != "" {
			require.NoError(t, f.SetCellStr(sheet, fmt.Sprintf("C%d", row), r.mark))
		}
	}
	var buf bytes.Buffer
	_, err := f.WriteTo(&buf)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return buf.Bytes()
}

// seedRussianPlateFormat заводит формат номера "буква-3цифры-2буквы-2или3цифры" (по
// образцу российского), с дополнением обеих числовых ячеек нулями слева - тот же
// формат, что подбирает форма подачи (VehicleForm.vue) при выборе по умолчанию.
func seedRussianPlateFormat(t *testing.T, db *gorm.DB, isDefault bool) models.LicensePlateFormat {
	t.Helper()
	format := models.LicensePlateFormat{Name: "РФ", IsActive: true, IsDefault: isDefault}
	require.NoError(t, db.Create(&format).Error)

	letters := "АВЕКМНОРСТУХ"
	cyrillic := "cyrillic"
	left := "left"
	zero := "0"
	cells := []models.LicensePlateFormatCell{
		{FormatID: format.ID, CellOrder: 1, CellType: "letters", MinLength: intPtr(1), MaxLength: intPtr(1), AllowedLetters: &letters, AlphabetType: &cyrillic},
		{FormatID: format.ID, CellOrder: 2, CellType: "numbers", MinLength: intPtr(3), MaxLength: intPtr(3), PaddingChar: &zero, PaddingSide: &left},
		{FormatID: format.ID, CellOrder: 3, CellType: "letters", MinLength: intPtr(2), MaxLength: intPtr(2), AllowedLetters: &letters, AlphabetType: &cyrillic},
		{FormatID: format.ID, CellOrder: 4, CellType: "numbers", MinLength: intPtr(2), MaxLength: intPtr(3), PaddingChar: &zero, PaddingSide: &left},
	}
	for _, c := range cells {
		require.NoError(t, db.Create(&c).Error)
	}
	return format
}

func intPtr(v int) *int { return &v }

// Разбор госномера по справочнику форматов (blank-import-ux U2): раскладка/омоглифы
// исправляются с предупреждением, короткая числовая часть дополняется по padding-правилам
// формата, произвольный текст отклоняется понятной ошибкой, "По факту" формат не проверяет.
func TestAttachmentImportListRows_LicensePlateFormat(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	uaID := seedCarsFieldsTemplate(t, db, "import_rows_cars", 6)
	seedRussianPlateFormat(t, db, true)

	t.Run("латиница похожая на кириллицу исправляется с предупреждением", func(t *testing.T) {
		data := buildCarsRowsUpload(t, 6, []importCarRow{{number: "A123BC077", mark: "Toyota"}})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		// Предупреждение не блокирует строку - Rejected остаётся 0, ответ 200 (не 207).
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Len(t, result.Rows, 1)
		row := result.Rows[0]
		require.Empty(t, row.Errors)
		require.Len(t, row.Warnings, 1)
		require.Contains(t, row.Warnings[0], `"A123BC077"`)
		require.Contains(t, row.Warnings[0], `"А 123 ВС 077"`)
		require.NotNil(t, row.Vehicle)
		require.Equal(t, "А 123 ВС 077", row.Vehicle.CarNumber)
	})

	t.Run("произвольный текст отклоняется понятной ошибкой", func(t *testing.T) {
		data := buildCarsRowsUpload(t, 6, []importCarRow{{number: "Писька", mark: "Toyota"}})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Equal(t, 1, result.Summary.Rejected)
		require.Len(t, result.Rows, 1)
		require.Equal(t, `Номер Т/С "Писька" не соответствует ни одному формату номеров`,
			errorByCode(t, result.Rows[0].Errors, services.ImportErrPlateFormat).Text)
	})

	t.Run("короткая числовая часть дополняется по правилам формата", func(t *testing.T) {
		data := buildCarsRowsUpload(t, 6, []importCarRow{{number: "А123ВС7", mark: "Toyota"}})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Len(t, result.Rows, 1)
		row := result.Rows[0]
		require.Empty(t, row.Errors)
		require.NotNil(t, row.Vehicle)
		require.Equal(t, "А 123 ВС 007", row.Vehicle.CarNumber)
		require.Len(t, row.Warnings, 1)
		require.Contains(t, row.Warnings[0], `"А123ВС7"`)
		require.Contains(t, row.Warnings[0], `"А 123 ВС 007"`)
	})

	t.Run("По факту не проверяется по формату, как и раньше", func(t *testing.T) {
		data := buildCarsRowsUpload(t, 6, []importCarRow{{number: "По факту", mark: "По факту"}})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Zero(t, result.Summary.Rejected)
		require.Len(t, result.Rows, 1)
		row := result.Rows[0]
		require.Empty(t, row.Errors)
		require.Empty(t, row.Warnings)
		require.NotNil(t, row.Vehicle)
		require.Equal(t, "По факту", row.Vehicle.CarNumber)
	})

	// Ниже по цепочке (форма подачи, привязка к организации) спецзначение сравнивают
	// строгим равенством, поэтому написание из файла приводится к каноническому.
	t.Run("По факту из файла приводится к каноническому написанию", func(t *testing.T) {
		data := buildCarsRowsUpload(t, 6, []importCarRow{{number: "ПО ФАКТУ", mark: "Toyota"}})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Len(t, result.Rows, 1)
		require.Empty(t, result.Rows[0].Errors)
		require.Equal(t, "По факту", result.Rows[0].Vehicle.CarNumber)
	})

	t.Run("слишком короткая строка не дотягивает пустыми ячейками", func(t *testing.T) {
		// Регресс: разбор по ячейкам не должен принимать пустой сегмент за
		// подошедшую ячейку, когда символов в строке физически не хватает.
		data := buildCarsRowsUpload(t, 6, []importCarRow{{number: "А", mark: "Toyota"}})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Equal(t, 1, result.Summary.Rejected)
		require.Equal(t, `Номер Т/С "А" не соответствует ни одному формату номеров`,
			errorByCode(t, result.Rows[0].Errors, services.ImportErrPlateFormat).Text)
	})

	t.Run("уже приведённый номер проходит без предупреждений", func(t *testing.T) {
		data := buildCarsRowsUpload(t, 6, []importCarRow{{number: "А123ВС777", mark: "Toyota"}})
		rec := postImportFile(t, e, uaID, "list.xlsx", data, admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Zero(t, result.Summary.Rejected)
		row := result.Rows[0]
		require.Empty(t, row.Errors)
		require.Empty(t, row.Warnings)
		require.Equal(t, "А 123 ВС 777", row.Vehicle.CarNumber)
	})
}
