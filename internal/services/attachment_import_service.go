package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/upload"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// maxImportListRows - потолок строк списка на один файл импорта (решение владельца,
// blank-import): больше - разбить на несколько заявок. Ограничивает и объём разбора
// в этом гейте, и объём построчной валидации (parseRows).
const maxImportListRows = 2000

// importMaxFileSize - потолок размера загружаемого бланка. Список на 2000 строк -
// обычный .xlsx на единицы мегабайт; тот же порядок, что у документов вложений
// (config.UploadMaxFileSize по умолчанию 10 МБ).
const importMaxFileSize = 10 * 1024 * 1024

// ImportListSummary - сводка разбора файла: сколько строк списка прочитано и сколько
// из них годится в заявку.
type ImportListSummary struct {
	Read     int `json:"read"`
	Accepted int `json:"accepted"`
	Rejected int `json:"rejected"`
}

// ImportRowResult - результат построчного разбора и валидации одной строки списка
// (срез C3). Employee/Vehicle/Item несут те же поля, что и ручной ввод формы подачи
// (EmployeeInput/VehicleInput/ItemInput) - ровно один из трёх заполнен, по типу
// вложения; следующий срез (D1D2) кладёт их прямо в список заявки без пересборки.
// Строка с непустым Errors в заявку не попадает - Warnings её не блокируют.
type ImportRowResult struct {
	RowNumber int              `json:"row_number"`
	Employee  *EmployeeInput   `json:"employee,omitempty"`
	Vehicle   *VehicleInput    `json:"vehicle,omitempty"`
	Item      *ItemInput       `json:"item,omitempty"`
	Errors    []ImportRowError `json:"errors"`
	Warnings  []string         `json:"warnings"`
}

// ImportListResult - ответ POST /attachments/:id/import-list (по образцу BulkOpResult,
// см. bulk.go).
type ImportListResult struct {
	Rows    []ImportRowResult `json:"rows"`
	Summary ImportListSummary `json:"summary"`
}

// HTTPStatus - 200 при чистом файле (ни одной отклонённой строки), 207 (MultiStatus)
// при частичном успехе - зеркало BulkOpResult.HTTPStatus.
func (r *ImportListResult) HTTPStatus() int {
	if r.Summary.Rejected > 0 {
		return http.StatusMultiStatus
	}
	return http.StatusOK
}

// AttachmentImportService - приём заполненного Excel-бланка для массового ввода
// участников/машин заявки (blank-import). Гейт файла отбраковывает кривой бланк
// целиком, до разбора отдельных строк: ошибся типом вложения или структура бланка
// разъехалась с шаблоном - об этом надо узнать одной понятной фразой, а не после
// разбора тысячи строк.
type AttachmentImportService interface {
	ImportList(ctx context.Context, uniqueAttachmentID, userID int, file *multipart.FileHeader) (*ImportListResult, error)
}

type attachmentImportService struct {
	db         *gorm.DB
	recorder   AuditRecorder
	uploadPath string // базовый путь, обычно cfg.UploadPath. Загрузки в <uploadPath>/imports/
}

