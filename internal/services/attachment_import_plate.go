package services

import (
	"context"
	"strings"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/normalize"
)

// Значения cell_type, которые реально пишет админка форматов номеров (NumberFormat.vue) -
// бэк их не валидирует при сохранении формата, поэтому сверяемся строго с тем, что
// сохраняет форма, а не с произвольными синонимами вроде "letter"/"digit" (такие
// встречаются только в генераторе демо-данных, internal/fakedata, и в реальные форматы
// не попадают).
const (
	plateCellNumbers = "numbers"
	plateCellMixed   = "mixed"
)

// defaultCyrillicPlateLetters - буквы номера по умолчанию, когда у ячейки не задан
// allowed_letters (зеркало regex-фолбэка filterCyrillicLetters в useNumberFormat.js).
const defaultCyrillicPlateLetters = "АВЕКМНОРСТУХ"

// matchedPlate - номер из бланка, разложенный по ячейкам подошедшего формата.
type matchedPlate struct {
	FormatID  int
	Formatted string // ячейки через пробел - тот же вид, что numberParts.join(' ') формы
	Changed   bool   // раскладка/омоглифы/дополнение изменили строку относительно бланка
}

// loadActiveLicensePlateFormats грузит активные форматы номеров с ячейками, отсортированными
// по cell_order, двумя запросами вместо N+1 на формат (license_format_service.GetAll так не
// делает, но там страница администрирования на десяток форматов, а не построчный разбор
// импорта на до 2000 строк одного файла).
func (s *attachmentImportService) loadActiveLicensePlateFormats(ctx context.Context) ([]models.LicensePlateFormatWithCells, error) {
	var formats []models.LicensePlateFormat
	if err := s.db.WithContext(ctx).Where("is_active = ?", true).Order("id").Find(&formats).Error; err != nil {
		return nil, apperr.Internal("Не удалось загрузить форматы номеров", err)
	}
	if len(formats) == 0 {
		return nil, nil
	}

	ids := make([]int, len(formats))
	for i, f := range formats {
		ids[i] = f.ID
	}
	var cells []models.LicensePlateFormatCell
	if err := s.db.WithContext(ctx).Where("format_id IN ?", ids).Order("cell_order").Find(&cells).Error; err != nil {
		return nil, apperr.Internal("Не удалось загрузить ячейки форматов номеров", err)
	}
	cellsByFormat := make(map[int][]models.LicensePlateFormatCell, len(formats))
	for _, c := range cells {
		cellsByFormat[c.FormatID] = append(cellsByFormat[c.FormatID], c)
	}

	result := make([]models.LicensePlateFormatWithCells, 0, len(formats))
	for _, f := range formats {
		result = append(result, models.LicensePlateFormatWithCells{Format: f, Cells: cellsByFormat[f.ID]})
	}
	return result, nil
}

// matchLicensePlate подбирает формат для сырой строки номера из бланка: перебирает активные
// форматы, пытаясь разложить строку по ячейкам каждого (splitPlateCells), и из подошедших
// предпочитает формат по умолчанию (решение владельца, blank-import-ux U2). Строка
// нормализуется ЦЕЛИКОМ только по разделителям и регистру (normalize.StripPlateSeparators) -
// раскладка/омоглифы и дополнение применяются НИЖЕ, по ячейке, а не по всей строке разом:
// normalize.Plate схлопывает 0->О везде, что сломало бы числовые ячейки с легитимным нулём
// (регион "790" стал бы "79О").
func matchLicensePlate(raw string, formats []models.LicensePlateFormatWithCells) (*matchedPlate, bool) {
	stripped := normalize.StripPlateSeparators(strings.ToUpper(strings.TrimSpace(raw)))
	if stripped == "" {
		return nil, false
	}

	var first, byDefault *matchedPlate
	for _, f := range formats {
		parts, ok := splitPlateCells(stripped, f.Cells)
		if !ok {
			continue
		}
		m := &matchedPlate{
			FormatID:  f.Format.ID,
			Formatted: strings.Join(parts, " "),
			Changed:   strings.Join(parts, "") != stripped,
		}
		if first == nil {
			first = m
		}
		if f.Format.IsDefault && byDefault == nil {
			byDefault = m
		}
	}
	if byDefault != nil {
		return byDefault, true
	}
	if first != nil {
		return first, true
	}
	return nil, false
}

