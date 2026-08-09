package services

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/normalize"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
)

// parseRows разбирает и валидирует строки списка от ListStartRow до конца листа теми
// же правилами, что форма подачи (см. пакет docs эпика blank-import, срез C3). Пустые
// строки (ни одна списочная колонка не заполнена) пропускаются молча - тот же критерий,
// которым гейт файла (C1C2) уже посчитал Summary.Read.
func (s *attachmentImportService) parseRows(ctx context.Context, f *excelize.File, template *models.AttachmentTemplate, ua *models.UniqueAttachment) ([]ImportRowResult, error) {
	sheet := f.GetSheetName(0)
	allRows, err := f.GetRows(sheet)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не удалось прочитать список из файла")
	}

	var overrides []models.AttachmentFieldConfig
	if err := s.db.WithContext(ctx).
		Where("unique_attachment_id = ?", ua.ID).
		Find(&overrides).Error; err != nil {
		return nil, apperr.Internal("Не удалось загрузить настройку полей", err)
	}
	merged := MergeFieldConfig(ua.AttachmentType, overrides)

	switch ua.AttachmentType {
	case "people":
		return s.parseEmployeeRows(ctx, allRows, template, merged)
	case "cars":
		return s.parseVehicleRows(ctx, allRows, template, merged)
	case "items":
		return parseItemRows(allRows, template, merged), nil
	default:
		return []ImportRowResult{}, nil
	}
}

// listRowNumbers перечисляет номера строк Excel (1-based), у которых заполнена хотя бы
// одна списочная колонка - те же строки, что уже посчитал countListRows в гейте файла.
func listRowNumbers(allRows [][]string, template *models.AttachmentTemplate) []int {
	cols := listDataColumns(template.Mappings)
	nums := make([]int, 0, len(allRows))
	for rowIdx := template.ListStartRow; rowIdx <= len(allRows); rowIdx++ {
		if rowHasListData(allRows[rowIdx-1], cols) {
			nums = append(nums, rowIdx)
		}
	}
	return nums
}

// fieldColumns - колонка (1-based) для каждого суффикса field_path (после префикса типа,
// напр. "employee.") среди списочных mappings шаблона. Несколько mappings на один и тот
// же путь - берёт первый по порядку (стабильный порядок ID).
func fieldColumns(mappings []models.AttachmentTemplateMapping, prefix string) map[string]int {
	out := make(map[string]int)
	for _, m := range mappings {
		if !m.IsListField || !strings.HasPrefix(m.FieldPath, prefix) {
			continue
		}
		key := strings.TrimPrefix(m.FieldPath, prefix)
		if _, ok := out[key]; ok {
			continue
		}
		col, _, err := excelize.CellNameToCoordinates(m.CellRef)
		if err != nil {
			continue
		}
		out[key] = col
	}
	return out
}

