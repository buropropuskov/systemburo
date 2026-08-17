package services

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"systemburo/internal/models"
	"systemburo/internal/normalize"
)

// Потолки длины при импорте - зеркала gorm-тегов size у соответствующих полей. Без них
// значение молча обрежется при вставке (см. lessons/backend.md), а завышенный потолок
// отбивал бы значения, которые схема принимает. Расходиться с моделью нельзя в обе стороны.
const (
	// maxImportTextFieldLen - last_name/first_name/middle_name/position (models.Employee),
	// car_brand/mark_name (models.Car).
	maxImportTextFieldLen = 100
	// maxImportCarNumberLen - car_number (models.Car) уже остальных полей машины.
	maxImportCarNumberLen = 50
	// maxImportItemNameLen - name (models.Item).
	maxImportItemNameLen = 255
)

// ImportRowErrorCode - машинный код причины отказа строки.
type ImportRowErrorCode string

const (
	ImportErrFieldRequired      ImportRowErrorCode = "field_required"
	ImportErrFieldTooLong       ImportRowErrorCode = "field_too_long"
	ImportErrCitizenshipUnknown ImportRowErrorCode = "citizenship_unknown"
	ImportErrPatentRequired     ImportRowErrorCode = "patent_required"
	ImportErrPlateFormat        ImportRowErrorCode = "plate_format_unknown"
	ImportErrDuplicateInFile    ImportRowErrorCode = "duplicate_in_file"
	ImportErrBlacklisted        ImportRowErrorCode = "blacklisted"
)

// ImportRowError - причина, по которой строка не уходит в заявку. Text пишется для
// человека и переформулируется свободно; Code и Fixable - контракт с интерфейсом.
//
// Fixable считает сервер: сводка импорта (BlankImportResult.vue) правит строку прямо
// в таблице разбора и по этому признаку решает, разблокировать ли галочку. Раньше она
// выводила его сама, сверяя текст причины с префиксом "Поле «<подпись>»", и каждая
// формулировка вне шаблона молча блокировала строку навсегда - так галочка перестала
// работать у номера, не подошедшего ни под один формат.
type ImportRowError struct {
	Text    string             `json:"text"`
	Code    ImportRowErrorCode `json:"code"`
	Field   string             `json:"field,omitempty"`
	Fixable bool               `json:"fixable"`
}

// importInlineFixableFields - ключи реестра полей (attachment_fields_registry.go), у
// которых в таблице разбора есть своя ячейка правки. Паспорта и патента там нет и не
// будет (152-ФЗ), должности - нет колонки, поэтому их причины остаются блокирующими:
// такую строку заводят обычной формой. Список меняется только вместе с колонками
// таблицы разбора.
var importInlineFixableFields = map[string]map[string]bool{
	"people": {"last_name": true, "first_name": true, "middle_name": true, "citizenship": true},
	"cars":   {"number": true, "mark": true},
	"items":  {},
}

// importFieldFixable сообщает, правится ли поле прямо в таблице разбора.
func importFieldFixable(attachmentType, fieldKey string) bool {
	return importInlineFixableFields[attachmentType][fieldKey]
}

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
	// Согласие субъекта на обработку персональных данных в бланке не собирается: колонки
	// под него в файле нет, а заявитель подтверждает его на сайте один раз на весь
	// загружаемый список - тем же порядком, что и места прохода. Без исключения разбор
	// любого бланка падал бы на каждой строке.
	PDConsentFieldKey: true,
}

