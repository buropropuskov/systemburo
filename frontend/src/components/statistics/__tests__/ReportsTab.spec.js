import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { nextTick } from 'vue';

// Каталог и управляемые промисы runReport вынесены в hoisted — vi.mock поднимается
// над импортами, его фабрика не видит обычные переменные модуля.
const { state, CATALOG } = vi.hoisted(() => ({
  state: { deferred: [], templates: [], saved: [], deleted: [] },
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
  getReportTemplates: () => Promise.resolve(state.templates),
  saveReportTemplate: (payload) => { state.saved.push(payload); return Promise.resolve({ id: 99, ...payload, is_system: false }); },
  deleteReportTemplate: (id) => { state.deleted.push(id); return Promise.resolve(); },
}));

// Стор уведомлений мокаем: ошибку экспорта показывают тостом, а не подменой :error.
const { notifySpy, confirmSpy } = vi.hoisted(() => ({ notifySpy: vi.fn(), confirmSpy: vi.fn(() => Promise.resolve(true)) }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify: notifySpy }) }));
vi.mock('@/stores/ui', () => ({ useUiStore: () => ({ confirm: confirmSpy }) }));

import ReportsTab from '../ReportsTab.vue';
import ReportBuilder from '../ReportBuilder.vue';
import ReportResult from '../ReportResult.vue';

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

  it('ошибку экспорта показывает тостом и не подменяет результат отчёта', async () => {
    notifySpy.mockClear();
    state.deferred.length = 0;
    const wrapper = mount(ReportsTab, { props: { from: '2026-06-01', to: '2026-06-07' } });
    await flushPromises();

    // Построили отчёт.
    wrapper.findComponent(ReportBuilder).vm.$emit('run', { mode: 'list', entity: 'cars' });
    await nextTick();
    state.deferred[0]({ mode: 'list', columns: [], rows: [{}], total: 1 });
    await flushPromises();

    // Экспорт упал.
    wrapper.findComponent(ReportResult).vm.$emit('export-error', 'диск переполнен');
    await nextTick();

    expect(notifySpy).toHaveBeenCalledTimes(1);
    expect(notifySpy.mock.calls[0][0]).toMatchObject({ type: 'error' });
    // Результат на месте, :error не выставлен.
    expect(wrapper.findComponent(ReportResult).props('result')).not.toBeNull();
    expect(wrapper.findComponent(ReportResult).props('error')).toBe('');
  });

  it('грузит личные шаблоны, системные пресеты в «Мои шаблоны» не попадают', async () => {
    state.templates = [
      { id: 1, name: 'Личный набор', config: { mode: 'list', entity: 'cars' }, is_system: false },
      { id: 2, name: 'Системный', config: {}, is_system: true },
    ];
    const wrapper = mount(ReportsTab, { props: { from: '', to: '' } });
    await flushPromises();

    const items = wrapper.findAll('.tpl-item');
    expect(items).toHaveLength(1);
    expect(items[0].text()).toContain('Личный набор');
  });

  it('применение шаблона прокидывает его config в ReportBuilder', async () => {
    state.templates = [{ id: 5, name: 'Мой', config: { mode: 'list', entity: 'cars' }, is_system: false }];
    const wrapper = mount(ReportsTab, { props: { from: '', to: '' } });
    await flushPromises();

    await wrapper.find('.tpl-apply').trigger('click');
    await nextTick();
    expect(wrapper.findComponent(ReportBuilder).props('preset')).toMatchObject({ mode: 'list', entity: 'cars' });
  });

  it('сохранение шаблона шлёт имя и текущий config мастера', async () => {
    state.templates = [];
    state.saved.length = 0;
    const wrapper = mount(ReportsTab, { props: { from: '', to: '' } });
    await flushPromises();

    // Снимок мастера с полным config.
    wrapper.findComponent(ReportBuilder).vm.$emit('change', {
      mode: 'aggregate', metric: 'applications_count', dimension: 'status', entity: '',
      filterCount: 0, periodApplicable: true, periodFilled: false,
      config: { mode: 'aggregate', metrics: ['applications_count'], dimension: 'status', granularity: 'day', entity: '', filters: {}, period: { from: '', to: '' } },
    });
    await nextTick();

    await wrapper.find('.tpl-save-btn').trigger('click');
    await nextTick();
    await wrapper.find('.tpl-save-form input').setValue('Новый набор');
    await wrapper.find('.tpl-save-actions .lk-button--primary').trigger('click');
    await flushPromises();

    expect(state.saved).toHaveLength(1);
    expect(state.saved[0]).toMatchObject({
      name: 'Новый набор',
      config: { mode: 'aggregate', metrics: ['applications_count'], dimension: 'status' },
    });
  });

  it('удаление шаблона требует подтверждения и дёргает API по id', async () => {
    state.templates = [{ id: 7, name: 'Удалить меня', config: { mode: 'list', entity: 'cars' }, is_system: false }];
    state.deleted.length = 0;
    confirmSpy.mockClear();
    confirmSpy.mockResolvedValueOnce(true);
    const wrapper = mount(ReportsTab, { props: { from: '', to: '' } });
    await flushPromises();

    await wrapper.find('.tpl-del').trigger('click');
    await flushPromises();
    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(state.deleted).toContain(7);
  });

  it('отмена подтверждения не удаляет шаблон', async () => {
    state.templates = [{ id: 8, name: 'Оставить', config: { mode: 'list', entity: 'cars' }, is_system: false }];
    state.deleted.length = 0;
    confirmSpy.mockClear();
    confirmSpy.mockResolvedValueOnce(false);
    const wrapper = mount(ReportsTab, { props: { from: '', to: '' } });
    await flushPromises();

    await wrapper.find('.tpl-del').trigger('click');
    await flushPromises();
    expect(state.deleted).toHaveLength(0);
  });
  // Лимит запроса нужен результату, чтобы отличить «данных ровно столько» от
  // «упёрлись в лимит»: движок признака обрезки не отдаёт.
  it('лимит построенного запроса доходит до результата, ошибка его сбрасывает', async () => {
    state.deferred.length = 0;
    const wrapper = mount(ReportsTab, { props: { from: '2026-06-01', to: '2026-06-07' } });
    await flushPromises();

    const builder = wrapper.findComponent(ReportBuilder);
    builder.vm.$emit('run', { mode: 'aggregate', metric: 'applications_count', dimension: 'period', limit: 1000 });
    await nextTick();
    state.deferred[0]({ mode: 'aggregate', dimension: 'period', rows: [], total: 0, unit: 'шт' });
    await flushPromises();
    expect(wrapper.findComponent(ReportResult).props('limit')).toBe(1000);

    builder.vm.$emit('run', { mode: 'aggregate', metric: 'applications_count', dimension: 'period', limit: 1000 });
    await nextTick();
    state.deferred[1](Promise.reject(new Error('бэк упал')));
    await flushPromises();
    expect(wrapper.findComponent(ReportResult).props('limit')).toBe(0);
  });
});

/*
 * jsdom не считает медиа-запросы, поэтому мобильный контракт вкладки (#1097 r3d)
 * сверяем по SFC: на телефоне у карточки-конструктора padding 20px с обеих сторон
 * съедал ширину полей мастера.
 */
describe('ReportsTab — мобильная адаптивность (#1097 r3d)', () => {
  const src = readFileSync(resolve(__dirname, '../ReportsTab.vue'), 'utf8');
  const mobile = src.slice(src.indexOf('@media (max-width: 768px)'));

  it('на мобилке карточка-конструктор получает узкий padding', () => {
    expect(src).toContain('@media (max-width: 768px)');
    expect(mobile).toContain('.wizard');
    expect(mobile).toMatch(/padding:\s*16px 14px/);
  });
});
