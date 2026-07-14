import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

const carsBulkApi = vi.hoisted(() => ({
  bulkMoveCarsTable: vi.fn(),
  bulkAddCarsTable: vi.fn(),
}));
vi.mock('@/api/cars', () => carsBulkApi);

import CarsTable from '../CarsTable.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

// Текущая таблица (tableId=42) обязана исчезнуть из списка целей, а people-таблица
// (table_type=people) - отфильтроваться по типу (#1194 S4).
const SYSTEM_TABLES = [
  { id: 42, name: 'kpp-1', display_name: 'КПП 1', table_type: 'cars', status: 'active' },
  { id: 50, name: 'kpp-2', display_name: 'КПП 2', table_type: 'cars', status: 'active' },
  { id: 51, name: 'kpp-3', display_name: 'КПП 3 (архив)', table_type: 'cars', status: 'inactive', status_comment: 'на обслуживании' },
  { id: 60, name: 'people-1', display_name: 'Проход 1', table_type: 'people', status: 'active' },
];

function baseItem(overrides) {
  return {
    id: 1,
    car_number: 'А1',
    car_brand: 'BMW',
    plateNumber: 'А1',
    mark: 'BMW',
    status: 'В работе',
    entry_date_to: '2026-06-05',
    entry_time_from: '08:00',
    entry_time_to: '18:00',
    unloadPlaces: [],
    organization_name: 'ООО',
    ...overrides,
  };
}

function mountTable() {
  return mount(CarsTable, {
    props: {
      tableName: 'КПП 1',
      tableId: 42,
      currentUserId: 1,
      currentUserName: 'Тест',
    },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });
}

