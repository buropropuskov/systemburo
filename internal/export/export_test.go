package export

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"testing"
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
