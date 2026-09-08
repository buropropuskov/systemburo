// Package export рендерит формат-нейтральные табличные данные (export.Table) в
// файлы для выгрузки: Excel (excelize) и PDF (go-pdf/fpdf со встроенным
// кириллическим шрифтом). Пакет не знает домена - значения приходят уже строками;
// доменное форматирование (статусы, даты) делает вызывающая сторона.
package export

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"
)

// MIME-типы генерируемых файлов.
const (
	MIMEXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	MIMEPDF  = "application/pdf"
)

// pdfFontFamily - имя, под которым регистрируется встроенный кириллический шрифт.
const pdfFontFamily = "DejaVu"

// Table - формат-нейтральные табличные данные для выгрузки: заголовок документа,
// подзаголовок (напр. дата снимка), шапка колонок и строки. Значения уже приведены
// к строкам вызывающей стороной.
type Table struct {
	Title    string
	Subtitle string
	Headers  []string
	Rows     [][]string
}

// ToXLSX рендерит таблицу в .xlsx и возвращает байты файла.
func ToXLSX(t Table) ([]byte, error) {
	return ToXLSXMulti([]Table{t})
}

// ToXLSXMulti рендерит несколько таблиц в одну книгу - лист на таблицу. Имя листа
// берётся из Table.Title, потому что для получателя книги вкладка «Проходы» понятнее,
// чем «Лист2».
//
// Нужно там, где выгрузка не одна таблица, а справка из разделов: сведения о человеке
// (#2356) - это его данные, участие в заявках, проходы и привязки к постам, и сводить
// их в одну плоскую таблицу значит сделать файл нечитаемым.
func ToXLSXMulti(tables []Table) ([]byte, error) {
	if len(tables) == 0 {
		return nil, fmt.Errorf("нечего выгружать: ни одной таблицы")
	}

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	for i, t := range tables {
		sheet := sheetNameFor(t, i)
		if i == 0 {
			if err := f.SetSheetName(f.GetSheetName(0), sheet); err != nil {
				return nil, fmt.Errorf("не удалось назвать первый лист: %w", err)
			}
		} else if _, err := f.NewSheet(sheet); err != nil {
			return nil, fmt.Errorf("не удалось создать лист %q: %w", sheet, err)
		}
		if err := writeSheet(f, sheet, t); err != nil {
			return nil, err
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to write xlsx: %w", err)
	}
	return buf.Bytes(), nil
}

// sheetNameFor - имя листа из заголовка таблицы. Excel не принимает символы \ / ? * [ ]
// и длину больше 31 знака, поэтому имя чистится; пустой заголовок даёт «Лист N».
func sheetNameFor(t Table, index int) string {
	name := strings.TrimSpace(t.Title)
	for _, bad := range []string{"\\", "/", "?", "*", "[", "]", ":"} {
		name = strings.ReplaceAll(name, bad, " ")
	}
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return fmt.Sprintf("Лист %d", index+1)
	}
	if utf8.RuneCountInString(name) > 31 {
		runes := []rune(name)
		name = string(runes[:31])
	}
	return name
}

// writeSheet раскладывает одну таблицу на готовый лист.
func writeSheet(f *excelize.File, sheet string, t Table) error {
	row := 1
	if t.Title != "" {
		setCell(f, sheet, 1, row, t.Title)
		row++
	}
	if t.Subtitle != "" {
		setCell(f, sheet, 1, row, t.Subtitle)
		row++
	}
	if row > 1 {
		row++ // пустая строка-разделитель между шапкой документа и таблицей
	}

	headerRow := row
	for i, h := range t.Headers {
		setCell(f, sheet, i+1, headerRow, h)
	}
	row++
	for _, r := range t.Rows {
		for i, v := range r {
			setCell(f, sheet, i+1, row, v)
		}
		row++
	}

	styleHeader(f, sheet, headerRow, len(t.Headers))
	adjustColWidths(f, sheet, t.Headers, t.Rows)
	return enableFilterAndFreeze(f, sheet, headerRow, len(t.Headers), len(t.Rows))
}

// enableFilterAndFreeze вешает на шапку автофильтр и закрепляет её при прокрутке.
// Без этого выгрузка открывается «плоским» листом: чтобы отобрать строки или просто
// не терять шапку на второй сотне записей, получатель каждый раз делает это руками.
//
// Диапазон фильтра идёт от строки шапки до последней строки данных: excelize вешает
// фильтр на прямоугольник, и указать одну строку шапки недостаточно - Excel тогда
// считает таблицей только её. Пустая выборка (0 строк) - лист остаётся без фильтра:
// фильтровать нечего, а диапазон из одной строки Excel открывает с предупреждением.
func enableFilterAndFreeze(f *excelize.File, sheet string, headerRow, cols, rows int) error {
	if cols == 0 {
		return nil
	}
	// Закрепляем всё, что выше первой строки данных: у выгрузок с заголовком и
	// подзаголовком это шапка документа плюс шапка таблицы, у голых - только шапка.
	firstDataRow := headerRow + 1
	if err := f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		YSplit:      headerRow,
		TopLeftCell: fmt.Sprintf("A%d", firstDataRow),
		ActivePane:  "bottomLeft",
	}); err != nil {
		return fmt.Errorf("закрепление шапки xlsx: %w", err)
	}
	if rows == 0 {
		return nil
	}

	lastCol, err := excelize.ColumnNumberToName(cols)
	if err != nil {
		return fmt.Errorf("имя последнего столбца xlsx: %w", err)
	}
	rangeRef := fmt.Sprintf("A%d:%s%d", headerRow, lastCol, headerRow+rows)
	if err := f.AutoFilter(sheet, rangeRef, nil); err != nil {
		return fmt.Errorf("автофильтр xlsx: %w", err)
	}
	return nil
}