describe('CarsTable - групповые операции Перенести/Добавить (#1194 S4)', () => {
  let items;

  beforeEach(() => {
    setActivePinia(createPinia());
    // BE гейтит requireAdmin (page.admin) - без права кнопки скрыты (см. отдельный тест).
    usePermissionsStore().mode = 'super';
    apiRequest.mockReset();
    carsBulkApi.bulkMoveCarsTable.mockReset();
    carsBulkApi.bulkAddCarsTable.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url === '/system-tables') return okResponse(SYSTEM_TABLES);
      if (url.startsWith('/unload-places')) return okResponse([]);
      if (url.startsWith('/license-plate-formats')) return okResponse([]);
      if (url.startsWith('/cars/active-for-table/')) return okResponse([]);
      if (url.startsWith('/cars/unload-places')) return okResponse([]);
      if (url.startsWith('/cars/history/current-status')) return okResponse([]);
      if (url.startsWith('/organizations')) return okResponse([]);
      if (url.startsWith('/notifications/deletion-settings')) return okResponse({});
      return okResponse({});
    });
    items = [
      baseItem({ id: 1, car_number: 'А1' }),
      baseItem({ id: 2, car_number: 'А2' }),
    ];
  });

  async function mountWithSelection() {
    const wrapper = mountTable();
    await flushPromises();
    await wrapper.setData({ itemsData: items });
    const rows = wrapper.findAll('.item-row');
    await rows[0].trigger('click', { ctrlKey: true });
    await rows[1].trigger('click', { ctrlKey: true });
    return wrapper;
  }

  it('кнопка "Перенести" открывает модалку с заголовком "Перенести в таблицу"', async () => {
    const wrapper = await mountWithSelection();
    await wrapper.find('[data-testid="cars-bulk-move"]').trigger('click');
    await flushPromises();

    expect(wrapper.find('[data-testid="table-bulk-target-modal"]').exists()).toBe(true);
    expect(wrapper.find('.base-modal__title').text()).toBe('Перенести в таблицу');
  });

  it('кнопка "Добавить в таблицу" открывает модалку с заголовком "Добавить в таблицу"', async () => {
    const wrapper = await mountWithSelection();
    await wrapper.find('[data-testid="cars-bulk-add"]').trigger('click');
    await flushPromises();

    expect(wrapper.find('.base-modal__title').text()).toBe('Добавить в таблицу');
  });

  it('модалка исключает текущую таблицу и таблицы другого типа', async () => {
    const wrapper = await mountWithSelection();
    await wrapper.find('[data-testid="cars-bulk-move"]').trigger('click');
    await flushPromises();

    const labels = wrapper.findAll('.passage__item').map((t) => t.text());
    expect(labels).toEqual(['КПП 2', 'КПП 3 (архив)']);
  });

  it('кнопка подтверждения дизейблена, пока не выбрана ни одна таблица', async () => {
    const wrapper = await mountWithSelection();
    await wrapper.find('[data-testid="cars-bulk-move"]').trigger('click');
    await flushPromises();

    expect(wrapper.find('[data-testid="table-bulk-target-apply"]').attributes('disabled')).toBeDefined();
  });

  it('Перенести: выбор таблицы + подтверждение зовёт bulkMoveCarsTable с ids/from/to', async () => {
    carsBulkApi.bulkMoveCarsTable.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] });
    const wrapper = await mountWithSelection();
    await wrapper.find('[data-testid="cars-bulk-move"]').trigger('click');
    await flushPromises();

    await wrapper.findAll('.passage__item')[0].trigger('click'); // КПП 2, id 50
    await wrapper.find('[data-testid="table-bulk-target-apply"]').trigger('click');
    await flushPromises();

    expect(carsBulkApi.bulkMoveCarsTable).toHaveBeenCalledWith([1, 2], 42, [50]);
    expect(carsBulkApi.bulkAddCarsTable).not.toHaveBeenCalled();
  });

  it('Добавить: выбор таблицы + подтверждение зовёт bulkAddCarsTable с ids/table_ids', async () => {
    carsBulkApi.bulkAddCarsTable.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] });
    const wrapper = await mountWithSelection();
    await wrapper.find('[data-testid="cars-bulk-add"]').trigger('click');
    await flushPromises();

    await wrapper.findAll('.passage__item')[0].trigger('click');
    await wrapper.find('[data-testid="table-bulk-target-apply"]').trigger('click');
    await flushPromises();

    expect(carsBulkApi.bulkAddCarsTable).toHaveBeenCalledWith([1, 2], [50]);
    expect(carsBulkApi.bulkMoveCarsTable).not.toHaveBeenCalled();
  });

  it('полный успех -> success-notify, выбор сброшен, модалка закрыта', async () => {
    carsBulkApi.bulkMoveCarsTable.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] });
    const wrapper = await mountWithSelection();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    await wrapper.find('[data-testid="cars-bulk-move"]').trigger('click');
    await flushPromises();
    await wrapper.findAll('.passage__item')[0].trigger('click');
    await wrapper.find('[data-testid="table-bulk-target-apply"]').trigger('click');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Перенесено машин: ', bold: '2' }));
    expect(wrapper.vm.selectedIds).toEqual([]);
    expect(wrapper.vm.bulkModalVisible).toBe(false);
  });

  it('частичный успех -> warning-notify с непрошедшими именами, выбор всё равно сброшен', async () => {
    carsBulkApi.bulkMoveCarsTable.mockResolvedValue({
      success_count: 1,
      error_count: 1,
      errors: [{ id: 2, name: 'А2', error: 'занята' }],
    });
    const wrapper = await mountWithSelection();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    await wrapper.find('[data-testid="cars-bulk-move"]').trigger('click');
    await flushPromises();
    await wrapper.findAll('.passage__item')[0].trigger('click');
    await wrapper.find('[data-testid="table-bulk-target-apply"]').trigger('click');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'warning', suffix: expect.stringContaining('А2') }),
    );
    expect(wrapper.vm.selectedIds).toEqual([]);
  });

  it('структурная ошибка (envelope message) -> error-notify, модалка остаётся открытой', async () => {
    carsBulkApi.bulkMoveCarsTable.mockResolvedValue({ message: 'Нет доступа' });
    const wrapper = await mountWithSelection();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    await wrapper.find('[data-testid="cars-bulk-move"]').trigger('click');
    await flushPromises();
    await wrapper.findAll('.passage__item')[0].trigger('click');
    await wrapper.find('[data-testid="table-bulk-target-apply"]').trigger('click');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error', prefix: 'Нет доступа' }));
    expect(wrapper.vm.bulkModalVisible).toBe(true);
  });

  it('без права page.admin кнопки Перенести/Добавить не рендерятся', async () => {
    usePermissionsStore().mode = 'normal';
    const wrapper = await mountWithSelection();

    expect(wrapper.find('[data-testid="cars-bulk-move"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="cars-bulk-add"]').exists()).toBe(false);
    // "Снять выбор" остаётся доступной всем (S3, безопасное действие).
    expect(wrapper.find('[data-testid="cars-bulk-clear"]').exists()).toBe(true);
  });
});
