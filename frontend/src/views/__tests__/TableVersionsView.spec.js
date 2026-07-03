import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// FE #980 срез 5: вкладка "Версии" таблицы - master-detail список снимков +
// просмотр состава. Проверяем рендер списка/футера, автовыбор первого снимка,
// колонки по типу таблицы, статус-метки, пустые состояния, защиту детали от
// гонки устаревшего ответа (#632).

vi.mock('@/api/system-tables', () => ({
  listTableSnapshots: vi.fn(),
  getTableSnapshot: vi.fn(),
}));

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}));

const notify = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}));

const { routeState } = vi.hoisted(() => ({
  routeState: { params: { tableName: 'kpp-1' } },
}));
vi.mock('vue-router', () => ({
  useRoute: () => ({ params: routeState.params }),
}));

import TableVersionsView from '@/views/TableVersionsView.vue';
import { listTableSnapshots, getTableSnapshot } from '@/api/system-tables';
import { apiRequest } from '@/api/client';

const stubs = {
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
  RefreshButton: { template: '<button class="refresh-stub" @click="$emit(\'refresh\')" />' },
};

function mockTable(over = {}) {
  apiRequest.mockResolvedValue({
    json: () => Promise.resolve({ table: { id: 5, table_type: 'cars', display_name: 'КПП-1', ...over } }),
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

function carsSnapshot(id, rows) {
  return {
    id,
    table_id: 5,
    taken_at: '2026-07-01T03:00:00Z',
    reason: 'scheduled',
    counts: { on_territory: 1, exited: 1, not_entered: 0, total: 2 },
    payload: { table_type: 'cars', rows },
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
  mockTable();
  getTableSnapshot.mockResolvedValue(carsSnapshot(1, []));
});

afterEach(() => {
  wrapper?.unmount();
});

describe('TableVersionsView (#980 срез 5)', () => {
  it('рендерит список снимков и футер "Всего: N"', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1), snapItem(2)], total: 2 });
    wrapper = mountView();
    await flushPromises();

    const cards = wrapper.findAll('[data-testid="tv-card"]');
    expect(cards).toHaveLength(2);
    expect(wrapper.find('[data-testid="tv-footer"]').text()).toContain('Всего: 2');
    expect(wrapper.find('[data-testid="tv-empty"]').exists()).toBe(false);
  });

  it('автоматически выбирает первый снимок и показывает его состав', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(7), snapItem(8)], total: 2 });
    getTableSnapshot.mockResolvedValue(carsSnapshot(7, [
      { car_number: 'А123ВС', car_brand: 'BMW', organization: 'ООО Ромашка', entry_date_to: '2026-07-05', territory_status: 1 },
    ]));
    wrapper = mountView();
    await flushPromises();

    expect(getTableSnapshot).toHaveBeenCalledWith(5, 7);
    expect(wrapper.find('[data-testid="tv-detail"]').exists()).toBe(true);
    const rows = wrapper.findAll('[data-testid="tv-row"]');
    expect(rows).toHaveLength(1);
    expect(rows[0].text()).toContain('А123ВС');
    expect(rows[0].text()).toContain('BMW');
    expect(rows[0].text()).toContain('На территории');
  });

  it('состав cars: даты форматируются, статусы выехал/не въезжал корректны', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });
    getTableSnapshot.mockResolvedValue(carsSnapshot(1, [
      { car_number: 'Х001ХХ', car_brand: 'Kia', organization: 'ООО А', entry_date_to: '2026-07-05', territory_status: 2 },
      { car_number: 'У002УУ', car_brand: 'Lada', organization: 'ООО Б', entry_date_to: null, territory_status: 0 },
    ]));
    wrapper = mountView();
    await flushPromises();

    const rows = wrapper.findAll('[data-testid="tv-row"]');
    expect(rows[0].text()).toContain('05.07.2026');
    expect(rows[0].text()).toContain('Выехал');
    // Пустая дата действия -> прочерк, статус 0 -> "Не въезжал".
    expect(rows[1].text()).toContain('—');
    expect(rows[1].text()).toContain('Не въезжал');
  });

  it('people-снимок рендерит колонки ФИО/должность', async () => {
    mockTable({ table_type: 'people' });
    listTableSnapshots.mockResolvedValue({ items: [snapItem(3, { reason: 'manual', actor_name: 'Иванов И.И.' })], total: 1 });
    getTableSnapshot.mockResolvedValue({
      id: 3,
      taken_at: '2026-07-01T03:00:00Z',
      reason: 'manual',
      counts: { on_territory: 1, exited: 0, not_entered: 0, total: 1 },
      payload: {
        table_type: 'people',
        rows: [
          { last_name: 'Петров', first_name: 'Пётр', middle_name: 'Петрович', organization: 'ООО В', position: 'Грузчик', territory_status: 1 },
        ],
      },
    });
    wrapper = mountView();
    await flushPromises();

    const head = wrapper.find('[data-testid="tv-composition"]').find('thead');
    expect(head.text()).toContain('Фамилия');
    expect(head.text()).toContain('Должность');
    const row = wrapper.find('[data-testid="tv-row"]');
    expect(row.text()).toContain('Петров');
    expect(row.text()).toContain('Грузчик');
    // Ручной снимок показывает автора.
    expect(wrapper.find('[data-testid="tv-card"]').text()).toContain('Иванов И.И.');
    expect(wrapper.find('[data-testid="tv-card"]').text()).toContain('Ручной');
  });

  it('показывает пустое состояние без снимков', async () => {
    listTableSnapshots.mockResolvedValue({ items: [], total: 0 });
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.find('[data-testid="tv-empty"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="tv-detail"]').exists()).toBe(false);
    expect(getTableSnapshot).not.toHaveBeenCalled();
  });

  it('показывает "таблица была пуста" для снимка без строк', async () => {
    listTableSnapshots.mockResolvedValue({ items: [snapItem(1)], total: 1 });
    getTableSnapshot.mockResolvedValue(carsSnapshot(1, []));
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.find('[data-testid="tv-detail-empty"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="tv-composition"]').exists()).toBe(false);
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
    await flushPromises(); // автовыбор снимка 1 (медленный, ждёт resolver)

    const cards = wrapper.findAll('[data-testid="tv-card"]');
    await cards[0].trigger('click'); // снова снимок 1
    await cards[1].trigger('click'); // быстро переключились на снимок 2 (последний)

    // Резолвим последний выбор первым, затем устаревший.
    resolvers.get(2)(carsSnapshot(2, [
      { car_number: 'В222ВВ', car_brand: 'Audi', organization: 'ООО Два', entry_date_to: '2026-07-09', territory_status: 1 },
    ]));
    await flushPromises();
    resolvers.get(1)(carsSnapshot(1, [
      { car_number: 'О111ОО', car_brand: 'Ford', organization: 'ООО Один', entry_date_to: '2026-07-08', territory_status: 1 },
    ]));
    await flushPromises();

    // Деталь показывает снимок 2 (актуальный выбор), а не затёртый устаревшим ответом.
    expect(wrapper.find('[data-testid="tv-detail"]').text()).toContain('В222ВВ');
    expect(wrapper.find('[data-testid="tv-detail"]').text()).not.toContain('О111ОО');
  });

  it('подгружает следующую страницу по "Показать ещё"', async () => {
    listTableSnapshots
      .mockResolvedValueOnce({ items: [snapItem(1), snapItem(2)], total: 3 })
      .mockResolvedValueOnce({ items: [snapItem(3)], total: 3 });
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.findAll('[data-testid="tv-card"]')).toHaveLength(2);
    await wrapper.find('[data-testid="tv-load-more"]').trigger('click');
    await flushPromises();

    expect(listTableSnapshots).toHaveBeenLastCalledWith(5, expect.objectContaining({ page: 2 }));
    expect(wrapper.findAll('[data-testid="tv-card"]')).toHaveLength(3);
  });
});
