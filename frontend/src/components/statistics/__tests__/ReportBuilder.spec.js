import { describe, it, expect } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { nextTick } from 'vue';
import ReportBuilder from '../ReportBuilder.vue';
import FilterTabs from '@/components/ui/FilterTabs.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';

const CATALOG = {
  metrics: [
    { key: 'car_entries_count', label: 'Въезды машин', unit: 'шт', group: 'Машины', dimensions: ['unload_place', 'period'] },
    { key: 'applications_count', label: 'Количество заявок', unit: 'шт', group: 'Заявки', dimensions: ['status', 'period'] },
  ],
  dimensions: [
    { key: 'none', label: 'Без разреза' },
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

/** Чекбоксы метрик в порядке каталога (по группам — порядок появления групп). */
function metricInputs(wrapper) {
  return wrapper.findAll('.rb__metric-input');
}

/** Радио-разрез по его подписи. */
function dimRadioByLabel(wrapper, label) {
  return wrapper.findAll('.rb__dim').find((d) => d.text().includes(label)).find('.rb__dim-input');
}

describe('ReportBuilder', () => {
  it('по умолчанию выбрана первая метрика, разрез — её первый реальный, строит metrics[]', async () => {
    const wrapper = mountBuilder();
    await nextTick();
    await clickBuild(wrapper);

    const req = lastRun(wrapper);
    expect(req.mode).toBe('aggregate');
    expect(req.metrics).toEqual(['car_entries_count']);
    expect(req.metric).toBeUndefined();
    expect(req.dimension).toBe('unload_place'); // первый реальный разрез, не «без разреза»
    expect(req.filters).toContainEqual({ key: 'date_range', from: '2026-06-01', to: '2026-06-07' });
  });

  it('мультивыбор метрик сужает разрезы до общих и шлёт обе метрики', async () => {
    const wrapper = mountBuilder();
    await nextTick();
    // Добавляем вторую метрику. Общий разрез car∩apps = только period (+ «без разреза»).
    await metricInputs(wrapper)[1].trigger('change');
    await nextTick();

    // unload_place больше не общий -> разрез автоматически переехал на period.
    const dims = wrapper.findAll('.rb__dim').map((d) => d.text());
    expect(dims).toContain('Без разреза');
    expect(dims).toContain('Период (дата)');
    expect(dims).not.toContain('Место разгрузки');

    await clickBuild(wrapper);
    const req = lastRun(wrapper);
    expect(req.metrics).toEqual(['car_entries_count', 'applications_count']);
    expect(req.dimension).toBe('period');
    expect(req.granularity).toBe('day');
  });

  it('«без разреза» выбирается кликом и уходит в запрос как none', async () => {
    const wrapper = mountBuilder();
    await nextTick();
    await dimRadioByLabel(wrapper, 'Без разреза').setValue();
    await clickBuild(wrapper);

    const req = lastRun(wrapper);
    expect(req.metrics).toEqual(['car_entries_count']);
    expect(req.dimension).toBe('none');
  });

  it('снятие добавленной метрики возвращает её собственные разрезы', async () => {
    const wrapper = mountBuilder();
    await nextTick();
    // Добавили applications_count -> общий разрез только period, unload_place пропал.
    await metricInputs(wrapper)[1].trigger('change');
    await nextTick();
    expect(wrapper.findAll('.rb__dim').map((d) => d.text())).not.toContain('Место разгрузки');

    // Сняли её обратно -> разрезы car_entries_count снова доступны.
    await metricInputs(wrapper)[1].trigger('change');
    await nextTick();
    expect(wrapper.findAll('.rb__dim').map((d) => d.text())).toContain('Место разгрузки');

    await clickBuild(wrapper);
    expect(lastRun(wrapper).metrics).toEqual(['car_entries_count']);
  });

  it('в режиме list строит запрос сущности с выбранным фильтром', async () => {
    const wrapper = mountBuilder();
    wrapper.findComponent(FilterTabs).vm.$emit('update:modelValue', 'list');
    await nextTick();

    const pills = wrapper.findAll('.rb__pill');
    expect(pills.length).toBe(2); // организация + статус, date_range = период из шапки

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

    const orgPill = wrapper.findAll('.rb__pill').find((p) => p.text() === 'ООО А');
    await orgPill.trigger('click');

    const entityDropdown = wrapper.findAllComponents(BaseDropdown)[0];
    entityDropdown.vm.$emit('update:modelValue', 'cars');
    await nextTick();
    entityDropdown.vm.$emit('update:modelValue', 'work_applications');
    await nextTick();
    await clickBuild(wrapper);

    expect(lastRun(wrapper).filters.find((f) => f.key === 'organization')).toBeUndefined();
  });

  it('применяет aggregate-пресет (одиночная метрика) и сразу строит metrics[]', async () => {
    const wrapper = mount(ReportBuilder, { props: { catalog: CATALOG, period: PERIOD, preset: null } });
    await nextTick();

    await wrapper.setProps({
      preset: { mode: 'aggregate', metric: 'applications_count', dimension: 'status' },
    });
    await flushPromises();

    const req = lastRun(wrapper);
    expect(req.mode).toBe('aggregate');
    expect(req.metrics).toEqual(['applications_count']);
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
    expect(req.filters.find((f) => f.key === 'date_range')).toBeUndefined();
  });

  it('повторное применение пресета новым объектом (тот же контент) строит заново', async () => {
    const wrapper = mount(ReportBuilder, { props: { catalog: CATALOG, period: PERIOD, preset: null } });
    await nextTick();

    await wrapper.setProps({ preset: { mode: 'aggregate', metric: 'applications_count', dimension: 'status' } });
    await flushPromises();
    const firstCount = wrapper.emitted('run').length;

    await wrapper.setProps({ preset: { mode: 'aggregate', metric: 'applications_count', dimension: 'status' } });
    await flushPromises();
    expect(wrapper.emitted('run').length).toBe(firstCount + 1);
  });
});
