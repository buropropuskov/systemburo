import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const { notifyMock } = vi.hoisted(() => ({ notifyMock: vi.fn() }));

vi.mock('@/api/permissions', () => ({
  listPermissionGroups: vi.fn(),
  getPermissionGroup: vi.fn(),
  createPermissionGroup: vi.fn(),
  updatePermissionGroup: vi.fn(),
  deletePermissionGroup: vi.fn(),
  getPermissionCatalog: vi.fn(),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify: notifyMock }),
}));

vi.mock('@/utils/dirtyTracker', () => ({
  registerDirtyTracker: () => () => {},
  confirmIfAnyDirty: () => Promise.resolve(true),
}));

vi.mock('@/composables/useOverlayClose', () => ({
  useOverlayClose: () => ({ onOverlayMousedown: () => {}, onOverlayMouseup: () => {} }),
}));

import AdminPermissionGroups from '../AdminPermissionGroups.vue';
import {
  listPermissionGroups,
  getPermissionGroup,
  createPermissionGroup,
  updatePermissionGroup,
  deletePermissionGroup,
  getPermissionCatalog,
} from '@/api/permissions';

const GROUPS = [
  { id: 10, name: 'Все таблицы', description: 'Доступ ко всем таблицам', keys: ['table.a', 'table.b'] },
  { id: 11, name: 'Только чтение', description: null, keys: ['read.x'] },
  { id: 12, name: 'Пустая', description: null, keys: [] },
];

const CATALOG = [
  { key: 'table', display_name: 'Таблицы', children: [{ key: 'table.a', display_name: 'A' }] },
];

const stubs = {
  AdminPageShell: { template: '<div><slot /></div>' },
  SearchComponent: true,
  RefreshButton: true,
  ConfirmationModal: true,
  LoaderSpinner: true,
  GroupPermissionsModal: true,
};

function mountGroups() {
  return mount(AdminPermissionGroups, {
    global: {
      stubs,
      directives: { 'permission-scope': {} },
    },
  });
}

async function mountReady() {
  const wrapper = mountGroups();
  await flushPromises();
  return wrapper;
}

