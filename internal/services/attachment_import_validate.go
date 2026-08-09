package services

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"systemburo/internal/models"
	"systemburo/internal/normalize"
)

// maxImportTextFieldLen - потолок длины текстовых полей сотрудника/машины при импорте,
// зеркало gorm-тега size:100 у last_name/first_name/middle_name/position (models.Employee)
// и car_brand/mark_name (models.Car). Без этой проверки значение молча обрежется при
// вставке (см. lessons/backend.md) - импорт обязан поймать это ДО записи в БД.
const maxImportTextFieldLen = 100

// importExcludedEmployeeKeys - ключи реестра, обязательность которых импорт НЕ проверяет
// построчно: места прохода задаются на сайте на весь список целиком (решение владельца
// эпика blank-import, см. context.md), а не читаются из файла построчно.
var importExcludedEmployeeKeys = map[string]bool{
	"target_tables": true,
	// "patent" - собственная citizenship-зависимая проверка (patentErrors), не общий
	// цикл: MergeFieldConfig.Required для него по умолчанию false, а форма подачи
	// (EmployeeForm.vue) требует патент/разрешение по признаку гражданства, а не по
	// этому тумблеру - см. patentErrors ниже.
	"patent": true,
}

// importExcludedVehicleKeys - зеркало importExcludedEmployeeKeys для машин: места
// разгрузки и таблицы "Проезд" тоже задаются на сайте, а не в файле.
var importExcludedVehicleKeys = map[string]bool{
	"unloading_places": true,
	"passage_tables":   true,
}

// mergedFieldByKey ищет поле реестра по ключу среди смерженных с оверрайдами полей типа.
func mergedFieldByKey(merged []models.MergedField, key string) (models.MergedField, bool) {
	for _, m := range merged {
		if m.Key == key {
			return m, true
		}
	}
	return models.MergedField{}, false
}

// requiredEmployeeErrors проверяет обязательные поля сотрудника через MergeFieldConfig
// (реестр + оверрайды, а НЕ requiredFieldKeys submit'а - иначе импорт был бы мягче
// ручного ввода, ключевая находка эпика blank-import) и employeeFieldPresent - тот же
// предикат, что использует форма подачи.
func requiredEmployeeErrors(e EmployeeInput, merged []models.MergedField) []string {
	var errs []string
	for _, f := range merged {
		if !f.Required || importExcludedEmployeeKeys[f.Key] {
			continue
		}
		if !employeeFieldPresent(e, f.Key) {
			errs = append(errs, fmt.Sprintf("Поле «%s» обязательно для заполнения", f.Label))
		}
	}
	return errs
}

// requiredVehicleErrors - зеркало requiredEmployeeErrors для машин.
func requiredVehicleErrors(v VehicleInput, merged []models.MergedField) []string {
	var errs []string
	for _, f := range merged {
		if !f.Required || importExcludedVehicleKeys[f.Key] {
			continue
		}
		if !vehicleFieldPresent(v, f.Key) {
			errs = append(errs, fmt.Sprintf("Поле «%s» обязательно для заполнения", f.Label))
		}
	}
	return errs
}

// requiredItemErrors - зеркало requiredEmployeeErrors для ТМЦ, без исключений: у items
// нет полей, назначаемых на сайте отдельно от файла.
func requiredItemErrors(i ItemInput, merged []models.MergedField) []string {
	var errs []string
	for _, f := range merged {
		if !f.Required {
			continue
		}
		if !itemFieldPresent(i, f.Key) {
			errs = append(errs, fmt.Sprintf("Поле «%s» обязательно для заполнения", f.Label))
		}
	}
	return errs
}

// patentErrors зеркалит правило EmployeeForm.vue (rules для patent, ~строки 510-515):
// патент проверяется НЕ по тумблеру "обязательно" поля patent, а по признаку гражданства
// patent_required, и только когда поле patent вообще видимо в конфиге вложения.
// citizenship=nil (гражданство не заполнено или не найдено) - проверка пропускается,
// об этом уже сообщает отдельная ошибка резолва гражданства.
func patentErrors(e EmployeeInput, merged []models.MergedField, citizenship *models.Citizenship) []string {
	patentCfg, ok := mergedFieldByKey(merged, "patent")
	if !ok || !patentCfg.Visible {
		return nil
	}
	if citizenship == nil || !citizenship.PatentRequired {
		return nil
	}
	if employeeFieldPresent(e, "patent") {
		return nil
	}
	return []string{fmt.Sprintf("Для гражданства %q нужен номер патента или иное разрешение на работы", citizenship.Name)}
}

