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
  bulkUnbindCarsTable: vi.fn(),
}));
vi.mock('@/api/cars', () => carsBulkApi);

import CarsTable from '../CarsTable.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

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
    target_tables_count: 1,
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

describe('CarsTable - групповое "Убрать" (#1194 S5)', () => {
  let items;

  beforeEach(() => {
    setActivePinia(createPinia());
    usePermissionsStore().mode = 'super';
    apiRequest.mockReset();
    carsBulkApi.bulkMoveCarsTable.mockReset();
    carsBulkApi.bulkAddCarsTable.mockReset();
    carsBulkApi.bulkUnbindCarsTable.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url === '/system-tables') return okResponse([]);
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

  it('кнопка "Убрать" открывает подтверждение', async () => {
    const wrapper = await mountWithSelection();
    await wrapper.find('[data-testid="cars-bulk-remove"]').trigger('click');

    expect(wrapper.vm.bulkRemoveConfirmVisible).toBe(true);
  });

  it('подтверждение зовёт bulkUnbindCarsTable с ids/table_id таблицы', async () => {
    carsBulkApi.bulkUnbindCarsTable.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] });
    const wrapper = await mountWithSelection();
    await wrapper.find('[data-testid="cars-bulk-remove"]').trigger('click');

    await wrapper.vm.confirmBulkRemove();
    await flushPromises();

    expect(carsBulkApi.bulkUnbindCarsTable).toHaveBeenCalledWith([1, 2], 42);
  });

  it('полный успех -> success-notify, выбор сброшен, подтверждение закрыто', async () => {
    carsBulkApi.bulkUnbindCarsTable.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] });
    const wrapper = await mountWithSelection();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    await wrapper.find('[data-testid="cars-bulk-remove"]').trigger('click');

    await wrapper.vm.confirmBulkRemove();
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Убрано машин: ', bold: '2' }));
    expect(wrapper.vm.selectedIds).toEqual([]);
    expect(wrapper.vm.bulkRemoveConfirmVisible).toBe(false);
  });

  it('частичный успех -> warning-notify с непрошедшими именами, выбор всё равно сброшен', async () => {
    carsBulkApi.bulkUnbindCarsTable.mockResolvedValue({
      success_count: 1,
      error_count: 1,
      errors: [{ id: 2, name: 'А2', error: 'не привязана' }],
    });
    const wrapper = await mountWithSelection();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    await wrapper.find('[data-testid="cars-bulk-remove"]').trigger('click');

    await wrapper.vm.confirmBulkRemove();
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'warning', suffix: expect.stringContaining('А2') }),
    );
    expect(wrapper.vm.selectedIds).toEqual([]);
  });

  it('структурная ошибка (envelope message) -> error-notify, подтверждение остаётся открытым', async () => {
    carsBulkApi.bulkUnbindCarsTable.mockResolvedValue({ message: 'Нет доступа' });
    const wrapper = await mountWithSelection();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    await wrapper.find('[data-testid="cars-bulk-remove"]').trigger('click');

    await wrapper.vm.confirmBulkRemove();
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error', prefix: 'Нет доступа' }));
    expect(wrapper.vm.bulkRemoveConfirmVisible).toBe(true);
  });

  it('без права page.admin кнопка "Убрать" не рендерится', async () => {
    usePermissionsStore().mode = 'normal';
    const wrapper = await mountWithSelection();

    expect(wrapper.find('[data-testid="cars-bulk-remove"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="cars-bulk-clear"]').exists()).toBe(true);
  });
});