// Границы адаптивной ширины столбца xlsx (в «символах» - единица ширины excelize,
// примерно равная числу знаков моноширинного текста).
const (
	minColWidth = 10.0
	maxColWidth = 50.0
	colWidthPad = 2.0 // запас на пропорциональный шрифт и отступы ячейки
)

// adjustColWidths подгоняет ширину каждого столбца под самое длинное значение (шапка или
// ячейка) в пределах [minColWidth, maxColWidth]. Без этого excelize оставляет всем
// дефолтные ~8.43 знака, и длинные значения (организация, места разгрузки) обрезаются.
func adjustColWidths(f *excelize.File, sheet string, headers []string, rows [][]string) {
	cols := len(headers)
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	for i := 0; i < cols; i++ {
		maxLen := 0
		if i < len(headers) {
			maxLen = utf8.RuneCountInString(headers[i])
		}
		for _, r := range rows {
			if i < len(r) {
				if l := utf8.RuneCountInString(r[i]); l > maxLen {
					maxLen = l
				}
			}
		}
		w := float64(maxLen) + colWidthPad
		if w < minColWidth {
			w = minColWidth
		}
		if w > maxColWidth {
			w = maxColWidth
		}
		name, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			continue
		}
		_ = f.SetColWidth(sheet, name, name, w)
	}
}

// setCell пишет значение в ячейку (col,row) 1-based. Координаты строятся здесь же и
// валидны by construction, поэтому ошибка excelize (только на битом имени листа/
// координате) не наступает - игнорируем осознанно.
func setCell(f *excelize.File, sheet string, col, row int, v string) {
	cell, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return
	}
	_ = f.SetCellValue(sheet, cell, v)
}

// styleHeader делает шапку колонок жирной. Ошибка стиля не критична для выгрузки -
// файл валиден и без неё, поэтому не всплывает наверх.
func styleHeader(f *excelize.File, sheet string, headerRow, cols int) {
	if cols == 0 {
		return
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return
	}
	first, _ := excelize.CoordinatesToCellName(1, headerRow)
	last, _ := excelize.CoordinatesToCellName(cols, headerRow)
	_ = f.SetCellStyle(sheet, first, last, style)
}

// ToPDF рендерит таблицу в PDF (A4 landscape) со встроенным кириллическим шрифтом.
// Шрифт встраивается как FontFile2 (CIDFontType2, Identity-H) - кириллица рендерится
// её собственными глифами, а не заменяется на «?» как у core-шрифтов.
func ToPDF(t Table) ([]byte, error) {
	return ToPDFMulti([]Table{t})
}

// ToPDFMulti рендерит несколько таблиц одним документом - раздел с новой страницы.
// Парная к ToXLSXMulti: справка из разделов должна одинаково читаться в обоих форматах,
// иначе получатель ответа увидит в них разное.
func ToPDFMulti(tables []Table) ([]byte, error) {
	if len(tables) == 0 {
		return nil, fmt.Errorf("нечего выгружать: ни одной таблицы")
	}

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes(pdfFontFamily, "", dejaVuSans)
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(true, 12)

	for _, t := range tables {
		pdf.AddPage()
		if err := writePDFSection(pdf, t); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to write pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// writePDFSection рисует один раздел на уже открытой странице.
func writePDFSection(pdf *fpdf.Fpdf, t Table) error {
	pageW, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	usableW := pageW - left - right

	if t.Title != "" {
		pdf.SetFont(pdfFontFamily, "", 14)
		pdf.MultiCell(usableW, 7, t.Title, "", "L", false)
	}
	if t.Subtitle != "" {
		pdf.SetFont(pdfFontFamily, "", 9)
		pdf.MultiCell(usableW, 5, t.Subtitle, "", "L", false)
	}
	pdf.Ln(2)

	if len(t.Headers) > 0 {
		colW := usableW / float64(len(t.Headers))
		const rowH = 6.0
		_, pageH := pdf.GetPageSize()
		_, _, _, bottomM := pdf.GetMargins()

		// Шапку колонок повторяем на каждой странице - иначе на 2-й+ странице крупной
		// таблицы (700-1500 строк) непонятно, что за колонки.
		drawHeader := func() {
			pdf.SetFont(pdfFontFamily, "", 8)
			pdf.SetFillColor(230, 230, 230)
			drawPDFRow(pdf, t.Headers, colW, rowH, true)
		}
		drawHeader()
		for _, r := range t.Rows {
			if pdf.GetY()+rowH > pageH-bottomM {
				pdf.AddPage()
				drawHeader()
			}
			drawPDFRow(pdf, r, colW, rowH, false)
		}
	}
	return nil
}

// drawPDFRow рисует одну строку таблицы фиксированной высоты; значения, не влезающие
// в колонку, усекаются с многоточием. fill=true - заливка (для шапки).
func drawPDFRow(pdf *fpdf.Fpdf, cells []string, colW, rowH float64, fill bool) {
	for _, c := range cells {
		pdf.CellFormat(colW, rowH, truncateToWidth(pdf, c, colW-2), "1", 0, "L", fill, 0, "")
	}
	pdf.Ln(rowH)
}

// truncateToWidth усекает строку до ширины maxW (в единицах документа), добавляя
// многоточие. Нужен потому, что fpdf.CellFormat не переносит и не обрезает текст сам.
func truncateToWidth(pdf *fpdf.Fpdf, s string, maxW float64) string {
	if maxW <= 0 {
		return ""
	}
	if pdf.GetStringWidth(s) <= maxW {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		if pdf.GetStringWidth(string(runes)+"…") <= maxW {
			return string(runes) + "…"
		}
	}
	return ""
}
