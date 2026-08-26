package services

import (
	"log/slog"
	"sort"

	"github.com/xuri/excelize/v2"
)

// Пагинация бланка: когда таблица не помещается на страницу, её продолжение на
// следующей странице должно начинаться со своей шапки столбцов.
//
// Сквозная строка Excel (Print_Titles) для этого не годится: она одна на лист и
// печатается на КАЖДОЙ странице, поэтому у бланка с двумя таблицами (сотрудники и
// ввозимый товар) над второй висела бы шапка первой. Поэтому разбиение считаем сами:
// высоты строк в бланках заданы явно, а размер листа, поля и масштаб лежат в самом
// шаблоне - этого достаточно, чтобы посчитать, где ляжет разрыв, и поставить его
// принудительно, а перед продолжением таблицы вставить копию её шапки.

// paperHeightsInch - высота листа в дюймах по коду размера бумаги Excel. Перечислены
// форматы, которые встречаются в бланках бюро; для остальных берём A4.
var paperHeightsInch = map[int]float64{
	1:  11.0,  // Letter
	5:  14.0,  // Legal
	8:  16.54, // A3
	9:  11.69, // A4
	11: 8.27,  // A5
}

const (
	defaultPaperHeightInch = 11.69 // A4
	defaultRowHeightPt     = 15.0
	pointsPerInch          = 72.0
)

// tableForPagination - таблица бланка в координатах готового файла: строка её шапки и
// строки данных. Шапка - строка над списком, как размечены бланки.
type tableForPagination struct {
	headerRow int
	firstRow  int
	lastRow   int
}

// pageContentHeightPt - сколько пунктов содержимого помещается на печатной странице.
// Учитываем размер листа, ориентацию, вертикальные поля и масштаб печати: при 95%
// на странице помещается больше строк, чем при 100%.
func pageContentHeightPt(f *excelize.File, sheet string) float64 {
	height := defaultPaperHeightInch
	scale := 100.0

	if layout, err := f.GetPageLayout(sheet); err == nil {
		if layout.Size != nil {
			if h, ok := paperHeightsInch[*layout.Size]; ok {
				height = h
			}
		}
		if layout.Orientation != nil && *layout.Orientation == "landscape" {
			// Альбомная ориентация меняет стороны местами: считаем по короткой.
			height = paperWidthInch(layout.Size)
		}
		if layout.AdjustTo != nil && *layout.AdjustTo >= 10 && *layout.AdjustTo <= 400 {
			scale = float64(*layout.AdjustTo)
		}
	}

	top, bottom := 0.75, 0.75
	if margins, err := f.GetPageMargins(sheet); err == nil {
		if margins.Top != nil {
			top = *margins.Top
		}
		if margins.Bottom != nil {
			bottom = *margins.Bottom
		}
	}

	usable := (height - top - bottom) * pointsPerInch
	if usable <= 0 {
		usable = (defaultPaperHeightInch - 1.5) * pointsPerInch
	}
	// Масштаб печати сжимает содержимое, поэтому на страницу влезает больше пунктов.
	return usable * 100.0 / scale
}

// paperWidthInch - ширина листа в дюймах: нужна для альбомной ориентации, где по
// вертикали идёт короткая сторона.
func paperWidthInch(size *int) float64 {
	widths := map[int]float64{1: 8.5, 5: 8.5, 8: 11.69, 9: 8.27, 11: 5.83}
	if size != nil {
		if w, ok := widths[*size]; ok {
			return w
		}
	}
	return 8.27
}

// rowHeightPt - высота строки в пунктах. У бланков высоты заданы явно; на всякий случай
// подстраховываемся значением по умолчанию, чтобы нулевая высота не давала бесконечный
// цикл при разбиении.
func rowHeightPt(f *excelize.File, sheet string, row int) float64 {
	h, err := f.GetRowHeight(sheet, row)
	if err != nil || h <= 0 {
		return defaultRowHeightPt
	}
	return h
}

