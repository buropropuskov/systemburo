import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { nextTick } from 'vue';
import ReportBuilder from '../ReportBuilder.vue';
import FilterTabs from '@/components/ui/FilterTabs.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import DateFilter from '@/components/DateFilter.vue';
import HintTooltip from '@/components/ui/HintTooltip.vue';

const CATALOG = {
  metrics: [
    { key: 'car_entries_count', label: 'Въезды машин', unit: 'шт', group: 'Машины', dimensions: ['unload_place', 'period'], filters: ['date_range', 'organization'] },
    { key: 'applications_count', label: 'Количество заявок', unit: 'шт', group: 'Заявки', dimensions: ['status', 'period'], filters: ['date_range', 'status', 'organization'] },
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
  pivots: [
    { key: 'attachment_type', label: 'Тип вложения', metrics: ['applications_count'] },
  ],
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

/**
 * Чекбокс метрики по её подписи. Секции шага «Что считаем» («Основное» + группы
 * каталога) переставляют карточки, поэтому обращение по индексу ломается при
 * первой же перегруппировке.
 */
function metricByLabel(wrapper, label) {
  const card = wrapper.findAll('.rb__metric').find((l) => l.text().includes(label));
  if (!card) throw new Error(`карточка метрики «${label}» не найдена`);
  return card.find('.rb__metric-input');
}

/**
 * Выбрать значение справочного фильтра (организация, компания). После #2308 они
 * живут в BaseDropdown с поиском, а не чипами: справочники растут вместе с базой.
 */
async function pickDictFilter(wrapper, label, values) {
  const field = wrapper.findAll('.rb__filter').find((f) => f.text().includes(label));
  if (!field) throw new Error(`фильтр «${label}» не найден`);
  field.findComponent(BaseDropdown).vm.$emit('update:modelValue', values);
  await nextTick();
}

/** Радио-разрез по его подписи. */
function dimRadioByLabel(wrapper, label) {
  return wrapper.findAll('.rb__dim').find((d) => d.text().includes(label)).find('.rb__dim-input');
}

/** Кнопки оси разворота (cross-tab) внутри блока периода. [] если блока нет. */
function pivotPills(wrapper) {
  const gran = wrapper.find('.rb__gran');
  return gran.exists() ? gran.findAll('.rb__pill') : [];
}

/** Выбрать только метрику «Количество заявок» и разрез «Период (дата)». */
async function setupAppsPeriod(wrapper) {
  await metricByLabel(wrapper, 'Количество заявок').trigger('change'); // + applications_count
  await metricByLabel(wrapper, 'Въезды машин').trigger('change'); // - car_entries_count
  await nextTick();
  await dimRadioByLabel(wrapper, 'Период (дата)').setValue();
  await nextTick();
}

// Каталог с показателями обработки: в основном CATALOG обе метрики ходовые, и
// раскрывашка шага «Что считаем» там не появляется (#2296).
const CATALOG_WITH_PROCESSING = {
  ...CATALOG,
  metrics: [
    ...CATALOG.metrics,
    { key: 'p90_approval_time', label: '90-й перцентиль времени согласования', group: 'Обработка заявок', dimensions: ['period'], filters: ['date_range'] },
    { key: 'refusal_rate', label: 'Доля отказов и несогласований', unit: '%', group: 'Обработка заявок', dimensions: ['period'], filters: ['date_range'] },
  ],
};

function mountWide() {
  return mount(ReportBuilder, { props: { catalog: CATALOG_WITH_PROCESSING, period: PERIOD } });
}

describe('ReportBuilder — разгрузка конструктора (#2296)', () => {
  it('показывает только ходовые показатели, остальные прячет под раскрывашку', async () => {
    const wrapper = mountWide();
    await nextTick();

    const shown = wrapper.findAll('.rb__metric').map((m) => m.text());
    expect(shown.join(' ')).toContain('Количество заявок');
    expect(shown.join(' ')).not.toContain('перцентиль');

    const more = wrapper.find('.rb__more');
    expect(more.text()).toContain('Показатели обработки');
    expect(more.text()).toContain('2');

    await more.trigger('click');
    expect(wrapper.findAll('.rb__metric').map((m) => m.text()).join(' ')).toContain('перцентиль');
  });

  it('раскрывает блок сам, когда пресет включил показатель из скрытой части', async () => {
    const wrapper = mountWide();
    await nextTick();
    expect(wrapper.findAll('.rb__metric').map((m) => m.text()).join(' ')).not.toContain('Доля отказов');

    await wrapper.setProps({ preset: { mode: 'aggregate', metrics: ['refusal_rate'], dimension: 'period' } });
    await nextTick();

    expect(wrapper.findAll('.rb__metric').map((m) => m.text()).join(' ')).toContain('Доля отказов');
  });

  it('к перцентилям и долям даёт подсказку', async () => {
    const wrapper = mountWide();
    await nextTick();
    await wrapper.find('.rb__more').trigger('click');

    const p90 = wrapper.findAll('.rb__metric').find((m) => m.text().includes('перцентиль'));
    expect(p90.findComponent(HintTooltip).props('text')).toContain('Девять заявок из десяти');

    const rate = wrapper.findAll('.rb__metric').find((m) => m.text().includes('Доля отказов'));
    expect(rate.findComponent(HintTooltip).props('text')).toContain('отказал принимающий');
  });

  it('фильтры и период свёрнуты, а их состояние видно в заголовке', async () => {
    const wrapper = mountBuilder();
    await nextTick();

    expect(wrapper.find('.rb__filters').isVisible()).toBe(false);
    expect(wrapper.find('.rb__period').isVisible()).toBe(false);

    const heads = wrapper.findAll('.rb__step-head--toggle').map((h) => h.text());
    expect(heads.join(' ')).toContain('не заданы');
    expect(heads.join(' ')).toContain('01.06.2026 - 07.06.2026');
  });

  it('заголовок фильтров считает выбранные значения, а раскрытие показывает сам блок', async () => {
    const wrapper = mountBuilder();
    await nextTick();

    const filtersHead = wrapper.findAll('.rb__step-head--toggle').find((h) => h.text().includes('Фильтры'));
    await filtersHead.trigger('click');
    expect(wrapper.find('.rb__filters').isVisible()).toBe(true);

    await pickDictFilter(wrapper, 'Организация', ['ООО А']);
    expect(filtersHead.text()).toContain('Организация: 1');
  });
});

describe('ReportBuilder', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

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
    await metricByLabel(wrapper, 'Количество заявок').trigger('change');
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
    await metricByLabel(wrapper, 'Количество заявок').trigger('change');
    await nextTick();
    expect(wrapper.findAll('.rb__dim').map((d) => d.text())).not.toContain('Место разгрузки');

    // Сняли её обратно -> разрезы car_entries_count снова доступны.
    await metricByLabel(wrapper, 'Количество заявок').trigger('change');
    await nextTick();
    expect(wrapper.findAll('.rb__dim').map((d) => d.text())).toContain('Место разгрузки');

    await clickBuild(wrapper);
    expect(lastRun(wrapper).metrics).toEqual(['car_entries_count']);
  });

  it('в режиме list строит запрос сущности с выбранным фильтром', async () => {
    const wrapper = mountBuilder();
    wrapper.findComponent(FilterTabs).vm.$emit('update:modelValue', 'list');
    await nextTick();

    // организация ушла в дропдаун, статус остался чипом; date_range = период (шаг 4)
    expect(wrapper.findAll('.rb__filter')).toHaveLength(2);
    expect(wrapper.find('.rb__filters').findAll('.rb__pill')).toHaveLength(1);

    await pickDictFilter(wrapper, 'Организация', ['ООО А']);
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

    await pickDictFilter(wrapper, 'Организация', ['ООО А']);

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

  it('применяет шаблон с фильтрами и собственным периодом', async () => {
    const wrapper = mount(ReportBuilder, { props: { catalog: CATALOG, period: PERIOD, preset: null } });
    await nextTick();

    await wrapper.setProps({
      preset: {
        mode: 'aggregate', metrics: ['applications_count'], dimension: 'status',
        filters: { organization: ['ООО А'] }, period: { from: '2026-01-01', to: '2026-03-31' },
      },
    });
    await flushPromises();

    const req = lastRun(wrapper);
    expect(req.metrics).toEqual(['applications_count']);
    expect(req.dimension).toBe('status');
    // Фильтры и период из шаблона, а не из шапки.
    expect(req.filters).toContainEqual({ key: 'organization', values: ['ООО А'] });
    expect(req.filters).toContainEqual({ key: 'date_range', from: '2026-01-01', to: '2026-03-31' });
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

  it('шаг «Фильтры» в aggregate показывает применимые к метрике чипсы и шлёт выбранный', async () => {
    const wrapper = mountBuilder();
    await nextTick();
    // car_entries_count -> применимый фильтр organization (date_range вынесен в период).
    expect(wrapper.findAll('.rb__filter').some((f) => f.text().includes('Организация'))).toBe(true);
    await pickDictFilter(wrapper, 'Организация', ['ООО А']);
    await clickBuild(wrapper);

    expect(lastRun(wrapper).filters).toContainEqual({ key: 'organization', values: ['ООО А'] });
  });

  it('снятие метрики чистит фильтр, который поддерживала только она', async () => {
    const wrapper = mountBuilder();
    await nextTick();
    // Добавили applications_count — единственную метрику, поддерживающую status.
    await metricByLabel(wrapper, 'Количество заявок').trigger('change'); // metrics = [car, apps]
    await nextTick();
    const statusPill = wrapper.find('.rb__filters').findAll('.rb__pill').find((p) => p.text() === 'Завершено');
    await statusPill.trigger('click');
    // Сняли applications_count -> status больше не применим -> значение чистится.
    await metricByLabel(wrapper, 'Количество заявок').trigger('change'); // metrics = [car]
    await nextTick();
    await clickBuild(wrapper);

    expect(lastRun(wrapper).filters.find((f) => f.key === 'status')).toBeUndefined();
  });

  it('фильтр, применимый ко всему набору метрик, переживает смену метрик', async () => {
    const wrapper = mountBuilder();
    await nextTick();
    // organization поддерживают обе метрики.
    await pickDictFilter(wrapper, 'Организация', ['ООО А']);
    await metricByLabel(wrapper, 'Количество заявок').trigger('change'); // добавили applications_count
    await nextTick();
    await clickBuild(wrapper);

    expect(lastRun(wrapper).filters).toContainEqual({ key: 'organization', values: ['ООО А'] });
  });

  it('пресет периода «Этот год» задаёт диапазон с начала года', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 5, 18)); // 18 июня 2026, локальное время
    const wrapper = mountBuilder();
    await nextTick();

    const yearBtn = wrapper.find('.rb__period-presets').findAll('.rb__pill').find((b) => b.text() === 'Этот год');
    await yearBtn.trigger('click');
    await clickBuild(wrapper);

    const dr = lastRun(wrapper).filters.find((f) => f.key === 'date_range');
    expect(dr.from).toBe('2026-01-01');
    expect(dr.to).toBe('2026-06-18');
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

  it('ось разворота доступна при период+метрике из каталога и уходит в запрос как pivot', async () => {
    const wrapper = mountBuilder();
    await nextTick();
    await setupAppsPeriod(wrapper);

    const pills = pivotPills(wrapper);
    expect(pills.map((p) => p.text())).toEqual(['Без разворота', 'Тип вложения']);

    await pills.find((p) => p.text() === 'Тип вложения').trigger('click');
    await clickBuild(wrapper);

    const req = lastRun(wrapper);
    expect(req.metrics).toEqual(['applications_count']);
    expect(req.dimension).toBe('period');
    expect(req.pivot).toBe('attachment_type');
  });

  it('смена разреза с period сбрасывает ось разворота (не уходит в запрос)', async () => {
    const wrapper = mountBuilder();
    await nextTick();
    await setupAppsPeriod(wrapper);
    await pivotPills(wrapper).find((p) => p.text() === 'Тип вложения').trigger('click');

    // Уходим с period -> ось неприменима, блок исчезает, pivot сбрасывается.
    await dimRadioByLabel(wrapper, 'Статус заявки').setValue();
    await nextTick();
    expect(pivotPills(wrapper)).toEqual([]);

    await clickBuild(wrapper);
    expect(lastRun(wrapper).pivot).toBeUndefined();
  });

  it('добавление метрики, не поддерживающей ось, сбрасывает разворот', async () => {
    const wrapper = mountBuilder();
    await nextTick();
    await setupAppsPeriod(wrapper);
    await pivotPills(wrapper).find((p) => p.text() === 'Тип вложения').trigger('click');

    // Возвращаем car_entries_count: ось attachment_type его не поддерживает.
    await metricByLabel(wrapper, 'Въезды машин').trigger('change');
    await nextTick();
    expect(pivotPills(wrapper)).toEqual([]);

    await clickBuild(wrapper);
    const req = lastRun(wrapper);
    expect(req.metrics).toEqual(['applications_count', 'car_entries_count']);
    expect(req.pivot).toBeUndefined();
  });

  it('скелет «Что построим» отражает разрез, метрику и колонку разворота', async () => {
    const wrapper = mountBuilder();
    await nextTick();
    await setupAppsPeriod(wrapper);
    await pivotPills(wrapper).find((p) => p.text() === 'Тип вложения').trigger('click');
    await nextTick();

    const cols = wrapper.findAll('.rb__skel-col');
    expect(cols.length).toBe(3); // разрез + метрика + ось разворота
    expect(wrapper.find('.rb__skel-col--metric').text()).toContain('Количество заявок');
    const pivotCol = wrapper.find('.rb__skel-col--pivot');
    expect(pivotCol.exists()).toBe(true);
    expect(pivotCol.text()).toContain('Тип вложения');
  });

  it('применяет шаблон с осью разворота и шлёт pivot', async () => {
    const wrapper = mount(ReportBuilder, { props: { catalog: CATALOG, period: PERIOD, preset: null } });
    await nextTick();

    await wrapper.setProps({
      preset: {
        mode: 'aggregate', metrics: ['applications_count'], dimension: 'period',
        granularity: 'week', pivot: 'attachment_type',
      },
    });
    await flushPromises();

    const req = lastRun(wrapper);
    expect(req.metrics).toEqual(['applications_count']);
    expect(req.dimension).toBe('period');
    expect(req.pivot).toBe('attachment_type');
  });

  it('период в описании показан как дд.мм.гггг', async () => {
    const wrapper = mountBuilder();
    await nextTick();
    const summary = wrapper.find('.rb__summary').text();
    expect(summary).toContain('01.06.2026 — 07.06.2026');
    expect(summary).not.toContain('2026-06-01');
  });

  it('начальный период из шапки прокинут в календарь DateFilter как Date-диапазон', () => {
    const wrapper = mountBuilder();
    const cal = wrapper.findComponent(DateFilter);
    expect(cal.exists()).toBe(true);
    expect(cal.props('mode')).toBe('range');

    const start = cal.props('dateRangeStart');
    const end = cal.props('dateRangeEnd');
    expect(start.getFullYear()).toBe(2026);
    expect(start.getMonth()).toBe(5); // июнь (0-based)
    expect(start.getDate()).toBe(1);
    expect(end.getMonth()).toBe(5);
    expect(end.getDate()).toBe(7);
  });

  it('выбор диапазона в DateFilter обновляет период запроса (ISO) и снимает пресет', async () => {
    const wrapper = mountBuilder();
    await nextTick();

    const cal = wrapper.findComponent(DateFilter);
    cal.vm.$emit('update:dateRangeStart', new Date(2026, 2, 5)); // 5 марта 2026
    cal.vm.$emit('update:dateRangeEnd', new Date(2026, 2, 20)); // 20 марта 2026
    cal.vm.$emit('apply');
    await nextTick();

    await clickBuild(wrapper);
    const dr = lastRun(wrapper).filters.find((f) => f.key === 'date_range');
    expect(dr).toEqual({ key: 'date_range', from: '2026-03-05', to: '2026-03-20' });

    // Ручной выбор уводит период из-под пресета — ни одна кнопка не подсвечена.
    expect(wrapper.find('.rb__period-presets').findAll('.rb__pill--on').length).toBe(0);
  });

  it('«Применить» в календаре без смены дат не сбивает активный пресет', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 5, 18)); // 18 июня 2026
    const wrapper = mountBuilder();
    await nextTick();

    const yearBtn = wrapper.find('.rb__period-presets').findAll('.rb__pill').find((b) => b.text() === 'Этот год');
    await yearBtn.trigger('click');
    await nextTick();
    expect(yearBtn.classes()).toContain('rb__pill--on');

    // DateFilter всегда эмитит update:* + apply, даже когда даты пресета не менялись.
    const cal = wrapper.findComponent(DateFilter);
    cal.vm.$emit('update:dateRangeStart', new Date(2026, 0, 1));
    cal.vm.$emit('update:dateRangeEnd', new Date(2026, 5, 18));
    cal.vm.$emit('apply');
    await nextTick();

    // Пресет «Этот год» остаётся активным — границы те же, в custom не ушло.
    expect(yearBtn.classes()).toContain('rb__pill--on');
  });
});