// cellAt читает значение колонки (1-based) строки, trim-ленное. Колонка не размечена
// (0) или строка короче - пустая строка, не паника.
func cellAt(row []string, col int) string {
	idx := col - 1
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// splitFullName разбирает склеенное ФИО (админ смаппил одну колонку employee.full_name
// вместо трёх отдельных) на фамилию/имя/отчество по пробелам: первое слово - фамилия,
// второе - имя, остаток - отчество. Разбор грубый (реальные ФИО не всегда идут в этом
// порядке), поэтому вызывающая сторона обязана пометить строку предупреждением
// "проверьте разбор", а не молча доверять результату.
func splitFullName(full string) (last, first, middle string) {
	parts := strings.Fields(full)
	switch len(parts) {
	case 0:
		return "", "", ""
	case 1:
		return parts[0], "", ""
	case 2:
		return parts[0], parts[1], ""
	default:
		return parts[0], parts[1], strings.Join(parts[2:], " ")
	}
}

// parseEmployeeRows - построчный разбор и валидация списка сотрудников. Справочник
// гражданств и чёрный список людей грузятся ОДНИМ запросом каждый до цикла по строкам
// (не на строку - person_blacklist_service.FindSimilar делает полный просмотр на
// каждый вызов, на 2000 строк это тысячи round-trip, см. lessons/backend.md).
func (s *attachmentImportService) parseEmployeeRows(ctx context.Context, allRows [][]string, template *models.AttachmentTemplate, merged []models.MergedField) ([]ImportRowResult, error) {
	cols := fieldColumns(template.Mappings, "employee.")

	var citizenships []models.Citizenship
	if err := s.db.WithContext(ctx).Where("is_active = ?", true).Find(&citizenships).Error; err != nil {
		return nil, apperr.Internal("Не удалось загрузить справочник гражданств", err)
	}
	citByKey := make(map[string]models.Citizenship, len(citizenships))
	for _, c := range citizenships {
		citByKey[normalize.Name(c.Name)] = c
	}

	var blacklist []models.PersonBlacklist
	if err := s.db.WithContext(ctx).Where("is_active = ?", true).Find(&blacklist).Error; err != nil {
		return nil, apperr.Internal("Не удалось загрузить чёрный список", err)
	}
	blSet := make(map[string]string, len(blacklist))
	for _, b := range blacklist {
		middle := ""
		if b.MiddleName != nil {
			middle = *b.MiddleName
		}
		blSet[blacklistPersonKey(b.LastName, b.FirstName, middle)] = b.Reason
	}

	dedup := newEmployeeDedup()
	rowNums := listRowNumbers(allRows, template)
	rows := make([]ImportRowResult, 0, len(rowNums))

	for _, rowIdx := range rowNums {
		row := allRows[rowIdx-1]

		last := cellAt(row, cols["last_name"])
		first := cellAt(row, cols["first_name"])
		middle := cellAt(row, cols["middle_name"])
		var warnings []string

		// Склеенное ФИО (одна колонка employee.full_name вместо трёх): разбираем и
		// помечаем как требующее проверки (решение владельца, blank-import C3).
		if last == "" && first == "" {
			if col, ok := cols["full_name"]; ok {
				if full := cellAt(row, col); full != "" {
					last, first, middle = splitFullName(full)
					warnings = append(warnings, fmtWarnFullNameSplit(full, last, first, middle))
				}
			}
		}

		last, first, middle, nameWarnings := fixNameLatin(last, first, middle)
		warnings = append(warnings, nameWarnings...)

		position := cellAt(row, cols["position"])
		passport := cellAt(row, cols["passport_series_number"])
		patent := cellAt(row, cols["patent_number"])
		permission := cellAt(row, cols["other_permission"])
		citizenshipRaw := cellAt(row, cols["citizenship"])

		emp := EmployeeInput{
			LastName:             last,
			FirstName:            first,
			MiddleName:           nilIfBlank(middle),
			Position:             position,
			PassportSeriesNumber: passport,
			PatentNumber:         nilIfBlank(patent),
			OtherPermission:      nilIfBlank(permission),
		}

		var errs []ImportRowError
		citizenship, found := resolveCitizenship(citizenshipRaw, citByKey)
		if !found {
			errs = append(errs, fmtErrCitizenshipNotFound(citizenshipRaw))
		} else if citizenship != nil {
			emp.CitizenshipID = citizenship.ID
		}

		errs = append(errs, requiredEmployeeErrors(emp, merged)...)
		errs = append(errs, patentErrors(emp, merged, citizenship)...)
		errs = append(errs, checkFieldLength("people", "last_name", "Фамилия", last)...)
		errs = append(errs, checkFieldLength("people", "first_name", "Имя", first)...)
		errs = append(errs, checkFieldLength("people", "middle_name", "Отчество", middle)...)
		errs = append(errs, checkFieldLength("people", "position", "Должность", position)...)

		fioKey := fioDedupKey(last, first, middle)
		if dup := dedup.checkAndRecord(rowIdx, passport, fioKey); dup != "" {
			errs = append(errs, errDuplicateInFile(dup))
		}

		if reason, blocked := blSet[blacklistPersonKey(last, first, middle)]; blocked && fioKey != "" {
			errs = append(errs, fmtErrEmployeeBlacklisted(last, first, middle, reason))
		}

		rows = append(rows, ImportRowResult{
			RowNumber: rowIdx,
			Employee:  &emp,
			Errors:    emptyErrorsIfNil(errs),
			Warnings:  emptyIfNil(warnings),
		})
	}
	return rows, nil
}

// parseVehicleRows - построчный разбор и валидация списка машин. Места разгрузки и
// таблицы "Проезд" в файле не читаются: по решению владельца эпика они задаются на
// сайте на весь импорт целиком (см. context.md эпика blank-import), поэтому
// requiredVehicleErrors их не проверяет здесь - валидация переносится в интерфейс
// импорта (срез D1D2), где эти поля наконец получают значение.
func (s *attachmentImportService) parseVehicleRows(ctx context.Context, allRows [][]string, template *models.AttachmentTemplate, merged []models.MergedField) ([]ImportRowResult, error) {
	cols := fieldColumns(template.Mappings, "car.")

	var blacklist []models.VehicleBlacklist
	if err := s.db.WithContext(ctx).Where("is_active = ?", true).Find(&blacklist).Error; err != nil {
		return nil, apperr.Internal("Не удалось загрузить чёрный список машин", err)
	}
	blSet := make(map[string]string, len(blacklist))
	for _, b := range blacklist {
		blSet[blacklistVehicleKey(b.CarNumber, b.MarkName)] = b.Reason
	}

	formats, err := s.loadActiveLicensePlateFormats(ctx)
	if err != nil {
		return nil, err
	}

	dedup := newVehicleDedup()
	rowNums := listRowNumbers(allRows, template)
	rows := make([]ImportRowResult, 0, len(rowNums))

	for _, rowIdx := range rowNums {
		row := allRows[rowIdx-1]
		number := cellAt(row, cols["car_number"])
		mark := cellAt(row, cols["mark_name"])
		var warnings []string
		var plateFormatErr *ImportRowError

		// "По факту" - существующий особый случай (не опознаёт конкретную машину, см.
		// vehicleByFactPlate), формат номера для него не проверяется. Значение приводим
		// к каноническому виду: дальше по цепочке (форма, привязка к организации)
		// сравнение идёт строгим равенством, и "по факту" из файла туда не пройдёт.
		if isByFactPlate(number) {
			number = vehicleByFactCanonical
		}
		if number != "" && !isByFactPlate(number) {
			if match, ok := matchLicensePlate(number, formats); ok {
				if match.Changed {
					warnings = append(warnings, fmtWarnPlateFixed(number, match.Formatted))
				}
				number = match.Formatted
			} else {
				formatErr := fmtErrPlateFormatNotFound(number)
				plateFormatErr = &formatErr
			}
		}

		veh := VehicleInput{CarNumber: number, CarBrand: mark}

		var errs []ImportRowError
		errs = append(errs, requiredVehicleErrors(veh, merged)...)
		errs = append(errs, checkFieldLengthMax("cars", "number", "Номер ТС", number, maxImportCarNumberLen)...)
		errs = append(errs, checkFieldLength("cars", "mark", "Марка ТС", mark)...)
		if plateFormatErr != nil {
			errs = append(errs, *plateFormatErr)
		}

		if dup := dedup.checkAndRecord(rowIdx, number); dup != "" {
			errs = append(errs, errDuplicateInFile(dup))
		}

		if number != "" && mark != "" {
			if reason, blocked := blSet[blacklistVehicleKey(number, mark)]; blocked {
				errs = append(errs, fmtErrVehicleBlacklisted(number, mark, reason))
			}
		}

		rows = append(rows, ImportRowResult{
			RowNumber: rowIdx,
			Vehicle:   &veh,
			Errors:    emptyErrorsIfNil(errs),
			Warnings:  emptyIfNil(warnings),
		})
	}
	return rows, nil
}

// parseItemRows - построчный разбор списка ТМЦ. Проще людей и машин: реестр не знает
// ни гражданства, ни чёрного списка для имущества - только обязательность полей и длина.
func parseItemRows(allRows [][]string, template *models.AttachmentTemplate, merged []models.MergedField) []ImportRowResult {
	cols := fieldColumns(template.Mappings, "item.")
	rowNums := listRowNumbers(allRows, template)
	rows := make([]ImportRowResult, 0, len(rowNums))

	for _, rowIdx := range rowNums {
		row := allRows[rowIdx-1]
		name := cellAt(row, cols["name"])
		countRaw := cellAt(row, cols["count"])
		count, _ := strconv.Atoi(countRaw)

		item := ItemInput{Name: name, Count: count}

		var errs []ImportRowError
		errs = append(errs, requiredItemErrors(item, merged)...)
		errs = append(errs, checkFieldLengthMax("items", "item_name", "Наименование ТМЦ", name, maxImportItemNameLen)...)

		rows = append(rows, ImportRowResult{
			RowNumber: rowIdx,
			Item:      &item,
			Errors:    emptyErrorsIfNil(errs),
			Warnings:  emptyIfNil(nil),
		})
	}
	return rows
}

// fixNameLatin применяет normalize.FixLatinInName к трём частям ФИО и собирает
// предупреждения по тем, где нашлась латиница - показ исправленного варианта, а не
// блокирующая ошибка (решение владельца, blank-import C3).
func fixNameLatin(last, first, middle string) (string, string, string, []string) {
	var warnings []string

	fixedLast, foundLast := normalize.FixLatinInName(last)
	if foundLast {
		warnings = append(warnings, fmtWarnLatinFixed("Фамилия", last, fixedLast))
	}
	fixedFirst, foundFirst := normalize.FixLatinInName(first)
	if foundFirst {
		warnings = append(warnings, fmtWarnLatinFixed("Имя", first, fixedFirst))
	}
	fixedMiddle, foundMiddle := normalize.FixLatinInName(middle)
	if foundMiddle {
		warnings = append(warnings, fmtWarnLatinFixed("Отчество", middle, fixedMiddle))
	}

	return fixedLast, fixedFirst, fixedMiddle, warnings
}

func emptyIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// emptyErrorsIfNil - то же для причин отказа: пустой массив в JSON, а не null, иначе
// фронт разбирал бы отсутствие ошибок отдельной веткой.
func emptyErrorsIfNil(s []ImportRowError) []ImportRowError {
	if s == nil {
		return []ImportRowError{}
	}
	return s
}