describe('AdminPermissionGroups master-detail', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listPermissionGroups.mockResolvedValue(GROUPS);
    getPermissionCatalog.mockResolvedValue(CATALOG);
    getPermissionGroup.mockResolvedValue({ id: 10, name: 'Все таблицы', description: 'Доступ ко всем таблицам', keys: ['table.a', 'table.b'] });
    createPermissionGroup.mockResolvedValue({ id: 99, name: 'Копия: Все таблицы', description: 'Доступ ко всем таблицам', keys: ['table.a', 'table.b'] });
    updatePermissionGroup.mockResolvedValue({ updated: true });
    deletePermissionGroup.mockResolvedValue({ deleted: true });
  });

  it('рендерит список групп и футер «Всего: N»', async () => {
    const wrapper = await mountReady();
    expect(wrapper.findAll('[data-testid="group-row"]')).toHaveLength(3);
    expect(wrapper.find('.items-count').text()).toBe('Всего: 3');
    // Правая панель до выбора — заглушка
    expect(wrapper.find('.no-selection-message').exists()).toBe(true);
    expect(wrapper.find('[data-testid="group-details"]').exists()).toBe(false);
  });

  it('выбор группы открывает детали с наименованием и счётчиком прав', async () => {
    const wrapper = await mountReady();
    await wrapper.findAll('[data-testid="group-row"]')[0].trigger('click');
    await flushPromises();
    expect(wrapper.find('[data-testid="group-details"]').exists()).toBe(true);
    // Отсортировано по имени (нет sortField): «Все таблицы» первым
    expect(wrapper.find('.details-title').text()).toBe('Все таблицы');
    expect(wrapper.find('[data-testid="group-keys-count"]').text()).toBe('Выбрано прав: 2');
    expect(wrapper.vm.selectedKeys).toEqual(['table.a', 'table.b']);
  });

  it('openCopy предзаполняет модалку «Копия: X», описание и права источника', async () => {
    const wrapper = await mountReady();
    await wrapper.vm.selectGroup(GROUPS[0]);
    wrapper.vm.openCopy(wrapper.vm.selectedGroup);
    expect(wrapper.vm.modalMode).toBe('copy');
    expect(wrapper.vm.metaForm.name).toBe('Копия: Все таблицы');
    expect(wrapper.vm.metaForm.description).toBe('Доступ ко всем таблицам');
    expect(wrapper.vm.copySourceKeys).toEqual(['table.a', 'table.b']);
  });

  it('submitMeta в режиме copy берёт свежие права источника и создаёт группу одним вызовом', async () => {
    getPermissionGroup.mockResolvedValueOnce({ id: 10, keys: ['table.a', 'table.b', 'extra'] });
    const wrapper = await mountReady();
    await wrapper.vm.selectGroup(GROUPS[0]);
    wrapper.vm.openCopy(wrapper.vm.selectedGroup);
    await wrapper.vm.submitMeta();
    await flushPromises();

    expect(getPermissionGroup).toHaveBeenCalledWith(10);
    expect(createPermissionGroup).toHaveBeenCalledWith({
      name: 'Копия: Все таблицы',
      description: 'Доступ ко всем таблицам',
      keys: ['table.a', 'table.b', 'extra'],
    });
    expect(notifyMock).toHaveBeenCalledWith(
      expect.objectContaining({ bold: 'Копия: Все таблицы', suffix: ' создана как копия' }),
    );
    expect(wrapper.vm.showMetaModal).toBe(false);
  });

  it('копия пустой группы создаётся с пустым keys (валидация required не падает)', async () => {
    getPermissionGroup.mockResolvedValueOnce({ id: 12, keys: [] });
    const wrapper = await mountReady();
    await wrapper.vm.selectGroup(GROUPS[2]); // Пустая
    wrapper.vm.openCopy(wrapper.vm.selectedGroup);
    await wrapper.vm.submitMeta();
    await flushPromises();

    expect(createPermissionGroup).toHaveBeenCalledWith(
      expect.objectContaining({ keys: [] }),
    );
  });

  it('создание новой группы не дёргает getPermissionGroup, создаёт с keys=[] и открывает редактор прав', async () => {
    createPermissionGroup.mockResolvedValueOnce({ id: 77, name: 'Кураторы', description: null, keys: [] });
    listPermissionGroups
      .mockResolvedValueOnce(GROUPS)
      .mockResolvedValueOnce([...GROUPS, { id: 77, name: 'Кураторы', description: null, keys: [] }]);
    const wrapper = await mountReady();
    wrapper.vm.openCreate();
    wrapper.vm.metaForm = { name: 'Кураторы', description: '' };
    await wrapper.vm.submitMeta();
    await flushPromises();

    expect(getPermissionGroup).not.toHaveBeenCalled();
    expect(createPermissionGroup).toHaveBeenCalledWith({ name: 'Кураторы', description: null, keys: [] });
    expect(wrapper.vm.modalMode).toBe('create');
    // Новую группу сразу открыли на настройку прав
    expect(wrapper.vm.permsModal.show).toBe(true);
  });

  it('saveSelected сохраняет мету с текущими правами группы', async () => {
    const wrapper = await mountReady();
    // Список отсортирован по имени: row[0] = «Все таблицы» (id 10)
    await wrapper.findAll('[data-testid="group-row"]')[0].trigger('click');
    await flushPromises();
    await wrapper.find('[data-testid="group-detail-name"]').setValue('Все таблицы v2');
    expect(wrapper.vm.isDetailsDirty).toBe(true);
    await wrapper.vm.saveSelected();
    await flushPromises();

    expect(updatePermissionGroup).toHaveBeenCalledWith(10, {
      name: 'Все таблицы v2',
      description: 'Доступ ко всем таблицам',
      keys: ['table.a', 'table.b'],
    });
  });

  it('handleSavePermissions сохраняет новый набор прав и обновляет счётчик', async () => {
    const wrapper = await mountReady();
    await wrapper.vm.selectGroup(GROUPS[0]);
    // После сохранения сервер отдаёт обновлённый набор прав
    listPermissionGroups.mockResolvedValueOnce([{ ...GROUPS[0], keys: ['table.a'] }, GROUPS[1], GROUPS[2]]);
    await wrapper.vm.handleSavePermissions(['table.a']);
    await flushPromises();

    expect(updatePermissionGroup).toHaveBeenCalledWith(10, {
      name: 'Все таблицы',
      description: 'Доступ ко всем таблицам',
      keys: ['table.a'],
    });
    expect(wrapper.vm.selectedKeys).toEqual(['table.a']);
    expect(notifyMock).toHaveBeenCalledWith(
      expect.objectContaining({ bold: 'Все таблицы', suffix: ' сохранены' }),
    );
  });

  it('submitMeta показывает ошибку из envelope при неуспешном создании', async () => {
    createPermissionGroup.mockResolvedValueOnce({ message: 'Название уже занято' });
    const wrapper = await mountReady();
    wrapper.vm.openCreate();
    wrapper.vm.metaForm = { name: 'Дубль', description: '' };
    await wrapper.vm.submitMeta();
    await flushPromises();

    expect(wrapper.vm.metaError).toBe('Название уже занято');
    expect(wrapper.vm.showMetaModal).toBe(true); // не закрылась
  });

  it('поиск группы находит по вводу в EN-раскладке (common util вместо плоского includes)', async () => {
    const wrapper = await mountReady();
    // "dct" на физических клавишах = "все" (группа "Все таблицы").
    wrapper.vm.searchQuery = 'dct';
    await flushPromises();

    const rows = wrapper.findAll('[data-testid="group-row"]');
    expect(rows).toHaveLength(1);
    expect(rows[0].text()).toContain('Все таблицы');
  });

  it('performDelete удаляет группу и уведомляет', async () => {
    const wrapper = await mountReady();
    await wrapper.vm.selectGroup(GROUPS[0]);
    wrapper.vm.deleteConfirm = { id: 10, name: 'Все таблицы' };
    await wrapper.vm.performDelete();
    await flushPromises();

    expect(deletePermissionGroup).toHaveBeenCalledWith(10);
    expect(notifyMock).toHaveBeenCalledWith(
      expect.objectContaining({ bold: 'Все таблицы', suffix: ' удалена' }),
    );
  });
});
