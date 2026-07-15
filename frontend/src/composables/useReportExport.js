import { ref } from 'vue';
import { formatDateRu, formatReportCell } from '@/utils/datetime';
import { isDurationColumn, metricValue } from '@/utils/reportColumns';

// Значение колонки строки/итогов по общему контракту колонок (тому же, что у таблицы
// на экране). Длительность выгружаем в читаемом виде («2 ч 15 мин»), иначе в файле
// остались бы сырые секунды; «нет данных» -> пустая ячейка (как и прочие пустые
// значения выгрузки), а не 0 — движок производной метрике ноль не дорисовывает.
function colValue(bucket, floatBucket, col) {
  const value = metricValue(bucket, floatBucket, col);
  if (value === null) return '';
  return isDurationColumn(col) ? formatReportCell(value, 'duration') : value;
}

/**
 * Приводит результат отчёта (aggregate или list) к плоской таблице для выгрузки.
 * Поддерживает обе формы aggregate-ответа движка: мультиметрик (columns +
 * metric_rows + totals) и одиночную метрику (rows[{label,value}] + total + unit).
 * Cross-tab pivot-колонки уже лежат в columns/values, дробные метрики — в
 * float_values/float_totals. Период-разрез (день/неделя/месяц) выводит подписи
 * строк в дд.мм.гггг.
 *
 * @param {object} result результат POST /statistics/report
 * @returns {{ sheetName: string, header: string[], rows: Array<Array<string|number>>,
 *   totalsRow: Array<string|number>|null, numericColumns: boolean[] }}
 *   numericColumns — колонки, выравниваемые вправо (метрики; у list не размечаем —
 *   там тип ячейки определяется по значению).
 */
export function reportToTable(result) {
  if (!result) return { sheetName: 'Отчёт', header: [], rows: [], totalsRow: null, numericColumns: [] };

  if (result.mode === 'list') {
    const columns = result.columns || [];
    return {
      sheetName: 'Выгрузка',
      header: columns.map((c) => c.label),
      rows: (result.rows || []).map((row) => columns.map((c) => cellText(row[c.key], c.type))),
      totalsRow: null,
      numericColumns: [],
    };
  }

  const columns = result.columns?.length
    ? result.columns
    : [{ key: 'value', label: 'Количество', unit: result.unit || '' }];
  const metricRows = result.metric_rows
    || (result.rows || []).map((row) => ({ label: row.label, values: { value: row.value } }));
  const totals = result.totals || { value: result.total ?? 0 };
  const floatTotals = result.float_totals || {};
  const isPeriod = result.dimension === 'period';

  const dimHeader = result.dimension === 'none' ? 'Итог' : 'Значение разреза';
  const header = [dimHeader, ...columns.map((c) => (c.unit ? `${c.label}, ${c.unit}` : c.label))];
  const rows = metricRows.map((r) => [
    isPeriod ? formatDateRu(r.label) : r.label,
    ...columns.map((c) => colValue(r.values, r.float_values, c)),
  ]);
  // «Без разреза» — единственная строка уже итоговая, отдельная строка итогов лишняя.
  const totalsRow = result.dimension === 'none'
    ? null
    : ['Итого', ...columns.map((c) => colValue(totals, floatTotals, c))];

  // Первая колонка — подпись разреза (текст), остальные метрики выравниваются вправо
  // и когда значение стало текстом («2 ч 15 мин»), как в таблице на экране.
  const numericColumns = [false, ...columns.map(() => true)];

  return { sheetName: 'Сводка', header, rows, totalsRow, numericColumns };
}

function cellText(value, type) {
  if (value === null || value === undefined || value === '') return '';
  return formatReportCell(value, type);
}

