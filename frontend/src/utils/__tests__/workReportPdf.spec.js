import { describe, expect, it } from 'vitest';
import { workReportDocument } from '../workReportPdf';

describe('реестр проведения работ', () => {
  it('считает заявки по номеру и включает пересекающие ночь интервалы', () => {
    const result = {
      columns: [{ key: 'number' }, { key: 'work_time' }],
      rows: [
        { number: '1', work_time: '08:00 - 22:00' },
        { number: '1', work_time: '21:00 - 23:00' },
        { number: '2', work_time: '23:00 - 02:00' },
        { number: '3', work_time: '09:00 - 18:00' },
      ],
    };
    const doc = workReportDocument(result, {}, {}, {});
    const cells = doc.content.find(node => node.table).table.body[0];
    expect(cells.map(cell => cell.text.map(t => t.text.trim()))).toEqual([
      ['3', 'заявки'], ['2', 'ночные заявки'],
    ]);
    expect(doc.header(1)).toBeNull();
    expect(doc.header(2)).toBeTruthy();
    expect(doc.footer(2, 4).columns.at(-1).text).toBe('2 / 4');
  });
});
