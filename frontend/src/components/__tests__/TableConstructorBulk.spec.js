import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// TableConstructor.vue дёргает apiRequest напрямую (не через api/system-tables.js)
// для списка/CRUD одиночных таблиц - мокаем клиент общим обработчиком по URL. Форма
// ответа списка - РЕАЛЬНАЯ: [{table:{id,display_name,name,table_type,is_active,
// status,...}, fields, fact_fields, time_slots, photos, current_status}]
// (см. models.SystemTableWithDetails, system_tables_test.go).
const apiClientMock = vi.hoisted(() => ({ apiRequest: vi.fn() }));
vi.mock('@/api/client', () => apiClientMock);

const systemTablesApi = vi.hoisted(() => ({
  bulkArchiveSystemTables: vi.fn(),
  bulkRestoreSystemTables: vi.fn(),
}));
vi.mock('@/api/system-tables', () => systemTablesApi);

import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';
import TableConstructor from '../TableConstructor.vue';

const shellStub = { template: '<div><slot /></div>' };

function seedTables() {
  return [
    { table: { id: 1, name: 'alpha', display_name: 'Alpha', table_type: 'cars', is_active: true, status: 'active' }, current_status: 'open' },
    { table: { id: 2, name: 'beta', display_name: 'Beta', table_type: 'cars', is_active: true, status: 'active' }, current_status: 'open' },
    { table: { id: 3, name: 'gamma', display_name: 'Gamma', table_type: 'people', is_active: true, status: 'active' }, current_status: 'closed' },
  ];
}

function mountTables(list = seedTables()) {
  setActivePinia(createPinia());
  usePermissionsStore().mode = 'super';
  apiClientMock.apiRequest.mockImplementation(async (url) => {
    if (url.startsWith('/system-tables')) return { ok: true, json: async () => list };
    return { ok: true, json: async () => ({}) };
  });
  return mount(TableConstructor, {
    global: {
      stubs: {
        AdminPageShell: shellStub,
        SearchComponent: true,
        RefreshButton: true,
        BaseDropdown: true,
        TextConstructor: true,
        WorkScheduleTab: true,
        SystemTableColumnsTab: true,
        SystemTableAppearanceTab: true,
        TableConstructorCreateModal: true,
        TableConstructorPhotoSection: true,
        SystemTableHistoryModal: true,
        ConfirmationModal: true,
      },
      mocks: { $router: { push: vi.fn(), replace: vi.fn() }, $route: { query: {} } },
    },
  });
}

const rowChecks = w => w.findAll('[data-testid="systemtables-row-check"]');
const bulkBar = w => w.find('[data-testid="systemtables-bulk-bar"]');