/**
 * Ширина колонок в «символах» — максимум длины заголовка и всех ячеек таблицы
 * (включая строку итогов). База для авто-ширины: колонка не должна быть уже своего
 * содержимого. Excel применяет это напрямую (ширина листа в символах), PDF —
 * через widths:'auto' (фитит содержимое тем же смыслом).
 *
 * @param {{ header: string[], rows: Array<Array<string|number>>, totalsRow: Array<string|number>|null }} table
 * @returns {number[]} длина (в символах) самой длинной ячейки каждой колонки
 */
export function computeColumnWidths(table) {
  const cols = table.header.length;
  const allRows = [table.header, ...table.rows, ...(table.totalsRow ? [table.totalsRow] : [])];
  const widths = new Array(cols).fill(0);
  for (const row of allRows) {
    for (let i = 0; i < cols; i++) {
      const len = String(row[i] ?? '').length;
      if (len > widths[i]) widths[i] = len;
    }
  }
  return widths;
}

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

function excelColumnWidths(table) {
  const widths = computeColumnWidths(table).map((w) => Math.min(COL_MAX, Math.max(COL_MIN, w + 2)));
  if (widths.length) widths[0] = Math.max(widths[0], FIRST_COL_MIN);
  return widths;
}

function periodLabel(opts) {
  return opts.period?.from || opts.period?.to
    ? `${formatDateRu(opts.period.from) || '...'} - ${formatDateRu(opts.period.to) || '...'}`
    : 'весь период';
}

function exportStamp() {
  return new Date().toLocaleString('ru-RU').replace(/[.:,\s]/g, '-');
}

function downloadFileName(opts, ext) {
  return `Отчёт_${(opts.title || 'аналитика').replace(/\s+/g, '_')}_${exportStamp()}.${ext}`;
}

function downloadBlob(blob, filename) {
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.download = filename;
  a.href = url;
  // Без attach в DOM Firefox/Safari не запускают скачивание по click().
  document.body.appendChild(a);
  a.click();
  a.remove();
  window.URL.revokeObjectURL(url);
}

/**
 * Экспорт в .xlsx в фирменном стиле (заливка шапки primary, чередование строк,
 * рамка, строка итогов, авто-ширина колонок и подпись формирования). ExcelJS
 * тянется лениво, чтобы не утяжелять основной бандл.
 */
async function exportExcel(table, opts) {
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
    worksheet.addRow(['Период:', periodLabel(opts)]),
    worksheet.addRow(['Сформировал:', opts.author || 'Пользователь']),
    worksheet.addRow(['Дата формирования:', new Date().toLocaleString('ru-RU')]),
  ];
  infoRows.forEach((row) => {
    row.eachCell((cell) => {
      cell.font = { name: 'Verdana', size: 10, color: { argb: 'FF333333' } };
      cell.alignment = { vertical: 'middle' };
    });
  });

  const buffer = await workbook.xlsx.writeBuffer();
  const blob = new Blob([buffer], {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  });
  downloadBlob(blob, downloadFileName(opts, 'xlsx'));
}

const PDF_PRIMARY = '#4F5BDF';
const PDF_ROW_EVEN = '#F0F5FF';
const PDF_TOTALS = '#D3DCFF';
const PDF_BORDER = '#E6E6E6';

// Число выравниваем вправо (как в таблице на экране), текст и первая колонка-разрез —
// влево. Колонки-метрики размечены сводкой (numericColumns): длительность уезжает в
// файл текстом («2 ч 15 мин»), но остаётся числовой колонкой, как на экране.
function pdfAlign(value, colIndex, numericColumns) {
  if (colIndex === 0) return 'left';
  if (numericColumns?.[colIndex]) return 'right';
  return typeof value === 'number' ? 'right' : 'left';
}

function pdfCellText(value) {
  return value === null || value === undefined ? '' : String(value);
}

