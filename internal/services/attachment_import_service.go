package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"systemburo/internal/models"
	"systemburo/internal/upload"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// maxImportListRows - потолок строк списка на один файл импорта (решение владельца,
// blank-import): больше - разбить на несколько заявок. Ограничивает и объём разбора
// в этом гейте, и объём будущей построчной валидации (срез C3).
const maxImportListRows = 2000

// importMaxFileSize - потолок размера загружаемого бланка. Список на 2000 строк -
// обычный .xlsx на единицы мегабайт; тот же порядок, что у документов вложений
// (config.UploadMaxFileSize по умолчанию 10 МБ).
const importMaxFileSize = 10 * 1024 * 1024

// ImportListSummary - сводка разбора файла: сколько строк списка прочитано и сколько
// из них годится в заявку. Accepted/Rejected считает построчный разбор (срез C3) -
// в этом срезе гейт файла отвечает только за Read: до построчной проверки дело не
// доходит, пока сам файл не прошёл структурные проверки.
type ImportListSummary struct {
	Read     int `json:"read"`
	Accepted int `json:"accepted"`
	Rejected int `json:"rejected"`
}

// ImportRowResult - результат разбора одной строки списка. Поля errors/warnings
// заполнит построчный разбор (срез C3); в этом срезе гейт файла отбраковывает
// кривой бланк целиком, до того как до отдельных строк доходит дело, поэтому Rows
// в ответе всегда пуст.
type ImportRowResult struct {
	RowNumber int      `json:"row_number"`
	Errors    []string `json:"errors"`
	Warnings  []string `json:"warnings"`
}

// ImportListResult - ответ POST /attachments/:id/import-list. Форма заложена под
// построчный разбор (по образцу BulkOpResult, см. bulk.go): summary уже считает
// прочитанные строки, Rows наполнит следующий срез.
type ImportListResult struct {
	Rows    []ImportRowResult `json:"rows"`
	Summary ImportListSummary `json:"summary"`
}

// AttachmentImportService - приём заполненного Excel-бланка для массового ввода
// участников/машин заявки (blank-import). Гейт файла отбраковывает кривой бланк
// целиком, до разбора отдельных строк: ошибся типом вложения или структура бланка
// разъехалась с шаблоном - об этом надо узнать одной понятной фразой, а не после
// разбора тысячи строк.
type AttachmentImportService interface {
	ImportList(ctx context.Context, uniqueAttachmentID int, file *multipart.FileHeader) (*ImportListResult, error)
}

type attachmentImportService struct {
	db *gorm.DB
}

// NewAttachmentImportService создаёт сервис.
func NewAttachmentImportService(db *gorm.DB) AttachmentImportService {
	return &attachmentImportService{db: db}
}

// ImportList - шаги гейта файла, по порядку; первая непройденная проверка
// заканчивает разбор понятным русским текстом:
//  1. активный шаблон у типа вложения есть, в нём размечен список;
//  2. размер файла и magic bytes .xlsx (расширению не доверяем);
//  3. файл открывается excelize;
//  4. отпечаток (если есть) указывает на этот же тип вложения;
//  5. подписи колонок над списком совпадают с эталонным шаблоном;
//  6. список непустой и укладывается в потолок строк.
//
// Построчный разбор и валидация - срез C3.
func (s *attachmentImportService) ImportList(ctx context.Context, uniqueAttachmentID int, file *multipart.FileHeader) (*ImportListResult, error) {
	template, ua, err := s.loadActiveTemplate(ctx, uniqueAttachmentID)
	if err != nil {
		return nil, err
	}

	data, err := s.readAndValidateFile(file)
	if err != nil {
		return nil, err
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest,
			"Файл повреждён или не является Excel-таблицей. Скачайте бланк заново и заполните его")
	}
	defer f.Close()

	if err := s.checkFingerprint(ctx, f, uniqueAttachmentID, ua); err != nil {
		return nil, err
	}
	if err := s.checkStructure(f, template); err != nil {
		return nil, err
	}

	read, err := countListRows(f, template)
	if err != nil {
		return nil, err
	}
	if read == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest,
			"В файле нет ни одной заполненной строки списка. Заполните бланк и загрузите его снова")
	}
	if read > maxImportListRows {
		return nil, echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("В файле %d строк, максимум %d. Разбейте на несколько заявок", read, maxImportListRows))
	}

	// Построчный разбор и заполнение Rows/Accepted/Rejected - точка расширения C3.
	return &ImportListResult{
		Rows:    []ImportRowResult{},
		Summary: ImportListSummary{Read: read},
	}, nil
}

