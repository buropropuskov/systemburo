import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// FE #980 polish-r2: вкладка "Версии" переделана из master-detail в dropdown выбора
// версии + таблицу снимка на всю ширину (preview-режим CarsTable/PeopleTable).
// Проверяем: опции дропдауна, автовыбор первой версии, что таблице передаются
// нормализованные строки и колонки снимка, пустые состояния, гонку детали (#632),
// подгрузку версий и действия (снимок/экспорт/чистка).

vi.mock('@/api/system-tables', () => ({
  listTableSnapshots: vi.fn(),
  getTableSnapshot: vi.fn(),
  createTableSnapshot: vi.fn(),
  exportTableSnapshot: vi.fn(),
  cleanupTableSnapshots: vi.fn(),
}));

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}));

const saveBlobAs = vi.fn();
vi.mock('@/api/attachment-templates', () => ({
  saveBlobAs: (...a) => saveBlobAs(...a),
}));

const { permState } = vi.hoisted(() => ({ permState: { can: () => true } }));
vi.mock('@/composables/usePermission', () => ({
  usePermission: () => ({ can: (k) => permState.can(k) }),
}));

const notify = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}));

const { routeState } = vi.hoisted(() => ({
  routeState: { params: { tableName: 'kpp-1' }, query: {} },
}));
vi.mock('vue-router', () => ({
  useRoute: () => ({ params: routeState.params, query: routeState.query }),
}));

import TableVersionsView from '@/views/TableVersionsView.vue';
import {
  listTableSnapshots,
  getTableSnapshot,
  createTableSnapshot,
  exportTableSnapshot,
  cleanupTableSnapshots,
} from '@/api/system-tables';
import { apiRequest } from '@/api/client';

// Стаб реальной таблицы: отражает полученные preview-props в дерево, чтобы можно
// было ассертить, что versions-view нормализовал строки и передал колонки снимка.
const tableStub = (testid) => ({
  props: ['preview', 'previewFields', 'previewItems', 'searchQuery', 'tableId', 'tableName'],
  // Рендерим слот header-actions - в него versions-view кладёт поиск по строкам
  // (теперь в родной шапке таблицы), чтобы .tv-search-input был в дереве.
  template: `<div :data-testid="'${testid}'"
      :data-rows="previewItems ? previewItems.length : 0"
      :data-fields="previewFields ? previewFields.length : 0"
      :data-search="searchQuery || ''">
      <slot name="header-actions" />
      <span v-for="it in (previewItems || [])" :key="it.id" class="preview-cell"
        :data-org="it.organization_name" :data-entry="String(it.entry_checked)"
        :data-exit="String(it.exit_checked)">{{ it.car_number || it.last_name }}</span>
    </div>`,
});

const stubs = {
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
  RefreshButton: { template: '<button class="refresh-stub" @click="$emit(\'refresh\')" />' },
  Badge: { template: '<span class="badge-stub"><slot /></span>' },
  // Дропдаун версий: <select>, чтобы триггерить смену через change.
  BaseDropdown: {
    props: ['modelValue', 'options', 'labelKey', 'valueKey', 'placeholder'],
    emits: ['update:modelValue'],
    template: `<select data-testid="tv-version-select" :value="modelValue"
      @change="$emit('update:modelValue', Number($event.target.value))">
      <option v-for="o in options" :key="o.id" :value="o.id">{{ o.label }}</option>
    </select>`,
  },
  CarsTable: tableStub('tv-cars'),
  PeopleTable: tableStub('tv-people'),
  // Поиск: отражаем ввод как update:modelValue, чтобы проверить проброс в таблицу.
  SearchComponent: {
    props: ['title', 'modelValue'],
    emits: ['update:modelValue'],
    template: `<input class="tv-search-input" :placeholder="title" :value="modelValue"
      @input="$emit('update:modelValue', $event.target.value)" />`,
  },
  // BaseModal рендерит содержимое инлайн (без Teleport, чтобы find видел слоты).
  BaseModal: {
    props: ['show', 'title', 'width', 'closeOnOverlay', 'closable'],
    emits: ['close'],
    template: `<div v-if="show" data-testid="tv-cleanup-modal" :data-title="title">
      <slot />
      <div class="modal-actions"><slot name="actions" /></div>
    </div>`,
  },
  // Сегмент периода: pill-кнопки, клик эмитит выбранный key.
  FilterTabs: {
    props: ['modelValue', 'tabs'],
    emits: ['update:modelValue'],
    template: `<div data-testid="tv-cleanup-period">
      <button v-for="t in tabs" :key="t.key" :data-testid="'tv-period-' + t.key"
        :class="{ active: modelValue === t.key }"
        @click="$emit('update:modelValue', t.key)">{{ t.label }}</button>
    </div>`,
  },
  // Календарь версии: маркер-корень, границы/apply/clear тесты эмитят через vm.$emit
  // (как реальный DateFilter в range-режиме: applySelection -> границы + apply).
  DateFilter: {
    props: ['mode', 'dateRangeStart', 'dateRangeEnd'],
    emits: ['update:dateRangeStart', 'update:dateRangeEnd', 'apply', 'clear'],
    template: '<div data-testid="tv-date-filter"></div>',
  },
};

