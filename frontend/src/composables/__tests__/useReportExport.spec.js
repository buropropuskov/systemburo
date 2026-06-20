import { describe, it, expect } from 'vitest';
import { reportToTable } from '../useReportExport';

describe('reportToTable', () => {
  it('мультиметрик aggregate: колонки-метрики, строки значений и строка итогов', () => {
    const t = reportToTable({
      mode: 'aggregate',
      dimension: 'organization',
      columns: [{ key: 'a', label: 'Заявки', unit: 'шт' }, { key: 'b', label: 'Товары' }],
      metric_rows: [
        { label: 'ООО А', values: { a: 10, b: 120 } },
        { label: 'ООО Б', values: { a: 4, b: 30 } },
      ],
      totals: { a: 14, b: 150 },
    });
    expect(t.header).toEqual(['Значение разреза', 'Заявки, шт', 'Товары']);
    expect(t.rows).toEqual([['ООО А', 10, 120], ['ООО Б', 4, 30]]);
    expect(t.totalsRow).toEqual(['Итого', 14, 150]);
  });

  it('разрез «без разреза»: заголовок «Итог», без отдельной строки итогов', () => {
    const t = reportToTable({
      mode: 'aggregate',
      dimension: 'none',
      columns: [{ key: 'a', label: 'Заявки', unit: 'шт' }],
      metric_rows: [{ label: 'Итого', values: { a: 14 } }],
      totals: { a: 14 },
    });
    expect(t.header[0]).toBe('Итог');
    expect(t.totalsRow).toBeNull();
    expect(t.rows).toEqual([['Итого', 14]]);
  });

  it('одиночная метрика (legacy rows/total/unit) сводится к одной колонке', () => {
    const t = reportToTable({
      mode: 'aggregate', dimension: 'status', unit: 'шт',
      rows: [{ label: 'Завершено', value: 3 }], total: 3,
    });
    expect(t.header).toEqual(['Значение разреза', 'Количество, шт']);
    expect(t.rows).toEqual([['Завершено', 3]]);
    expect(t.totalsRow).toEqual(['Итого', 3]);
  });

  it('cross-tab pivot + float: pivot в values, дробная метрика в float_values/float_totals, период -> дд.мм.гггг', () => {
    const t = reportToTable({
      mode: 'aggregate',
      dimension: 'period',
      columns: [
        { key: 'car_entries_count', label: 'Машины', unit: 'шт' },
        { key: 'avg_cars_per_day', label: 'Среднее/день', unit: 'шт/день', float: true },
        { key: 'att_propusk', label: 'Пропуск', kind: 'pivot' },
      ],
      metric_rows: [
        {
          label: '2026-06-01',
          values: { car_entries_count: 10, att_propusk: 6 },
          float_values: { avg_cars_per_day: 2.5 },
        },
      ],
      totals: { car_entries_count: 10, att_propusk: 6 },
      float_totals: { avg_cars_per_day: 2.5 },
    });
    expect(t.header).toEqual(['Значение разреза', 'Машины, шт', 'Среднее/день, шт/день', 'Пропуск']);
    expect(t.rows).toEqual([['01.06.2026', 10, 2.5, 6]]);
    expect(t.totalsRow).toEqual(['Итого', 10, 2.5, 6]);
  });

  it('list: заголовки и строки по колонкам сущности, без итогов', () => {
    const t = reportToTable({
      mode: 'list',
      columns: [{ key: 'number', label: 'Номер' }, { key: 'org', label: 'Организация' }],
      rows: [{ number: 'A-1', org: 'ООО Ромашка' }, { number: 'A-2', org: '' }],
      total: 2,
    });
    expect(t.header).toEqual(['Номер', 'Организация']);
    expect(t.rows).toEqual([['A-1', 'ООО Ромашка'], ['A-2', '']]);
    expect(t.totalsRow).toBeNull();
  });

  it('пустой результат не падает', () => {
    expect(reportToTable(null).header).toEqual([]);
  });
});