// importExcludedVehicleKeys - зеркало importExcludedEmployeeKeys для машин: места
// разгрузки и таблицы "Проезд" тоже задаются на сайте, а не в файле.
var importExcludedVehicleKeys = map[string]bool{
	"unloading_places": true,
	"passage_tables":   true,
	// Согласие подтверждается на сайте на весь список, см. importExcludedEmployeeKeys.
	PDConsentFieldKey: true,
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
func requiredEmployeeErrors(e EmployeeInput, merged []models.MergedField) []ImportRowError {
	var errs []ImportRowError
	for _, f := range merged {
		if !f.Required || importExcludedEmployeeKeys[f.Key] {
			continue
		}
		if !employeeFieldPresent(e, f.Key) {
			errs = append(errs, errFieldRequired("people", f.Key, f.Label))
		}
	}
	return errs
}

// requiredVehicleErrors - зеркало requiredEmployeeErrors для машин.
func requiredVehicleErrors(v VehicleInput, merged []models.MergedField) []ImportRowError {
	var errs []ImportRowError
	for _, f := range merged {
		if !f.Required || importExcludedVehicleKeys[f.Key] {
			continue
		}
		if !vehicleFieldPresent(v, f.Key) {
			errs = append(errs, errFieldRequired("cars", f.Key, f.Label))
		}
	}
	return errs
}

// requiredItemErrors - зеркало requiredEmployeeErrors для ТМЦ, без исключений: у items
// нет полей, назначаемых на сайте отдельно от файла.
func requiredItemErrors(i ItemInput, merged []models.MergedField) []ImportRowError {
	var errs []ImportRowError
	for _, f := range merged {
		if !f.Required {
			continue
		}
		if !itemFieldPresent(i, f.Key) {
			errs = append(errs, errFieldRequired("items", f.Key, f.Label))
		}
	}
	return errs
}

// errFieldRequired - незаполненное обязательное поле.
func errFieldRequired(attachmentType, fieldKey, label string) ImportRowError {
	return ImportRowError{
		Text:    fmt.Sprintf("Поле «%s» обязательно для заполнения", label),
		Code:    ImportErrFieldRequired,
		Field:   fieldKey,
		Fixable: importFieldFixable(attachmentType, fieldKey),
	}
}

// patentErrors зеркалит effectivePatentRequired из EmployeeForm.vue: оверрайд
// "обязательно" у поля patent делает патент обязательным ВСЕГДА, и только при его
// отсутствии решает признак гражданства patent_required. Проверка работает, лишь пока
// поле patent видимо в конфиге вложения. citizenship=nil (гражданство не заполнено или
// не найдено) - об этом сообщает отдельная ошибка резолва гражданства.
func patentErrors(e EmployeeInput, merged []models.MergedField, citizenship *models.Citizenship) []ImportRowError {
	patentCfg, ok := mergedFieldByKey(merged, "patent")
	if !ok || !patentCfg.Visible {
		return nil
	}
	byCitizenship := citizenship != nil && citizenship.PatentRequired
	if !patentCfg.Required && !byCitizenship {
		return nil
	}
	if employeeFieldPresent(e, "patent") {
		return nil
	}
	text := "Нужен номер патента или иное разрешение на работы"
	if byCitizenship {
		text = fmt.Sprintf("Для гражданства %q нужен номер патента или иное разрешение на работы", citizenship.Name)
	}
	return []ImportRowError{{
		Text:    text,
		Code:    ImportErrPatentRequired,
		Field:   "patent",
		Fixable: importFieldFixable("people", "patent"),
	}}
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
func checkFieldLength(attachmentType, fieldKey, label, value string) []ImportRowError {
	return checkFieldLengthMax(attachmentType, fieldKey, label, value, maxImportTextFieldLen)
}

// checkFieldLengthMax - для полей, у которых свой size в модели (номер машины, ТМЦ).
func checkFieldLengthMax(attachmentType, fieldKey, label, value string, max int) []ImportRowError {
	if utf8.RuneCountInString(value) <= max {
		return nil
	}
	return []ImportRowError{{
		Text:    fmt.Sprintf("Поле «%s» длиннее %d символов - сократите значение", label, max),
		Code:    ImportErrFieldTooLong,
		Field:   fieldKey,
		Fixable: importFieldFixable(attachmentType, fieldKey),
	}}
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

// errDuplicateInFile - совпадение с более ранней строкой того же файла. Правкой в
// таблице разбора не снимается: решение "это тот же человек/машина" принято по данным,
// которых в таблице нет (паспорт), а вторую копию заводить незачем.
func errDuplicateInFile(text string) ImportRowError {
	return ImportRowError{Text: text, Code: ImportErrDuplicateInFile}
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

// vehicleByFactCanonical - написание, которое пишет форма и с которым сравнивают
// строгим равенством VehicleForm.vue, CreateApplication.vue и UniversalBindingModal.vue.
// Импорт обязан приводить значение к нему, иначе "ПО ФАКТУ" из файла перестанет
// опознаваться как спецзначение ниже по цепочке.
const vehicleByFactCanonical = "По факту"

// isByFactPlate сообщает, является ли номер спецзначением "По факту" - сравнение
// нормализованное (регистр, пробелы), а не побайтовое: значение как в файле бланка
// (может прийти "по факту" или "По Факту"), так и введённое вручную (форма всегда
// пишет каноническое "По факту") обязаны опознаваться одинаково.
func isByFactPlate(plate string) bool {
	return normCompactKey(plate) == normCompactKey(vehicleByFactPlate)
}

func (d *vehicleDedup) checkAndRecord(rowNumber int, plate string) string {
	key := normCompactKey(plate)
	if key == "" || isByFactPlate(plate) {
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

// fmtErrCitizenshipNotFound - гражданство из файла не нашлось в справочнике. Исправимо:
// в таблице разбора гражданство выбирается из того же справочника.
func fmtErrCitizenshipNotFound(raw string) ImportRowError {
	return ImportRowError{
		Text:    fmt.Sprintf("Гражданство %q не найдено в справочнике", strings.TrimSpace(raw)),
		Code:    ImportErrCitizenshipUnknown,
		Field:   "citizenship",
		Fixable: importFieldFixable("people", "citizenship"),
	}
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

// Чёрный список правкой в таблице разбора не обходится: решение бюро о человеке или
// машине не переигрывается сменой написания.
func fmtErrEmployeeBlacklisted(last, first, middle, reason string) ImportRowError {
	return ImportRowError{
		Text: fmt.Sprintf("Человек %s в чёрном списке: %s", joinFIO(last, first, middle), reason),
		Code: ImportErrBlacklisted,
	}
}

func fmtErrVehicleBlacklisted(number, mark, reason string) ImportRowError {
	return ImportRowError{
		Text: fmt.Sprintf("Машина %s %s в чёрном списке: %s", number, mark, reason),
		Code: ImportErrBlacklisted,
	}
}

func fmtWarnLatinFixed(label, original, fixed string) string {
	return fmt.Sprintf("Поле «%s»: похожие латинские буквы заменены на русские, %q -> %q", label, original, fixed)
}

func fmtWarnFullNameSplit(full, last, first, middle string) string {
	return fmt.Sprintf("ФИО распознано из одного поля %q как %q - проверьте разбор", full, joinFIO(last, first, middle))
}

// fmtWarnPlateFixed - номер машины разложен по формату с исправлением (раскладка,
// похожие буквы, дополнение короткой цифровой части) - показ обоих вариантов, а не
// молчаливая правка (решение владельца, blank-import-ux U2), зеркало fmtWarnLatinFixed.
func fmtWarnPlateFixed(original, fixed string) string {
	return fmt.Sprintf("Номер Т/С приведён к формату номера, %q -> %q", strings.TrimSpace(original), fixed)
}

// fmtErrPlateFormatNotFound - строка из бланка не разложилась ни по одному активному
// формату номеров (решение владельца, blank-import-ux U2). Номер правится прямо в
// таблице разбора, поэтому причина исправимая.
func fmtErrPlateFormatNotFound(raw string) ImportRowError {
	return ImportRowError{
		Text:    fmt.Sprintf("Номер Т/С %q не соответствует ни одному формату номеров", strings.TrimSpace(raw)),
		Code:    ImportErrPlateFormat,
		Field:   "number",
		Fixable: importFieldFixable("cars", "number"),
	}
}
