import { ref } from 'vue';

/**
 * Приводит результат отчёта (aggregate или list) к плоской таблице для выгрузки.
 * Поддерживает обе формы aggregate-ответа движка: мультиметрик (columns +
 * metric_rows + totals) и одиночную метрику (rows[{label,value}] + total + unit).
 *
 * @param {object} result результат POST /statistics/report
 * @returns {{ sheetName: string, header: string[], rows: Array<Array<string|number>>,
 *   totalsRow: Array<string|number>|null }}
 */
export function reportToTable(result) {
  if (!result) return { sheetName: 'Отчёт', header: [], rows: [], totalsRow: null };

  if (result.mode === 'list') {
    const columns = result.columns || [];
    return {
      sheetName: 'Выгрузка',
      header: columns.map((c) => c.label),
      rows: (result.rows || []).map((row) => columns.map((c) => cellText(row[c.key]))),
      totalsRow: null,
    };
  }

  const columns = result.columns?.length
    ? result.columns
    : [{ key: 'value', label: 'Количество', unit: result.unit || '' }];
  const metricRows = result.metric_rows
    || (result.rows || []).map((row) => ({ label: row.label, values: { value: row.value } }));
  const totals = result.totals || { value: result.total ?? 0 };

  const dimHeader = result.dimension === 'none' ? 'Итог' : 'Значение разреза';
  const header = [dimHeader, ...columns.map((c) => (c.unit ? `${c.label}, ${c.unit}` : c.label))];
  const rows = metricRows.map((r) => [r.label, ...columns.map((c) => Number(r.values?.[c.key] ?? 0))]);
  // «Без разреза» — единственная строка уже итоговая, отдельная строка итогов лишняя.
  const totalsRow = result.dimension === 'none'
    ? null
    : ['Итого', ...columns.map((c) => Number(totals[c.key] ?? 0))];

  return { sheetName: 'Сводка', header, rows, totalsRow };
}

function cellText(value) {
  if (value === null || value === undefined || value === '') return '';
  return value;
}

const HEADER_FILL = 'FF4F5BDF';
const ROW_FILL_EVEN = 'FFF0F5FF';
const ROW_FILL_ODD = 'FFE0E9FF';
const TOTALS_FILL = 'FFD3DCFF';
const THIN = { style: 'thin', color: { argb: 'FFE6E6E6' } };
const THIN_BORDER = { top: THIN, bottom: THIN, left: THIN, right: THIN };

/**
 * Экспорт результата отчёта в .xlsx в фирменном стиле (заливка шапки primary,
 * чередование строк, рамка, строка итогов и подпись формирования). ExcelJS тянется
 * лениво, чтобы не утяжелять основной бандл.
 */
export function useReportExport() {
  const exporting = ref(false);

  /**
   * @param {object} result результат отчёта
   * @param {{ title?: string, period?: {from?: string, to?: string}, author?: string }} [opts]
   */
  async function exportReport(result, opts = {}) {
    const table = reportToTable(result);
    if (!table.header.length) throw new Error('Нет данных для выгрузки');

    exporting.value = true;
    try {
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

      worksheet.columns = table.header.map((_, i) => ({ width: i === 0 ? 32 : 22 }));

      worksheet.addRow([]);
      const period = opts.period?.from || opts.period?.to
        ? `${opts.period.from || '...'} - ${opts.period.to || '...'}`
        : 'весь период';
      const infoRows = [
        worksheet.addRow(['Отчёт:', opts.title || 'Отчёт по аналитике']),
        worksheet.addRow(['Период:', period]),
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
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      const stamp = new Date().toLocaleString('ru-RU').replace(/[.:,\s]/g, '-');
      a.download = `Отчёт_${(opts.title || 'аналитика').replace(/\s+/g, '_')}_${stamp}.xlsx`;
      a.href = url;
      // Без attach в DOM Firefox/Safari не запускают скачивание по click().
      document.body.appendChild(a);
      a.click();
      a.remove();
      window.URL.revokeObjectURL(url);
    } finally {
      exporting.value = false;
    }
  }

  return { exporting, exportReport };
}