// insertRepeatedHeaders расставляет разрывы страниц и повторяет шапку той таблицы,
// которая переходит на следующую страницу. Возвращает число вставленных строк.
//
// Идём сверху вниз, накапливая высоту страницы. Когда очередная строка не помещается,
// ставим разрыв. Если строка принадлежит таблице, перед ней встаёт копия шапки этой
// таблицы: так продолжение таблицы читается само по себе, а не с середины списка.
func insertRepeatedHeaders(f *excelize.File, sheet string, tables []tableForPagination, lastRow int, sectionShifts []rowShift) []rowShift {
	headerShifts := make([]rowShift, 0, 4)
	if len(tables) == 0 || lastRow < 1 {
		return headerShifts
	}
	pageHeight := pageContentHeightPt(f, sheet)
	if pageHeight <= 0 {
		return headerShifts
	}
	applied := appliedShifts(sectionShifts)

	inserted := 0
	used := 0.0
	// Таблицы в срезе идут сверху вниз; их границы сдвигаются по мере вставок.
	for row := 1; row <= lastRow+inserted; row++ {
		h := rowHeightPt(f, sheet, row)
		if used+h <= pageHeight {
			used += h
			continue
		}

		// Строка на страницу не влезла - с неё начинается следующая.
		used = 0
		table := tableAt(tables, row)
		if table == nil || table.headerRow < 1 {
			if err := setPageBreak(f, sheet, row); err != nil {
				slog.Error("не удалось поставить разрыв страницы бланка", "error", err, "row", row)
			}
			used = h
			continue
		}

		// Копия шапки встаёт перед строкой и сдвигает всё ниже на одну строку.
		if err := f.DuplicateRowTo(sheet, table.headerRow, row); err != nil {
			slog.Error("не удалось повторить шапку таблицы бланка", "error", err, "row", row)
			used = h
			continue
		}
		shiftTablesFrom(tables, row, 1)
		// Формулы условного форматирования хранят координаты шаблона, поэтому сдвиг от
		// вставленной шапки переводим обратно в них.
		headerShifts = append(headerShifts, rowShift{
			fromRow: templateRowOf(row, applied, inserted),
			offset:  1,
		})
		inserted++

		if err := setPageBreak(f, sheet, row); err != nil {
			slog.Error("не удалось поставить разрыв страницы бланка", "error", err, "row", row)
		}
		// На новой странице уже стоит шапка, следом идёт перенесённая строка.
		used = rowHeightPt(f, sheet, row) + rowHeightPt(f, sheet, row+1)
		row++
	}
	return headerShifts
}

// appliedShift - расширение списка в координатах готового файла: с какой строки оно
// сдвинуло разметку и на сколько строк.
type appliedShift struct {
	realFrom int
	offset   int
}

// appliedShifts переводит сдвиги секций из координат шаблона в координаты готового
// файла: каждое следующее расширение уезжает вниз на все предыдущие.
func appliedShifts(shifts []rowShift) []appliedShift {
	ordered := make([]rowShift, 0, len(shifts))
	for _, sh := range shifts {
		if sh.offset > 0 {
			ordered = append(ordered, sh)
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].fromRow < ordered[j].fromRow })

	out := make([]appliedShift, 0, len(ordered))
	acc := 0
	for _, sh := range ordered {
		out = append(out, appliedShift{realFrom: sh.fromRow + acc, offset: sh.offset})
		acc += sh.offset
	}
	return out
}

// templateRowOf - какой строке шаблона соответствует строка готового файла: вычитаем
// расширения списков выше неё и уже вставленные шапки.
func templateRowOf(realRow int, applied []appliedShift, headersAbove int) int {
	row := realRow - headersAbove
	for _, sh := range applied {
		if sh.realFrom <= realRow {
			row -= sh.offset
		}
	}
	if row < 1 {
		return 1
	}
	return row
}

// lastMarkupRow - последняя строка с разметкой: ниже неё считать разбиение незачем.
func lastMarkupRow(f *excelize.File, sheet string) int {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return 0
	}
	return len(rows)
}

// setPageBreak ставит горизонтальный разрыв перед строкой row.
func setPageBreak(f *excelize.File, sheet string, row int) error {
	cell, err := excelize.CoordinatesToCellName(1, row)
	if err != nil {
		return err
	}
	return f.InsertPageBreak(sheet, cell)
}

// tableAt - таблица, которой принадлежит строка данных. Шапка таблицы своей строкой
// не считается: повторять её перед самой собой не нужно.
func tableAt(tables []tableForPagination, row int) *tableForPagination {
	for i := range tables {
		if row >= tables[i].firstRow && row <= tables[i].lastRow {
			return &tables[i]
		}
	}
	return nil
}

// shiftTablesFrom сдвигает границы таблиц, оказавшихся ниже вставленной строки.
func shiftTablesFrom(tables []tableForPagination, row, offset int) {
	for i := range tables {
		if tables[i].headerRow >= row {
			tables[i].headerRow += offset
		}
		if tables[i].firstRow >= row {
			tables[i].firstRow += offset
		}
		if tables[i].lastRow >= row {
			tables[i].lastRow += offset
		}
	}
}