// NewAttachmentImportService создаёт сервис.
func NewAttachmentImportService(db *gorm.DB, recorder AuditRecorder, uploadPath string) AttachmentImportService {
	return &attachmentImportService{db: db, recorder: recorder, uploadPath: uploadPath}
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
// Дальше идёт построчный разбор и валидация (срез C3) теми же правилами, что форма
// подачи (см. parseEmployeeRows/parseVehicleRows/parseItemRows). Прошедший гейт файл
// сохраняется на диск как первоисточник (uploads/imports/), в аудит пишется кто, когда,
// из какого файла и со сколькими строками.
func (s *attachmentImportService) ImportList(ctx context.Context, uniqueAttachmentID, userID int, file *multipart.FileHeader) (*ImportListResult, error) {
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

	rows, err := s.parseRows(ctx, f, template, ua)
	if err != nil {
		return nil, err
	}
	summary := ImportListSummary{Read: read}
	for _, r := range rows {
		if len(r.Errors) > 0 {
			summary.Rejected++
		} else {
			summary.Accepted++
		}
	}

	storedPath, err := s.storeSourceFile(uniqueAttachmentID, file.Filename, data)
	if err != nil {
		return nil, err
	}

	s.recorder.Log(ctx, nil, models.AuditEntityUniqueAttachment, &uniqueAttachmentID,
		models.UniqueAttachmentActionListImported, &userID, map[string]any{
			"file_name":   file.Filename,
			"stored_path": storedPath,
			"read":        summary.Read,
			"accepted":    summary.Accepted,
			"rejected":    summary.Rejected,
		})

	return &ImportListResult{Rows: rows, Summary: summary}, nil
}

// storeSourceFile сохраняет прошедший гейт файл как первоисточник импорта (решение
// владельца, blank-import C3): если строку из отчёта оспорят, у бюро должен остаться
// оригинал, а не только распарсенные значения. Провал записи не глушим - без файла
// на диске аудит-запись "откуда взяты данные" будет враньём.
func (s *attachmentImportService) storeSourceFile(uniqueAttachmentID int, originalName string, data []byte) (string, error) {
	dir := filepath.Join(s.uploadPath, "imports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", apperr.Internal("Не удалось сохранить файл импорта", fmt.Errorf("mkdir %s: %w", dir, err))
	}
	dst := filepath.Join(dir, fmt.Sprintf("%d_%d_%s", uniqueAttachmentID, time.Now().UnixMilli(), sanitizeFilename(originalName)))
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return "", apperr.Internal("Не удалось сохранить файл импорта", fmt.Errorf("write %s: %w", dst, err))
	}
	return dst, nil
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

	// apperr, а не echo.HTTPError: CustomHTTPErrorHandler логирует 5xx только у apperr.Error,
	// иначе пропавший с диска шаблон дал бы недиагностируемый 500.
	refBytes, err := os.ReadFile(template.FilePath)
	if err != nil {
		return apperr.Internal("Не удалось сверить структуру бланка",
			fmt.Errorf("чтение эталонного шаблона %s: %w", template.FilePath, err))
	}
	ref, err := excelize.OpenReader(bytes.NewReader(refBytes))
	if err != nil {
		return apperr.Internal("Не удалось сверить структуру бланка",
			fmt.Errorf("открытие эталонного шаблона %s: %w", template.FilePath, err))
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
	return listColumns(mappings, false)
}

// listDataColumns - только те списочные колонки, по которым видно, что строку
// заполнил человек (см. nonDataFieldSuffixes). Без этого нетронутый бланк
// возвращался списком строк с ошибками: его строки пронумерованы заранее, а ниже
// списка стоят подписи бланка в колонке мест разгрузки.
func listDataColumns(mappings []models.AttachmentTemplateMapping) []int {
	return listColumns(mappings, true)
}

// nonDataFieldSuffixes - списочные поля, по которым нельзя судить, заполнил ли
// человек строку. Нумерацию (row_number) проставляет система при выдаче бланка.
// Места разгрузки, проезда и прохода в файле не передаются вовсе - решение
// владельца, они задаются на сайте на весь список сразу, - зато их подписи стоят
// в бланке ниже списка ("(контактный телефон)", "(дд.мм.гггг)"), и по ним разбор
// принимал за участника оформительскую строку бланка.
var nonDataFieldSuffixes = []string{
	".row_number",
	".unload_places",
	".passage_tables",
	".target_tables",
}

func isNonDataListField(fieldPath string) bool {
	for _, suffix := range nonDataFieldSuffixes {
		if strings.HasSuffix(fieldPath, suffix) {
			return true
		}
	}
	return false
}

func listColumns(mappings []models.AttachmentTemplateMapping, dataOnly bool) []int {
	seen := make(map[int]struct{}, len(mappings))
	cols := make([]int, 0, len(mappings))
	for _, m := range mappings {
		if !m.IsListField {
			continue
		}
		if dataOnly && isNonDataListField(m.FieldPath) {
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
// строка", которым построчный разбор (listRowNumbers) присваивает row_number.
func countListRows(f *excelize.File, template *models.AttachmentTemplate) (int, error) {
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Не удалось прочитать список из файла")
	}
	cols := listDataColumns(template.Mappings)
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
