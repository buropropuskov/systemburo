/** Выгрузка отчёта в .xlsx (лист рисует общий excelSheet). */
import { formatMoscowDateTime } from '@/utils/serverTime';
import { computeColumnWidths, periodLabel } from '@/utils/reportTable';
import { downloadFileName } from '@/utils/reportDownload';
import { downloadExcelSheet } from '@/utils/excelSheet';

// Ширина колонки в Excel меряется символами шрифта книги по умолчанию (Calibri 11),
// а лист мы печатаем Verdana - она заметно шире, и «в символах» ровно по содержимому
// обрезало и заголовки, и длинные значения (#2332). Заголовок к тому же полужирный
// и на два пункта крупнее строк, поэтому у него свой коэффициент.
const HEADER_CHAR_RATIO = 1.45;
const BODY_CHAR_RATIO = 1.15;
// Excel: символьная ширина с запасом и разумными границами, чтобы текст не обрезался,
// но колонка не растягивалась на пол-листа от одной длинной строки.
const COL_MIN = 8;
const COL_MAX = 60;
// Первая колонка несёт и подписи нижнего блока («Дата формирования:» = 18) — не уже их.
const FIRST_COL_MIN = 20;

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
 * Экспорт в .xlsx в фирменном стиле: заливка шапки, чередование строк, рамка, строка
 * итогов и подпись формирования. Сам лист рисует общий excelSheet (#2418) - здесь
 * остаётся только то, что специфично отчёту: свои ширины и свой блок подписей.
 */
export async function exportExcel(table, opts) {
  await downloadExcelSheet({
    sheetName: table.sheetName,
    header: table.header,
    rows: table.rows,
    widths: excelColumnWidths(table),
    totalsRow: table.totalsRow,
    info: [
      ['Отчёт:', opts.title || 'Отчёт по аналитике'],
      ['Период:', periodLabel(opts, table)],
      ['Сформировал:', opts.author || 'Пользователь'],
      ['Строк:', table.rows.length],
      ['Дата формирования:', formatMoscowDateTime()],
    ],
  }, downloadFileName(opts, 'xlsx'));
}
