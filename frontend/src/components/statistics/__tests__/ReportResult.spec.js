import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { nextTick } from 'vue';
import ReportResult from '../ReportResult.vue';

// Экспорт в Excel тянет ExcelJS и пишет файл — в юнит-тесте подменяем composable
// и проверяем, что клик «Excel» зовёт exportReport с результатом и meta.
const { exportSpy } = vi.hoisted(() => ({ exportSpy: vi.fn() }));
vi.mock('@/composables/useReportExport', async () => {
  const { ref } = await import('vue');
  return { useReportExport: () => ({ exporting: ref(false), exportReport: exportSpy }) };
});

// Apex-обёртки тянут vue3-apexcharts (нужен реальный SVG/измерения) — в юнит-тесте
// переключателя они не нужны, подменяем стабами и читаем переданные props.
const areaStub = {
  name: 'AnalyticsAreaChart',
  props: ['data', 'height', 'seriesName', 'unitForms', 'isFloat'],
  template: '<div class="area-stub" />',
};
const barStub = {
  name: 'AnalyticsBarChart',
  props: ['data', 'height', 'seriesName', 'unitForms', 'isFloat'],
  template: '<div class="bar-stub" />',
};

function mountResult(result) {
  return mount(ReportResult, {
    props: { result },
    global: { stubs: { AnalyticsAreaChart: areaStub, AnalyticsBarChart: barStub } },
  });
}

const aggPeriod = {
  mode: 'aggregate', dimension: 'period', unit: 'шт',
  rows: [{ label: '2026-06-01', value: 2 }], total: 2,
};
const aggStatus = {
  mode: 'aggregate', dimension: 'status', unit: 'шт',
  rows: [{ label: 'Завершено', value: 5 }], total: 5,
};
const listRes = { mode: 'list', columns: [{ key: 'a', label: 'A' }], rows: [{ a: 'x' }], total: 1 };

// Мультиметрика (формат движка GR0): columns + metric_rows{label,values} + totals.
const aggMulti = {
  mode: 'aggregate',
  dimension: 'organization',
  columns: [
    { key: 'applications_count', label: 'Количество заявок', unit: 'шт' },
    { key: 'items_sum', label: 'Количество товаров', unit: 'шт' },
  ],
  metric_rows: [
    { label: 'ООО А', values: { applications_count: 10, items_sum: 120 } },
    { label: 'ООО Б', values: { applications_count: 4, items_sum: 30 } },
  ],
  totals: { applications_count: 14, items_sum: 150 },
};
const aggNone = {
  mode: 'aggregate',
  dimension: 'none',
  columns: [{ key: 'applications_count', label: 'Количество заявок', unit: 'шт' }],
  metric_rows: [{ label: 'Итого', values: { applications_count: 14 } }],
  totals: { applications_count: 14 },
};

// Cross-tab pivot + дробная метрика: период-разрез, обычная метрика + float-колонка
// (среднее, значение в float_values/float_totals) + pivot-колонки (значение в values).
const aggPivotFloat = {
  mode: 'aggregate',
  dimension: 'period',
  columns: [
    { key: 'car_entries_count', label: 'Машины', unit: 'шт', kind: 'metric' },
    { key: 'avg_cars_per_day', label: 'Среднее/день', unit: 'шт/день', kind: 'metric', float: true },
    { key: 'att_propusk', label: 'Пропуск', kind: 'pivot' },
    { key: 'att_zayavka', label: 'Заявка', kind: 'pivot' },
  ],
  metric_rows: [
    {
      label: '2026-06-01',
      values: { car_entries_count: 10, att_propusk: 6, att_zayavka: 4 },
      float_values: { avg_cars_per_day: 2.5 },
    },
  ],
  totals: { car_entries_count: 10, att_propusk: 6, att_zayavka: 4 },
  float_totals: { avg_cars_per_day: 2.5 },
};

