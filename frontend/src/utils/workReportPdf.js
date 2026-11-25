import { getAttachmentTimePeriod } from './attachmentTimePeriod';
import { periodLabel } from './reportTable';
import { formatMoscowDateTime } from './serverTime';
import { pluralRu } from './entityCount';
import { workSnapshotLabel } from './workReportScope';

export const workReportColors = {
  ink: '#1B2540', accent: '#484FBC', muted: '#525C6C',
  header: '#E6E8F9', stripe: '#F8F9FC', line: '#A8B0C0', frame: '#78849C',
};

function summary(result) {
  const keys = new Set(result.columns.map(column => column.key));
  if (!keys.has('number')) return [];
  const numbers = new Set();
  const nights = new Set();
  for (const row of result.rows) {
    if (!row.number) continue;
    numbers.add(row.number);
    const [from, to] = String(row.work_time || '').split(' - ');
    const period = getAttachmentTimePeriod(from, to);
    if (period === 'night' || period === 'mixed') nights.add(row.number);
  }
  const counts = [[pluralRu(numbers.size, ['заявка', 'заявки', 'заявок']), numbers.size]];
  if (keys.has('work_time')) counts.push([pluralRu(nights.size, ['ночная заявка', 'ночные заявки', 'ночных заявок']), nights.size]);
  return [{
    table: { widths: counts.map(() => '*'), body: [counts.map(([label, value]) => ({
      text: [{ text: String(value) + '   ', fontSize: 16, bold: true }, { text: label, fontSize: 9 }],
      fillColor: '#F1F2FD', margin: [8, 4, 0, 4],
    }))] },
    layout: 'noBorders', margin: [0, 0, 0, 12],
  }];
}

export function workReportDocument(result, table, opts, tableNode) {
  const { ink, muted, accent } = workReportColors;
  const period = workSnapshotLabel(result) || periodLabel(opts, table);
  const generated = formatMoscowDateTime();
  return {
    info: { title: 'Проведение работ' },
    pageSize: 'A4', pageOrientation: 'landscape', pageMargins: [24, 38, 24, 32],
    defaultStyle: { font: 'Roboto', fontSize: 9, color: ink },
    header: page => page === 1 ? null : {
      margin: [24, 14, 24, 0], columns: [
        { text: 'Бюро пропусков   /   Проведение работ', bold: true },
        { text: period, alignment: 'right', color: muted },
      ],
    },
    footer: (page, pages) => ({
      margin: [24, 4, 24, 0], columns: [
        { text: 'Сформировано ' + generated + (opts.author ? '   ·   ' + opts.author : ''), color: muted, fontSize: 8 },
        { text: page + ' / ' + pages, bold: true, width: 'auto' },
      ],
    }),
    content: [
      { text: 'БЮРО ПРОПУСКОВ', bold: true, characterSpacing: 1, color: accent },
      { canvas: [{ type: 'line', x1: 0, x2: 793, y1: 0, y2: 0, lineWidth: 1.4, lineColor: accent }], margin: [0, 7, 0, 12] },
      { columns: [
        { text: 'Проведение работ', fontSize: 25, bold: true, width: '*' },
        { width: 'auto', stack: [{ text: result.work_scope ? 'По состоянию на' : 'Период подачи заявок', color: muted }, { text: period, bold: true, color: accent, margin: [0, 3, 0, 0] }] },
      ], margin: [0, 0, 0, 12] },
      ...summary(result), tableNode,
    ],
  };
}
