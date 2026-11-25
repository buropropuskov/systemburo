/** Выгрузка отчёта в .pdf (pdfmake тянется лениво). */
import { formatMoscowDateTime } from '@/utils/serverTime';
import { isTightColumnValues } from '@/utils/reportColumns';
import { periodLabel } from '@/utils/reportTable';
import { downloadBlob, downloadFileName } from '@/utils/reportDownload';

const PDF_PRIMARY = '#4F5BDF';
const PDF_ROW_EVEN = '#F0F5FF';
const PDF_TOTALS = '#D3DCFF';
const PDF_BORDER = '#E6E6E6';

// Вправо — только размеченные колонки-метрики сводки (длительность уезжает в файл
// текстом «2 ч 15 мин», но остаётся метрикой). Выгрузка строк не размечена и идёт
// сплошным левым — как та же таблица на экране.
function pdfAlign(colIndex, numericColumns) {
  if (colIndex === 0) return 'left';
  return numericColumns?.[colIndex] ? 'right' : 'left';
}

function pdfCellText(value) {
  return value === null || value === undefined ? '' : String(value);
}

// Ширина листа под таблицей: A4 минус поля, в той ориентации, которую выберет
// exportPdf. Числа pdfmake считает в пунктах.
const PDF_CONTENT_WIDTH = { portrait: 547, landscape: 794 };
// Roboto 9pt: средняя ширина символа и горизонтальные поля ячейки (6 + 6 + запас).
const PDF_CHAR_WIDTH = 5.1;
const PDF_CELL_PADDING = 14;
// Оставляем текстовым колонкам не меньше четверти доступной ширины.
const PDF_TIGHT_BUDGET = 0.75;

/**
 * Ширины колонок PDF. Узкой колонке даём ширину под её содержимое (строки вместе с
 * итогами; заголовок мерим по самому длинному слову - его переносить можно),
 * остальным - долю остатка.
 * Когда все колонки узкие, вызывающий центрирует таблицу на листе.
 */
function pdfColumnWidths(table, rows, tight, orientation, scale) {
  const fixed = table.header.map((label, i) => {
    if (!tight[i]) return 0;
    const valueLen = rows.reduce((max, row) => Math.max(max, String(row[i] ?? '').length), 0);
    const wordLen = String(label ?? '').split(/\s+/).reduce((max, w) => Math.max(max, w.length), 0);
    return Math.ceil(Math.max(valueLen, wordLen) * PDF_CHAR_WIDTH) + PDF_CELL_PADDING;
  });
  // pdfmake прибавляет боковые отступы и линии к ширине содержимого каждой ячейки.
  const available = PDF_CONTENT_WIDTH[orientation] / scale - 12.5 * fixed.length - 1;
  const total = fixed.reduce((sum, w) => sum + w, 0);
  const flexible = tight.filter(value => !value).length;
  const budget = flexible ? available * PDF_TIGHT_BUDGET : available;
  const shrink = total > budget ? budget / total : 1;
  const widths = fixed.map(w => w ? Math.floor(w * shrink) : '*');
  const occupied = widths.reduce((sum, w) => sum + (w === '*' ? 0 : w), 0);
  const space = widths.map(w => w === '*' ? (available - occupied) / flexible : w);
  return { widths: widths.map(w => w === '*' ? w : w * scale), space };
}

// Значение узкой колонки не разрываем по пробелу: «15.08.2026 - 31.08.2026» и
// «№ 20260815/001» иначе ложатся в две строки, хотя колонка под них и вымерена.
// Строка короткая по определению узкой колонки — за поля не уйдёт.
function nowrapText(text) {
  return text.replace(/ /g, '\u00A0');
}

