package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// BlankContext - данные одной заявки + вложения, передаваемые в генератор.
// Заполняется backend-ом перед вызовом GenerateBlank: тащит application,
// attachment, sender, cars/employees/items, custom values.
type BlankContext struct {
	Application      *models.Application
	Sender           *models.User
	Organization     *models.Organization
	Company          *models.Company
	Attachment       *models.Attachment
	UniqueAttachment *models.UniqueAttachment
	Cars             []models.Car
	Employees        []models.Employee
	Items            []models.Item
	Citizenships     map[int]string // citizenship_id → name
	CustomValues     map[int]string // custom_field_id → value
	// Approvers - согласовавшие заявку в порядке согласования.
	Approvers []Approver
	// AttachmentUnloadPlaces - имена мест разгрузки вложения в порядке привязки. Для
	// вложения-имущества это единственный источник мест: у ТМЦ своих машин нет (#706).
	AttachmentUnloadPlaces []string
	// Привязки элементов, по одной записи на строку списка (#1454): полные места
	// разгрузки машины (car_unload_places), её посты «Проезд» (car_target_tables) и
	// места прохода сотрудника (employee_target_tables).
	CarUnloadPlaces      map[int][]string // car_id → имена мест
	CarPassageTables     map[int][]string // car_id → имена постов
	EmployeeTargetTables map[int][]string // employee_id → имена постов
	// IncludeDocuments - подставлять ли в бланк документы участников (серия и номер
	// паспорта, номер патента, иное разрешение). false заменяет их прочерком: право
	// detail.documents.export есть не у каждого, кому доступна сама заявка, а бланк
	// уносится из системы файлом.
	IncludeDocuments bool
	// ApplicationItems - ТМЦ всех «Заявок на ввоз» этой заявки, в порядке вложений.
	// Списочная секция бланка одна и занята его собственным типом (у заявки на работы -
	// сотрудниками), поэтому чужие ТМЦ перечисляются одной ячейкой через app_items.*.
	ApplicationItems []ApplicationItemRow
	// ApplicationCars - машины «Автозаявок» этой же заявки: в бланке ввоза есть поле
	// «Марка и гос. номер Т/С», а собственных машин у такого вложения нет.
	ApplicationCars []ApplicationCarRow
}

// Approver - согласовавший заявку. Required - согласование было обязательным: такие
// подписи в бланке перечисляются все, необязательное согласование представляет первый
// согласовавший.
type Approver struct {
	LastName   string
	FirstName  string
	MiddleName string
	Required   bool
}

// ApplicationCarRow - машина из вложения-соседа: номер, марка и название вложения,
// откуда она приехала.
type ApplicationCarRow struct {
	Number     string
	Mark       string
	SourceName string
}

// ApplicationItemRow - позиция ТМЦ из вложения-соседа с названием вложения-источника:
// при нескольких «Заявках на ввоз» перечень объединяется, и происхождение позиции
// иначе теряется.
type ApplicationItemRow struct {
	Name       string
	Count      *int
	SourceName string
}

// BlankOptions - настройки одной генерации бланка. Параметр обязательный, а не
// значение по умолчанию: каждый вызывающий обязан решить судьбу документов участников
// осознанно. Умолчание «как было» означало бы, что новый путь генерации молча уносит
// паспорта, и заметят это уже в скачанном файле.
type BlankOptions struct {
	// IncludeDocuments - подставлять паспорт, патент и иное разрешение как есть.
	// false ставит в эти ячейки прочерк.
	IncludeDocuments bool
}

// AttachmentBlankService - генерация заполненных .xlsx-бланков на основе
// шаблона UniqueAttachment + данных заявки (#183, часть 2).
type AttachmentBlankService interface {
	GenerateBlank(ctx context.Context, applicationID, attachmentID int, opts BlankOptions) (io.Reader, string, error)
	GenerateEmptyBlank(ctx context.Context, uniqueAttachmentID int) (io.Reader, string, error)
}

type attachmentBlankService struct {
	db *gorm.DB
	// templateCache - сырые байты уже читанных .xlsx-шаблонов (#1615, B4): массовый
	// прогон (бэкфилл за период, ночная сверка) генерирует бланк за бланком одним и
	// тем же файлом шаблона, и без кэша каждый бланк заново читал бы его с диска.
	//
	// Ключ - путь файла, но запись хранит ещё время изменения и размер, и они
	// сверяются на каждом обращении. Сегодня загрузка пишет новый шаблон под новым
	// именем, то есть путь не меняет содержимого - но строить кэш на этом инварианте
	// значит поставить корректность бланков в зависимость от чужого файла: стоит
	// однажды добавить «заменить шаблон на месте», и система молча раздавала бы
	// старые бланки до перезапуска. Stat дешевле чтения на два порядка.
	templateCache   map[string]cachedTemplate
	templateCacheMu sync.Mutex
}