// loadActiveTemplate загружает активный тип вложения и его активный шаблон с
// разметкой списка. Отсутствие любого из них - гейт файла ещё не начался, а
// загружать уже нечего: та же тройка условий, что у GenerateEmptyBlank (B1).
func (s *attachmentImportService) loadActiveTemplate(ctx context.Context, uaID int) (*models.AttachmentTemplate, *models.UniqueAttachment, error) {
	var ua models.UniqueAttachment
	if err := s.db.WithContext(ctx).
		Where("id = ? AND is_active = ?", uaID, true).
		First(&ua).Error; err != nil {
		return nil, nil, echo.NewHTTPError(http.StatusNotFound, "Тип вложения не найден")
	}

	var template models.AttachmentTemplate
	if err := s.db.WithContext(ctx).
		Preload("Mappings").
		Where("unique_attachment_id = ? AND is_active = ?", uaID, true).
		First(&template).Error; err != nil {
		return nil, nil, echo.NewHTTPError(http.StatusNotFound, "Шаблон бланка не настроен")
	}
	if !hasListMappings(template.Mappings) {
		return nil, nil, echo.NewHTTPError(http.StatusNotFound, "В бланке не размечен список участников")
	}
	return &template, &ua, nil
}

// readAndValidateFile проверяет размер и сигнатуру содержимого файла - расширению
// не доверяем, переименованный .txt не должен пройти дальше под видом .xlsx. Magic
// bytes - тот же образец, что у документов вложений (document_file_service.go).
func (s *attachmentImportService) readAndValidateFile(file *multipart.FileHeader) ([]byte, error) {
	if err := upload.ValidateFileSize(file.Size, importMaxFileSize); err != nil {
		maxMB := importMaxFileSize / 1024 / 1024
		return nil, echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("Файл слишком большой. Максимальный размер: %d МБ", maxMB))
	}

	src, err := file.Open()
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не удалось прочитать файл")
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Файл повреждён или пуст")
	}
	if len(data) < len(magicOOXML) || !bytes.Equal(data[:len(magicOOXML)], magicOOXML) {
		return nil, echo.NewHTTPError(http.StatusBadRequest,
			"Файл не похож на .xlsx. Скачайте пустой бланк и заполните именно его")
	}
	return data, nil
}

// checkFingerprint сверяет отпечаток загруженного файла (если он есть) с типом
// вложения из маршрута. Отсутствие отпечатка - не ошибка: бланк могли пересобрать
// руками, или он потерялся при пересохранении, проверку типа в этом случае берёт на
// себя сверка структуры колонок ниже.
func (s *attachmentImportService) checkFingerprint(ctx context.Context, f *excelize.File, uaID int, ua *models.UniqueAttachment) error {
	fp, ok := ReadBlankFingerprint(f)
	if !ok || fp.UniqueAttachmentID == uaID {
		return nil
	}

	otherName := "другого вида пропуска"
	var other models.UniqueAttachment
	if err := s.db.WithContext(ctx).Where("id = ?", fp.UniqueAttachmentID).First(&other).Error; err == nil {
		otherName = uniqueAttachmentDisplayName(&other)
	}
	return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf(
		"Это бланк другого вида пропуска: файл для %q, а вы подаёте %q",
		otherName, uniqueAttachmentDisplayName(ua)))
}

