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
}));
vi.mock('@/api/employees', () => employeesBulkApi);

import PeopleTable from '../PeopleTable.vue';

function okResponse(data) {
  return { ok: true, json: async () => data };
}

// currentTableId разрешается через /system-tables/name/:name -> id=7 (см. PeopleTable.selection.spec.js).
// Она обязана исчезнуть из списка целей, а cars-таблица - отфильтроваться по типу (#1194 S4).
const SYSTEM_TABLES = [
  { id: 7, name: 'kpp-72', display_name: 'КПП-72', table_type: 'people', status: 'active' },
  { id: 8, name: 'kpp-73', display_name: 'КПП-73', table_type: 'people', status: 'active' },
  { id: 9, name: 'kpp-74', display_name: 'КПП-74 (архив)', table_type: 'people', status: 'inactive', status_comment: 'на обслуживании' },
  { id: 20, name: 'cars-1', display_name: 'Проезд 1', table_type: 'cars', status: 'active' },
];

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

describe('PeopleTable - групповые операции Перенести/Добавить (#1194 S4)', () => {
  let items;

  beforeEach(() => {
    setActivePinia(createPinia());
    usePermissionsStore().mode = 'super';
    apiRequest.mockReset();
    employeesBulkApi.bulkMoveEmployeesTable.mockReset();
    employeesBulkApi.bulkAddEmployeesTable.mockReset();
    apiRequest.mockImplementation((url) => {
      if (url.startsWith('/system-tables/name/')) return okResponse({ table: { id: 7 } });
      if (url === '/system-tables') return okResponse(SYSTEM_TABLES);
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

  it('кнопка "Перенести" открывает модалку с заголовком "Перенести в таблицу"', async () => {
    const wrapper = await mountWithSelection();
    await wrapper.find('[data-testid="people-bulk-move"]').trigger('click');
    await flushPromises();

    expect(wrapper.find('[data-testid="table-bulk-target-modal"]').exists()).toBe(true);
    expect(wrapper.find('.base-modal__title').text()).toBe('Перенести в таблицу');
  });

  it('модалка исключает текущую таблицу (id=7) и таблицы другого типа', async () => {
    const wrapper = await mountWithSelection();
    await wrapper.find('[data-testid="people-bulk-move"]').trigger('click');
    await flushPromises();

    const labels = wrapper.findAll('.passage__item').map((t) => t.text());
    expect(labels).toEqual(['КПП-73', 'КПП-74 (архив)']);
  });

  it('Перенести: выбор таблицы + подтверждение зовёт bulkMoveEmployeesTable с ids/from/to', async () => {
    employeesBulkApi.bulkMoveEmployeesTable.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] });
    const wrapper = await mountWithSelection();
    await wrapper.find('[data-testid="people-bulk-move"]').trigger('click');
    await flushPromises();

    await wrapper.findAll('.passage__item')[0].trigger('click'); // КПП-73, id 8
    await wrapper.find('[data-testid="table-bulk-target-apply"]').trigger('click');
    await flushPromises();

    expect(employeesBulkApi.bulkMoveEmployeesTable).toHaveBeenCalledWith([1, 2], 7, [8]);
    expect(employeesBulkApi.bulkAddEmployeesTable).not.toHaveBeenCalled();
  });

  it('Добавить: выбор таблицы + подтверждение зовёт bulkAddEmployeesTable с ids/table_ids', async () => {
    employeesBulkApi.bulkAddEmployeesTable.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] });
    const wrapper = await mountWithSelection();
    await wrapper.find('[data-testid="people-bulk-add"]').trigger('click');
    await flushPromises();

    await wrapper.findAll('.passage__item')[0].trigger('click');
    await wrapper.find('[data-testid="table-bulk-target-apply"]').trigger('click');
    await flushPromises();

    expect(employeesBulkApi.bulkAddEmployeesTable).toHaveBeenCalledWith([1, 2], [8]);
    expect(employeesBulkApi.bulkMoveEmployeesTable).not.toHaveBeenCalled();
  });

  it('полный успех -> success-notify, выбор сброшен, модалка закрыта', async () => {
    employeesBulkApi.bulkMoveEmployeesTable.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] });
    const wrapper = await mountWithSelection();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    await wrapper.find('[data-testid="people-bulk-move"]').trigger('click');
    await flushPromises();
    await wrapper.findAll('.passage__item')[0].trigger('click');
    await wrapper.find('[data-testid="table-bulk-target-apply"]').trigger('click');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ prefix: 'Перенесено сотрудников: ', bold: '2' }));
    expect(wrapper.vm.selectedIds).toEqual([]);
    expect(wrapper.vm.bulkModalVisible).toBe(false);
  });

  it('частичный успех -> warning-notify с непрошедшими именами, выбор всё равно сброшен', async () => {
    employeesBulkApi.bulkMoveEmployeesTable.mockResolvedValue({
      success_count: 1,
      error_count: 1,
      errors: [{ id: 2, name: 'Петров', error: 'занят' }],
    });
    const wrapper = await mountWithSelection();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    await wrapper.find('[data-testid="people-bulk-move"]').trigger('click');
    await flushPromises();
    await wrapper.findAll('.passage__item')[0].trigger('click');
    await wrapper.find('[data-testid="table-bulk-target-apply"]').trigger('click');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'warning', suffix: expect.stringContaining('Петров') }),
    );
    expect(wrapper.vm.selectedIds).toEqual([]);
  });

  it('без права page.admin кнопки Перенести/Добавить не рендерятся', async () => {
    usePermissionsStore().mode = 'normal';
    const wrapper = await mountWithSelection();

    expect(wrapper.find('[data-testid="people-bulk-move"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="people-bulk-add"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="people-bulk-clear"]').exists()).toBe(true);
  });
});
