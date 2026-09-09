package export

import (
	"strings"
	"testing"

	"github.com/go-pdf/fpdf"
)

// newTestPDF - документ с тем же шрифтом, что у выгрузки: ширина строки считается по
// шрифту, и без него замеры не имели бы отношения к настоящему файлу.
func newTestPDF() *fpdf.Fpdf {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes(pdfFontFamily, "", dejaVuSans)
	pdf.SetFont(pdfFontFamily, "", 8)
	pdf.AddPage()
	return pdf
}

// Колонки в PDF раздаются по содержимому, а не поровну (#2356). Поровну - значит
// «Патент» с прочерком получает столько же, сколько ФИО с организацией, и длинные
// значения режутся многоточием при пустом месте рядом: справка из двенадцати колонок
// приходила с «Мякотных С…» и «Реестр сотр…».
func TestPDFColWidths_GivesRoomToLongColumns(t *testing.T) {
	table := Table{
		Headers: []string{"Источник", "Патент", "ФИО"},
		Rows: [][]string{
			{"Реестр сотрудников", "-", "Мякотных Сергей Михайлович"},
		},
	}

	pdf := newTestPDF()
	widths := pdfColWidths(pdf, table, 200)

	if widths[2] <= widths[1] {
		t.Fatalf("колонка ФИО обязана быть шире колонки с прочерками: %v", widths)
	}
	// Всё содержимое влезает в 200 мм - значит резать нечего.
	for i, w := range widths {
		content := table.Rows[0][i]
		if pdf.GetStringWidth(content) > w {
			t.Errorf("колонка %d режет содержимое %q при ширине %.1f", i, content, w)
		}
	}
}

// Когда содержимое шире страницы, узкие колонки не должны схлопываться в полоску.
func TestPDFColWidths_KeepsMinimumForNarrowColumns(t *testing.T) {
	long := strings.Repeat("длинное значение ", 20)
	table := Table{
		Headers: []string{"Дата", "Текст"},
		Rows:    [][]string{{"01.01.2026", long}},
	}

	widths := pdfColWidths(newTestPDF(), table, 100)

	if widths[0] < 12 {
		t.Errorf("узкая колонка схлопнулась: %.1f", widths[0])
	}
	if sum := widths[0] + widths[1]; sum > 100.5 {
		t.Errorf("колонки вышли за ширину страницы: %.1f", sum)
	}
}