// checkStructure сверяет подписи колонок в строке над ListStartRow с эталонным
// шаблоном - файлом, который сейчас лежит на диске как активный шаблон этого типа
// вложения. Сверяются только колонки списочных полей: остальная разметка бланка
// (шапка заявки, подписи) админ волен менять между выгрузками бланка.
func (s *attachmentImportService) checkStructure(uploaded *excelize.File, template *models.AttachmentTemplate) error {
	headerRow := template.ListStartRow - 1
	cols := listMappingColumns(template.Mappings)
	if headerRow < 1 || len(cols) == 0 {
		return nil
	}

	refBytes, err := os.ReadFile(template.FilePath)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось сверить структуру бланка")
	}
	ref, err := excelize.OpenReader(bytes.NewReader(refBytes))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось сверить структуру бланка")
	}
	defer ref.Close()

	refSheet := ref.GetSheetName(0)
	uploadedSheet := uploaded.GetSheetName(0)
	for _, col := range cols {
		cell, err := excelize.CoordinatesToCellName(col, headerRow)
		if err != nil {
			continue
		}
		refVal, _ := ref.GetCellValue(refSheet, cell)
		gotVal, _ := uploaded.GetCellValue(uploadedSheet, cell)
		if strings.TrimSpace(refVal) != strings.TrimSpace(gotVal) {
			return echo.NewHTTPError(http.StatusBadRequest,
				"Бланк изменён: колонки не на своих местах. Скачайте бланк заново и заполните его")
		}
	}
	return nil
}

// listMappingColumns - уникальные номера колонок (1-based) списочных полей шаблона,
// по порядку появления в mappings.
func listMappingColumns(mappings []models.AttachmentTemplateMapping) []int {
	seen := make(map[int]struct{}, len(mappings))
	cols := make([]int, 0, len(mappings))
	for _, m := range mappings {
		if !m.IsListField {
			continue
		}
		col, _, err := excelize.CellNameToCoordinates(m.CellRef)
		if err != nil {
			continue
		}
		if _, ok := seen[col]; ok {
			continue
		}
		seen[col] = struct{}{}
		cols = append(cols, col)
	}
	return cols
}

// countListRows считает строки списка, у которых заполнена хотя бы одна списочная
// колонка, от ListStartRow до конца листа. Строки полностью пустых списочных колонок
// (в середине списка или в хвосте) в счёт не идут - тот же критерий "непустая
// строка", которым построчный разбор (C3) будет присваивать row_number.
func countListRows(f *excelize.File, template *models.AttachmentTemplate) (int, error) {
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Не удалось прочитать список из файла")
	}
	cols := listMappingColumns(template.Mappings)
	if len(cols) == 0 || template.ListStartRow < 1 {
		return 0, nil
	}

	count := 0
	for rowIdx := template.ListStartRow; rowIdx <= len(rows); rowIdx++ {
		if rowHasListData(rows[rowIdx-1], cols) {
			count++
		}
	}
	return count, nil
}

// rowHasListData сообщает, заполнена ли хоть одна списочная колонка строки.
func rowHasListData(row []string, cols []int) bool {
	for _, col := range cols {
		idx := col - 1
		if idx < len(row) && strings.TrimSpace(row[idx]) != "" {
			return true
		}
	}
	return false
}

// uniqueAttachmentDisplayName - человекочитаемое имя типа вложения для текста
// ошибки: показное название важнее служебного (см. attachment.display_name в
// словаре полей бланка), Name - запасной вариант, если показное не задано.
func uniqueAttachmentDisplayName(ua *models.UniqueAttachment) string {
	if ua == nil {
		return "вложение"
	}
	if ua.DisplayName != nil && strings.TrimSpace(*ua.DisplayName) != "" {
		return strings.TrimSpace(*ua.DisplayName)
	}
	if ua.Name != nil && strings.TrimSpace(*ua.Name) != "" {
		return strings.TrimSpace(*ua.Name)
	}
	return "вложение"
}
