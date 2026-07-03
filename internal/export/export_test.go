package export

import (
	"bytes"
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