describe('ReportResult — переключатель Таблица/График', () => {
  it('list-режим: переключателя нет, только таблица', () => {
    const w = mountResult(listRes);
    expect(w.find('[data-testid="rr-view-chart"]').exists()).toBe(false);
    expect(w.find('.rr__table').exists()).toBe(true);
  });

  it('aggregate: переключатель есть, по умолчанию таблица', () => {
    const w = mountResult(aggStatus);
    expect(w.find('[data-testid="rr-view-chart"]').exists()).toBe(true);
    expect(w.find('.rr__table').exists()).toBe(true);
    expect(w.findComponent(barStub).exists()).toBe(false);
    expect(w.findComponent(areaStub).exists()).toBe(false);
  });

  it('aggregate период: график — area (динамика во времени)', async () => {
    const w = mountResult(aggPeriod);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.findComponent(areaStub).exists()).toBe(true);
    expect(w.findComponent(barStub).exists()).toBe(false);
    // area получает ISO-подпись в timestamp — компонент сам форматит дд.мм.
    expect(w.findComponent(areaStub).props('data')).toEqual([{ timestamp: '2026-06-01', count: 2 }]);
  });

  it('aggregate разрез (не период): график — столбцы', async () => {
    const w = mountResult(aggStatus);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.findComponent(barStub).exists()).toBe(true);
    expect(w.findComponent(areaStub).exists()).toBe(false);
    expect(w.findComponent(barStub).props('data')).toEqual([{ label: 'Завершено', value: 5 }]);
  });

  it('aggregate с 0 строк: кнопка График недоступна', () => {
    const w = mountResult({ ...aggStatus, rows: [], total: 0 });
    expect(w.find('[data-testid="rr-view-chart"]').attributes('disabled')).toBeDefined();
  });

  it('в режиме график пришёл пустой результат: график убран, таблица и блокировка кнопки', async () => {
    const w = mountResult(aggStatus);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.findComponent(barStub).exists()).toBe(true);

    await w.setProps({ result: { ...aggStatus, rows: [], total: 0 } });
    await nextTick();
    expect(w.findComponent(barStub).exists()).toBe(false);
    expect(w.find('.rr__table').exists()).toBe(true);
    expect(w.find('[data-testid="rr-view-chart"]').attributes('disabled')).toBeDefined();
  });

  it('период-строки выводятся как дд.мм.гггг', () => {
    const w = mountResult(aggPeriod);
    const firstCell = w.findAll('.rr__table tbody tr')[0].findAll('td')[0].text();
    expect(firstCell).toBe('01.06.2026');
  });

  it('мультиметрика: колонка на метрику, значения по строкам и итоговая строка', () => {
    const w = mountResult(aggMulti);
    const headers = w.findAll('.rr__table thead th').map((th) => th.text());
    expect(headers[0]).toBe('Значение разреза');
    expect(headers[1]).toContain('Количество заявок');
    expect(headers[2]).toContain('Количество товаров');

    const firstRow = w.findAll('.rr__table tbody tr')[0].findAll('td').map((td) => td.text());
    expect(firstRow).toEqual(['ООО А', '10', '120']);

    const footRow = w.findAll('.rr__table tfoot td').map((td) => td.text());
    expect(footRow).toEqual(['Итого', '14', '150']);
  });

  it('cross-tab pivot + float: pivot-колонки в values, float — в float_values/float_totals', () => {
    const w = mountResult(aggPivotFloat);
    const headers = w.findAll('.rr__table thead th').map((th) => th.text());
    expect(headers).toEqual(['Значение разреза', 'Машины, шт', 'Среднее/день, шт/день', 'Пропуск', 'Заявка']);

    const row = w.findAll('.rr__table tbody tr')[0].findAll('td').map((td) => td.text());
    // дата + счётчик + float (с дробной) + два pivot-счётчика
    expect(row).toEqual(['01.06.2026', '10', '2,5', '6', '4']);

    const foot = w.findAll('.rr__table tfoot td').map((td) => td.text());
    expect(foot).toEqual(['Итого', '10', '2,5', '6', '4']);
  });

  it('cross-tab: график float-метрики берёт значение из float_values', async () => {
    const w = mountResult(aggPivotFloat);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await w.find('[data-testid="rr-metric-avg_cars_per_day"]').trigger('click');
    await nextTick();
    expect(w.findComponent(areaStub).props('data')).toEqual([{ timestamp: '2026-06-01', count: 2.5 }]);
    expect(w.findComponent(areaStub).props('seriesName')).toBe('Среднее/день');
    // float-метрика -> график не округляет (тултип/ось в дробях, как таблица).
    expect(w.findComponent(areaStub).props('isFloat')).toBe(true);
  });

  it('целочисленная метрика: isFloat=false (график округляет до целых)', async () => {
    const w = mountResult(aggPeriod);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.findComponent(areaStub).props('isFloat')).toBe(false);
  });

  it('мультиметрика: график показывает селектор метрик и рисует выбранную', async () => {
    const w = mountResult(aggMulti);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();

    // По умолчанию первая метрика (organization -> bar).
    expect(w.findComponent(barStub).props('data')).toEqual([
      { label: 'ООО А', value: 10 }, { label: 'ООО Б', value: 4 },
    ]);
    expect(w.findComponent(barStub).props('seriesName')).toBe('Количество заявок');

    // Переключаем на вторую метрику.
    await w.find('[data-testid="rr-metric-items_sum"]').trigger('click');
    await nextTick();
    expect(w.findComponent(barStub).props('data')).toEqual([
      { label: 'ООО А', value: 120 }, { label: 'ООО Б', value: 30 },
    ]);
    expect(w.findComponent(barStub).props('seriesName')).toBe('Количество товаров');
  });

  it('разрез «без разреза»: заголовок «Итог», без отдельной строки итогов', () => {
    const w = mountResult(aggNone);
    expect(w.findAll('.rr__table thead th')[0].text()).toBe('Итог');
    expect(w.find('.rr__table tfoot').exists()).toBe(false);
    const row = w.findAll('.rr__table tbody tr')[0].findAll('td').map((td) => td.text());
    expect(row).toEqual(['Итого', '14']);
  });

  it('одиночная метрика (legacy rows): селектора метрик нет', async () => {
    const w = mountResult(aggStatus);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.find('[data-testid="rr-metric-value"]').exists()).toBe(false);
    expect(w.findComponent(barStub).props('data')).toEqual([{ label: 'Завершено', value: 5 }]);
  });

  it('после сброса в null и нового мультиметрик-отчёта график берёт первую метрику', async () => {
    const w = mountResult(aggMulti);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await w.find('[data-testid="rr-metric-items_sum"]').trigger('click');
    await nextTick();
    expect(w.findComponent(barStub).props('seriesName')).toBe('Количество товаров');

    // Новый запуск: result -> null (пустой экран, вид сбрасывается на таблицу) ->
    // снова мультиметрика -> открываем график: метрика снова первая.
    await w.setProps({ result: null });
    await nextTick();
    await w.setProps({ result: aggMulti });
    await nextTick();
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();

    expect(w.findComponent(barStub).props('seriesName')).toBe('Количество заявок');
  });

  it('кнопка «Excel» зовёт экспорт с текущим результатом и meta', async () => {
    exportSpy.mockClear();
    const meta = { period: { from: '2026-06-01', to: '2026-06-07' } };
    const w = mount(ReportResult, {
      props: { result: aggMulti, meta },
      global: { stubs: { AnalyticsAreaChart: areaStub, AnalyticsBarChart: barStub } },
    });
    await w.find('[data-testid="rr-export"]').trigger('click');
    expect(exportSpy).toHaveBeenCalledWith(aggMulti, meta);
  });

  it('кнопка «Excel» недоступна на пустом результате', () => {
    const w = mountResult({ ...aggStatus, rows: [], total: 0 });
    expect(w.find('[data-testid="rr-export"]').attributes('disabled')).toBeDefined();
  });

  it('list-режим тоже отдаёт кнопку выгрузки', () => {
    const w = mountResult(listRes);
    expect(w.find('[data-testid="rr-export"]').exists()).toBe(true);
  });

  it('падение экспорта эмитит export-error с сообщением', async () => {
    exportSpy.mockRejectedValueOnce(new Error('диск переполнен'));
    const w = mount(ReportResult, {
      props: { result: aggMulti },
      global: { stubs: { AnalyticsAreaChart: areaStub, AnalyticsBarChart: barStub } },
    });
    await w.find('[data-testid="rr-export"]').trigger('click');
    await nextTick();
    expect(w.emitted('export-error')[0]).toEqual(['диск переполнен']);
  });

  it('смена разреза period->status переключает график area->столбцы', async () => {
    const w = mountResult(aggPeriod);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.findComponent(areaStub).exists()).toBe(true);

    await w.setProps({ result: aggStatus });
    await nextTick();
    // вид сохраняется (график), тип переключился на дефолт нового разреза
    expect(w.findComponent(areaStub).exists()).toBe(false);
    expect(w.findComponent(barStub).exists()).toBe(true);
  });
});
