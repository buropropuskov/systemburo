import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import { nextTick } from 'vue';
import ReportResult from '../ReportResult.vue';

// ReportChart лениво тянет chart.js + canvas — в юнит-тесте логики переключателя
// он не нужен, подменяем стабом и читаем переданные ему props.
const chartStub = {
  name: 'ReportChart',
  props: ['rows', 'type', 'unit', 'label'],
  template: '<div class="chart-stub" />',
};

function mountResult(result) {
  return mount(ReportResult, {
    props: { result },
    global: { stubs: { ReportChart: chartStub } },
  });
}

const aggPeriod = { mode: 'aggregate', dimension: 'period', unit: 'шт', rows: [{ label: 'Пн', value: 2 }], total: 2 };
const aggStatus = { mode: 'aggregate', dimension: 'status', unit: 'шт', rows: [{ label: 'Завершено', value: 5 }], total: 5 };
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
    expect(w.findComponent(chartStub).exists()).toBe(false);
  });

  it('aggregate разрез: типы графика — столбцы и круговая', async () => {
    const w = mountResult(aggStatus);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.find('[data-testid="rr-chart-bar"]').exists()).toBe(true);
    expect(w.find('[data-testid="rr-chart-pie"]').exists()).toBe(true);
    expect(w.find('[data-testid="rr-chart-line"]').exists()).toBe(false);
    expect(w.findComponent(chartStub).props('type')).toBe('bar');
  });

  it('aggregate период: по умолчанию линия, доступны линия и столбцы', async () => {
    const w = mountResult(aggPeriod);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.find('[data-testid="rr-chart-line"]').exists()).toBe(true);
    expect(w.find('[data-testid="rr-chart-bar"]').exists()).toBe(true);
    expect(w.find('[data-testid="rr-chart-pie"]').exists()).toBe(false);
    expect(w.findComponent(chartStub).props('type')).toBe('line');
  });

  it('aggregate с 0 строк: кнопка График недоступна', () => {
    const w = mountResult({ ...aggStatus, rows: [], total: 0 });
    expect(w.find('[data-testid="rr-view-chart"]').attributes('disabled')).toBeDefined();
  });

  it('в режиме график пришёл пустой результат: график убран, таблица и блокировка кнопки', async () => {
    const w = mountResult(aggStatus);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.findComponent(chartStub).exists()).toBe(true);

    await w.setProps({ result: { ...aggStatus, rows: [], total: 0 } });
    await nextTick();
    expect(w.findComponent(chartStub).exists()).toBe(false);
    expect(w.find('.rr__table').exists()).toBe(true);
    expect(w.find('[data-testid="rr-view-chart"]').attributes('disabled')).toBeDefined();
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

  it('мультиметрика: график показывает селектор метрик и рисует выбранную', async () => {
    const w = mountResult(aggMulti);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();

    // По умолчанию первая метрика.
    expect(w.findComponent(chartStub).props('rows')).toEqual([
      { label: 'ООО А', value: 10 }, { label: 'ООО Б', value: 4 },
    ]);
    expect(w.findComponent(chartStub).props('label')).toBe('Количество заявок');

    // Переключаем на вторую метрику.
    await w.find('[data-testid="rr-metric-items_sum"]').trigger('click');
    await nextTick();
    expect(w.findComponent(chartStub).props('rows')).toEqual([
      { label: 'ООО А', value: 120 }, { label: 'ООО Б', value: 30 },
    ]);
    expect(w.findComponent(chartStub).props('label')).toBe('Количество товаров');
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
    expect(w.findComponent(chartStub).props('rows')).toEqual([{ label: 'Завершено', value: 5 }]);
  });

  it('после сброса в null и нового мультиметрик-отчёта график берёт первую метрику', async () => {
    const w = mountResult(aggMulti);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await w.find('[data-testid="rr-metric-items_sum"]').trigger('click');
    await nextTick();
    expect(w.findComponent(chartStub).props('label')).toBe('Количество товаров');

    // Новый запуск: result -> null (пустой экран, вид сбрасывается на таблицу) ->
    // снова мультиметрика -> открываем график: метрика снова первая.
    await w.setProps({ result: null });
    await nextTick();
    await w.setProps({ result: aggMulti });
    await nextTick();
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();

    expect(w.findComponent(chartStub).props('label')).toBe('Количество заявок');
  });

  it('смена разреза period->status сбрасывает тип графика на столбцы', async () => {
    const w = mountResult(aggPeriod);
    await w.find('[data-testid="rr-view-chart"]').trigger('click');
    await nextTick();
    expect(w.findComponent(chartStub).props('type')).toBe('line');

    await w.setProps({ result: aggStatus });
    await nextTick();
    // вид сохраняется (график), тип переключился на дефолт нового разреза
    expect(w.findComponent(chartStub).exists()).toBe(true);
    expect(w.findComponent(chartStub).props('type')).toBe('bar');
  });
});
