/**
 * Выгрузка истории записи справочника в Excel (#2418).
 *
 * Девять модалок (гражданства, компании, организации, типы пользователей, марки, форматы
 * номеров, наименования вложений, учётные записи, места разгрузки) выгружали историю
 * буквально одинаково: те же шесть колонок, те же ширины, та же подпись - и по сто строк
 * кода на каждую. Здесь остался один пресет: вид листа рисует excelSheet, а модалка
 * передаёт имя листа, имя файла и строки.
 */
import { downloadExcelSheet } from '@/utils/excelSheet';

/** Колонки истории справочника - одинаковые у всех девяти разделов. */
export const DIRECTORY_HISTORY_HEADER = [
  'Дата и время',
  'Пользователь',
  'Действие',
  'Детали',
  'Тип действия',
  'ID записи',
];

/** Ширины под эти колонки: «Детали» шире всех, идентификатор узкий. */
export const DIRECTORY_HISTORY_WIDTHS = [22, 30, 30, 60, 22, 12];

/**
 * Собирает и отдаёт файл истории справочника.
 *
 * @param {{sheetName: string, filename: string, rows: Array<Array<string|number>>, author: string, formedAt: string}} spec
 * @returns {Promise<void>}
 */
export async function downloadDirectoryHistory(spec) {
  await downloadExcelSheet({
    sheetName: spec.sheetName,
    header: DIRECTORY_HISTORY_HEADER,
    rows: spec.rows,
    widths: DIRECTORY_HISTORY_WIDTHS,
    info: [
      ['Отчёт сформировал:', spec.author],
      ['Дата формирования:', spec.formedAt],
    ],
    outerBorder: true,
  }, spec.filename);
}