// splitPlateCells раскладывает уже нормализованную (без разделителей, в верхнем регистре)
// строку по ячейкам формата слева направо, с возвратом: перебор длин на ячейку дешёвый
// (ячеек единицы, длины до десяти символов), полный разбор без эвристик надёжнее, чем
// деление строки на "буквенные/цифровые пробеги" - оно не заметило бы короткую числовую
// ячейку, которую нужно дополнить (padding), и посчитало бы её остаток частью соседней.
func splitPlateCells(s string, cells []models.LicensePlateFormatCell) ([]string, bool) {
	if len(cells) == 0 {
		return nil, false
	}
	runes := []rune(s)
	parts := make([]string, len(cells))
	if !trySplitPlateCell(runes, cells, 0, parts) {
		return nil, false
	}
	return parts, true
}

func trySplitPlateCell(remaining []rune, cells []models.LicensePlateFormatCell, cellIdx int, parts []string) bool {
	if cellIdx == len(cells) {
		return len(remaining) == 0
	}
	cell := cells[cellIdx]

	lower, upper := plateCellLengthBounds(cell, len(remaining))
	// От длинного к короткому: единственная реально короткая (дополняемая) ячейка на
	// практике последняя в формате (регион) - при переборе с конца её длина однозначно
	// определяется остатком строки, порядок перебора здесь только про скорость находки.
	for length := upper; length >= lower; length-- {
		segment := string(remaining[:length])
		fixed, ok := fixPlateCellSegment(segment, cell)
		if !ok {
			continue
		}
		parts[cellIdx] = padPlateCellValue(fixed, cell)
		if trySplitPlateCell(remaining[length:], cells, cellIdx+1, parts) {
			return true
		}
	}
	return false
}

// plateCellLengthBounds - допустимая длина СЫРОГО (до дополнения) сегмента для ячейки.
// Числовая ячейка с настроенным дополнением принимает от 1 символа - padPlateCellValue
// (зеркало formatPartValue формы) дополнит её до max_length независимо от min_length;
// без дополнения и для остальных типов ячеек действует [min_length, max_length], как в
// проверке формы (VehicleForm.vue canAddVehicle: part.length >= min && <= max).
func plateCellLengthBounds(cell models.LicensePlateFormatCell, remaining int) (lower, upper int) {
	upper = remaining
	if cell.MaxLength != nil && *cell.MaxLength < upper {
		upper = *cell.MaxLength
	}
	lower = 1
	if cell.MinLength != nil && *cell.MinLength > lower {
		lower = *cell.MinLength
	}
	if cell.CellType == plateCellNumbers && plateCellHasPadding(cell) {
		lower = 1
	}
	// lower > upper здесь легален и означает "на эту ячейку не осталось символов" -
	// trySplitPlateCell.for length := upper; length >= lower; length-- ни разу не
	// выполнится, и ветка разбора провалится, как положено (см. fixPlateCellSegment:
	// пустой сегмент отдельно отклоняется на случай, если границы всё же совпадут).
	return lower, upper
}

func plateCellHasPadding(cell models.LicensePlateFormatCell) bool {
	return cell.PaddingSide != nil && *cell.PaddingSide != ""
}

// fixPlateCellSegment проверяет и (для буквенных кириллических ячеек) исправляет один
// сегмент строки под конкретную ячейку формата.
func fixPlateCellSegment(segment string, cell models.LicensePlateFormatCell) (string, bool) {
	switch cell.CellType {
	case plateCellNumbers:
		return fixPlateNumberSegment(segment)
	case plateCellMixed:
		return fixPlateMixedSegment(segment, cell)
	default: // "letters" и любой нераспознанный тип - буквенная ячейка
		return fixPlateLetterSegment(segment, cell)
	}
}