// cachedTemplate - содержимое шаблона вместе с приметами файла, по которым видно,
// что на диске лежит уже другой файл.
type cachedTemplate struct {
	data    []byte
	modTime time.Time
	size    int64
}

// NewAttachmentBlankService создаёт сервис.
func NewAttachmentBlankService(db *gorm.DB) AttachmentBlankService {
	return &attachmentBlankService{db: db, templateCache: make(map[string]cachedTemplate)}
}

// loadTemplateFile отдаёт байты .xlsx-шаблона, читая файл с диска только когда он
// изменился. Возвращаемый срез не мутируется вызывающими (excelize.OpenReader только
// читает), поэтому безопасен для конкурентного использования без копирования.
//
// Пропавший файл выбрасывается из кэша: удалённый шаблон должен давать честную
// ошибку генерации, а не бланк из памяти процесса.
func (s *attachmentBlankService) loadTemplateFile(path string) ([]byte, error) {
	info, statErr := os.Stat(path)
	if statErr != nil {
		s.templateCacheMu.Lock()
		delete(s.templateCache, path)
		s.templateCacheMu.Unlock()
		return nil, statErr
	}

	s.templateCacheMu.Lock()
	cached, ok := s.templateCache[path]
	s.templateCacheMu.Unlock()
	if ok && cached.size == info.Size() && cached.modTime.Equal(info.ModTime()) {
		return cached.data, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	s.templateCacheMu.Lock()
	s.templateCache[path] = cachedTemplate{data: data, modTime: info.ModTime(), size: info.Size()}
	s.templateCacheMu.Unlock()
	return data, nil
}

// GenerateBlank возвращает Reader с готовым .xlsx и filename.
// Шаги:
//  1. Загрузить шаблон (attachment_templates + mappings) по unique_attachment_id.
//  2. Собрать BlankContext из заявки.
//  3. Открыть .xlsx через excelize, проставить значения в ячейки.
//  4. Для list-fields - заполнить строки списка с авторасширением.
//  5. Сохранить в buffer, вернуть.
func (s *attachmentBlankService) GenerateBlank(ctx context.Context, applicationID, attachmentID int, opts BlankOptions) (io.Reader, string, error) {
	// 1. Attachment + UniqueAttachment + Template.
	var att models.Attachment
	if err := s.db.WithContext(ctx).
		Preload("UniqueAttachment").
		Where("id = ? AND application_id = ?", attachmentID, applicationID).
		First(&att).Error; err != nil {
		return nil, "", echo.NewHTTPError(http.StatusNotFound, "Вложение не найдено")
	}
	if att.UniqueAttachmentID == nil {
		return nil, "", echo.NewHTTPError(http.StatusBadRequest, "У вложения нет привязанного шаблона")
	}

	var template models.AttachmentTemplate
	if err := s.db.WithContext(ctx).
		Preload("Mappings").
		Where("unique_attachment_id = ? AND is_active = ?", *att.UniqueAttachmentID, true).
		First(&template).Error; err != nil {
		return nil, "", echo.NewHTTPError(http.StatusNotFound, "Шаблон бланка не настроен")
	}

	// 2. Собрать контекст.
	bctx, err := s.buildContext(ctx, applicationID, &att)
	if err != nil {
		return nil, "", err
	}
	bctx.IncludeDocuments = opts.IncludeDocuments

	// 3. Открыть шаблон - байты берутся из кэша, а не с диска на каждый вызов
	// (массовый прогон бьётся об один и тот же файл сотнями заявок подряд).
	templateBytes, err := s.loadTemplateFile(template.FilePath)
	if err != nil {
		return nil, "", echo.NewHTTPError(http.StatusInternalServerError, "Не удалось открыть шаблон: "+err.Error())
	}
	f, err := excelize.OpenReader(bytes.NewReader(templateBytes))
	if err != nil {
		return nil, "", echo.NewHTTPError(http.StatusInternalServerError, "Не удалось открыть шаблон: "+err.Error())
	}
	defer f.Close()
	sheet := f.GetSheetName(0)

	// 4. Простые (не list) маппинги - группировка по cell_ref для совмещения.
	var listMappings []models.AttachmentTemplateMapping
	cellValues := make(map[string][]string)
	cellOrder := make([]string, 0)
	for _, m := range template.Mappings {
		if m.IsListField {
			listMappings = append(listMappings, m)
			continue
		}
		val := resolveValue(bctx, m.FieldPath, 0)
		if val == "" {
			continue
		}
		if _, exists := cellValues[m.CellRef]; !exists {
			cellOrder = append(cellOrder, m.CellRef)
		}
		cellValues[m.CellRef] = append(cellValues[m.CellRef], val)
	}
	// Разделитель совмещённых полей: nil - настройки нет, берём запятую с пробелом;
	// заданная пустая строка - осознанный выбор "склеивать без разделителя" (#1454).
	sep := ", "
	if template.ConcatSeparator != nil {
		sep = *template.ConcatSeparator
	}
	staticCells := make(map[string]string, len(cellOrder))
	for _, ref := range cellOrder {
		joined := strings.Join(cellValues[ref], sep)
		_ = f.SetCellValue(sheet, ref, joined)
		applyWrapIfMultiline(f, sheet, ref, joined)
		staticCells[ref] = joined
	}

	// 5. Списочные секции с авторасширением: собственная таблица вложения и, если её
	// разметили в шаблоне, таблица ТМЦ «Заявок на ввоз» этой же заявки.
	shifts := make([]rowShift, 0, 2)
	tables := make([]tableForPagination, 0, 2)
	if len(listMappings) > 0 {
		shifts, tables = s.fillListSections(f, sheet, &template, listMappings, staticCells, bctx)
	}

	// 6. Таблица, не поместившаяся на страницу, продолжается со своей шапки столбцов:
	// ставим разрывы страниц сами и повторяем заголовки той таблицы, которая переносится.
	shifts = append(shifts, insertRepeatedHeaders(f, sheet, tables, lastMarkupRow(f, sheet), shifts)...)

	// 7. Записать в buffer.
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", echo.NewHTTPError(http.StatusInternalServerError, "Ошибка генерации файла")
	}

	// 8. Добавленные строки сдвинули диапазоны условного форматирования, но не формулы
	// внутри правил - дошиваем их сами. Сбой здесь бланк не отменяет: отдаём файл со
	// старыми формулами правил, но громко пишем в лог.
	// Секции обрабатываем снизу вверх: формулы хранят строки шаблона, поэтому правило
	// под обеими таблицами должно получить сдвиг каждой из них, а правило между ними -
	// только сдвиг верхней.
	out := buf.Bytes()
	sort.Slice(shifts, func(i, j int) bool { return shifts[i].fromRow > shifts[j].fromRow })
	for _, sh := range shifts {
		if sh.offset <= 0 {
			continue
		}
		shifted, shiftErr := shiftConditionalFormatFormulas(out, sh.fromRow, sh.offset)
		if shiftErr != nil {
			slog.Error("не удалось сдвинуть формулы условного форматирования бланка",
				"error", shiftErr, "template", template.ID, "from", sh.fromRow, "inserted", sh.offset)
			continue
		}
		out = shifted
	}

	filename := formatBlankFilename(bctx)
	return bytes.NewReader(out), filename, nil
}