/*
 * jsdom не считает медиа-запросы и layout, поэтому контракт мобильной раскладки
 * (#1097 r3d) сверяем по объявлениям в SFC. Замок против «уборки» медиа-блоков,
 * которая тихо вернула бы отступы под номер шага и зажатую кнопку на телефоне.
 */
describe('ReportBuilder — справочники в дропдауне (#2308)', () => {
  it('организацию отдаёт дропдауну с поиском, короткий перечень статусов оставляет чипами', async () => {
    const wrapper = mountBuilder();
    wrapper.findComponent(FilterTabs).vm.$emit('update:modelValue', 'list');
    await nextTick();

    const org = wrapper.findAll('.rb__filter').find((f) => f.text().includes('Организация'));
    const dropdown = org.findComponent(BaseDropdown);
    expect(dropdown.exists()).toBe(true);
    expect(dropdown.props('searchable')).toBe(true);
    expect(dropdown.props('multiple')).toBe(true);
    expect(org.findAll('.rb__pill')).toHaveLength(0);

    const status = wrapper.findAll('.rb__filter').find((f) => f.text().includes('Статус'));
    expect(status.findComponent(BaseDropdown).exists()).toBe(false);
    expect(status.findAll('.rb__pill').length).toBeGreaterThan(0);
  });

  it('выбранное показывает чипами под полем, и клик по чипу снимает значение', async () => {
    const wrapper = mountBuilder();
    await nextTick();
    await pickDictFilter(wrapper, 'Организация', ['ООО А']);

    const chips = wrapper.findAll('.rb__chosen-chip');
    expect(chips).toHaveLength(1);
    expect(chips[0].text()).toContain('ООО А');

    await chips[0].trigger('click');
    await nextTick();
    expect(wrapper.findAll('.rb__chosen-chip')).toHaveLength(0);

    await clickBuild(wrapper);
    expect(lastRun(wrapper).filters.find((f) => f.key === 'organization')).toBeUndefined();
  });
});

