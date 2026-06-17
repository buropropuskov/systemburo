import { describe, it, expect } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { nextTick } from 'vue';
import ReportBuilder from '../ReportBuilder.vue';
import FilterTabs from '@/components/ui/FilterTabs.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';

const CATALOG = {
  metrics: [
    { key: 'car_entries_count', label: 'Въезды машин', unit: 'шт', dimensions: ['unload_place', 'period'] },
    { key: 'applications_count', label: 'Количество заявок', unit: 'шт', dimensions: ['status', 'period'] },
  ],
  dimensions: [
    { key: 'unload_place', label: 'Место разгрузки' },
    { key: 'status', label: 'Статус заявки' },
    { key: 'period', label: 'Период (дата)' },
  ],
  filters: [
    { key: 'date_range', label: 'Период', type: 'date' },
    { key: 'organization', label: 'Организация', type: 'dict', options: [{ value: 'ООО А', label: 'ООО А' }] },
    { key: 'status', label: 'Статус заявки', type: 'enum', options: [{ value: 'Завершено', label: 'Завершено' }] },
  ],
  list_entities: [
    {
      key: 'work_applications',
      label: 'Заявка на работы',
      columns: [{ key: 'number', label: 'Номер' }],
      filters: ['date_range', 'organization', 'status'],
    },
    { key: 'cars', label: 'Машины', columns: [{ key: 'car_number', label: 'Номер' }], filters: ['organization'] },
  ],
  granularities: [{ value: 'day', label: 'По дням' }, { value: 'week', label: 'По неделям' }],
};

const PERIOD = { from: '2026-06-01', to: '2026-06-07' };

function mountBuilder() {
  return mount(ReportBuilder, { props: { catalog: CATALOG, period: PERIOD } });
}

function clickBuild(wrapper) {
  return wrapper.find('button.lk-button--primary').trigger('click');
}

function lastRun(wrapper) {
  const runs = wrapper.emitted('run');
  return runs[runs.length - 1][0];
}

describe('ReportBuilder', () => {
  it('по умолчанию строит aggregate первой метрики с её первым разрезом и периодом', async () => {
    const wrapper = mountBuilder();
    await nextTick();
    await clickBuild(wrapper);

    const req = lastRun(wrapper);
    expect(req.mode).toBe('aggregate');
    expect(req.metric).toBe('car_entries_count');
    expect(req.dimension).toBe('unload_place'); // первый разрез метрики
    expect(req.filters).toContainEqual({ key: 'date_range', from: '2026-06-01', to: '2026-06-07' });
  });

  it('в режиме list строит запрос сущности с выбранным фильтром', async () => {
    const wrapper = mountBuilder();
    wrapper.findComponent(FilterTabs).vm.$emit('update:modelValue', 'list');
    await nextTick();

    // Фильтры сущности отрисованы (организация + статус, date_range = период из шапки)
    const pills = wrapper.findAll('.rb__pill');
    expect(pills.length).toBe(2);

    // Выбираем организацию
    const orgPill = pills.find((p) => p.text() === 'ООО А');
    await orgPill.trigger('click');
    await clickBuild(wrapper);

    const req = lastRun(wrapper);
    expect(req.mode).toBe('list');
    expect(req.entity).toBe('work_applications');
    expect(req.filters).toContainEqual({ key: 'organization', values: ['ООО А'] });
    expect(req.filters).toContainEqual({ key: 'date_range', from: '2026-06-01', to: '2026-06-07' });
  });

  it('сбрасывает выбранные фильтры при смене выгружаемой сущности', async () => {
    const wrapper = mountBuilder();
    wrapper.findComponent(FilterTabs).vm.$emit('update:modelValue', 'list');
    await nextTick();

    // Выбрали организацию у work_applications
    const orgPill = wrapper.findAll('.rb__pill').find((p) => p.text() === 'ООО А');
    await orgPill.trigger('click');

    // Переключились на машины (у них набор фильтров другой) и обратно
    const entityDropdown = wrapper.findAllComponents(BaseDropdown)[0];
    entityDropdown.vm.$emit('update:modelValue', 'cars');
    await nextTick();
    entityDropdown.vm.$emit('update:modelValue', 'work_applications');
    await nextTick();
    await clickBuild(wrapper);

    // Фильтр организации не должен переехать через смену сущности
    expect(lastRun(wrapper).filters.find((f) => f.key === 'organization')).toBeUndefined();
  });

  it('сбрасывает разрез на валидный при смене метрики', async () => {
    const wrapper = mountBuilder();
    await nextTick();
    // applications_count не поддерживает unload_place -> разрез должен переключиться на status.
    // Меняем метрику через emit первого BaseDropdown (script setup не экспонирует form).
    const metricDropdown = wrapper.findAllComponents(BaseDropdown)[0];
    metricDropdown.vm.$emit('update:modelValue', 'applications_count');
    await nextTick();
    await clickBuild(wrapper);

    expect(lastRun(wrapper).dimension).toBe('status');
  });

  it('применяет aggregate-пресет и сразу строит отчёт', async () => {
    const wrapper = mount(ReportBuilder, { props: { catalog: CATALOG, period: PERIOD, preset: null } });
    await nextTick();

    await wrapper.setProps({
      preset: { mode: 'aggregate', metric: 'applications_count', dimension: 'status' },
    });
    await flushPromises();

    const req = lastRun(wrapper);
    expect(req.mode).toBe('aggregate');
    expect(req.metric).toBe('applications_count');
    expect(req.dimension).toBe('status');
    expect(req.filters).toContainEqual({ key: 'date_range', from: '2026-06-01', to: '2026-06-07' });
  });

  it('применяет list-пресет и сразу строит отчёт', async () => {
    const wrapper = mount(ReportBuilder, { props: { catalog: CATALOG, period: PERIOD, preset: null } });
    await nextTick();

    await wrapper.setProps({ preset: { mode: 'list', entity: 'cars' } });
    await flushPromises();

    const req = lastRun(wrapper);
    expect(req.mode).toBe('list');
    expect(req.entity).toBe('cars');
    // У машин нет date_range -> период не попадает в запрос.
    expect(req.filters.find((f) => f.key === 'date_range')).toBeUndefined();
  });
});