// GenerateEmptyBlank отдаёт активный бланк типа вложения как файл для заполнения
// (массовый ввод участников). От GenerateBlank отличается тем, что заявки ещё нет:
// в файл не подставляется ничего, кроме отпечатка, по которому загруженный обратно
// файл узнаётся как бланк именно этого типа вложения.
func (s *attachmentBlankService) GenerateEmptyBlank(ctx context.Context, uniqueAttachmentID int) (io.Reader, string, error) {
	// Архивный тип вложения бланка не получает: подать по нему заявку всё равно
	// нельзя, а форма подачи такие типы не показывает (attachments.GetActive). У
	// заполненного бланка ограничения нет и быть не может - заявку подали, когда тип
	// был живым.
	var ua models.UniqueAttachment
	if err := s.db.WithContext(ctx).
		Where("id = ? AND is_active = ?", uniqueAttachmentID, true).
		First(&ua).Error; err != nil {
		return nil, "", echo.NewHTTPError(http.StatusNotFound, "Тип вложения не найден")
	}

	var template models.AttachmentTemplate
	if err := s.db.WithContext(ctx).
		Preload("Mappings").
		Where("unique_attachment_id = ? AND is_active = ?", uniqueAttachmentID, true).
		First(&template).Error; err != nil {
		return nil, "", echo.NewHTTPError(http.StatusNotFound, "Шаблон бланка не настроен")
	}
	// Без списочных привязок бланк заполнять нечем: строки участников в нём просто
	// некуда писать, а значит и загружать обратно нечего.
	if !hasListMappings(template.Mappings) {
		return nil, "", echo.NewHTTPError(http.StatusNotFound,
			"В бланке не размечен список участников")
	}

	templateBytes, err := s.loadTemplateFile(template.FilePath)
	if err != nil {
		return nil, "", echo.NewHTTPError(http.StatusInternalServerError, "Не удалось открыть шаблон: "+err.Error())
	}
	f, err := excelize.OpenReader(bytes.NewReader(templateBytes))
	if err != nil {
		return nil, "", echo.NewHTTPError(http.StatusInternalServerError, "Не удалось открыть шаблон: "+err.Error())
	}
	defer f.Close()

	if err := StampBlankFingerprint(f, BlankFingerprint{
		UniqueAttachmentID: uniqueAttachmentID,
		TemplateID:         template.ID,
		ListStartRow:       template.ListStartRow,
	}); err != nil {
		return nil, "", echo.NewHTTPError(http.StatusInternalServerError,
			fmt.Sprintf("Не удалось подготовить бланк (шаблон %d): %s", template.ID, err.Error()))
	}

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		return nil, "", echo.NewHTTPError(http.StatusInternalServerError,
			fmt.Sprintf("Не удалось собрать бланк (шаблон %d): %s", template.ID, err.Error()))
	}
	return bytes.NewReader(buf.Bytes()), emptyBlankFilename(&ua, template.OriginalFileName), nil
}

