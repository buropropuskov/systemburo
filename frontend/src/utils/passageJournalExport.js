/**
 * Выгрузка журнала проходов в Excel (#2469).
 *
 * Лист рисует общий excelSheet (#2418): вид у выгрузок системы один, и держать его копию
 * здесь незачем. Остался перевод спецификации журнала в общую - имена полей и блок
 * подписей с оговоркой про предел выгрузки.
 */
import { buildExcelSheetBlob } from '@/utils/excelSheet';
import { downloadBlob } from '@/utils/reportDownload';

/**
 * Собирает лист журнала: шапка, строки через строку по цвету, внешняя рамка и подпись
 * «кто и когда сформировал». note дописывается отдельной строкой - ею говорят, что в
 * файл попало не всё.
 *
 * @param {{sheetName: string, headers: string[], rows: Array<Array<string>>, columnWidths: number[], author: string, formedAt: string, note?: string}} spec
 * @returns {Promise<Blob>}
 */
export async function buildPassageJournalBlob(spec) {
  const info = [
    ['Отчёт сформировал:', spec.author],
    ['Дата формирования:', spec.formedAt],
  ];
  if (spec.note) info.push(['Выгружено строк:', spec.note]);

  return buildExcelSheetBlob({
    sheetName: spec.sheetName,
    header: spec.headers,
    rows: spec.rows,
    widths: spec.columnWidths,
    info,
    outerBorder: true,
  });
}

/**
 * Отдаёт собранный файл браузеру.
 *
 * @param {Blob} blob
 * @param {string} filename
 */
export function saveJournalBlob(blob, filename) {
  downloadBlob(blob, filename);
}
