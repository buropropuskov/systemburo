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
// Выше этой доли листа фиксированные ширины перестают быть безопасными - отдаём
// раскладку обратно pdfmake.
const PDF_TIGHT_BUDGET = 0.75;

/**
 * Ширины колонок PDF. Узкой колонке даём ширину под её содержимое (и под самое
 * длинное слово заголовка - переносить заголовок можно), остальным - долю остатка.
 * Сплошное 'auto' раньше ужимало узкие колонки ради колонки с длинным текстом, и
 * номер заявки с периодом работ переносились (#2332). Когда узкие все, конкуренции
 * за ширину нет - 'auto' там точнее, а таблицу центрирует вызывающий.
 */
function pdfColumnWidths(table, tight, allTight) {
  if (allTight) return table.header.map(() => 'auto');
  const fixed = table.header.map((label, i) => {
    if (!tight[i]) return 0;
    const valueLen = table.rows.reduce((max, row) => Math.max(max, String(row[i] ?? '').length), 0);
    const wordLen = String(label ?? '').split(/\s+/).reduce((max, w) => Math.max(max, w.length), 0);
    return Math.ceil(Math.max(valueLen, wordLen) * PDF_CHAR_WIDTH) + PDF_CELL_PADDING;
  });
  const available = PDF_CONTENT_WIDTH[table.header.length > 4 ? 'landscape' : 'portrait'];
  const total = fixed.reduce((sum, w) => sum + w, 0);
  if (total > available * PDF_TIGHT_BUDGET) return table.header.map(() => 'auto');
  return fixed.map((w, i) => (tight[i] ? w : '*'));
}

// Значение узкой колонки не разрываем по пробелу: «15.08.2026 - 31.08.2026» и
// «№ 20260815/001» иначе ложатся в две строки, хотя колонка под них и вымерена.
// Строка короткая по определению узкой колонки — за поля не уйдёт.
function nowrapText(text) {
  return text.replace(/ /g, '\u00A0');
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
export async function exportPdf(table, opts) {
  const pdfMake = (await import('pdfmake/build/pdfmake')).default;
  const vfs = (await import('pdfmake/build/vfs_fonts')).default;
  pdfMake.addVirtualFileSystem(vfs);

  const headerCells = table.header.map((text) => ({
    text: pdfCellText(text), bold: true, color: '#FFFFFF', fillColor: PDF_PRIMARY, alignment: 'center',
  }));
  // Узкой колонке — ширина по содержимому, остальным — доля остатка: при сплошном
  // 'auto' pdfmake ужимал всё пропорционально, и номер заявки с периодом работ
  // переносились ради колонки с длинным текстом (#2332).
  const tight = table.header.map((_, i) => isTightColumnValues(table.rows.map((row) => row[i])));
  const allTight = tight.every(Boolean);

  const bodyRows = table.rows.map((cells, index) => cells.map((value, i) => ({
    text: tight[i] ? nowrapText(pdfCellText(value)) : pdfCellText(value),
    alignment: pdfAlign(i, table.numericColumns),
    fillColor: index % 2 === 0 ? PDF_ROW_EVEN : '#FFFFFF',
  })));
  const body = [headerCells, ...bodyRows];
  if (table.totalsRow) {
    body.push(table.totalsRow.map((value, i) => ({
      text: pdfCellText(value), bold: true, alignment: pdfAlign(i, table.numericColumns), fillColor: PDF_TOTALS,
    })));
  }

  const tableNode = {
    table: { headerRows: 1, widths: pdfColumnWidths(table, tight, allTight), body },
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
  };

  const docDefinition = {
    pageOrientation: table.header.length > 4 ? 'landscape' : 'portrait',
    pageMargins: [24, 28, 24, 28],
    defaultStyle: { font: 'Roboto', fontSize: 9, color: '#333333' },
    content: [
      { text: opts.title || 'Отчёт по аналитике', fontSize: 14, bold: true, color: '#1A1A1A', margin: [0, 0, 0, 2] },
      { text: `Период: ${periodLabel(opts, table)}`, fontSize: 10, color: '#666666', margin: [0, 0, 0, 8] },
      // Таблица из одних узких колонок уже страницы и раньше прижималась к левому
      // полю — центрируем распорками. Там, где есть доля остатка, она и так во всю
      // ширину листа, и обёртка только съела бы место (#2332).
      allTight
        ? { columns: [{ width: '*', text: '' }, { ...tableNode, width: 'auto' }, { width: '*', text: '' }] }
        : tableNode,
      {
        text: `Сформировал: ${opts.author || 'Пользователь'} · ${formatMoscowDateTime()} · строк: ${table.rows.length}`,
        fontSize: 9, color: '#999999', margin: [0, 10, 0, 0],
      },
    ],
  };

  const blob = await pdfDocToBlob(pdfMake.createPdf(docDefinition));
  downloadBlob(blob, downloadFileName(opts, 'pdf'));
}