// hasListMappings - размечена ли в шаблоне списочная часть.
func hasListMappings(mappings []models.AttachmentTemplateMapping) bool {
	for _, m := range mappings {
		if m.IsListField {
			return true
		}
	}
	return false
}

// emptyBlankFilename - имя файла пустого бланка. Владелец ждёт то же имя, под
// которым шаблон загрузили в систему (OriginalFileName), - служебное "Бланк_<тип>"
// только для шаблонов без сохранённого оригинального имени (загружены до того, как
// поле появилось).
func emptyBlankFilename(ua *models.UniqueAttachment, originalFileName string) string {
	if trimmed := strings.TrimSpace(originalFileName); trimmed != "" {
		return sanitizeFilename(trimmed)
	}
	name := ""
	if ua != nil && ua.Name != nil {
		name = strings.TrimSpace(*ua.Name)
	}
	if name == "" {
		name = "бланк"
	}
	return sanitizeFilename(fmt.Sprintf("Бланк_%s.xlsx", strings.ReplaceAll(name, " ", "-")))
}

// listSource возвращает префикс field_path списочной части и число записей для типа
// вложения. Пустой префикс - у вложения нет списка (неизвестный тип).
func listSource(bctx *BlankContext) (string, int) {
	if bctx.Attachment == nil {
		return "", 0
	}
	prefix := ListFieldPrefix(bctx.Attachment.AttachmentType)
	switch prefix {
	case "car.":
		return prefix, len(bctx.Cars)
	case "employee.":
		return prefix, len(bctx.Employees)
	case "item.":
		return prefix, len(bctx.Items)
	}
	return "", 0
}

// repeatedCell - значение, которое повторяется в каждой строке списка: колонка и уже
// собранная строка (со склейкой совмещённых полей).
type repeatedCell struct {
	col   int
	value string
}

// repeatedListCells отбирает из обычных (не списочных) ячеек те, что админ поставил
// внутрь строк списка. Такое поле относится ко всей заявке - организация, компания,
// период, - и в разметке бланка живёт колонкой таблицы, поэтому его значение должно
// стоять в каждой строке, а не только в первой.
func repeatedListCells(startRow, endRow int, staticCells map[string]string) []repeatedCell {
	if startRow < 1 || endRow < startRow {
		return nil
	}
	out := make([]repeatedCell, 0, len(staticCells))
	for ref, val := range staticCells {
		col, row, err := excelize.CellNameToCoordinates(ref)
		if err != nil || row < startRow || row > endRow {
			continue
		}
		out = append(out, repeatedCell{col: col, value: val})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].col < out[j].col })
	return out
}

// listSection - одна таблица бланка: строки шаблона, префикс привязок, число записей и
// как достать значение поля для строки.
type listSection struct {
	startRow int
	endRow   int
	maxRows  int
	prefix   string
	count    int
	resolve  func(path string, idx int) string
}

// rowShift - сколько строк добавилось ниже определённой строки шаблона. Нужен, чтобы
// досдвинуть формулы условного форматирования и сместить нижние таблицы.
type rowShift struct {
	fromRow int
	offset  int
}