// pdfmake расширяет все колонки '*' под самое длинное неделимое слово.
// Разрешаем посимвольный перенос только для слов, которые могут выйти за
// выделенную ширину. Вставлять пробелы в исходное значение при этом не нужно.
function cellText(value, width, tight = false) {
  const text = pdfCellText(value);
  const pieces = text.split(/(\s+)/u);
  const mayOverflow = word => word.length * (/^[A-ZА-ЯЁ0-9]+$/u.test(word) ? 9 : 6.5) > width && word.trim();
  if (pieces.some(mayOverflow)) {
    return pieces.map(word => mayOverflow(word) ? { text: word, wordBreak: 'break-all' } : word);
  }
  return tight && text.length * PDF_CHAR_WIDTH + PDF_CELL_PADDING <= width ? nowrapText(text) : text;
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

function reportTableNode(table, orientation) {
  // Строка итогов идёт тем же столбцом, что и тело, поэтому меряется и не рвётся
  // наравне с ним: «2 ч 15 мин» в итоге иначе переносился там, где строки выше нет.
  const allRows = table.totalsRow ? [...table.rows, table.totalsRow] : table.rows;
  const tight = table.header.map((_, i) => isTightColumnValues(allRows.map((row) => row[i])));
  const allTight = tight.every(Boolean);
  // Очень широкий разрез целиком остаётся на одном листе по горизонтали.
  // Масштабируем только таблицу, сохраняя размер заголовка документа.
  const scale = Math.min(1, 12 / table.header.length);
  const { widths, space } = pdfColumnWidths(table, allRows, tight, orientation, scale);
  const headerCells = table.header.map((text, i) => ({
    text: pdfCellText(text),
    wordBreak: Array.isArray(cellText(text, space[i])) ? 'break-all' : 'normal',
    bold: true, color: '#FFFFFF', fillColor: PDF_PRIMARY, alignment: 'center',
  }));

  const bodyRows = table.rows.map((cells, index) => cells.map((value, i) => ({
    text: cellText(value, space[i], tight[i]),
    alignment: pdfAlign(i, table.numericColumns),
    fillColor: index % 2 === 0 ? PDF_ROW_EVEN : '#FFFFFF',
  })));
  const body = [headerCells, ...bodyRows];
  if (table.totalsRow) {
    body.push(table.totalsRow.map((value, i) => ({
      text: cellText(value, space[i], tight[i]),
      bold: true,
      alignment: pdfAlign(i, table.numericColumns),
      fillColor: PDF_TOTALS,
    })));
  }

  const tableNode = {
    fontSize: 9 * scale,
    table: { headerRows: 1, widths, body },
    layout: {
      hLineWidth: () => 0.5 * scale,
      vLineWidth: () => 0.5 * scale,
      hLineColor: () => PDF_BORDER,
      vLineColor: () => PDF_BORDER,
      paddingTop: () => 4 * scale,
      paddingBottom: () => 4 * scale,
      paddingLeft: () => 6 * scale,
      paddingRight: () => 6 * scale,
    },
  };

  return allTight
    ? { columns: [{ width: '*', text: '' }, { ...tableNode, width: 'auto' }, { width: '*', text: '' }] }
    : tableNode;
}

export async function exportPdf(table, opts) {
  const pdfMake = (await import('pdfmake/build/pdfmake')).default;
  const vfs = (await import('pdfmake/build/vfs_fonts')).default;
  pdfMake.addVirtualFileSystem(vfs);

  const orientation = table.header.length > 4 ? 'landscape' : 'portrait';
  const docDefinition = {
    pageOrientation: orientation,
    pageMargins: [24, 28, 24, 28],
    defaultStyle: { font: 'Roboto', fontSize: 9, color: '#333333' },
    content: [
      { text: opts.title || 'Отчёт по аналитике', fontSize: 14, bold: true, color: '#1A1A1A', margin: [0, 0, 0, 2] },
      { text: `Период: ${periodLabel(opts, table)}`, fontSize: 10, color: '#666666', margin: [0, 0, 0, 8] },
      reportTableNode(table, orientation),
      {
        text: `Сформировал: ${opts.author || 'Пользователь'} · ${formatMoscowDateTime()} · строк: ${table.rows.length}`,
        fontSize: 9, color: '#999999', margin: [0, 10, 0, 0],
      },
    ],
  };

  const blob = await pdfDocToBlob(pdfMake.createPdf(docDefinition));
  downloadBlob(blob, downloadFileName(opts, 'pdf'));
}
