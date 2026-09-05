/** Выгрузка отчёта в .xlsx (ExcelJS тянется лениво). */
import { formatMoscowDateTime } from '@/utils/serverTime';
import { computeColumnWidths, periodLabel } from '@/utils/reportTable';
import { downloadBlob, downloadFileName } from '@/utils/reportDownload';

const HEADER_FILL = 'FF4F5BDF';
const ROW_FILL_EVEN = 'FFF0F5FF';
const ROW_FILL_ODD = 'FFE0E9FF';
const TOTALS_FILL = 'FFD3DCFF';
const THIN = { style: 'thin', color: { argb: 'FFE6E6E6' } };
const THIN_BORDER = { top: THIN, bottom: THIN, left: THIN, right: THIN };

// Excel: символьная ширина с запасом и разумными границами, чтобы текст не обрезался,
// но колонка не растягивалась на пол-листа от одной длинной строки.
const COL_MIN = 8;
const COL_MAX = 60;
// Первая колонка несёт и подписи нижнего блока («Дата формирования:» = 18) — не уже их.
const FIRST_COL_MIN = 20;
// Ширина колонки в Excel меряется символами шрифта книги по умолчанию (Calibri 11),
// а лист мы печатаем Verdana - она заметно шире, и «в символах» ровно по содержимому
// обрезало и заголовки, и длинные значения (#2332). Заголовок к тому же полужирный
// и на два пункта крупнее строк, поэтому у него свой коэффициент.
const HEADER_CHAR_RATIO = 1.45;
const BODY_CHAR_RATIO = 1.15;

function excelColumnWidths(table) {
  const headerWidths = computeColumnWidths({ header: table.header, rows: [], totalsRow: null });
  const bodyWidths = computeColumnWidths({ header: table.header.map(() => ''), rows: table.rows, totalsRow: table.totalsRow });
  const widths = headerWidths.map((headerLen, i) => {
    const fit = Math.max(headerLen * HEADER_CHAR_RATIO, (bodyWidths[i] || 0) * BODY_CHAR_RATIO);
    return Math.min(COL_MAX, Math.max(COL_MIN, Math.ceil(fit) + 2));
  });
  if (widths.length) widths[0] = Math.max(widths[0], FIRST_COL_MIN);
  return widths;
}

/**
 * Экспорт в .xlsx в фирменном стиле (заливка шапки primary, чередование строк,
 * рамка, строка итогов, авто-ширина колонок и подпись формирования). ExcelJS
 * тянется лениво, чтобы не утяжелять основной бандл.
 */
export async function exportExcel(table, opts) {
  const ExcelJS = (await import('exceljs')).default;
  const workbook = new ExcelJS.Workbook();
  const worksheet = workbook.addWorksheet(table.sheetName);

  const headerRow = worksheet.addRow(table.header);
  headerRow.height = 25;
  headerRow.eachCell((cell) => {
    cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: HEADER_FILL } };
    cell.font = { name: 'Verdana', size: 11, bold: true, color: { argb: 'FFFFFFFF' } };
    cell.alignment = { vertical: 'middle', horizontal: 'center' };
    cell.border = THIN_BORDER;
  });

  table.rows.forEach((cells, index) => {
    const row = worksheet.addRow(cells);
    row.height = 20;
    const fill = index % 2 === 0 ? ROW_FILL_EVEN : ROW_FILL_ODD;
    row.eachCell((cell) => {
      cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: fill } };
      cell.font = { name: 'Verdana', size: 9, color: { argb: 'FF333333' } };
      cell.alignment = { vertical: 'middle' };
      cell.border = THIN_BORDER;
    });
  });

  if (table.totalsRow) {
    const row = worksheet.addRow(table.totalsRow);
    row.height = 22;
    row.eachCell((cell) => {
      cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: TOTALS_FILL } };
      cell.font = { name: 'Verdana', size: 10, bold: true, color: { argb: 'FF1A1A1A' } };
      cell.alignment = { vertical: 'middle' };
      cell.border = THIN_BORDER;
    });
  }

  worksheet.columns = excelColumnWidths(table).map((width) => ({ width }));

  worksheet.addRow([]);
  const infoRows = [
    worksheet.addRow(['Отчёт:', opts.title || 'Отчёт по аналитике']),
    worksheet.addRow(['Период:', periodLabel(opts, table)]),
    worksheet.addRow(['Сформировал:', opts.author || 'Пользователь']),
    worksheet.addRow(['Строк:', table.rows.length]),
    worksheet.addRow(['Дата формирования:', formatMoscowDateTime()]),
  ];
  infoRows.forEach((row) => {
    row.eachCell((cell) => {
      cell.font = { name: 'Verdana', size: 10, color: { argb: 'FF333333' } };
      // Влево явно: число строк Excel иначе прижимает вправо, и подпись выпадает
      // из столбика соседних (#2332).
      cell.alignment = { vertical: 'middle', horizontal: 'left' };
    });
  });

  const buffer = await workbook.xlsx.writeBuffer();
  const blob = new Blob([buffer], {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  });
  downloadBlob(blob, downloadFileName(opts, 'xlsx'));
}