// blankSections собирает таблицы бланка сверху вниз: собственную (её тип задаёт тип
// вложения) и таблицу ТМЦ «Заявок на ввоз» заявки, если её разметили в шаблоне.
// Вторая таблица заполняется теми же привязками группы item.*, что и бланк ввоза, -
// админу не надо заводить отдельные поля под «чужие» ТМЦ.
func blankSections(t *models.AttachmentTemplate, mappings []models.AttachmentTemplateMapping, bctx *BlankContext) []listSection {
	sections := make([]listSection, 0, 2)
	ownPrefix, count := listSource(bctx)
	if ownPrefix != "" && count > 0 {
		sections = append(sections, listSection{
			startRow: t.ListStartRow, endRow: t.ListEndRow, maxRows: t.MaxListRows,
			prefix: ownPrefix, count: count,
			resolve: func(path string, idx int) string { return resolveValue(bctx, path, idx) },
		})
	}
	// Таблица ТМЦ: сколько строк под неё отведено, админ задаёт числом, а где она
	// начинается - видно по привязкам группы «Имущество (список)». У бланка самого ввоза
	// этой таблицы нет: там ТМЦ и так заполняют строки собственного списка.
	if t.ItemsMaxListRows > 0 && len(bctx.ApplicationItems) > 0 && ownPrefix != "item." {
		if start := itemsSectionStart(mappings); start > 0 {
			rows := bctx.ApplicationItems
			sections = append(sections, listSection{
				startRow: start, endRow: start + t.ItemsMaxListRows - 1, maxRows: t.ItemsMaxListRows,
				prefix: "item.", count: len(rows),
				resolve: func(path string, idx int) string { return resolveApplicationItemRow(rows, path, idx) },
			})
		}
	}
	sort.Slice(sections, func(i, j int) bool { return sections[i].startRow < sections[j].startRow })
	return sections
}

// itemsSectionStart - строка, с которой начинается таблица ТМЦ: верхняя из ячеек, куда
// админ привязал поля группы «Имущество (список)». Отдельного поля под неё в настройке
// нет - привязка и есть указание места.
func itemsSectionStart(mappings []models.AttachmentTemplateMapping) int {
	start := 0
	for _, m := range mappings {
		if !strings.HasPrefix(m.FieldPath, "item.") {
			continue
		}
		_, row, err := excelize.CellNameToCoordinates(m.CellRef)
		if err != nil || row < 1 {
			continue
		}
		if start == 0 || row < start {
			start = row
		}
	}
	return start
}

// fillListSections заполняет таблицы бланка сверху вниз. Расширение верхней таблицы
// сдвигает нижние, поэтому строки нижних пишутся со смещением на уже добавленные строки.
// Возвращает сдвиги по каждой таблице для правки формул условного форматирования.
func (s *attachmentBlankService) fillListSections(f *excelize.File, sheet string, t *models.AttachmentTemplate, mappings []models.AttachmentTemplateMapping, staticCells map[string]string, bctx *BlankContext) ([]rowShift, []tableForPagination) {
	sections := blankSections(t, mappings, bctx)
	shifts := make([]rowShift, 0, len(sections))
	tables := make([]tableForPagination, 0, len(sections))
	shift := 0
	for _, sec := range sections {
		inserted := s.fillListSection(f, sheet, sec, shift, mappings, staticCells)
		shifts = append(shifts, rowShift{fromRow: sec.endRow + 1, offset: inserted})
		// Границы таблицы в готовом файле: строки шаблона сдвинуты таблицами выше, а
		// снизу к ним добавились строки, которые дописал сам список.
		tables = append(tables, tableForPagination{
			headerRow: sec.startRow - 1 + shift,
			firstRow:  sec.startRow + shift,
			lastRow:   sec.endRow + shift + inserted,
		})
		shift += inserted
	}
	return shifts, tables
}

