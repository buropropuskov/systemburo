import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { nextTick } from 'vue';

// Каталог и управляемые промисы runReport вынесены в hoisted — vi.mock поднимается
// над импортами, его фабрика не видит обычные переменные модуля.
const { state, CATALOG } = vi.hoisted(() => ({
  state: { deferred: [] },
  CATALOG: {
    metrics: [{ key: 'applications_count', label: 'Заявки', unit: 'шт', dimensions: ['status'] }],
    dimensions: [{ key: 'status', label: 'Статус заявки' }],
    filters: [{ key: 'date_range', label: 'Период', type: 'date' }],
    list_entities: [{ key: 'cars', label: 'Машины', columns: [{ key: 'car_number', label: 'Номер' }], filters: ['organization'] }],
    granularities: [{ value: 'day', label: 'По дням' }],
  },
}));

vi.mock('@/api/statistics', () => ({
  getReportCatalog: () => Promise.resolve(CATALOG),
  runReport: () => new Promise((resolve) => { state.deferred.push(resolve); }),
}));

import ReportsTab from '../ReportsTab.vue';
import ReportBuilder from '../ReportBuilder.vue';
import ReportResult from '../ReportResult.vue';
import ReportStepper from '../ReportStepper.vue';

describe('ReportsTab', () => {
  it('при двух параллельных запусках показывает результат последнего, медленный предыдущий игнорирует', async () => {
    state.deferred.length = 0;
    const wrapper = mount(ReportsTab, { props: { from: '2026-06-01', to: '2026-06-07' } });
    await flushPromises(); // каталог загружен -> ReportBuilder отрисован

    const builder = wrapper.findComponent(ReportBuilder);
    builder.vm.$emit('run', { mode: 'aggregate', metric: 'applications_count', dimension: 'status' }); // A (seq 1)
    await nextTick();
    builder.vm.$emit('run', { mode: 'list', entity: 'cars' }); // B (seq 2)
    await nextTick();

    expect(state.deferred).toHaveLength(2);

    // Последний запрос (B) приходит первым, устаревший A — позже и должен быть отброшен.
    state.deferred[1]({ mode: 'list', columns: [], rows: [], total: 7 });
    await flushPromises();
    state.deferred[0]({ mode: 'aggregate', rows: [], total: 999, unit: 'шт' });
    await flushPromises();

    expect(wrapper.findComponent(ReportResult).props('result').total).toBe(7);
  });

  it('снимок мастера с заполненным периодом закрывает шаг «Период» степпера', async () => {
    const wrapper = mount(ReportsTab, { props: { from: '', to: '' } });
    await flushPromises();

    const builder = wrapper.findComponent(ReportBuilder);
    builder.vm.$emit('change', {
      mode: 'aggregate', metric: 'applications_count', dimension: 'status', entity: '', filterCount: 0, periodFilled: true,
    });
    await nextTick();

    const periodStep = wrapper.findComponent(ReportStepper).props('steps')[3];
    expect(periodStep.label).toContain('Период');
    expect(periodStep.state).toBe('done');
  });

  it('в list-режиме без периода шаг «Период» считается пройденным', async () => {
    const wrapper = mount(ReportsTab, { props: { from: '', to: '' } });
    await flushPromises();

    const builder = wrapper.findComponent(ReportBuilder);
    builder.vm.$emit('change', {
      mode: 'list', metric: '', dimension: '', entity: 'cars', filterCount: 0, periodApplicable: false, periodFilled: false,
    });
    await nextTick();

    expect(wrapper.findComponent(ReportStepper).props('steps')[3].state).toBe('done');
  });
});