func fixPlateNumberSegment(segment string) (string, bool) {
	if segment == "" {
		return "", false
	}
	for _, r := range segment {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return segment, true
}

// fixPlateLetterSegment - латиница-омоглиф и 0->О (только для кириллического алфавита,
// см. normalize.FixPlateLetterCell - для латиницы/"both" направление исправления
// неоднозначно, не трогаем), затем сверка с разрешёнными буквами ячейки.
func fixPlateLetterSegment(segment string, cell models.LicensePlateFormatCell) (string, bool) {
	if segment == "" {
		return "", false
	}
	candidate := segment
	if plateCellAlphabet(cell) == "cyrillic" {
		candidate, _ = normalize.FixPlateLetterCell(segment)
	}
	allowed := plateCellAllowedLetter(cell)
	for _, r := range candidate {
		if !allowed(r) {
			return "", false
		}
	}
	return candidate, true
}

// fixPlateMixedSegment - цифры проходят как есть, остаток сверяется как буквенный, без
// омоглиф-фикса: смешанная ячейка не встречается ни в одном реально настроенном формате
// на проекте, специально не усложняем её эвристиками разбора сверх проверки допустимости.
func fixPlateMixedSegment(segment string, cell models.LicensePlateFormatCell) (string, bool) {
	if segment == "" {
		return "", false
	}
	allowed := plateCellAllowedLetter(cell)
	for _, r := range segment {
		if r >= '0' && r <= '9' {
			continue
		}
		if !allowed(r) {
			return "", false
		}
	}
	return segment, true
}

func plateCellAlphabet(cell models.LicensePlateFormatCell) string {
	if cell.AlphabetType == nil || *cell.AlphabetType == "" {
		return "cyrillic"
	}
	return *cell.AlphabetType
}

// plateCellAllowedLetter - предикат "символ разрешён в этой буквенной/смешанной ячейке",
// зеркало filterCyrillicLetters/filterLatinLetters/filterBothLetters (useNumberFormat.js):
// явный allowed_letters побеждает, иначе дефолт по алфавиту ячейки.
func plateCellAllowedLetter(cell models.LicensePlateFormatCell) func(rune) bool {
	if cell.AllowedLetters != nil && *cell.AllowedLetters != "" {
		allowed := *cell.AllowedLetters
		return func(r rune) bool { return strings.ContainsRune(allowed, r) }
	}
	switch plateCellAlphabet(cell) {
	case "latin":
		return func(r rune) bool { return r >= 'A' && r <= 'Z' }
	case "both":
		return func(r rune) bool { return (r >= 'A' && r <= 'Z') || (r >= 'А' && r <= 'Я') }
	default:
		return func(r rune) bool { return strings.ContainsRune(defaultCyrillicPlateLetters, r) }
	}
}

// padPlateCellValue - зеркало formatPartValue (useNumberFormat.js): числовая ячейка с
// настроенным дополнением добивается до max_length, "left" - слева, иначе справа (тот же
// дефолт формы - не padding_side, а любое значение, отличное от буквального "left").
func padPlateCellValue(value string, cell models.LicensePlateFormatCell) string {
	if cell.CellType != plateCellNumbers || !plateCellHasPadding(cell) || value == "" {
		return value
	}
	target := 0
	if cell.MaxLength != nil {
		target = *cell.MaxLength
	}
	cur := len([]rune(value))
	if target <= cur {
		return value
	}
	padChar := "0"
	if cell.PaddingChar != nil && *cell.PaddingChar != "" {
		padChar = *cell.PaddingChar
	}
	pad := strings.Repeat(padChar, target-cur)
	if cell.PaddingSide != nil && *cell.PaddingSide == "left" {
		return pad + value
	}
	return value + pad
}
