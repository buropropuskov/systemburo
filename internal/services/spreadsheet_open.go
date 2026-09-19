package services

import (
	"bytes"

	"github.com/xuri/excelize/v2"
)

// Пределы распаковки книги Excel.
//
// По умолчанию excelize готов распаковать 16 ГБ и выносит части XML крупнее 16 МБ во
// временный файл на диске. Оба значения нам не подходят.
//
// Первое - открытая дверь для архива-бомбы: загрузка ограничена десятью мегабайтами, а XML
// сжимается в разы, так что распаковка счёта не знает. Второе включает в библиотеке ветку
// чтения общих строк из временного файла, где индекс строки проверяется только сверху
// (GO-2026-6452): ячейка с отрицательным индексом роняет разбор паникой. В ветке чтения
// в память тот же индекс проверяется с обеих сторон, и пока исправление не вышло релизом,
// держим разбор в ней.
//
// 64 МБ - шестикратный запас к пределу загрузки: наши бланки весят сотни килобайт, а книга,
// распаковка которой больше, для бюро пропусков не бывает законной.
const (
	spreadsheetUnzipLimit    = 64 << 20
	spreadsheetUnzipXMLLimit = 64 << 20
)

// spreadsheetOptions - пределы, с которыми открывается любая книга: и присланная человеком,
// и шаблон из базы. Шаблон тоже кем-то загружен, поэтому исключения для него нет.
func spreadsheetOptions() excelize.Options {
	return excelize.Options{
		UnzipSizeLimit:    spreadsheetUnzipLimit,
		UnzipXMLSizeLimit: spreadsheetUnzipXMLLimit,
	}
}

// OpenSpreadsheet открывает книгу Excel из памяти с общими пределами распаковки.
func OpenSpreadsheet(data []byte) (*excelize.File, error) {
	return excelize.OpenReader(bytes.NewReader(data), spreadsheetOptions())
}