// fillListSection заполняет строки списка (cars/employees/items), при необходимости
// расширяя шаблон через InsertRows + копирование стилей последней шаблонной строки.
// staticCells - уже записанные значения обычных полей: те из них, что попали в строки
// списка, повторяются по строкам вместе со списочными.
// Возвращает число реально добавленных строк: на него потом сдвигаются формулы правил
// условного форматирования.
func (s *attachmentBlankService) fillListSection(f *excelize.File, sheet string, sec listSection, shift int, mappings []models.AttachmentTemplateMapping, staticCells map[string]string) int {
	// Секцию наполняет её собственная группа привязок (#1454): у items-вложения нет
	// машин, и привязка car.* не должна отменять заполнение ТМЦ. Раньше тип брался по
	// первому list-маппингу, из-за чего боевой бланк "Заявка на ввоз" с привязками к
	// номеру машины отдавал пустую таблицу имущества.
	own := make([]models.AttachmentTemplateMapping, 0, len(mappings))
	for _, m := range mappings {
		if strings.HasPrefix(m.FieldPath, sec.prefix) {
			own = append(own, m)
		}
	}
	if len(own) == 0 {
		return 0
	}
	mappings = own

	// Строки шаблона ниже уже расширенной таблицы физически уехали вниз - пишем со
	// смещением, иначе ТМЦ легли бы поверх разметки под списком сотрудников.
	startRow := sec.startRow + shift
	endRow := sec.endRow + shift

	// Записей больше, чем строк в шаблоне - добавляем недостающие копией последней
	// строки списка: так новая строка получает её оформление. Отдельный InsertRows
	// здесь не нужен и вреден - DuplicateRowTo сам вставляет строку со сдвигом, и
	// вдвоём они добавляли вдвое больше строк, оставляя в бланке пустоты, а разметку
	// под таблицей уводя ниже, чем нужно (#1480).
	inserted := 0
	if sec.count > sec.maxRows && sec.maxRows > 0 {
		extra := sec.count - sec.maxRows
		for i := 0; i < extra; i++ {
			if err := f.DuplicateRowTo(sheet, endRow, endRow+1+i); err != nil {
				slog.Error("не удалось расширить список бланка", "error", err, "row", endRow+1+i)
				break
			}
			inserted++
		}
	}

	// Сортируем mappings по колонке для предсказуемости.
	sort.Slice(mappings, func(i, j int) bool {
		ci, _, _ := excelize.CellNameToCoordinates(mappings[i].CellRef)
		cj, _, _ := excelize.CellNameToCoordinates(mappings[j].CellRef)
		return ci < cj
	})

	// Границы для поиска повторяемых ячеек - шаблонные: обычные поля записаны до вставок.
	repeated := repeatedListCells(sec.startRow, sec.endRow, staticCells)

	for idx := 0; idx < sec.count; idx++ {
		row := startRow + idx
		for _, m := range mappings {
			col, _, err := excelize.CellNameToCoordinates(m.CellRef) //nolint:nestif // ok
			if err != nil {
				continue
			}
			cell, _ := excelize.CoordinatesToCellName(col, row)
			val := sec.resolve(m.FieldPath, idx)
			if val != "" {
				_ = f.SetCellValue(sheet, cell, val)
			}
		}
		for _, r := range repeated {
			if r.value == "" {
				continue
			}
			cell, err := excelize.CoordinatesToCellName(r.col, row)
			if err != nil {
				continue
			}
			_ = f.SetCellValue(sheet, cell, r.value)
			applyWrapIfMultiline(f, sheet, cell, r.value)
		}
	}
	return inserted
}

func (s *attachmentBlankService) buildContext(ctx context.Context, appID int, att *models.Attachment) (*BlankContext, error) {
	bctx := &BlankContext{
		Attachment:       att,
		UniqueAttachment: att.UniqueAttachment,
		Citizenships:     make(map[int]string),
		CustomValues:     make(map[int]string),

		CarUnloadPlaces:      make(map[int][]string),
		CarPassageTables:     make(map[int][]string),
		EmployeeTargetTables: make(map[int][]string),
	}

	var app models.Application
	if err := s.db.WithContext(ctx).First(&app, appID).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Заявка не найдена")
	}
	bctx.Application = &app

	if app.SenderUserID != 0 {
		var u models.User
		s.db.WithContext(ctx).First(&u, app.SenderUserID)
		bctx.Sender = &u
	}
	if app.OrganizationID != 0 {
		var o models.Organization
		s.db.WithContext(ctx).First(&o, app.OrganizationID)
		bctx.Organization = &o
	}
	if app.CompanyID != nil && *app.CompanyID != 0 {
		var c models.Company
		s.db.WithContext(ctx).First(&c, *app.CompanyID)
		bctx.Company = &c
	}

	// Согласовавшие заявку - для подписи «СОГЛАСОВАНО» в бланке.
	bctx.Approvers = s.loadApprovers(ctx, appID)

	// Cars / employees / items - только для этого attachment и только допущенное на КПП:
	// бланк печатают и несут на пост как документ допуска, поэтому строка непринятого
	// дополнения в нём означала бы проход мимо согласования (#1685).
	s.db.WithContext(ctx).Where("attachment_id = ?", att.ID).Where(admittedSupplementCond("cars")).Order("id").Find(&bctx.Cars)
	s.db.WithContext(ctx).Where("attachment_id = ?", att.ID).Where(admittedSupplementCond("employees")).Order("id").Find(&bctx.Employees)
	s.db.WithContext(ctx).Where("attachment_id = ?", att.ID).Where(admittedSupplementCond("items")).Order("id").Find(&bctx.Items)

	// Привязки машин: места разгрузки и посты «Проезд» - по одному запросу на список,
	// иначе на каждой строке бланка был бы отдельный поход в базу.
	if len(bctx.Cars) > 0 {
		carIDs := make([]int, 0, len(bctx.Cars))
		for _, c := range bctx.Cars {
			carIDs = append(carIDs, c.ID)
		}
		bctx.CarUnloadPlaces = groupNamesByOwner(ctx, s.db, `
			SELECT cup.car_id AS owner_id, up.name
			FROM car_unload_places cup
			JOIN unload_places up ON cup.unload_place_id = up.id
			WHERE cup.car_id IN ?
			ORDER BY cup.order_index NULLS LAST, up.name
		`, carIDs)
		bctx.CarPassageTables = groupNamesByOwner(ctx, s.db, `
			SELECT ctt.car_id AS owner_id, COALESCE(NULLIF(st.display_name, ''), st.name) AS name
			FROM car_target_tables ctt
			JOIN system_tables st ON ctt.table_id = st.id
			WHERE ctt.car_id IN ?
			ORDER BY ctt.order_index NULLS LAST, name
		`, carIDs)
	}

	// Места прохода сотрудников - тем же запросом на весь список.
	if len(bctx.Employees) > 0 {
		empIDs := make([]int, 0, len(bctx.Employees))
		for _, e := range bctx.Employees {
			empIDs = append(empIDs, e.ID)
		}
		bctx.EmployeeTargetTables = groupNamesByOwner(ctx, s.db, `
			SELECT ett.employee_id AS owner_id, COALESCE(NULLIF(st.display_name, ''), st.name) AS name
			FROM employee_target_tables ett
			JOIN system_tables st ON ett.table_id = st.id
			WHERE ett.employee_id IN ?
			ORDER BY ett.order_index NULLS LAST, name
		`, empIDs)
	}

	// Citizenships для employees.
	if len(bctx.Employees) > 0 {
		ids := make([]int, 0)
		for _, e := range bctx.Employees {
			if e.CitizenshipID != nil {
				ids = append(ids, *e.CitizenshipID)
			}
		}
		if len(ids) > 0 {
			var cz []models.Citizenship
			s.db.WithContext(ctx).Where("id IN ?", ids).Find(&cz)
			for _, c := range cz {
				bctx.Citizenships[c.ID] = c.Name
			}
		}
	}

	// Места разгрузки вложения (attachment_unload_places): для items - единственный
	// источник, для cars дублирует дедуп-union мест машин.
	s.db.WithContext(ctx).Raw(`
		SELECT up.name
		FROM attachment_unload_places aup
		JOIN unload_places up ON aup.unload_place_id = up.id
		WHERE aup.attachment_id = ?
		ORDER BY aup.order_index NULLS LAST, up.name
	`, att.ID).Scan(&bctx.AttachmentUnloadPlaces)

	// ТМЦ соседних вложений заявки: бланк одного вложения перечисляет ввозимый товар
	// из «Заявок на ввоз» той же заявки. Своё вложение тоже попадает сюда - для бланка
	// самого ввоза это краткая сводка рядом с построчной таблицей.
	bctx.ApplicationItems = loadApplicationItems(ctx, s.db, appID)

	// Машины соседних вложений заявки: бланк ввоза печатает транспорт из «Автозаявки».
	bctx.ApplicationCars = loadApplicationCars(ctx, s.db, appID)

	// Custom values для этого attachment.
	var values []models.AttachmentCustomValue
	s.db.WithContext(ctx).Where("attachment_id = ?", att.ID).Find(&values)
	for _, v := range values {
		bctx.CustomValues[v.CustomFieldID] = v.Value
	}

	return bctx, nil
}