// getBlob/getBuffer в pdfmake 0.3 — Promise; в более старых — callback. Поддерживаем обе.
function pdfDocToBlob(pdfDoc) {
  return new Promise((resolve, reject) => {
    let settled = false;
    const done = (blob) => { if (!settled) { settled = true; resolve(blob); } };
    try {
      const ret = pdfDoc.getBlob(done);
      if (ret && typeof ret.then === 'function') ret.then(done, (e) => { if (!settled) reject(e); });
    } catch (e) {
      reject(e);
    }
  });
}

/**
 * Экспорт в .pdf через pdfmake: фирменная шапка primary, чередование строк, строка
 * итогов, авто-ширина колонок (widths:'auto' — не уже содержимого), широкие таблицы
 * (>4 колонок) в альбомной ориентации. pdfmake тянется лениво; Roboto (vfs) несёт
 * кириллицу из коробки.
 */
async function exportPdf(table, opts) {
  const pdfMake = (await import('pdfmake/build/pdfmake')).default;
  const vfs = (await import('pdfmake/build/vfs_fonts')).default;
  pdfMake.addVirtualFileSystem(vfs);

  const headerCells = table.header.map((text) => ({
    text: pdfCellText(text), bold: true, color: '#FFFFFF', fillColor: PDF_PRIMARY, alignment: 'center',
  }));
  const bodyRows = table.rows.map((cells, index) => cells.map((value, i) => ({
    text: pdfCellText(value),
    alignment: pdfAlign(value, i, table.numericColumns),
    fillColor: index % 2 === 0 ? PDF_ROW_EVEN : '#FFFFFF',
  })));
  const body = [headerCells, ...bodyRows];
  if (table.totalsRow) {
    body.push(table.totalsRow.map((value, i) => ({
      text: pdfCellText(value), bold: true, alignment: pdfAlign(value, i, table.numericColumns), fillColor: PDF_TOTALS,
    })));
  }

  const docDefinition = {
    pageOrientation: table.header.length > 4 ? 'landscape' : 'portrait',
    pageMargins: [24, 28, 24, 28],
    defaultStyle: { font: 'Roboto', fontSize: 8, color: '#333333' },
    content: [
      { text: opts.title || 'Отчёт по аналитике', fontSize: 13, bold: true, color: '#1A1A1A', margin: [0, 0, 0, 2] },
      { text: `Период: ${periodLabel(opts)}`, fontSize: 9, color: '#666666', margin: [0, 0, 0, 8] },
      {
        table: { headerRows: 1, widths: table.header.map(() => 'auto'), body },
        layout: {
          hLineWidth: () => 0.5,
          vLineWidth: () => 0.5,
          hLineColor: () => PDF_BORDER,
          vLineColor: () => PDF_BORDER,
          paddingTop: () => 4,
          paddingBottom: () => 4,
          paddingLeft: () => 6,
          paddingRight: () => 6,
        },
      },
      {
        text: `Сформировал: ${opts.author || 'Пользователь'} · ${new Date().toLocaleString('ru-RU')}`,
        fontSize: 8, color: '#999999', margin: [0, 10, 0, 0],
      },
    ],
  };

  const blob = await pdfDocToBlob(pdfMake.createPdf(docDefinition));
  downloadBlob(blob, downloadFileName(opts, 'pdf'));
}

/**
 * Выгрузка результата отчёта в выбранном формате. ExcelJS/pdfmake грузятся лениво.
 */
export function useReportExport() {
  const exporting = ref(false);

  /**
   * @param {object} result результат отчёта
   * @param {{ title?: string, period?: {from?: string, to?: string}, author?: string }} [opts]
   * @param {'excel'|'pdf'} [format] формат выгрузки
   */
  async function exportReport(result, opts = {}, format = 'excel') {
    const table = reportToTable(result);
    if (!table.header.length) throw new Error('Нет данных для выгрузки');

    exporting.value = true;
    try {
      if (format === 'pdf') {
        await exportPdf(table, opts);
      } else {
        await exportExcel(table, opts);
      }
    } finally {
      exporting.value = false;
    }
  }

  return { exporting, exportReport };
}
