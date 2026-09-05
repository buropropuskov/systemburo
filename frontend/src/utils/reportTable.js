/**
 * Результат отчёта -> плоская таблица для выгрузки, ширины колонок и подпись
 * периода. Общая часть Excel- и PDF-выгрузок: обе печатают одни и те же данные.
 */
import { formatDateRu, formatReportCell } from '@/utils/datetime';
import { isDurationColumn, metricValue, formatPhonesInText } from '@/utils/reportColumns';

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
 *   totalsRow: Array<string|number>|null, numericColumns: boolean[],
 *   dataPeriod: {from: string, to: string}|null }}
 *   numericColumns — колонки, выравниваемые вправо (метрики сводки; выгрузка строк
 *   идёт сплошным левым). dataPeriod — границы дат, реально попавших в отчёт: ими
 *   подписывается шапка, когда период не задан фильтром.
 */
export function reportToTable(result) {
  if (!result) return { sheetName: 'Отчёт', header: [], rows: [], totalsRow: null, numericColumns: [] };

  if (result.mode === 'list') {
    const columns = result.columns || [];
    return {
      sheetName: 'Выгрузка',
      header: columns.map((c) => c.label),
      rows: (result.rows || []).map((row) => columns.map((c) => cellValue(row[c.key], c.type))),
      totalsRow: null,
      // Выгрузка строк выравнивается сплошным левым - как таблица на экране (#2332).
      numericColumns: [],
      dataPeriod: listDataPeriod(result),
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

  return {
    sheetName: 'Сводка',
    header,
    rows,
    totalsRow,
    numericColumns,
    dataPeriod: isPeriod ? isoRange(metricRows.map((r) => r.label)) : null,
  };
}

// Число уходит в файл числом: строкой Excel помечает ячейку «число сохранено как
// текст» и не даёт по ней ни сортировки, ни суммы (#2332). Даты и время движок
// отдаёт строками, их форматируем для чтения.
function cellValue(value, type) {
  if (value === null || value === undefined || value === '') return '';
  if (typeof value === 'number') return value;
  return formatPhonesInText(formatReportCell(value, type));
}

// Даты, реально попавшие в выгрузку строк: движок отдаёт их ISO-строками, причём
// «период работ» несёт сразу две даты в одной ячейке - берём все, что нашли.
function listDataPeriod(result) {
  const dateKeys = (result.columns || [])
    .filter((c) => c.type === 'date' || c.type === 'datetime')
    .map((c) => c.key);
  if (!dateKeys.length) return null;
  const found = [];
  for (const row of (result.rows || [])) {
    for (const key of dateKeys) {
      const raw = row[key];
      if (typeof raw !== 'string') continue;
      for (const m of raw.matchAll(/\d{4}-\d{2}-\d{2}/g)) found.push(m[0]);
    }
  }
  return isoRange(found);
}

// Границы набора ISO-дат. Лексикографический порядок YYYY-MM-DD совпадает с
// хронологическим, поэтому сравниваем строки без разбора в Date.
function isoRange(values) {
  const dates = values.filter((v) => typeof v === 'string' && /^\d{4}-\d{2}-\d{2}/.test(v)).map((v) => v.slice(0, 10));
  if (!dates.length) return null;
  return { from: dates.reduce((a, b) => (a < b ? a : b)), to: dates.reduce((a, b) => (a > b ? a : b)) };
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

/**
 * Подпись периода в шапке выгрузки. Границы заданы фильтром - берём их; не заданы -
 * называем даты, которые реально попали в отчёт: «весь период» не говорит читателю
 * документа, за что этот документ (#2332). Дат в отчёте нет вовсе - остаётся слово.
 */
export function periodLabel(opts, table) {
  if (opts.period?.from || opts.period?.to) {
    return `${formatDateRu(opts.period.from) || '...'} - ${formatDateRu(opts.period.to) || '...'}`;
  }
  const data = table?.dataPeriod;
  if (!data) return 'весь период';
  const from = formatDateRu(data.from);
  const to = formatDateRu(data.to);
  return from === to ? from : `${from} - ${to}`;
}
