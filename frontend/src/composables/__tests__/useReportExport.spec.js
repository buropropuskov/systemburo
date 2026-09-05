import { describe, it, expect, vi } from 'vitest';
import { useReportExport } from '../useReportExport';
import { reportToTable, computeColumnWidths, resultDataPeriod } from '@/utils/reportTable';

// PDF-ветку тестируем с мок-pdfmake: проверяем, что формат 'pdf' строит документ
// через pdfmake (vfs + createPdf) и инициирует скачивание - без реального браузера.
vi.mock('pdfmake/build/pdfmake', () => ({
  default: {
    addVirtualFileSystem: vi.fn(),
    createPdf: vi.fn(() => ({ getBlob: (cb) => cb(new Blob(['%PDF'], { type: 'application/pdf' })) })),
  },
}));
vi.mock('pdfmake/build/vfs_fonts', () => ({ default: {} }));

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

  it('длительность выгружается читаемой, непройденный этап — пустой ячейкой (#1240)', () => {
    const t = reportToTable({
      mode: 'aggregate',
      dimension: 'organization',
      columns: [
        { key: 'avg_approval_time', label: 'Время согласования', type: 'duration' },
        { key: 'avg_completion_time', label: 'Время до завершения', type: 'duration' },
        { key: 'applications_count', label: 'Заявки', unit: 'шт' },
      ],
      metric_rows: [
        { label: 'ООО А', values: { avg_approval_time: 8100, avg_completion_time: 259200, applications_count: 10 } },
        // Этап завершения не пройден: движок ключ не выставляет, `?? 0` дал бы «0 мин».
        { label: 'ООО Б', values: { avg_approval_time: 0, applications_count: 4 } },
      ],
      totals: { avg_approval_time: 5400, applications_count: 14 },
    });
    expect(t.rows).toEqual([
      ['ООО А', '2 ч 15 мин', '3 сут', 10],
      ['ООО Б', '0 с', '', 4],
    ]);
    expect(t.totalsRow).toEqual(['Итого', '1 ч 30 мин', '', 14]);
    // Метрики остаются числовыми колонками (вправо), даже став текстом длительности.
    expect(t.numericColumns).toEqual([false, true, true, true]);
  });

  it('непосчитанная доля (float без ключа) -> пустая ячейка, счётчик -> честный 0', () => {
    const t = reportToTable({
      mode: 'aggregate',
      dimension: 'organization',
      columns: [
        { key: 'refusal_rate', label: 'Доля отказов', unit: '%', float: true },
        { key: 'applications_count', label: 'Заявки', unit: 'шт' },
      ],
      // Заявок в бине не было: доли нет (ключ не выставлен), счётчик честно 0.
      metric_rows: [{ label: 'ООО Б', values: {}, float_values: {} }],
      totals: {},
      float_totals: {},
    });
    expect(t.rows).toEqual([['ООО Б', '', 0]]);
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

describe('computeColumnWidths', () => {
  it('ширина колонки не меньше длины заголовка и самой длинной ячейки', () => {
    const table = {
      header: ['Значение разреза', 'Заявки, шт'],
      rows: [['ООО Длинное Название Организации', 4], ['А', 1200]],
      totalsRow: null,
    };
    const w = computeColumnWidths(table);
    // col0: max('Значение разреза'=16, 'ООО Длинное Название Организации'=31) -> 31
    expect(w[0]).toBe('ООО Длинное Название Организации'.length);
    expect(w[0]).toBeGreaterThanOrEqual('Значение разреза'.length);
    // col1: max('Заявки, шт'=10, '4'=1, '1200'=4) -> 10
    expect(w[1]).toBe('Заявки, шт'.length);
  });

  it('учитывает строку итогов при расчёте ширины', () => {
    const table = {
      header: ['Разрез', 'N'],
      rows: [['А', 1]],
      totalsRow: ['Итого по всем разрезам', 1],
    };
    const w = computeColumnWidths(table);
    expect(w[0]).toBe('Итого по всем разрезам'.length);
  });

  it('пустые/числовые ячейки не ломают расчёт', () => {
    const table = { header: ['A', 'B'], rows: [[null, 0], ['', undefined]], totalsRow: null };
    expect(computeColumnWidths(table)).toEqual([1, 1]);
  });
});

// Узкая таблица центрируется распорками, поэтому нода таблицы лежит внутри columns.
function findTableNode(nodes) {
  for (const node of nodes || []) {
    if (node?.table) return node;
    const nested = findTableNode(node?.columns);
    if (nested) return nested;
  }
  return null;
}

describe('useReportExport — PDF-экспорт', () => {
  it('формат pdf строит документ через pdfmake и скачивает файл', async () => {
    const pdfMake = (await import('pdfmake/build/pdfmake')).default;
    pdfMake.addVirtualFileSystem.mockClear();
    pdfMake.createPdf.mockClear();

    window.URL.createObjectURL = vi.fn(() => 'blob:test');
    window.URL.revokeObjectURL = vi.fn();
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    const { exporting, exportReport } = useReportExport();
    await exportReport(
      { mode: 'aggregate', dimension: 'status', unit: 'шт', rows: [{ label: 'Завершено', value: 3 }], total: 3 },
      { title: 'Отчёт', period: { from: '2026-06-01', to: '2026-06-07' } },
      'pdf',
    );

    expect(pdfMake.addVirtualFileSystem).toHaveBeenCalledTimes(1);
    expect(pdfMake.createPdf).toHaveBeenCalledTimes(1);
    const doc = pdfMake.createPdf.mock.calls[0][0];
    const tableNode = findTableNode(doc.content);
    expect(tableNode).toBeTruthy();
    // шапка таблицы и строка данных попали в документ
    expect(tableNode.table.body[0].map((c) => c.text)).toEqual(['Значение разреза', 'Количество, шт']);
    expect(tableNode.table.body).toHaveLength(3); // header + строка + итого
    expect(clickSpy).toHaveBeenCalled();
    expect(window.URL.revokeObjectURL).toHaveBeenCalled();
    expect(exporting.value).toBe(false);

    clickSpy.mockRestore();
  });

  it('пустой результат бросает ошибку до обращения к pdfmake', async () => {
    const pdfMake = (await import('pdfmake/build/pdfmake')).default;
    pdfMake.createPdf.mockClear();
    const { exportReport } = useReportExport();
    await expect(exportReport(null, {}, 'pdf')).rejects.toThrow('Нет данных');
    expect(pdfMake.createPdf).not.toHaveBeenCalled();
  });
});

describe('useReportExport — имя файла (#2324)', () => {
  it('вычищает кавычки и лишние символы из названия отчёта', async () => {
    window.URL.createObjectURL = vi.fn(() => 'blob:test');
    window.URL.revokeObjectURL = vi.fn();
    let downloadedAs = '';
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function click() {
      downloadedAs = this.download;
    });

    const { exportReport } = useReportExport();
    await exportReport(
      { mode: 'aggregate', dimension: 'status', unit: 'шт', rows: [{ label: 'Завершено', value: 1 }], total: 1 },
      { title: 'Количество заявок по разрезу «Организация»' },
      'pdf',
    );

    expect(downloadedAs).toContain('Количество_заявок_по_разрезу_Организация');
    expect(downloadedAs).not.toContain('«');
    expect(downloadedAs).toMatch(/^[\p{L}\p{N}_.-]+$/u);

    clickSpy.mockRestore();
  });
});

describe('useReportExport — выгрузка строк и период (#2332)', () => {
  it('list: числа остаются числами, текст форматируется', () => {
    const t = reportToTable({
      mode: 'list',
      columns: [
        { key: 'number', label: 'Номер' },
        { key: 'period', label: 'Период работ', type: 'date' },
        { key: 'people_count', label: 'Кол-во людей' },
      ],
      rows: [{ number: 'A-1', period: '2026-06-20 - 2026-06-21', people_count: 12 }],
      total: 1,
    });
    // число числом: строкой Excel ругается «число сохранено как текст»
    expect(t.rows[0][2]).toBe(12);
    expect(t.rows[0][1]).toBe('20.06.2026 - 21.06.2026');
  });

  it('период не задан фильтром -> шапка называет даты, попавшие в отчёт', async () => {
    const pdfMake = (await import('pdfmake/build/pdfmake')).default;
    pdfMake.createPdf.mockClear();
    window.URL.createObjectURL = vi.fn(() => 'blob:test');
    window.URL.revokeObjectURL = vi.fn();
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    const { exportReport } = useReportExport();
    await exportReport({
      mode: 'list',
      columns: [{ key: 'period', label: 'Период работ', type: 'date' }],
      rows: [{ period: '2026-06-20 - 2026-06-21' }, { period: '2026-04-02 - 2026-04-03' }],
      total: 2,
    }, { title: 'Проведение работ' }, 'pdf');

    const doc = pdfMake.createPdf.mock.calls[0][0];
    expect(doc.content[1].text).toBe('Период: 02.04.2026 - 21.06.2026');
    clickSpy.mockRestore();
  });

  it('дат в отчёте нет -> остаётся «весь период»', async () => {
    const pdfMake = (await import('pdfmake/build/pdfmake')).default;
    pdfMake.createPdf.mockClear();
    window.URL.createObjectURL = vi.fn(() => 'blob:test');
    window.URL.revokeObjectURL = vi.fn();
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    const { exportReport } = useReportExport();
    await exportReport({
      mode: 'list',
      columns: [{ key: 'car_number', label: 'Гос. номер' }],
      rows: [{ car_number: 'А123ВС' }],
      total: 1,
    }, { title: 'Машины по местам' }, 'pdf');

    const doc = pdfMake.createPdf.mock.calls[0][0];
    expect(doc.content[1].text).toBe('Период: весь период');
    clickSpy.mockRestore();
  });

  it('pdf: узкие колонки получают ширину под содержимое, длинный текст — долю остатка', async () => {
    const pdfMake = (await import('pdfmake/build/pdfmake')).default;
    pdfMake.createPdf.mockClear();
    window.URL.createObjectURL = vi.fn(() => 'blob:test');
    window.URL.revokeObjectURL = vi.fn();
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    const { exportReport } = useReportExport();
    await exportReport({
      mode: 'list',
      columns: [
        { key: 'number', label: 'Номер заявки' },
        { key: 'name', label: 'Наименование работ' },
        { key: 'people_count', label: 'Кол-во людей' },
      ],
      rows: [
        { number: '№ 20260815/001', name: 'Монтаж приточной вентиляции в помещении склада', people_count: 12 },
      ],
      total: 1,
    }, { title: 'Проведение работ' }, 'pdf');

    const doc = pdfMake.createPdf.mock.calls[0][0];
    const { widths, body } = findTableNode(doc.content).table;
    expect(typeof widths[0]).toBe('number');
    expect(widths[1]).toBe('*');
    expect(typeof widths[2]).toBe('number');
    // Выгрузка строк выравнивается сплошным левым — как таблица на экране.
    expect(body[1].map((c) => c.alignment)).toEqual(['left', 'left', 'left']);
    // Значение узкой колонки не рвётся по пробелу.
    expect(body[1][0].text).toBe('№\u00A020260815/001');
    clickSpy.mockRestore();
  });

  it('pdf: строка итогов узкой колонки не рвётся по пробелу наравне со строками', async () => {
    const pdfMake = (await import('pdfmake/build/pdfmake')).default;
    pdfMake.createPdf.mockClear();
    window.URL.createObjectURL = vi.fn(() => 'blob:test');
    window.URL.revokeObjectURL = vi.fn();
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    const { exportReport } = useReportExport();
    await exportReport({
      mode: 'aggregate',
      dimension: 'organization',
      columns: [{ key: 'p50_processing', label: 'Медиана обработки', type: 'duration' }],
      metric_rows: [
        { label: 'ООО «Очень длинное название организации-подрядчика»', values: { p50_processing: 8100 } },
      ],
      totals: { p50_processing: 9000 },
    }, { title: 'Сроки обработки' }, 'pdf');

    const doc = pdfMake.createPdf.mock.calls[0][0];
    const { widths, body } = findTableNode(doc.content).table;
    expect(typeof widths[1]).toBe('number'); // длительность — узкая колонка
    expect(body[1][1].text).toBe('2\u00A0ч\u00A015\u00A0мин');
    expect(body[2][1].text).toBe('2\u00A0ч\u00A030\u00A0мин'); // строка итогов
    clickSpy.mockRestore();
  });

  it('телефон в выгрузке идёт в той же маске, что и на экране (#2336)', () => {
    const t = reportToTable({
      mode: 'list',
      columns: [{ key: 'responsible', label: 'Ответственный' }],
      rows: [{ responsible: 'Системный администратор, 89100530055' }],
      total: 1,
    });
    expect(t.rows[0][0]).toBe('Системный администратор, +7 (910) 053 00-55');
  });
});

describe('resultDataPeriod (#2338)', () => {
  it('сводка по периоду: границы из подписей строк', () => {
    expect(resultDataPeriod({
      mode: 'aggregate',
      dimension: 'period',
      metric_rows: [{ label: '2026-09-05' }, { label: '2026-04-09' }, { label: '2026-06-01' }],
    })).toEqual({ from: '2026-04-09', to: '2026-09-05' });
  });

  it('выгрузка строк: границы по колонкам с датами, включая две даты в одной ячейке', () => {
    expect(resultDataPeriod({
      mode: 'list',
      columns: [{ key: 'work_period', label: 'Период работ', type: 'date' }, { key: 'name', label: 'Работы' }],
      rows: [
        { work_period: '2026-08-15 - 2026-08-31', name: 'Монтаж' },
        { work_period: '2026-04-22 - 2026-04-23', name: 'Осмотр' },
      ],
    })).toEqual({ from: '2026-04-22', to: '2026-08-31' });
  });

  it('разрез не по периоду и выгрузка без дат — границ нет', () => {
    expect(resultDataPeriod({
      mode: 'aggregate', dimension: 'status', rows: [{ label: 'Завершено', value: 2 }],
    })).toBeNull();
    expect(resultDataPeriod({
      mode: 'list', columns: [{ key: 'car_number', label: 'Гос. номер' }], rows: [{ car_number: 'А123ВС' }],
    })).toBeNull();
    expect(resultDataPeriod(null)).toBeNull();
  });
});