// applyWrapIfMultiline включает перенос текста ячейке, в которую легло значение с
// переносами строк (перечень ТМЦ), и снимает заданную в шаблоне высоту строки. Без
// первого Excel покажет перечень одной строкой, без второго - обрежет по старой высоте.
// Оформление ячейки сохраняется: правим копию её собственного стиля.
// Объединённые ячейки авто-высоту не считают - это ограничение Excel, там высоту
// задаёт разметка шаблона.
func applyWrapIfMultiline(f *excelize.File, sheet, ref, value string) {
	if !strings.Contains(value, "\n") {
		return
	}
	styleID, err := f.GetCellStyle(sheet, ref)
	if err != nil {
		slog.Error("не удалось прочитать стиль ячейки бланка", "error", err, "cell", ref)
		return
	}
	style, err := f.GetStyle(styleID)
	if err != nil || style == nil {
		style = &excelize.Style{}
	}
	if style.Alignment == nil {
		style.Alignment = &excelize.Alignment{}
	}
	style.Alignment.WrapText = true
	newID, err := f.NewStyle(style)
	if err != nil {
		slog.Error("не удалось включить перенос текста в ячейке бланка", "error", err, "cell", ref)
		return
	}
	if err := f.SetCellStyle(sheet, ref, ref, newID); err != nil {
		slog.Error("не удалось применить стиль переноса текста", "error", err, "cell", ref)
		return
	}
	if _, row, err := excelize.CellNameToCoordinates(ref); err == nil {
		_ = f.SetRowHeight(sheet, row, -1)
	}
}

