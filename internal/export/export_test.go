package export

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func sampleTable() Table {
	return Table{
		Title:    "Снимок таблицы",
		Subtitle: "Версия от 03.07.2026 06:00",
		Headers:  []string{"№ машины", "Марка", "Статус на территории"},
		Rows: [][]string{
			{"А123ВС77", "Камаз", "На территории"},
			{"О456РТ99", "ГАЗель", "Выехал"},
		},
	}
}

func TestToXLSX_NonEmptyValidZip(t *testing.T) {
	data, err := ToXLSX(sampleTable())
	if err != nil {
		t.Fatalf("ToXLSX: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("xlsx пустой")
	}
	// .xlsx - это zip-архив: сигнатура PK\x03\x04.
	if !bytes.HasPrefix(data, []byte{'P', 'K', 0x03, 0x04}) {
		t.Fatalf("не похоже на zip/xlsx, первые байты: %x", data[:4])
	}
}

func TestToPDF_NonEmptyValidPDF(t *testing.T) {
	data, err := ToPDF(sampleTable())
	if err != nil {
		t.Fatalf("ToPDF: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("pdf пустой")
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("нет сигнатуры %%PDF, первые байты: %x", data[:4])
	}
}

// TestToPDF_EmbedsCyrillicFont доказывает, что PDF рендерит кириллицу: встроен
// TTF-subset DejaVu (FontFile2 + CIDFontType2), а не заменён core-шрифтом, который
// кириллицу не содержит. Маркеры видны и в сжатом (прод) выводе - это ключи словарей
// объектов, а не тело сжатого потока.
func TestToPDF_EmbedsCyrillicFont(t *testing.T) {
	data, err := ToPDF(sampleTable())
	if err != nil {
		t.Fatalf("ToPDF: %v", err)
	}
	raw := string(data)
	for _, marker := range []string{"/BaseFont /utf8dejavu", "FontFile2", "CIDFontType2"} {
		if !strings.Contains(raw, marker) {
			t.Fatalf("в PDF нет маркера встроенного кириллического шрифта %q", marker)
		}
	}
}

// TestToXLSX_AdaptiveColumnWidths: ширина столбца подгоняется под контент - колонка с
// длинными значениями шире узкой, в пределах [minColWidth, maxColWidth]. Без этого
// excelize ставит всем дефолт ~8.43 и длинные значения обрезаются (замечание #980).
func TestToXLSX_AdaptiveColumnWidths(t *testing.T) {
	tbl := Table{
		Headers: []string{"№", "Организация"},
		Rows: [][]string{
			{"1", "Коротко"},
			{"2", "Очень длинное название организации для проверки ширины столбца"},
		},
	}
	data, err := ToXLSX(tbl)
	if err != nil {
		t.Fatalf("ToXLSX: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)

	wNarrow, err := f.GetColWidth(sheet, "A")
	if err != nil {
		t.Fatalf("GetColWidth A: %v", err)
	}
	wWide, err := f.GetColWidth(sheet, "B")
	if err != nil {
		t.Fatalf("GetColWidth B: %v", err)
	}
	if wWide <= wNarrow {
		t.Fatalf("колонка с длинными значениями должна быть шире узкой: A=%.1f B=%.1f", wNarrow, wWide)
	}
	if wNarrow < minColWidth {
		t.Fatalf("узкая колонка уже минимума: %.1f < %.1f", wNarrow, minColWidth)
	}
	if wWide > maxColWidth {
		t.Fatalf("широкая колонка шире потолка: %.1f > %.1f", wWide, maxColWidth)
	}
}

// Выгрузку открывают, чтобы отобрать в ней строки, поэтому шапка обязана нести
// автофильтр и оставаться на месте при прокрутке. Диапазон фильтра проверяем по XML
// листа: excelize не отдаёт его обратно геттером.
func TestToXLSX_HeaderHasAutoFilterAndFreeze(t *testing.T) {
	tbl := Table{
		Title:    "Реестр заявок",
		Subtitle: "заявок: 2",
		Headers:  []string{"Номер", "Организация", "Статус"},
		Rows: [][]string{
			{"A-1", "Орг", "Завершено"},
			{"A-2", "Орг", "В обработке"},
		},
	}
	data, err := ToXLSX(tbl)
	if err != nil {
		t.Fatalf("ToXLSX: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)

	// Заголовок, подзаголовок, пустая строка-разделитель - шапка таблицы четвёртая,
	// данные с пятой. Фильтр обязан накрывать шапку и обе строки данных.
	// excelize пишет диапазон абсолютными ссылками.
	if want := `autoFilter ref="$A$4:$C$6"`; !strings.Contains(sheetXML(t, data), want) {
		t.Fatalf("автофильтр не накрывает шапку и данные, ожидали %s", want)
	}

	panes, err := f.GetPanes(sheet)
	if err != nil {
		t.Fatalf("GetPanes: %v", err)
	}
	if !panes.Freeze {
		t.Fatal("шапка не закреплена: при прокрутке длинного реестра она уедет")
	}
	if panes.YSplit != 4 {
		t.Fatalf("закреплены не все строки шапки: YSplit=%d, ожидали 4", panes.YSplit)
	}
}

// Пустая выборка - это ответ «за период ничего нет», а не ошибка: файл обязан
// открываться, а фильтр на единственной строке шапки Excel считает поводом
// пожаловаться, поэтому его там нет.
func TestToXLSX_EmptyRows_NoFilterButValidFile(t *testing.T) {
	data, err := ToXLSX(Table{Headers: []string{"Номер", "Статус"}})
	if err != nil {
		t.Fatalf("ToXLSX: %v", err)
	}
	if _, err := excelize.OpenReader(bytes.NewReader(data)); err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if strings.Contains(sheetXML(t, data), "autoFilter") {
		t.Fatal("на пустой выборке автофильтра быть не должно")
	}
}

// sheetXML достаёт разметку первого листа из готового файла. Автофильтр excelize
// обратно не отдаёт ни одним геттером, а xlsx - это zip, поэтому читаем описание
// листа напрямую: проверка не зависит от версии библиотеки.
func sheetXML(t *testing.T, data []byte) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("xlsx не читается как zip: %v", err)
	}
	for _, file := range zr.File {
		if file.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("открыть %s: %v", file.Name, err)
		}
		defer func() { _ = rc.Close() }()
		body, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("прочитать %s: %v", file.Name, err)
		}
		return string(body)
	}
	t.Fatal("в xlsx нет xl/worksheets/sheet1.xml")
	return ""
}

// TestToPDF_ManyRows_MultiPage: большая таблица разбивается на несколько страниц
// (проверяем /Count в корневом Pages-объекте). Тем самым прогоняется путь переноса
// страницы с повтором шапки колонок (drawHeader на каждой новой странице).
func TestToPDF_ManyRows_MultiPage(t *testing.T) {
	rows := make([][]string, 0, 400)
	for i := 0; i < 400; i++ {
		rows = append(rows, []string{"А123ВС77", "Камаз", "На территории"})
	}
	data, err := ToPDF(Table{
		Title:   "Снимок",
		Headers: []string{"№ машины", "Марка", "Статус на территории"},
		Rows:    rows,
	})
	if err != nil {
		t.Fatalf("ToPDF: %v", err)
	}
	m := regexp.MustCompile(`/Count (\d+)`).FindStringSubmatch(string(data))
	if m == nil {
		t.Fatal("в PDF нет /Count Pages-объекта")
	}
	pages, _ := strconv.Atoi(m[1])
	if pages < 2 {
		t.Fatalf("ожидали многостраничный PDF, страниц: %d", pages)
	}
}