// Выбор периода в календаре версии: эмитит границы, затем apply (порядок как у
// реального DateFilter). Один день = start и end одного дня, null - сброс.
async function pickVersionRange(wr, start, end = start) {
  const df = wr.findComponent('[data-testid="tv-date-filter"]');
  df.vm.$emit('update:dateRangeStart', start ? dayStart(start) : null);
  df.vm.$emit('update:dateRangeEnd', end ? dayEnd(end) : null);
  df.vm.$emit('apply');
  await flushPromises();
}

// Границы дня, как их отдаёт DateFilter.applySelection.
function dayStart(date) {
  const d = new Date(date);
  d.setHours(0, 0, 0, 0);
  return d;
}

function dayEnd(date) {
  const d = new Date(date);
  d.setHours(23, 59, 59, 999);
  return d;
}

// Выбор одного дня - частый случай, отдельная обёртка для читаемости.
const pickVersionDate = (wr, date) => pickVersionRange(wr, date, date);

function mockTable(over = {}, fields = [{ field_name: 'car_number', is_visible: true }]) {
  apiRequest.mockResolvedValue({
    json: () => Promise.resolve({
      table: { id: 5, table_type: 'cars', display_name: 'КПП-1', ...over },
      fields,
    }),
  });
}

function snapItem(id, over = {}) {
  return {
    id,
    table_id: 5,
    taken_at: '2026-07-01T03:00:00Z',
    reason: 'scheduled',
    counts: { on_territory: 1, exited: 0, not_entered: 0, total: 1 },
    ...over,
  };
}

function carsSnapshot(id, rows, over = {}) {
  return {
    id,
    table_id: 5,
    taken_at: '2026-07-01T03:00:00Z',
    reason: 'scheduled',
    counts: { on_territory: 1, exited: 1, not_entered: 0, total: 2 },
    payload: {
      table_type: 'cars',
      rows,
      fields: [
        { field_name: 'car_number', is_visible: true, display_order: 0 },
        { field_name: 'organization', is_visible: true, display_order: 1 },
      ],
    },
    ...over,
  };
}

function mountView() {
  return mount(TableVersionsView, { global: { stubs } });
}

let wrapper;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  routeState.params = { tableName: 'kpp-1' };
  routeState.query = {};
  permState.can = () => true;
  mockTable();
  getTableSnapshot.mockResolvedValue(carsSnapshot(1, []));
  createTableSnapshot.mockResolvedValue({ id: 99, message: 'ok' });
  exportTableSnapshot.mockResolvedValue({ blob: new Blob(['x']), filename: 'КПП-1.xlsx' });
  cleanupTableSnapshots.mockResolvedValue({ deleted: 0, message: 'ok' });
});

afterEach(() => {
  wrapper?.unmount();
});

