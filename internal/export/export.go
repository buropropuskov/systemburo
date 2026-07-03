// Package export рендерит формат-нейтральные табличные данные (export.Table) в
// файлы для выгрузки: Excel (excelize) и PDF (go-pdf/fpdf со встроенным
// кириллическим шрифтом). Пакет не знает домена - значения приходят уже строками;
// доменное форматирование (статусы, даты) делает вызывающая сторона.
package export

import (
	"bytes"
	"fmt"

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
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)

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

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to write xlsx: %w", err)
	}
	return buf.Bytes(), nil
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
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes(pdfFontFamily, "", dejaVuSans)
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(true, 12)
	pdf.AddPage()

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

		pdf.SetFont(pdfFontFamily, "", 8)
		pdf.SetFillColor(230, 230, 230)
		drawPDFRow(pdf, t.Headers, colW, rowH, true)
		for _, r := range t.Rows {
			drawPDFRow(pdf, r, colW, rowH, false)
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to write pdf: %w", err)
	}
	return buf.Bytes(), nil
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
	if maxW <= 0 || pdf.GetStringWidth(s) <= maxW {
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
