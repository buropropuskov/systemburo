/**
 * Лист .xlsx в фирменном виде - одна реализация на все выгрузки системы (#2418).
 *
 * До этого стиль был скопирован в двадцати шести файлах: синяя шапка, чередование строк,
 * тонкая рамка, подпись «кто и когда сформировал». Каждая копия - примерно сто тридцать
 * строк, и правка вида требовала обойти их все; на деле их не обходили, и выгрузки
 * потихоньку расходились.
 *
 * ExcelJS тянется лениво: библиотека весит больше, чем любой экран, который её вызывает,
 * и в основной бандл ей незачем (приём взят из reportExcel.js, где он уже применён).
 */
import { downloadBlob } from '@/utils/reportDownload';

const HEADER_FILL = 'FF4F5BDF';
const ROW_FILL_EVEN = 'FFF0F5FF';
const ROW_FILL_ODD = 'FFE0E9FF';
const TOTALS_FILL = 'FFD3DCFF';
const THIN = { style: 'thin', color: { argb: 'FFE6E6E6' } };
const THIN_BORDER = { top: THIN, bottom: THIN, left: THIN, right: THIN };
const MEDIUM = { style: 'medium', color: { argb: 'FF000000' } };
const XLSX_MIME = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';

/** Excel обрезает имя листа на 31 знаке - обрезаем сами, иначе книга не открывается. */
const SHEET_NAME_LIMIT = 31;

// Ширина колонки в Excel меряется символами шрифта книги по умолчанию, а печатаем мы
// Verdana - она шире, и «ровно по содержимому» обрезало текст (#2332). Заголовок вдобавок
// полужирный и на два пункта крупнее строк, поэтому у него свой коэффициент.
const HEADER_CHAR_RATIO = 1.45;
const BODY_CHAR_RATIO = 1.15;
const COL_MIN = 8;
const COL_MAX = 60;

/**
 * @typedef {Object} ExcelSheetSpec
 * @property {string} sheetName имя листа (обрежется до 31 знака)
 * @property {string[]} header заголовки колонок
 * @property {Array<Array<string|number|null>>} rows строки данных
 * @property {number[]} [widths] ширины колонок в символах
 * @property {boolean} [autoWidths] посчитать ширины по содержимому, если своих нет.
 *   Без флага и без widths ширины не задаются вовсе - у части выгрузок их и не было, а
 *   «посчитать всем на всякий случай» изменило бы файл, который человек привык видеть
 * @property {Array<string|number>} [totalsRow] строка итогов под данными
 * @property {Array<[string, string|number]>} [info] подписи под таблицей: пары «метка, значение»
 * @property {boolean} [outerBorder] жирная внешняя рамка вокруг таблицы
 * @property {number[]} [boldRows] какие строки данных набрать полужирным - подытоги,
 *   стоящие внутри данных, а не отдельной строкой в конце (отчёт по проходам)
 */

function textLength(value) {
  return value === null || value === undefined ? 0 : String(value).length;
}

/** Ширины по содержимому: запас под Verdana и разумные границы. */
function autoWidths(spec) {
  const columns = spec.header.length;
  const widths = [];
  for (let col = 0; col < columns; col += 1) {
    const headerFit = textLength(spec.header[col]) * HEADER_CHAR_RATIO;
    let bodyFit = 0;
    for (const row of spec.rows) {
      bodyFit = Math.max(bodyFit, textLength(row[col]) * BODY_CHAR_RATIO);
    }
    if (spec.totalsRow) {
      bodyFit = Math.max(bodyFit, textLength(spec.totalsRow[col]) * BODY_CHAR_RATIO);
    }
    widths.push(Math.min(COL_MAX, Math.max(COL_MIN, Math.ceil(Math.max(headerFit, bodyFit)) + 2)));
  }
  return widths;
}

/**
 * Собирает лист и отдаёт готовый файл.
 *
 * @param {ExcelSheetSpec} spec
 * @returns {Promise<Blob>}
 */
export async function buildExcelSheetBlob(spec) {
  const ExcelJS = (await import('exceljs')).default;
  const workbook = new ExcelJS.Workbook();
  const sheet = workbook.addWorksheet(String(spec.sheetName || 'Лист').slice(0, SHEET_NAME_LIMIT));

  const headerRow = sheet.addRow(spec.header);
  headerRow.height = 25;
  headerRow.eachCell((cell) => {
    cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: HEADER_FILL } };
    cell.font = { name: 'Verdana', size: 11, bold: true, color: { argb: 'FFFFFFFF' } };
    cell.alignment = { vertical: 'middle', horizontal: 'center' };
    cell.border = THIN_BORDER;
  });

  const bold = new Set(spec.boldRows || []);
  spec.rows.forEach((cells, index) => {
    const row = sheet.addRow(cells);
    row.height = 20;
    const fgColor = { argb: index % 2 === 0 ? ROW_FILL_EVEN : ROW_FILL_ODD };
    row.eachCell((cell) => {
      cell.fill = { type: 'pattern', pattern: 'solid', fgColor };
      cell.font = { name: 'Verdana', size: 9, bold: bold.has(index), color: { argb: 'FF333333' } };
      cell.alignment = { vertical: 'middle' };
      cell.border = THIN_BORDER;
    });
  });

  if (spec.totalsRow) {
    const row = sheet.addRow(spec.totalsRow);
    row.height = 22;
    row.eachCell((cell) => {
      cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: TOTALS_FILL } };
      cell.font = { name: 'Verdana', size: 10, bold: true, color: { argb: 'FF1A1A1A' } };
      cell.alignment = { vertical: 'middle' };
      cell.border = THIN_BORDER;
    });
  }

  if (spec.outerBorder) {
    const lastRow = spec.rows.length + 1 + (spec.totalsRow ? 1 : 0);
    const lastCol = spec.header.length;
    for (let row = 1; row <= lastRow; row += 1) {
      const left = sheet.getCell(row, 1);
      left.border = { ...left.border, left: MEDIUM };
      const right = sheet.getCell(row, lastCol);
      right.border = { ...right.border, right: MEDIUM };
    }
    for (let col = 1; col <= lastCol; col += 1) {
      const top = sheet.getCell(1, col);
      top.border = { ...top.border, top: MEDIUM };
      const bottom = sheet.getCell(lastRow, col);
      bottom.border = { ...bottom.border, bottom: MEDIUM };
    }
  }

  const widths = spec.widths && spec.widths.length
    ? spec.widths
    : (spec.autoWidths ? autoWidths(spec) : null);
  if (widths) sheet.columns = widths.map(width => ({ width }));

  if (spec.info && spec.info.length) {
    sheet.addRow([]);
    spec.info.forEach(([label, value]) => {
      const row = sheet.addRow([label, value]);
      row.eachCell((cell) => {
        cell.font = { name: 'Verdana', size: 10, color: { argb: 'FF333333' } };
        // Влево явно: число Excel иначе прижимает вправо, и подпись выпадает из
        // столбика соседних (#2332).
        cell.alignment = { vertical: 'middle', horizontal: 'left' };
      });
    });
  }

  const buffer = await workbook.xlsx.writeBuffer();
  return new Blob([buffer], { type: XLSX_MIME });
}

/**
 * Собирает лист и отдаёт его браузеру.
 *
 * @param {ExcelSheetSpec} spec
 * @param {string} filename имя файла вместе с расширением
 * @returns {Promise<void>}
 */
export async function downloadExcelSheet(spec, filename) {
  downloadBlob(await buildExcelSheetBlob(spec), filename);
}
