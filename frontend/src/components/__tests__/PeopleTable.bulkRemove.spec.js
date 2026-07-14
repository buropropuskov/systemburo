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

const employeesBulkApi = vi.hoisted(() => ({
  bulkMoveEmployeesTable: vi.fn(),
  bulkAddEmployeesTable: vi.fn(),
  bulkUnbindEmployeesTable: vi.fn(),
}));
vi.mock('@/api/employees', () => employeesBulkApi);

import PeopleTable from '../PeopleTable.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

function baseItem(overrides) {
  return {
    id: 1,
    last_name: 'Иванов',
    first_name: 'Иван',
    middle_name: 'Иванович',
    position: 'Водитель',
    citizenship_name: 'РФ',
    organization_name: 'ООО',
    status: 'В работе',
    entry_date_to: '2026-06-05',
    pass_time: '08:00',
    target_tables_count: 1,
    ...overrides,
  };
}

function mountTable() {
  return mount(PeopleTable, {
    props: {
      tableName: 'КПП-72',
      currentUserId: 1,
      currentUserName: 'Тест',
    },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });
}

describe('PeopleTable - групповое "Убрать" (#1194 S5)', () => {
  let items;

  beforeEach(() => {
    setActivePinia(createPinia());
    usePermissionsStore().mode = 'super';
    apiRequest.mockReset();
    employeesBulkApi.bulkMoveEmployeesTable.mockReset();
    employeesBulkApi.bulkAddEmployeesTable.mockReset();
    employeesBulkApi.bulkUnbindEmployeesTable.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/system-tables/name/')) return okResponse({ table: { id: 7 } });
      if (url === '/system-tables') return okResponse([]);
      if (url.startsWith('/organizations')) return okResponse([]);
      if (url.startsWith('/employees/active-for-table/')) return okResponse([]);
      if (url.startsWith('/employees/history/current-status')) return okResponse([]);
      if (url.startsWith('/notifications/deletion-settings')) return okResponse({});
      return okResponse({});
    });
    items = [
      baseItem({ id: 1, last_name: 'Иванов' }),
      baseItem({ id: 2, last_name: 'Петров' }),
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
    await wrapper.find('[data-testid="people-bulk-remove"]').trigger('click');

    expect(wrapper.vm.bulkRemoveConfirmVisible).toBe(true);
  });

  it('подтверждение зовёт bulkUnbindEmployeesTable с ids/table_id таблицы (id=7)', async () => {
    employeesBulkApi.bulkUnbindEmployeesTable.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] });
    const wrapper = await mountWithSelection();
    await wrapper.find('[data-testid="people-bulk-remove"]').trigger('click');

    await wrapper.vm.confirmBulkRemove();
    await flushPromises();

    expect(employeesBulkApi.bulkUnbindEmployeesTable).toHaveBeenCalledWith([1, 2], 7);
  });

  it('полный успех -> success-notify, выбор сброшен, подтверждение закрыто', async () => {
    employeesBulkApi.bulkUnbindEmployeesTable.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] });
    const wrapper = await mountWithSelection();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    await wrapper.find('[data-testid="people-bulk-remove"]').trigger('click');

    await wrapper.vm.confirmBulkRemove();
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Убрано сотрудников: ', bold: '2' }));
    expect(wrapper.vm.selectedIds).toEqual([]);
    expect(wrapper.vm.bulkRemoveConfirmVisible).toBe(false);
  });

  it('частичный успех -> warning-notify с непрошедшими именами, выбор всё равно сброшен', async () => {
    employeesBulkApi.bulkUnbindEmployeesTable.mockResolvedValue({
      success_count: 1,
      error_count: 1,
      errors: [{ id: 2, name: 'Петров', error: 'не привязан' }],
    });
    const wrapper = await mountWithSelection();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    await wrapper.find('[data-testid="people-bulk-remove"]').trigger('click');

    await wrapper.vm.confirmBulkRemove();
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'warning', suffix: expect.stringContaining('Петров') }),
    );
    expect(wrapper.vm.selectedIds).toEqual([]);
  });

  it('структурная ошибка (envelope message) -> error-notify, подтверждение остаётся открытым', async () => {
    employeesBulkApi.bulkUnbindEmployeesTable.mockResolvedValue({ message: 'Нет доступа' });
    const wrapper = await mountWithSelection();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    await wrapper.find('[data-testid="people-bulk-remove"]').trigger('click');

    await wrapper.vm.confirmBulkRemove();
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error', prefix: 'Нет доступа' }));
    expect(wrapper.vm.bulkRemoveConfirmVisible).toBe(true);
  });

  it('без права page.admin кнопка "Убрать" не рендерится', async () => {
    usePermissionsStore().mode = 'normal';
    const wrapper = await mountWithSelection();

    expect(wrapper.find('[data-testid="people-bulk-remove"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="people-bulk-clear"]').exists()).toBe(true);
  });
});

describe('PeopleTable - per-row корзина "из этой/из всех" (#1194 S5)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    usePermissionsStore().mode = 'super';
    apiRequest.mockReset();
    employeesBulkApi.bulkMoveEmployeesTable.mockReset();
    employeesBulkApi.bulkAddEmployeesTable.mockReset();
    employeesBulkApi.bulkUnbindEmployeesTable.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/system-tables/name/')) return okResponse({ table: { id: 7 } });
      if (url === '/system-tables') return okResponse([]);
      if (url.startsWith('/organizations')) return okResponse([]);
      if (url.startsWith('/employees/active-for-table/')) return okResponse([]);
      if (url.startsWith('/employees/history/current-status')) return okResponse([]);
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

  it('"Убрать из этой таблицы" -> commit зовёт bulkUnbindEmployeesTable([id], tableId)', async () => {
    employeesBulkApi.bulkUnbindEmployeesTable.mockResolvedValue({ success_count: 1, error_count: 0, errors: [] });
    const wrapper = await mountWithItem({ id: 9, last_name: 'Сидоров', target_tables_count: 2 });
    const enqueue = vi.spyOn(useDeletionsStore(), 'enqueue');

    await wrapper.find('[data-testid="row-remove-trigger"]').trigger('click');
    await wrapper.find('[data-testid="row-remove-current"]').trigger('click');

    expect(enqueue).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Сотрудник ' }));
    const { onConfirm } = enqueue.mock.calls[0][0];
    await onConfirm();
    await flushPromises();

    expect(employeesBulkApi.bulkUnbindEmployeesTable).toHaveBeenCalledWith([9], 7);
  });

  it('"Убрать из всех таблиц" -> существующий deactivate-флоу не изменился', async () => {
    const wrapper = await mountWithItem({ id: 10, last_name: 'Кузнецов', target_tables_count: 2 });
    const enqueue = vi.spyOn(useDeletionsStore(), 'enqueue');

    await wrapper.find('[data-testid="row-remove-trigger"]').trigger('click');
    await wrapper.find('[data-testid="row-remove-all"]').trigger('click');

    expect(enqueue).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Сотрудник ', suffix: ' удалён' }));
    const { onConfirm } = enqueue.mock.calls[0][0];
    await onConfirm();
    await flushPromises();

    expect(apiRequest).toHaveBeenCalledWith('/employees/10/deactivate', expect.objectContaining({ method: 'PUT' }));
    expect(employeesBulkApi.bulkUnbindEmployeesTable).not.toHaveBeenCalled();
  });
});
