package services

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"

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
	ApproverName     string         // ФИО согласовавшего
	// AttachmentUnloadPlaces - имена мест разгрузки вложения в порядке привязки. Для
	// вложения-имущества это единственный источник мест: у ТМЦ своих машин нет (#706).
	AttachmentUnloadPlaces []string
	// Привязки элементов, по одной записи на строку списка (#1454): полные места
	// разгрузки машины (car_unload_places), её посты «Проезд» (car_target_tables) и
	// места прохода сотрудника (employee_target_tables).
	CarUnloadPlaces      map[int][]string // car_id → имена мест
	CarPassageTables     map[int][]string // car_id → имена постов
	EmployeeTargetTables map[int][]string // employee_id → имена постов
	// ApplicationItems - ТМЦ всех «Заявок на ввоз» этой заявки, в порядке вложений.
	// Списочная секция бланка одна и занята его собственным типом (у заявки на работы -
	// сотрудниками), поэтому чужие ТМЦ перечисляются одной ячейкой через app_items.*.
	ApplicationItems []ApplicationItemRow
}

// ApplicationItemRow - позиция ТМЦ из вложения-соседа с названием вложения-источника:
// при нескольких «Заявках на ввоз» перечень объединяется, и происхождение позиции
// иначе теряется.
type ApplicationItemRow struct {
	Name       string
	Count      *int
	SourceName string
}

// AttachmentBlankService - генерация заполненных .xlsx-бланков на основе
// шаблона UniqueAttachment + данных заявки (#183, часть 2).
type AttachmentBlankService interface {
	GenerateBlank(ctx context.Context, applicationID, attachmentID int) (io.Reader, string, error)
}

type attachmentBlankService struct {
	db *gorm.DB
}

// NewAttachmentBlankService создаёт сервис.
func NewAttachmentBlankService(db *gorm.DB) AttachmentBlankService {
	return &attachmentBlankService{db: db}
}

// GenerateBlank возвращает Reader с готовым .xlsx и filename.
// Шаги:
//  1. Загрузить шаблон (attachment_templates + mappings) по unique_attachment_id.
//  2. Собрать BlankContext из заявки.
//  3. Открыть .xlsx через excelize, проставить значения в ячейки.
//  4. Для list-fields - заполнить строки списка с авторасширением.
//  5. Сохранить в buffer, вернуть.
func (s *attachmentBlankService) GenerateBlank(ctx context.Context, applicationID, attachmentID int) (io.Reader, string, error) {
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

	// 3. Открыть шаблон.
	f, err := excelize.OpenFile(template.FilePath)
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

	// 5. List-fields с авторасширением.
	insertedRows := 0
	if len(listMappings) > 0 {
		insertedRows = s.fillListSection(f, sheet, &template, listMappings, staticCells, bctx)
	}

	// 6. Шапка столбцов сквозная: на следующей странице таблица начинается с неё.
	repeatHeaderOnEachPage(f, sheet, &template)

	// 7. Записать в buffer.
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", echo.NewHTTPError(http.StatusInternalServerError, "Ошибка генерации файла")
	}

	// 8. Добавленные строки сдвинули диапазоны условного форматирования, но не формулы
	// внутри правил - дошиваем их сами. Сбой здесь бланк не отменяет: отдаём файл со
	// старыми формулами правил, но громко пишем в лог.
	out := buf.Bytes()
	if insertedRows > 0 {
		shifted, shiftErr := shiftConditionalFormatFormulas(out, template.ListEndRow+1, insertedRows)
		if shiftErr != nil {
			slog.Error("не удалось сдвинуть формулы условного форматирования бланка",
				"error", shiftErr, "template", template.ID, "inserted", insertedRows)
		} else {
			out = shifted
		}
	}

	filename := formatBlankFilename(bctx)
	return bytes.NewReader(out), filename, nil
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
func repeatedListCells(t *models.AttachmentTemplate, staticCells map[string]string) []repeatedCell {
	if t.ListStartRow < 1 || t.ListEndRow < t.ListStartRow {
		return nil
	}
	out := make([]repeatedCell, 0, len(staticCells))
	for ref, val := range staticCells {
		col, row, err := excelize.CellNameToCoordinates(ref)
		if err != nil || row < t.ListStartRow || row > t.ListEndRow {
			continue
		}
		out = append(out, repeatedCell{col: col, value: val})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].col < out[j].col })
	return out
}