describe('TableVersionsView (#980 polish-r2)', () => {
  it('рендерит дропдаун версий и футер "Всего версий: N"', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1), snapItem(2)], total: 2 });
    wrapper = mountView();
    await flushPromises();

    const opts = wrapper.findAll('[data-testid="tv-version-select"] option');
    expect(opts).toHaveLength(2);
    expect(opts[0].text()).toContain('Плановый');
    expect(wrapper.find('[data-testid="tv-footer"]').text()).toContain('Всего версий: 2');
    expect(wrapper.find('[data-testid="tv-empty"]').exists()).toBe(false);
  });

  it('резолвит таблицу с allow_archived=1 - иначе архивная не найдётся', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });
    wrapper = mountView();
    await flushPromises();

    expect(apiRequest).toHaveBeenCalledWith('/system-tables/name/kpp-1?allow_archived=1');
  });

  it('«Назад» ведёт на страницу таблицы, а из админ-конструктора (from=admin) - в конструктор', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });

    wrapper = mountView();
    await flushPromises();
    expect(wrapper.findComponent('[data-testid="tv-back"]').props('to')).toBe('/table/kpp-1');
    wrapper.unmount();

    routeState.query = { from: 'admin' };
    wrapper = mountView();
    await flushPromises();
    // из админ-архива - в конструктор + open=<name>, чтобы он открыл ту же таблицу
    expect(wrapper.findComponent('[data-testid="tv-back"]').props('to')).toEqual({
      path: '/table-constructor',
      query: { open: 'kpp-1' },
    });
  });

  it('автовыбирает первую версию и передаёт нормализованные строки в CarsTable', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(7), snapItem(8)], total: 2 });
    getTableSnapshot.mockResolvedValue(carsSnapshot(7, [
      { id: 1, car_number: 'А123ВС', car_brand: 'BMW', organization: 'ООО Ромашка', entry_date_to: '2026-07-05', territory_status: 1 },
    ]));
    wrapper = mountView();
    await flushPromises();

    expect(getTableSnapshot).toHaveBeenCalledWith(5, 7);
    const cars = wrapper.find('[data-testid="tv-cars"]');
    expect(cars.exists()).toBe(true);
    // Строки нормализованы: organization -> organization_name, статус -> entry_checked.
    expect(cars.attributes('data-rows')).toBe('1');
    const cell = wrapper.find('.preview-cell');
    expect(cell.text()).toBe('А123ВС');
    expect(cell.attributes('data-org')).toBe('ООО Ромашка');
    expect(cell.attributes('data-entry')).toBe('true'); // territory_status=1
    // Колонки берутся из снимка (payload.fields), не хардкод.
    expect(cars.attributes('data-fields')).toBe('2');
  });

  it('счётчики выбранной версии показываются над таблицей', async () => {
    // Счётчики берутся из элемента списка (list-item.counts), не из payload детали.
    listTableSnapshots.mockResolvedValue({
      items: [snapItem(1, { counts: { on_territory: 0, exited: 1, not_entered: 0, total: 1 } })],
      total: 1,
    });
    getTableSnapshot.mockResolvedValue(carsSnapshot(1, [
      { id: 1, car_number: 'Х1', territory_status: 2 },
    ]));
    wrapper = mountView();
    await flushPromises();

    const meta = wrapper.find('[data-testid="tv-meta"]');
    expect(meta.text()).toContain('На территории: 0');
    expect(meta.text()).toContain('Выехал: 1');
    // Строка со статусом 2 (выехал): Въезд НЕ отмечен, Выезд отмечен - как на
    // основной странице, не противоречит счётчику "Выехал".
    const cell = wrapper.find('.preview-cell');
    expect(cell.attributes('data-entry')).toBe('false');
    expect(cell.attributes('data-exit')).toBe('true');
  });

  it('people-снимок рендерит PeopleTable с нормализованными строками', async () => {
    mockTable({ table_type: 'people' }, [{ field_name: 'last_name', is_visible: true }]);
    listTableSnapshots.mockResolvedValue({ items: [snapItem(3, { reason: 'manual', actor_name: 'Иванов И.И.' })], total: 1 });
    getTableSnapshot.mockResolvedValue({
      id: 3,
      taken_at: '2026-07-01T03:00:00Z',
      reason: 'manual',
      counts: { on_territory: 1, exited: 0, not_entered: 0, total: 1 },
      payload: {
        table_type: 'people',
        rows: [
          { id: 1, last_name: 'Петров', first_name: 'Пётр', position: 'Грузчик', organization: 'ООО В', territory_status: 1 },
        ],
        fields: [{ field_name: 'last_name', is_visible: true }],
      },
    });
    wrapper = mountView();
    await flushPromises();

    const people = wrapper.find('[data-testid="tv-people"]');
    expect(people.exists()).toBe(true);
    expect(wrapper.find('[data-testid="tv-cars"]').exists()).toBe(false);
    expect(wrapper.find('.preview-cell').text()).toBe('Петров');
    // Ручной снимок показывает автора и тип.
    expect(wrapper.find('[data-testid="tv-meta"]').text()).toContain('Иванов И.И.');
    expect(wrapper.find('[data-testid="tv-meta"]').text()).toContain('Ручной');
  });

  it('фолбэк колонок на текущие поля таблицы для старого снимка без fields', async () => {
    mockTable({}, [
      { field_name: 'car_number', is_visible: true },
      { field_name: 'organization', is_visible: true },
      { field_name: 'company', is_visible: true },
    ]);
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });
    getTableSnapshot.mockResolvedValue({
      id: 1,
      taken_at: '2026-07-01T03:00:00Z',
      reason: 'scheduled',
      counts: { on_territory: 0, exited: 0, not_entered: 1, total: 1 },
      payload: { table_type: 'cars', rows: [{ id: 1, car_number: 'Х1' }] }, // без fields
    });
    wrapper = mountView();
    await flushPromises();

    // previewFields взяты из текущих полей таблицы (3), раз снимок их не хранил.
    expect(wrapper.find('[data-testid="tv-cars"]').attributes('data-fields')).toBe('3');
  });

  it('показывает пустое состояние без версий', async () => {
    listTableSnapshots.mockResolvedValue({ items: [], total: 0 });
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.find('[data-testid="tv-empty"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="tv-cars"]').exists()).toBe(false);
    expect(getTableSnapshot).not.toHaveBeenCalled();
  });

  it('на первом заходе (список ещё грузится) показывает спиннер, а не "версий нет"', async () => {
    // Список висит: listLoading=true, items ещё пуст. Карточка таблицы должна
    // показать спиннер, а не пропасть (иначе флэш пустоты между тулбаром и футером).
    let resolveList;
    listTableSnapshots.mockReturnValue(new Promise((r) => { resolveList = r; }));
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.find('[data-testid="tv-body"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="tv-detail-loading"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="tv-empty"]').exists()).toBe(false);

    resolveList({ items: [], total: 0 });
    await flushPromises();
    // Пришёл пустой список -> "версий пока нет", карточки таблицы уже нет.
    expect(wrapper.find('[data-testid="tv-empty"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="tv-body"]').exists()).toBe(false);
  });

  it('показывает "таблица была пуста" для снимка без строк', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });
    getTableSnapshot.mockResolvedValue(carsSnapshot(1, []));
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.find('[data-testid="tv-detail-empty"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="tv-preview"]').exists()).toBe(false);
  });

  it('уведомляет об ошибке загрузки списка', async () => {
    listTableSnapshots.mockRejectedValue(new Error('boom'));
    wrapper = mountView();
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
  });

  it('показывает ошибку недоступной таблицы', async () => {
    apiRequest.mockResolvedValue({ json: () => Promise.resolve({ table: null }) });
    listTableSnapshots.mockResolvedValue({ items: [], total: 0 });
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.find('[data-testid="tv-error"]').exists()).toBe(true);
    expect(listTableSnapshots).not.toHaveBeenCalled();
  });

  it('не даёт устаревшему ответу детали затереть актуальный выбор (#632)', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1), snapItem(2)], total: 2 });
    const resolvers = new Map();
    getTableSnapshot.mockImplementation((tid, sid) => new Promise((resolve) => resolvers.set(sid, resolve)));

    wrapper = mountView();
    await flushPromises(); // автовыбор версии 1 (медленный, ждёт resolver)

    const select = wrapper.find('[data-testid="tv-version-select"]');
    await select.setValue('2'); // быстро переключились на версию 2 (последний выбор)

    // Резолвим последний выбор первым, затем устаревший.
    resolvers.get(2)(carsSnapshot(2, [{ id: 2, car_number: 'В222ВВ', organization: 'ООО Два', territory_status: 1 }]));
    await flushPromises();
    resolvers.get(1)(carsSnapshot(1, [{ id: 1, car_number: 'О111ОО', organization: 'ООО Один', territory_status: 1 }]));
    await flushPromises();

    // Таблица показывает версию 2 (актуальный выбор), а не затёртую устаревшим ответом.
    expect(wrapper.find('.preview-cell').text()).toBe('В222ВВ');
  });

  it('подгружает следующую страницу версий по "Ещё"', async () => {
    listTableSnapshots
      .mockResolvedValueOnce({ items: [snapItem(1), snapItem(2)], total: 3 })
      .mockResolvedValueOnce({ items: [snapItem(3)], total: 3 });
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.findAll('[data-testid="tv-version-select"] option')).toHaveLength(2);
    await wrapper.find('[data-testid="tv-load-more"]').trigger('click');
    await flushPromises();

    expect(listTableSnapshots).toHaveBeenLastCalledWith(5, expect.objectContaining({ page: 2 }));
    expect(wrapper.findAll('[data-testid="tv-version-select"] option')).toHaveLength(3);
  });
});