// loadApplicationItems собирает ТМЦ всех вложений заявки типа items. Ручные вложения
// (application_id NULL) сюда не попадают - они не принадлежат заявке. Позиции непринятого
// дополнения тоже: перечень печатается второй секцией того же бланка допуска, что и
// собственный список вложения (#1685).
// Поля приёмника перечислены плоско: у анонимно встроенной структуры gorm молча не
// маппит поля, и весь перечень пришёл бы пустым.
func loadApplicationItems(ctx context.Context, db *gorm.DB, appID int) []ApplicationItemRow {
	var rows []struct {
		Name       *string `gorm:"column:name"`
		Count      *int    `gorm:"column:count"`
		SourceName string  `gorm:"column:source_name"`
	}
	err := db.WithContext(ctx).Raw(`
		SELECT i.name AS name,
		       i.count AS count,
		       COALESCE(NULLIF(a.attachment_display_name, ''), NULLIF(a.attachment_name, ''), '') AS source_name
		FROM items i
		JOIN attachments a ON i.attachment_id = a.id
		WHERE a.application_id = ? AND a.attachment_type = 'items'
		  AND `+admittedSupplementCond("i")+`
		ORDER BY a.id, i.id
	`, appID).Scan(&rows).Error
	if err != nil {
		slog.Error("не удалось загрузить ТМЦ заявки для бланка", "error", err, "application", appID)
		return nil
	}
	out := make([]ApplicationItemRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, ApplicationItemRow{
			Name:       strings.TrimSpace(derefStr(r.Name)),
			Count:      r.Count,
			SourceName: strings.TrimSpace(r.SourceName),
		})
	}
	return out
}

// loadApplicationCars собирает машины всех вложений заявки типа cars. Ручные вложения
// (application_id NULL) сюда не попадают - они не принадлежат заявке.
func loadApplicationCars(ctx context.Context, db *gorm.DB, appID int) []ApplicationCarRow {
	var rows []struct {
		Number     *string `gorm:"column:number"`
		Mark       *string `gorm:"column:mark"`
		Brand      *string `gorm:"column:brand"`
		SourceName string  `gorm:"column:source_name"`
	}
	err := db.WithContext(ctx).Raw(`
		SELECT c.car_number AS number,
		       c.mark_name AS mark,
		       c.car_brand AS brand,
		       COALESCE(NULLIF(a.attachment_display_name, ''), NULLIF(a.attachment_name, ''), '') AS source_name
		FROM cars c
		JOIN attachments a ON c.attachment_id = a.id
		WHERE a.application_id = ? AND a.attachment_type = 'cars'
		ORDER BY a.id, c.id
	`, appID).Scan(&rows).Error
	if err != nil {
		slog.Error("не удалось загрузить транспорт заявки для бланка", "error", err, "application", appID)
		return nil
	}
	out := make([]ApplicationCarRow, 0, len(rows))
	for _, r := range rows {
		// Марку форма пишет в mark_name, у старых заявок она осталась в car_brand.
		mark := strings.TrimSpace(derefStr(r.Mark))
		if mark == "" {
			mark = strings.TrimSpace(derefStr(r.Brand))
		}
		out = append(out, ApplicationCarRow{
			Number:     strings.TrimSpace(derefStr(r.Number)),
			Mark:       mark,
			SourceName: strings.TrimSpace(r.SourceName),
		})
	}
	return out
}

// groupNamesByOwner выполняет запрос вида (owner_id, name) и раскладывает имена по
// владельцу с сохранением порядка сортировки запроса.
func groupNamesByOwner(ctx context.Context, db *gorm.DB, query string, ids []int) map[int][]string {
	out := make(map[int][]string, len(ids))
	if len(ids) == 0 {
		return out
	}
	var rows []struct {
		OwnerID int    `gorm:"column:owner_id"`
		Name    string `gorm:"column:name"`
	}
	if err := db.WithContext(ctx).Raw(query, ids).Scan(&rows).Error; err != nil {
		slog.Error("не удалось загрузить привязки для бланка", "error", err)
		return out
	}
	for _, r := range rows {
		out[r.OwnerID] = append(out[r.OwnerID], r.Name)
	}
	return out
}

// loadApprovers возвращает согласовавших заявку в порядке согласования: под подписью
// бланка стоит тот, кто согласовал, а признак обязательности решает, кого именно писать
// (см. approversForSignature).
func (s *attachmentBlankService) loadApprovers(ctx context.Context, appID int) []Approver {
	var responsible []models.ApplicationResponsibleUser
	s.db.WithContext(ctx).
		Preload("User").
		Where("application_id = ? AND approval_status = ?", appID, "approved").
		Order("approval_datetime ASC").
		Find(&responsible)

	out := make([]Approver, 0, len(responsible))
	for i := range responsible {
		u := responsible[i].User
		out = append(out, Approver{
			LastName:   derefStr(u.LastName),
			FirstName:  derefStr(u.FirstName),
			MiddleName: derefStr(u.MiddleName),
			Required:   responsible[i].RequiredApproval,
		})
	}
	return out
}
