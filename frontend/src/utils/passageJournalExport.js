/**
 * Выгрузка журнала проходов в Excel (#2469).
 *
 * Форматирование листа было скопировано внутри модалок машин и людей, а после
 * перехода на страницы к нему добавилась ещё и оговорка про предел выгрузки. Держим
 * это одним местом: журнал у обеих сущностей выглядит одинаково, и расходиться в
 * шапке или рамках ему незачем. Общая унификация всех выгрузок системы - #2418.
 */
import ExcelJS from 'exceljs';

const THIN = { style: 'thin', color: { argb: 'FFE6E6E6' } };
const MEDIUM = { style: 'medium', color: { argb: 'FF000000' } };
const XLSX_MIME = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';

function thinBorder() {
  return { top: THIN, bottom: THIN, left: THIN, right: THIN };
}

/**
 * Собирает лист журнала: шапка, строки через строку по цвету, внешняя рамка и подпись
 * «кто и когда сформировал». note дописывается отдельной строкой - ею говорят, что в
 * файл попало не всё.
 *
 * @param {{sheetName: string, headers: string[], rows: Array<Array<string>>, columnWidths: number[], author: string, formedAt: string, note?: string}} spec
 * @returns {Promise<Blob>}
 */
export async function buildPassageJournalBlob(spec) {
  const { sheetName, headers, rows, columnWidths, author, formedAt, note } = spec;
  const workbook = new ExcelJS.Workbook();
  const sheet = workbook.addWorksheet(sheetName);

  const headerRow = sheet.addRow(headers);
  headerRow.height = 25;
  headerRow.eachCell((cell) => {
    cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF4F5BDF' } };
    cell.font = { name: 'Verdana', size: 11, bold: true, color: { argb: 'FFFFFFFF' } };
    cell.alignment = { vertical: 'middle', horizontal: 'center' };
    cell.border = thinBorder();
  });

  rows.forEach((values, index) => {
    const row = sheet.addRow(values);
    row.height = 20;
    const fgColor = { argb: index % 2 === 0 ? 'FFF0F5FF' : 'FFE0E9FF' };
    row.eachCell((cell) => {
      cell.fill = { type: 'pattern', pattern: 'solid', fgColor };
      cell.font = { name: 'Verdana', size: 9, color: { argb: 'FF333333' } };
      cell.alignment = { vertical: 'middle' };
      cell.border = thinBorder();
    });
  });

  const lastRow = rows.length + 1;
  const lastCol = headers.length;
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

  sheet.addRow([]);
  const infoRows = [
    sheet.addRow(['Отчёт сформировал:', author]),
    sheet.addRow(['Дата формирования:', formedAt]),
  ];
  if (note) infoRows.push(sheet.addRow(['Выгружено строк:', note]));
  infoRows.forEach((row) => {
    row.eachCell((cell) => {
      cell.font = { name: 'Verdana', size: 10, color: { argb: 'FF333333' } };
      cell.alignment = { vertical: 'middle' };
      cell.border = thinBorder();
    });
  });

  sheet.columns = columnWidths.map(width => ({ width }));

  const buffer = await workbook.xlsx.writeBuffer();
  return new Blob([buffer], { type: XLSX_MIME });
}

/**
 * Отдаёт собранный файл браузеру.
 *
 * @param {Blob} blob
 * @param {string} filename
 */
export function saveJournalBlob(blob, filename) {
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.download = filename;
  link.href = url;
  link.click();
  window.URL.revokeObjectURL(url);
}
