package services

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// Обычная книга открывается: пределы не мешают законному файлу.
func TestOpenSpreadsheet_ReadsOrdinaryWorkbook(t *testing.T) {
	t.Parallel()

	f := excelize.NewFile()
	if err := f.SetCellValue("Sheet1", "A1", "Пропуск"); err != nil {
		t.Fatalf("подготовка книги: %v", err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("сохранение книги: %v", err)
	}

	got, err := OpenSpreadsheet(buf.Bytes())
	if err != nil {
		t.Fatalf("открытие книги: %v", err)
	}
	defer got.Close()

	value, err := got.GetCellValue("Sheet1", "A1")
	if err != nil {
		t.Fatalf("чтение ячейки: %v", err)
	}
	if value != "Пропуск" {
		t.Fatalf("прочитано %q, ожидалось %q", value, "Пропуск")
	}
}

// Книга, распаковка которой больше предела, до разбора не доходит: иначе десять мегабайт
// архива превращались бы в гигабайты памяти и временных файлов.
func TestOpenSpreadsheet_RejectsOversizedUnzip(t *testing.T) {
	t.Parallel()

	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	w, err := zw.Create("xl/sharedStrings.xml")
	if err != nil {
		t.Fatalf("создание записи архива: %v", err)
	}
	// Хорошо сжимаемая начинка: в архиве десятки килобайт, в распаковке больше предела.
	if _, err := w.Write([]byte(strings.Repeat("a", spreadsheetUnzipLimit+1))); err != nil {
		t.Fatalf("запись начинки: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("закрытие архива: %v", err)
	}
	if archive.Len() > 1<<20 {
		t.Fatalf("архив вышел %d байт - проверка перестала быть про сжатие", archive.Len())
	}

	if _, err := OpenSpreadsheet(archive.Bytes()); err == nil {
		t.Fatal("книга с распаковкой сверх предела открылась, ожидался отказ")
	}
}

// Пределы одинаковы, поэтому общие строки всегда читаются в память, а не из временного
// файла: в ветке временного файла индекс общей строки проверяется только сверху
// (GO-2026-6452), и ячейка с отрицательным индексом роняет разбор паникой.
func TestSpreadsheetOptions_KeepSharedStringsInMemory(t *testing.T) {
	t.Parallel()

	opts := spreadsheetOptions()
	if opts.UnzipXMLSizeLimit != opts.UnzipSizeLimit {
		t.Fatalf("предел XML %d не совпал с общим %d: части книги уедут во временный файл",
			opts.UnzipXMLSizeLimit, opts.UnzipSizeLimit)
	}
	if opts.UnzipSizeLimit == 0 {
		t.Fatal("общий предел распаковки нулевой - библиотека вернётся к своим 16 ГБ")
	}
}