describe('CarsTable - per-row корзина "из этой/из всех" (#1194 S5)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    usePermissionsStore().mode = 'super';
    apiRequest.mockReset();
    carsBulkApi.bulkMoveCarsTable.mockReset();
    carsBulkApi.bulkAddCarsTable.mockReset();
    carsBulkApi.bulkUnbindCarsTable.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url === '/system-tables') return okResponse([]);
      if (url.startsWith('/unload-places')) return okResponse([]);
      if (url.startsWith('/license-plate-formats')) return okResponse([]);
      if (url.startsWith('/cars/active-for-table/')) return okResponse([]);
      if (url.startsWith('/cars/unload-places')) return okResponse([]);
      if (url.startsWith('/cars/history/current-status')) return okResponse([]);
      if (url.startsWith('/organizations')) return okResponse([]);
      if (url.startsWith('/notifications/deletion-settings')) return okResponse({});
      return okResponse({});
    });
  });

  async function mountWithItem(overrides) {
    const wrapper = mountTable();
    await flushPromises();
    await wrapper.setData({ itemsData: [baseItem(overrides)] });
    return wrapper;
  }

  it('target_tables_count <= 1 - корзина как раньше, без подменю', async () => {
    const wrapper = await mountWithItem({ target_tables_count: 1 });

    expect(wrapper.find('[data-testid="row-remove-trigger"]').exists()).toBe(false);
    expect(wrapper.find('.delete-btn').exists()).toBe(true);
  });

  it('target_tables_count не задан (undefined) - тоже без подменю (безопасный дефолт)', async () => {
    const wrapper = await mountWithItem({ target_tables_count: undefined });

    expect(wrapper.find('[data-testid="row-remove-trigger"]').exists()).toBe(false);
  });

  it('target_tables_count > 1 - показывает подменю с двумя пунктами', async () => {
    const wrapper = await mountWithItem({ target_tables_count: 2 });

    expect(wrapper.find('[data-testid="row-remove-trigger"]').exists()).toBe(true);
    await wrapper.find('[data-testid="row-remove-trigger"]').trigger('click');

    expect(wrapper.find('[data-testid="row-remove-current"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="row-remove-all"]').exists()).toBe(true);
  });

  it('не-админ + count > 1 - подменю "из этой/из всех" скрыто (unbind гейтится page.admin)', async () => {
    usePermissionsStore().mode = 'normal';
    const wrapper = await mountWithItem({ target_tables_count: 2 });

    // главное: не-админ не получает опцию "из этой таблицы" -> нет пути к 403 на unbind
    expect(wrapper.find('[data-testid="row-remove-trigger"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="row-remove-current"]').exists()).toBe(false);
  });

  it('"Убрать из этой таблицы" -> commit зовёт bulkUnbindCarsTable([id], tableId)', async () => {
    carsBulkApi.bulkUnbindCarsTable.mockResolvedValue({ success_count: 1, error_count: 0, errors: [] });
    const wrapper = await mountWithItem({ id: 5, car_number: 'В5', target_tables_count: 2 });
    const enqueue = vi.spyOn(useDeletionsStore(), 'enqueue');

    await wrapper.find('[data-testid="row-remove-trigger"]').trigger('click');
    await wrapper.find('[data-testid="row-remove-current"]').trigger('click');

    expect(enqueue).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Машина ', bold: 'В5' }));
    const { onConfirm } = enqueue.mock.calls[0][0];
    await onConfirm();
    await flushPromises();

    expect(carsBulkApi.bulkUnbindCarsTable).toHaveBeenCalledWith([5], 42);
  });

  it('"Убрать из всех таблиц" -> существующий deactivate-флоу не изменился', async () => {
    const wrapper = await mountWithItem({ id: 6, car_number: 'В6', target_tables_count: 2 });
    const enqueue = vi.spyOn(useDeletionsStore(), 'enqueue');

    await wrapper.find('[data-testid="row-remove-trigger"]').trigger('click');
    await wrapper.find('[data-testid="row-remove-all"]').trigger('click');

    expect(enqueue).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Машина ', bold: 'В6', suffix: ' удалена' }));
    const { onConfirm } = enqueue.mock.calls[0][0];
    await onConfirm();
    await flushPromises();

    expect(apiRequest).toHaveBeenCalledWith('/cars/6/deactivate', expect.objectContaining({ method: 'PUT' }));
    expect(carsBulkApi.bulkUnbindCarsTable).not.toHaveBeenCalled();
  });
});