// fillListSection заполняет строки списка (cars/employees/items), при необходимости
// расширяя шаблон через InsertRows + копирование стилей последней шаблонной строки.
// staticCells - уже записанные значения обычных полей: те из них, что попали в строки
// списка, повторяются по строкам вместе со списочными.
// Возвращает число реально добавленных строк: на него потом сдвигаются формулы правил
// условного форматирования.
func (s *attachmentBlankService) fillListSection(f *excelize.File, sheet string, t *models.AttachmentTemplate, mappings []models.AttachmentTemplateMapping, staticCells map[string]string, bctx *BlankContext) int {
	// Список задаёт тип вложения, а не порядок привязок (#1454): у items-вложения нет
	// машин, и привязка car.* не должна отменять заполнение ТМЦ. Раньше тип брался по
	// первому list-маппингу, из-за чего боевой бланк "Заявка на ввоз" с привязками к
	// номеру машины отдавал пустую таблицу имущества.
	prefix, count := listSource(bctx)
	if count == 0 {
		return 0
	}
	// Привязки чужих групп заполнять нечем: их источник у этого вложения пуст.
	own := make([]models.AttachmentTemplateMapping, 0, len(mappings))
	for _, m := range mappings {
		if strings.HasPrefix(m.FieldPath, prefix) {
			own = append(own, m)
		}
	}
	if len(own) == 0 {
		return 0
	}
	mappings = own

	// Записей больше, чем строк в шаблоне - добавляем недостающие копией последней
	// строки списка: так новая строка получает её оформление. Отдельный InsertRows
	// здесь не нужен и вреден - DuplicateRowTo сам вставляет строку со сдвигом, и
	// вдвоём они добавляли вдвое больше строк, оставляя в бланке пустоты, а разметку
	// под таблицей уводя ниже, чем нужно (#1480).
	inserted := 0
	if count > t.MaxListRows && t.MaxListRows > 0 {
		extra := count - t.MaxListRows
		for i := 0; i < extra; i++ {
			if err := f.DuplicateRowTo(sheet, t.ListEndRow, t.ListEndRow+1+i); err != nil {
				slog.Error("не удалось расширить список бланка", "error", err, "row", t.ListEndRow+1+i)
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

	repeated := repeatedListCells(t, staticCells)

	for idx := 0; idx < count; idx++ {
		row := t.ListStartRow + idx
		for _, m := range mappings {
			col, _, err := excelize.CellNameToCoordinates(m.CellRef) //nolint:nestif // ok
			if err != nil {
				continue
			}
			cell, _ := excelize.CoordinatesToCellName(col, row)
			val := resolveValue(bctx, m.FieldPath, idx)
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

	// ApproverName: ФИО согласовавшего.
	bctx.ApproverName = s.resolveApproverName(ctx, appID)

	// Cars / employees / items - только для этого attachment.
	s.db.WithContext(ctx).Where("attachment_id = ?", att.ID).Order("id").Find(&bctx.Cars)
	s.db.WithContext(ctx).Where("attachment_id = ?", att.ID).Order("id").Find(&bctx.Employees)
	s.db.WithContext(ctx).Where("attachment_id = ?", att.ID).Order("id").Find(&bctx.Items)

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
// (application_id NULL) сюда не попадают - они не принадлежат заявке.
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

// resolveApproverName определяет ФИО согласовавшего заявку:
// - если есть обязательные согласующие (required_approval=true) с approval_status='approved',
//   берется последний по approval_datetime;
// - иначе берется первый согласовавший (approval_status='approved').
func (s *attachmentBlankService) resolveApproverName(ctx context.Context, appID int) string {
	var responsible []models.ApplicationResponsibleUser
	s.db.WithContext(ctx).
		Preload("User").
		Where("application_id = ? AND approval_status = ?", appID, "approved").
		Order("approval_datetime ASC").
		Find(&responsible)

	if len(responsible) == 0 {
		return ""
	}

	var required []models.ApplicationResponsibleUser
	for _, r := range responsible {
		if r.RequiredApproval {
			required = append(required, r)
		}
	}

	var approver *models.User
	if len(required) > 0 {
		approver = &required[len(required)-1].User
	} else {
		approver = &responsible[0].User
	}

	return joinFullName(derefStr(approver.LastName), derefStr(approver.FirstName), derefStr(approver.MiddleName))
}