describe('TableVersionsView действия (#980 polish-r2)', () => {
  it('"Сохранить сейчас" делает ручной снимок, уведомляет и перезагружает список', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });
    wrapper = mountView();
    await flushPromises();
    listTableSnapshots.mockClear();

    await wrapper.find('[data-testid="tv-snapshot-now"]').trigger('click');
    await flushPromises();

    expect(createTableSnapshot).toHaveBeenCalledWith(5);
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Сохранена версия таблицы', bold: 'КПП-1', type: 'success' }));
    expect(listTableSnapshots).toHaveBeenCalled();
  });

  it('уведомляет об ошибке ручного снимка', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });
    createTableSnapshot.mockRejectedValue(new Error('boom'));
    wrapper = mountView();
    await flushPromises();

    await wrapper.find('[data-testid="tv-snapshot-now"]').trigger('click');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Не удалось сохранить версию', type: 'error' }));
  });

  it('"Excel"/"PDF" выгружают выбранную версию и сохраняют файл', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(7)], total: 1 });
    getTableSnapshot.mockResolvedValue(carsSnapshot(7, []));
    wrapper = mountView();
    await flushPromises(); // автовыбор версии 7

    await wrapper.find('[data-testid="tv-export-xlsx"]').trigger('click');
    await flushPromises();
    expect(exportTableSnapshot).toHaveBeenCalledWith(5, 7, 'xlsx');
    expect(saveBlobAs).toHaveBeenCalledWith(expect.any(Blob), 'КПП-1.xlsx');

    await wrapper.find('[data-testid="tv-export-pdf"]').trigger('click');
    await flushPromises();
    expect(exportTableSnapshot).toHaveBeenLastCalledWith(5, 7, 'pdf');
  });

  it('уведомляет об ошибке экспорта, файл не сохраняет', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(7)], total: 1 });
    getTableSnapshot.mockResolvedValue(carsSnapshot(7, []));
    exportTableSnapshot.mockRejectedValue(new Error('boom'));
    wrapper = mountView();
    await flushPromises();

    await wrapper.find('[data-testid="tv-export-xlsx"]').trigger('click');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Не удалось выгрузить файл', type: 'error' }));
    expect(saveBlobAs).not.toHaveBeenCalled();
  });

  it('чистка: окно с сегментом периода (дефолт 2 года) удаляет и уведомляет', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });
    cleanupTableSnapshots.mockResolvedValue({ deleted: 3, message: 'ok' });
    wrapper = mountView();
    await flushPromises();

    // Окно закрыто, сегмент/пояснение не в дереве, пока не нажали "Очистить".
    expect(wrapper.find('[data-testid="tv-cleanup-modal"]').exists()).toBe(false);
    await wrapper.find('[data-testid="tv-cleanup"]').trigger('click');
    expect(wrapper.find('[data-testid="tv-cleanup-modal"]').exists()).toBe(true);
    // Дефолтный период - 2 года: пояснение говорит "двух лет".
    expect(wrapper.find('[data-testid="tv-cleanup-hint"]').text()).toContain('двух лет');

    await wrapper.find('[data-testid="tv-cleanup-confirm"]').trigger('click');
    await flushPromises();

    expect(cleanupTableSnapshots).toHaveBeenCalledWith(5, 24);
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Удалено старых версий:', bold: '3', type: 'success' }));
    expect(wrapper.find('[data-testid="tv-cleanup-modal"]').exists()).toBe(false);
  });

  it('чистка учитывает выбранный сегмент 1 год (12 мес.) и пояснение меняется', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });
    wrapper = mountView();
    await flushPromises();

    await wrapper.find('[data-testid="tv-cleanup"]').trigger('click');
    await wrapper.find('[data-testid="tv-period-12"]').trigger('click');
    expect(wrapper.find('[data-testid="tv-cleanup-hint"]').text()).toContain('одного года');
    await wrapper.find('[data-testid="tv-cleanup-confirm"]').trigger('click');
    await flushPromises();

    expect(cleanupTableSnapshots).toHaveBeenCalledWith(5, 12);
  });

  it('чистка: Отмена закрывает окно без вызова API', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });
    wrapper = mountView();
    await flushPromises();

    await wrapper.find('[data-testid="tv-cleanup"]').trigger('click');
    // Кнопка "Отмена" - первая в слоте actions окна (перед "Удалить").
    await wrapper.find('[data-testid="tv-cleanup-modal"] .modal-actions button').trigger('click');
    await flushPromises();

    expect(cleanupTableSnapshots).not.toHaveBeenCalled();
    expect(wrapper.find('[data-testid="tv-cleanup-modal"]').exists()).toBe(false);
  });

  it('блокирует действия, пока таблица не загружена (нет тихого no-op)', async () => {
    let resolveTable;
    apiRequest.mockReturnValue(new Promise((r) => { resolveTable = r; }));
    listTableSnapshots.mockResolvedValue({ items: [], total: 0 });
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.find('[data-testid="tv-snapshot-now"]').attributes('disabled')).toBeDefined();
    expect(wrapper.find('[data-testid="tv-cleanup"]').attributes('disabled')).toBeDefined();

    resolveTable({ json: () => Promise.resolve({ table: { id: 5, table_type: 'cars', display_name: 'КПП-1' }, fields: [] }) });
    await flushPromises();

    expect(wrapper.find('[data-testid="tv-snapshot-now"]').attributes('disabled')).toBeUndefined();
  });

  it('кнопка чистки скрыта без права page.admin (гейт под BE requireAdmin, #976)', async () => {
    permState.can = (k) => k !== 'page.admin';
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.find('[data-testid="tv-cleanup"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="tv-cleanup-period"]').exists()).toBe(false);
    // Снимок и экспорт доступны всем, кто видит вкладку версий.
    expect(wrapper.find('[data-testid="tv-snapshot-now"]').exists()).toBe(true);
  });
});