describe('TableConstructor — групповой выбор и bulk архив/восстановление', () => {
  let wrapper;
  beforeEach(() => {
    vi.clearAllMocks();
  });
  afterEach(() => wrapper?.unmount());

  it('панель скрыта без выбора, появляется со счётчиком при выборе строки', async () => {
    wrapper = mountTables();
    await flushPromises();
    expect(bulkBar(wrapper).exists()).toBe(false);

    await rowChecks(wrapper)[0].trigger('click');
    expect(bulkBar(wrapper).exists()).toBe(true);
    expect(bulkBar(wrapper).find('.bulk-count').text()).toBe('Выбрано: 1');
    expect(wrapper.vm.selectedIds).toEqual([1]);
  });

  it('shift-клик выделяет диапазон', async () => {
    wrapper = mountTables();
    await flushPromises();
    await rowChecks(wrapper)[0].trigger('click');
    await rowChecks(wrapper)[2].trigger('click', { shiftKey: true });
    expect([...wrapper.vm.selectedIds].sort()).toEqual([1, 2, 3]);
  });

  it('select-all выбирает всех, повторный клик снимает', async () => {
    wrapper = mountTables();
    await flushPromises();
    await wrapper.find('[data-testid="systemtables-select-all"]').trigger('change');
    expect(wrapper.vm.selectedIds).toHaveLength(3);
    await wrapper.find('[data-testid="systemtables-select-all"]').trigger('change');
    expect(wrapper.vm.selectedIds).toHaveLength(0);
  });

  it('bulk-архив: подтверждение -> API с ids, полный успех -> сброс выбора', async () => {
    systemTablesApi.bulkArchiveSystemTables.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] });
    wrapper = mountTables();
    await flushPromises();

    await rowChecks(wrapper)[0].trigger('click');
    await rowChecks(wrapper)[1].trigger('click');
    await wrapper.find('[data-testid="systemtables-bulk-archive"]').trigger('click');
    expect(wrapper.vm.bulkConfirmVisible).toBe(true);

    await wrapper.vm.applyBulkArchiveRestore();
    await flushPromises();
    expect(systemTablesApi.bulkArchiveSystemTables).toHaveBeenCalledWith([1, 2]);
    expect(wrapper.vm.selectedIds).toEqual([]);
    expect(wrapper.vm.bulkConfirmVisible).toBe(false);
  });

  it('частичный успех -> ui.warning с непрошедшими, выбор сброшен', async () => {
    systemTablesApi.bulkArchiveSystemTables.mockResolvedValue({
      success_count: 1, error_count: 1, errors: [{ id: 2, name: 'Beta', error: 'привязана к организациям' }],
    });
    wrapper = mountTables();
    await flushPromises();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await rowChecks(wrapper)[0].trigger('click');
    await rowChecks(wrapper)[1].trigger('click');
    wrapper.vm.startBulkOperation('archive');
    await wrapper.vm.applyBulkArchiveRestore();
    await flushPromises();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'warning', suffix: expect.stringContaining('Beta') }));
    expect(wrapper.vm.selectedIds).toEqual([]);
  });

  it('ошибка-envelope ({message}) -> error-notify, выбор НЕ сброшен, модалка держится', async () => {
    systemTablesApi.bulkArchiveSystemTables.mockResolvedValue({ message: 'Не выбраны таблицы' });
    wrapper = mountTables();
    await flushPromises();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await rowChecks(wrapper)[0].trigger('click');
    wrapper.vm.startBulkOperation('archive');
    await wrapper.vm.applyBulkArchiveRestore();
    await flushPromises();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    expect(wrapper.vm.selectedIds).toEqual([1]);
    expect(wrapper.vm.bulkConfirmVisible).toBe(true);
  });

  it('в архивном режиме кнопка Восстановить зовёт restore-API', async () => {
    systemTablesApi.bulkRestoreSystemTables.mockResolvedValue({ success_count: 1, error_count: 0, errors: [] });
    wrapper = mountTables([
      { table: { id: 5, name: 'arch', display_name: 'Arch', table_type: 'cars', is_active: false, status: 'active' }, current_status: 'open' },
    ]);
    await flushPromises();
    await wrapper.vm.onArchiveModeChange('archive');
    await flushPromises();

    await rowChecks(wrapper)[0].trigger('click');
    expect(wrapper.find('[data-testid="systemtables-bulk-restore"]').exists()).toBe(true);
    wrapper.vm.startBulkOperation('restore');
    await wrapper.vm.applyBulkArchiveRestore();
    await flushPromises();
    expect(systemTablesApi.bulkRestoreSystemTables).toHaveBeenCalledWith([5]);
  });

  it('успешный bulk-архив открытой детали сбрасывает панель деталей', async () => {
    systemTablesApi.bulkArchiveSystemTables.mockResolvedValue({ success_count: 1, error_count: 0, errors: [] });
    wrapper = mountTables();
    await flushPromises();
    wrapper.vm.selectedTable = { table: { id: 1, name: 'alpha', display_name: 'Alpha', table_type: 'cars', is_active: true } };

    await rowChecks(wrapper)[0].trigger('click');
    wrapper.vm.startBulkOperation('archive');
    await wrapper.vm.applyBulkArchiveRestore();
    await flushPromises();
    expect(wrapper.vm.selectedTable).toBeNull();
  });

  it('переключение режима активные/архив сбрасывает групповой выбор', async () => {
    wrapper = mountTables();
    await flushPromises();
    await rowChecks(wrapper)[0].trigger('click');
    expect(wrapper.vm.selectedIds).toEqual([1]);

    await wrapper.vm.onArchiveModeChange('archive');
    await flushPromises();
    expect(wrapper.vm.selectedIds).toEqual([]);
  });
});
