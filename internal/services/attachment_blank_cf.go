package services

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Вставка строк под список сдвигает диапазоны условного форматирования - это делает
// сам excelize, - но формулы внутри правил остаются прежними: в v2.11
// adjustConditionalFormats правит только SQRef. В бланке правило уезжает вниз на
// число добавленных строк, а условие продолжает читать прежнюю строку, и подсветка
// срабатывает не по той ячейке. Здесь дошиваем формулы на тот же шаг.

var (
	sheetXMLName      = regexp.MustCompile(`^xl/worksheets/sheet\d+\.xml$`)
	condFormatBlock   = regexp.MustCompile(`(?s)<conditionalFormatting\b.*?</conditionalFormatting>`)
	condFormatFormula = regexp.MustCompile(`(?s)(<formula[^>]*>)(.*?)(</formula>)`)
)

// shiftConditionalFormatFormulas переписывает сохранённый .xlsx: во всех правилах
// условного форматирования ссылки на строки от fromRow и ниже сдвигаются на offset.
// Остальные части архива копируются как есть, без перекодирования.
func shiftConditionalFormatFormulas(data []byte, fromRow, offset int) ([]byte, error) {
	if offset <= 0 || fromRow < 1 {
		return data, nil
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to read generated workbook: %w", err)
	}

	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, file := range zr.File {
		if !sheetXMLName.MatchString(file.Name) {
			if err := copyZipEntry(zw, file); err != nil {
				return nil, err
			}
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %w", file.Name, err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", file.Name, err)
		}
		shifted := shiftSheetConditionalFormulas(string(content), fromRow, offset)
		// Размеры и контрольную сумму zip.Writer посчитает сам: от исходной записи их
		// переносить нельзя, содержимое изменилось.
		header := file.FileHeader
		header.Method = zip.Deflate
		header.CompressedSize64, header.UncompressedSize64, header.CRC32 = 0, 0, 0
		header.CompressedSize, header.UncompressedSize = 0, 0
		w, err := zw.CreateHeader(&header)
		if err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", file.Name, err)
		}
		if _, err := w.Write([]byte(shifted)); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", file.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close workbook: %w", err)
	}
	return out.Bytes(), nil
}

// copyZipEntry переносит запись архива без распаковки: содержимое остальных частей
// книги нам менять не нужно.
func copyZipEntry(zw *zip.Writer, file *zip.File) error {
	src, err := file.OpenRaw()
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", file.Name, err)
	}
	header := file.FileHeader
	dst, err := zw.CreateRaw(&header)
	if err != nil {
		return fmt.Errorf("failed to copy %s: %w", file.Name, err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy %s: %w", file.Name, err)
	}
	return nil
}

// shiftSheetConditionalFormulas сдвигает ссылки в формулах правил условного
// форматирования листа. Диапазоны (SQRef) не трогаем - их уже сдвинул excelize.
func shiftSheetConditionalFormulas(sheetXML string, fromRow, offset int) string {
	return condFormatBlock.ReplaceAllStringFunc(sheetXML, func(block string) string {
		return condFormatFormula.ReplaceAllStringFunc(block, func(formula string) string {
			parts := condFormatFormula.FindStringSubmatch(formula)
			if len(parts) != 4 {
				return formula
			}
			return parts[1] + shiftFormulaRows(parts[2], fromRow, offset) + parts[3]
		})
	})
}

// shiftFormulaRows прибавляет offset к номерам строк в ссылках формулы, если строка
// не меньше fromRow. Ссылкой считаем 1-3 буквы и цифры с необязательными $; текст в
// кавычках пропускаем - "A38" внутри строки это значение, а не адрес.
func shiftFormulaRows(expr string, fromRow, offset int) string {
	var b strings.Builder
	b.Grow(len(expr) + 8)
	inQuotes := false
	for i := 0; i < len(expr); {
		c := expr[i]
		if c == '"' {
			inQuotes = !inQuotes
			b.WriteByte(c)
			i++
			continue
		}
		if inQuotes || !startsRef(expr, i) {
			b.WriteByte(c)
			i++
			continue
		}
		ref, row, next, ok := parseRef(expr, i)
		if !ok {
			b.WriteByte(c)
			i++
			continue
		}
		if row >= fromRow {
			b.WriteString(shiftedRef(ref, row+offset))
		} else {
			b.WriteString(ref)
		}
		i = next
	}
	return b.String()
}

// startsRef отсекает продолжение имени: в SUM1A1 или в имени листа буквы с цифрами
// адресом не являются.
func startsRef(expr string, i int) bool {
	if i == 0 {
		return true
	}
	prev := expr[i-1]
	switch {
	case prev >= 'A' && prev <= 'Z', prev >= 'a' && prev <= 'z':
		return false
	case prev >= '0' && prev <= '9':
		return false
	case prev == '_', prev == '.':
		return false
	}
	return true
}

// parseRef читает ссылку вида [$]A[$]38 и возвращает её текст, номер строки и позицию
// за ней. Функция вроде LOG10( ссылкой не считается.
func parseRef(expr string, i int) (string, int, int, bool) {
	pos := i
	if pos < len(expr) && expr[pos] == '$' {
		pos++
	}
	letters := 0
	for pos < len(expr) && letters < 3 {
		c := expr[pos]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			pos++
			letters++
			continue
		}
		break
	}
	if letters == 0 {
		return "", 0, 0, false
	}
	if pos < len(expr) && expr[pos] == '$' {
		pos++
	}
	digits := 0
	rowStart := pos
	for pos < len(expr) && digits < 7 {
		c := expr[pos]
		if c >= '0' && c <= '9' {
			pos++
			digits++
			continue
		}
		break
	}
	if digits == 0 {
		return "", 0, 0, false
	}
	// Продолжение слова или вызов функции - не ссылка.
	if pos < len(expr) {
		switch c := expr[pos]; {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c == '_', c == '(':
			return "", 0, 0, false
		case c >= '0' && c <= '9':
			return "", 0, 0, false
		}
	}
	row, err := strconv.Atoi(expr[rowStart:pos])
	if err != nil || row < 1 {
		return "", 0, 0, false
	}
	return expr[i:pos], row, pos, true
}

// shiftedRef собирает ссылку с новым номером строки, сохраняя знаки $.
func shiftedRef(ref string, row int) string {
	idx := strings.LastIndexFunc(ref, func(r rune) bool {
		return r < '0' || r > '9'
	})
	return ref[:idx+1] + strconv.Itoa(row)
}