describe('TableVersionsView поиск и фильтр даты (#980 polish-r3)', () => {
  it('поиск по строкам пробрасывается в CarsTable как search-query', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(7)], total: 1 });
    getTableSnapshot.mockResolvedValue(carsSnapshot(7, [
      { id: 1, car_number: 'А123ВС', car_brand: 'BMW', organization: 'ООО Ромашка', territory_status: 1 },
    ]));
    wrapper = mountView();
    await flushPromises();

    // Поиск виден, когда есть строки; фильтрация делегируется CarsTable.
    const search = wrapper.find('.tv-search-input');
    expect(search.exists()).toBe(true);
    await search.setValue('BMW');

    expect(wrapper.find('[data-testid="tv-cars"]').attributes('data-search')).toBe('BMW');
  });

  it('поиск не показывается, пока в версии нет строк', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });
    getTableSnapshot.mockResolvedValue(carsSnapshot(1, []));
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.find('.tv-search-input').exists()).toBe(false);
  });

  it('выбор дня в календаре сужает список версий границами локального дня (ISO)', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });
    wrapper = mountView();
    await flushPromises();
    listTableSnapshots.mockClear();

    await pickVersionDate(wrapper, new Date('2026-07-01T00:00:00'));

    expect(listTableSnapshots).toHaveBeenCalledTimes(1);
    const { from, to, page } = listTableSnapshots.mock.calls[0][1];
    expect(page).toBe(1);
    // from/to - ISO-границы локального дня 01.07.2026 (TZ-независимая проверка:
    // обе границы приходятся на этот календарный день в локальной зоне).
    const ref = new Date('2026-07-01T12:00:00').toDateString();
    expect(new Date(from).toDateString()).toBe(ref);
    expect(new Date(to).toDateString()).toBe(ref);
    expect(new Date(from).getTime()).toBeLessThan(new Date(to).getTime());
  });

  it('выбор периода сужает список версий границами периода, а не одного дня', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });
    wrapper = mountView();
    await flushPromises();
    listTableSnapshots.mockClear();

    // Так календарь отдаёт "Прошлый месяц" / выбор двух дат в режиме "Период".
    await pickVersionRange(wrapper, new Date('2026-06-01T00:00:00'), new Date('2026-06-30T00:00:00'));

    expect(listTableSnapshots).toHaveBeenCalledTimes(1);
    const { from, to } = listTableSnapshots.mock.calls[0][1];
    expect(new Date(from).toDateString()).toBe(new Date('2026-06-01T12:00:00').toDateString());
    expect(new Date(to).toDateString()).toBe(new Date('2026-06-30T12:00:00').toDateString());
  });

  it('быстрый повторный выбор дня: применяется только ответ последнего запроса (#632)', async () => {
    // onMounted-загрузку держим висящей (медленный запрос), второй по выбору дня -
    // быстрый; поздний резолв первого не должен затереть актуальный список.
    let resolveSlow;
    listTableSnapshots
      .mockReturnValueOnce(new Promise((r) => { resolveSlow = r; }))
      .mockResolvedValueOnce({ items: [snapItem(42)], total: 1 });
    wrapper = mountView();
    await flushPromises(); // fetchTable отработал, первый (висящий) fetchList стартовал

    await pickVersionDate(wrapper, new Date('2026-06-10T00:00:00')); // второй запрос -> версия 42

    // Поздний резолв устаревшего первого ответа - должен быть отброшен по listSeq.
    resolveSlow({ items: [snapItem(1), snapItem(2)], total: 2 });
    await flushPromises();

    const opts = wrapper.findAll('[data-testid="tv-version-select"] option');
    expect(opts).toHaveLength(1);
    expect(opts[0].attributes('value')).toBe('42');
  });

  it('выбор дня сбрасывает выбор и автовыбирает первую версию нового дня', async () => {
    listTableSnapshots
      .mockResolvedValueOnce({ items: [snapItem(1), snapItem(2)], total: 2 })
      .mockResolvedValueOnce({ items: [snapItem(9)], total: 1 });
    wrapper = mountView();
    await flushPromises();
    getTableSnapshot.mockClear();

    await pickVersionDate(wrapper, new Date('2026-06-15T00:00:00'));

    // Автовыбор первой версии отфильтрованного дня -> деталь запрошена по её id.
    expect(getTableSnapshot).toHaveBeenLastCalledWith(5, 9);
  });

  it('сброс даты в календаре возвращает полный список версий (from/to пустые)', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });
    wrapper = mountView();
    await flushPromises();

    await pickVersionDate(wrapper, new Date('2026-07-01T00:00:00'));
    listTableSnapshots.mockClear();

    const df = wrapper.findComponent('[data-testid="tv-date-filter"]');
    df.vm.$emit('update:dateRangeStart', null);
    df.vm.$emit('update:dateRangeEnd', null);
    df.vm.$emit('clear');
    await flushPromises();

    expect(listTableSnapshots).toHaveBeenCalledWith(
      5,
      expect.objectContaining({ from: '', to: '', page: 1 }),
    );
  });
});