// resolveCitizenship сопоставляет сырую строку гражданства из файла со справочником по
// названию с нормализацией (normalize.Name - регистр, ё/е, латинские омоглифы, лишние
// пробелы). Пустая строка - не ошибка (гражданство не заполнено, за required отвечает
// requiredEmployeeErrors); непустая без совпадения - found=false, вызывающая сторона
// формирует ошибку строки.
func resolveCitizenship(raw string, byNormalizedName map[string]models.Citizenship) (citizenship *models.Citizenship, found bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, true
	}
	c, ok := byNormalizedName[normalize.Name(trimmed)]
	if !ok {
		return nil, false
	}
	return &c, true
}

// checkFieldLength проверяет текстовое поле против потолка схемы (size:100): дальше
// значение молча обрежется на вставке в БД, если пропустить строку как есть.
func checkFieldLength(label, value string) []string {
	if utf8.RuneCountInString(value) <= maxImportTextFieldLen {
		return nil
	}
	return []string{fmt.Sprintf("Поле «%s» длиннее %d символов - сократите значение", label, maxImportTextFieldLen)}
}

// --- Дубли внутри файла ---

// normSpaces - нижний регистр со схлопнутыми пробелами (зеркало normText из
// frontend/src/utils/applicationDuplicates.js).
func normSpaces(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// normCompactKey - то же самое, но без пробелов вовсе (зеркало normCompact оттуда же):
// паспорт и госномер набирают по-разному, сравниваем без пробелов.
func normCompactKey(s string) string {
	return strings.ReplaceAll(normSpaces(s), " ", "")
}

// fioDedupKey - ключ ФИО для сравнения дублей и чёрного списка. Пустой, если ни одна
// часть не заполнена - пустой ключ никогда не считается совпадением.
func fioDedupKey(last, first, middle string) string {
	parts := []string{normSpaces(last), normSpaces(first), normSpaces(middle)}
	if parts[0] == "" && parts[1] == "" && parts[2] == "" {
		return ""
	}
	return strings.Join(parts, "|")
}

// employeeDedup - построчный поиск дублей внутри файла теми же правилами, что
// isSameEmployee (frontend/src/utils/applicationDuplicates.js): паспорт приоритетнее
// ФИО, но ТОЛЬКО когда паспорт заполнен у ОБЕИХ сравниваемых строк - иначе сравнение
// падает на ФИО. Хэш-карты вместо O(n^2): до 2000 строк на файл.
type employeeDedup struct {
	byPassport      map[string]int // паспорт -> номер первой строки
	byFIOAny        map[string]int // ФИО -> номер первой строки (независимо от паспорта)
	byFIONoPassport map[string]int // ФИО -> номер первой строки БЕЗ паспорта
}

func newEmployeeDedup() *employeeDedup {
	return &employeeDedup{
		byPassport:      map[string]int{},
		byFIOAny:        map[string]int{},
		byFIONoPassport: map[string]int{},
	}
}

// checkAndRecord ищет более раннюю совпадающую строку и запоминает текущую для
// следующих. Пустой результат - совпадений нет.
func (d *employeeDedup) checkAndRecord(rowNumber int, passport, fioKey string) string {
	pKey := normCompactKey(passport)
	msg := ""

	if pKey != "" {
		if first, ok := d.byPassport[pKey]; ok {
			msg = fmt.Sprintf("Дублирует строку %d: тот же паспорт", first)
		}
	}
	if msg == "" && fioKey != "" {
		if pKey == "" {
			// Текущая строка без паспорта - правило "хотя бы одна сторона без
			// паспорта" выполнено автоматически, сравниваем со всеми по ФИО.
			if first, ok := d.byFIOAny[fioKey]; ok {
				msg = fmt.Sprintf("Дублирует строку %d: то же ФИО", first)
			}
		} else if first, ok := d.byFIONoPassport[fioKey]; ok {
			// Текущая строка с паспортом совпадает по ФИО только с более ранней
			// строкой БЕЗ паспорта: если бы та тоже имела паспорт, сравнение шло
			// бы по паспорту, а не по имени (см. isSameEmployee).
			msg = fmt.Sprintf("Дублирует строку %d: то же ФИО", first)
		}
	}

	if pKey != "" {
		if _, ok := d.byPassport[pKey]; !ok {
			d.byPassport[pKey] = rowNumber
		}
	}
	if fioKey != "" {
		if _, ok := d.byFIOAny[fioKey]; !ok {
			d.byFIOAny[fioKey] = rowNumber
		}
		if pKey == "" {
			if _, ok := d.byFIONoPassport[fioKey]; !ok {
				d.byFIONoPassport[fioKey] = rowNumber
			}
		}
	}
	return msg
}

// vehicleDedup - построчный поиск дублей машин по номеру (зеркало isSameVehicle):
// "По факту" не опознаёт конкретную машину, таких строк может быть сколько угодно.
type vehicleDedup struct {
	byPlate map[string]int
}

func newVehicleDedup() *vehicleDedup {
	return &vehicleDedup{byPlate: map[string]int{}}
}

const vehicleByFactPlate = "по факту"

func (d *vehicleDedup) checkAndRecord(rowNumber int, plate string) string {
	key := normCompactKey(plate)
	if key == "" || normCompactKey(vehicleByFactPlate) == key {
		return ""
	}
	if first, ok := d.byPlate[key]; ok {
		return fmt.Sprintf("Дублирует строку %d: тот же номер машины", first)
	}
	d.byPlate[key] = rowNumber
	return ""
}

// --- Чёрный список: ключи пакетного сравнения ---

// blacklistPersonKey - ключ точного совпадения ЧС людей, зеркало SQL-сравнения в
// PersonBlacklistService.Check (LOWER(TRIM(...)) по каждой части). Строится один раз
// пакетно для всей активной таблицы person_blacklists, вместо запроса на строку файла.
func blacklistPersonKey(last, first, middle string) string {
	return strings.ToLower(strings.TrimSpace(last)) + "|" +
		strings.ToLower(strings.TrimSpace(first)) + "|" +
		strings.ToLower(strings.TrimSpace(middle))
}

// blacklistVehicleKey - ключ точного совпадения ЧС машин, зеркало
// VehicleBlacklistService.CheckByName (по номеру + имени марки - у импортированной
// строки нет mark_id, только текст марки из ячейки бланка).
func blacklistVehicleKey(carNumber, markName string) string {
	return strings.ToLower(strings.TrimSpace(carNumber)) + "|" + strings.ToLower(strings.TrimSpace(markName))
}

// --- Тексты ошибок/предупреждений ---

func fmtErrCitizenshipNotFound(raw string) string {
	return fmt.Sprintf("Гражданство %q не найдено в справочнике", strings.TrimSpace(raw))
}

// joinFIO склеивает непустые части ФИО через один пробел - страхует от лишнего
// пробела, когда отчество (или имя) не заполнено.
func joinFIO(last, first, middle string) string {
	parts := make([]string, 0, 3)
	for _, p := range []string{last, first, middle} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " ")
}

func fmtErrEmployeeBlacklisted(last, first, middle, reason string) string {
	return fmt.Sprintf("Человек %s в чёрном списке: %s", joinFIO(last, first, middle), reason)
}

func fmtErrVehicleBlacklisted(number, mark, reason string) string {
	return fmt.Sprintf("Машина %s %s в чёрном списке: %s", number, mark, reason)
}

func fmtWarnLatinFixed(label, original, fixed string) string {
	return fmt.Sprintf("Поле «%s»: похожие латинские буквы заменены на русские, %q -> %q", label, original, fixed)
}

func fmtWarnFullNameSplit(full, last, first, middle string) string {
	return fmt.Sprintf("ФИО распознано из одного поля %q как %q - проверьте разбор", full, joinFIO(last, first, middle))
}
