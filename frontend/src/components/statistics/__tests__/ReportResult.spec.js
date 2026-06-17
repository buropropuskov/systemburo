import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import { nextTick } from 'vue';
import ReportResult from '../ReportResult.vue';

// ReportChart лениво тянет chart.js + canvas — в юнит-тесте логики переключателя
// он не нужен, подменяем стабом и читаем переданные ему props.
const chartStub = {
  name: 'ReportChart',
  props: ['rows', 'type', 'unit'],
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