describe('ReportBuilder — мобильная адаптивность (#1097 r3d)', () => {
  const src = readFileSync(resolve(__dirname, '../ReportBuilder.vue'), 'utf8');
  const mobile = src.slice(src.indexOf('@media (max-width: 768px)'));
  const marginReset = mobile;
  // Шаг «Что считаем» уехал в свой компонент (#2296), мобильные правила метрик - вместе с ним.
  const pickerSrc = readFileSync(resolve(__dirname, '../ReportMetricPicker.vue'), 'utf8');
  const pickerMobile = pickerSrc.slice(pickerSrc.indexOf('@media (max-width: 768px)'));

  it('канонический брейкпоинт мобилки 768 (эталон #1097), прежний 620 убран', () => {
    expect(src).toContain('@media (max-width: 768px)');
    expect(src).not.toContain('max-width: 620px');
  });

  it('на мобилке снят левый отступ под номер шага у всех сеток и блоков', () => {
    for (const sel of ['.rb__dims', '.rb__gran', '.rb__filters', '.rb__period']) {
      expect(marginReset).toContain(sel);
    }
    for (const sel of ['.rb__metrics', '.rb__group-title']) {
      expect(pickerMobile).toContain(sel);
    }
    expect(marginReset).toMatch(/margin-left:\s*0/);
    expect(pickerMobile).toMatch(/margin-left:\s*0/);
  });

  it('кнопка построения тянется на всю ширину под полем «Строк»', () => {
    expect(mobile).toContain('.rb__footer .lk-button--primary');
    expect(mobile).toMatch(/flex:\s*1 1 100%/);
  });

  it('на очень узких экранах (<=480) метрики в один столбец', () => {
    const narrow = pickerSrc.slice(pickerSrc.indexOf('@media (max-width: 480px)'));
    expect(narrow).toContain('.rb__metrics');
    expect(narrow).toMatch(/grid-template-columns:\s*1fr/);
  });
});
