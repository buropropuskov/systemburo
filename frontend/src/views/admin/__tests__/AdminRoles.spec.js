import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const { notifyMock } = vi.hoisted(() => ({ notifyMock: vi.fn() }));

vi.mock('@/api/permissions', () => ({
  listRoles: vi.fn(),
  listPermissionGroups: vi.fn(),
  getPermissionCatalog: vi.fn(),
  createRole: vi.fn(),
  updateRole: vi.fn(),
  deleteRole: vi.fn(),
  setRoleDefaultGroups: vi.fn(),
  setRolePermissions: vi.fn(),
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

import AdminRoles from '../AdminRoles.vue';
import {
  listRoles,
  listPermissionGroups,
  getPermissionCatalog,
  createRole,
  updateRole,
  deleteRole,
  setRoleDefaultGroups,
  setRolePermissions,
} from '@/api/permissions';

const ROLES = [
  {
    id: 1,
    code: 'tenant',
    name: 'Арендатор',
    description: 'Права арендатора',
    default_groups: [{ id: 10, name: 'G1' }, { id: 11, name: 'G2' }],
    direct_grants: ['page.center'],
  },
  { id: 2, code: 'guard', name: 'Охранник', description: null, default_groups: [] },
];

const CATALOG = [
  { key: 'page.center', display_name: 'Центр заявок', category: 'Навигация' },
];

const GROUPS = [
  { id: 10, name: 'G1', keys: ['a', 'b'] },
  { id: 11, name: 'G2', keys: ['c'] },
  { id: 12, name: 'G3', keys: [] },
];

const stubs = {
  AdminPageShell: { template: '<div><slot /></div>' },
  SearchComponent: true,
  RefreshButton: true,
  ConfirmationModal: true,
  LoaderSpinner: true,
  RolePermissionsModal: true,
};

function mountRoles() {
  return mount(AdminRoles, {
    global: {
      stubs,
      directives: { 'permission-scope': {} },
    },
  });
}

async function mountReady() {
  const wrapper = mountRoles();
  await flushPromises();
  return wrapper;
}

describe('AdminRoles master-detail', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listRoles.mockResolvedValue(ROLES);
    listPermissionGroups.mockResolvedValue(GROUPS);
    getPermissionCatalog.mockResolvedValue(CATALOG);
    createRole.mockResolvedValue({ id: 99, code: 'tenant_copy', name: 'Копия: Арендатор', description: 'Права арендатора' });
    updateRole.mockResolvedValue({ updated: true });
    deleteRole.mockResolvedValue({ deleted: true });
    setRoleDefaultGroups.mockResolvedValue({ updated: true });
    setRolePermissions.mockResolvedValue({ updated: true });
  });

  it('рендерит список ролей и футер «Всего: N»', async () => {
    const wrapper = await mountReady();
    expect(wrapper.findAll('[data-testid="role-row"]')).toHaveLength(2);
    expect(wrapper.find('.items-count').text()).toBe('Всего: 2');
    // Правая панель до выбора — заглушка
    expect(wrapper.find('.no-selection-message').exists()).toBe(true);
    expect(wrapper.find('[data-testid="role-details"]').exists()).toBe(false);
  });

  it('выбор роли открывает детали и загружает её группы и точечные права', async () => {
    const wrapper = await mountReady();
    await wrapper.findAll('[data-testid="role-row"]')[0].trigger('click');
    await flushPromises();
    expect(wrapper.find('[data-testid="role-details"]').exists()).toBe(true);
    expect(wrapper.find('.details-title').text()).toBe('Арендатор');
    // Группы и собственные гранты роли подгружены из ответа listRoles
    expect(wrapper.vm.currentGroupIds).toEqual([10, 11]);
    expect(wrapper.vm.currentDirectKeys).toEqual(['page.center']);
  });

  it('openCopy предзаполняет модалку «Копия: X», код и описание источника', async () => {
    const wrapper = await mountReady();
    await wrapper.vm.selectRole(ROLES[0]);
    wrapper.vm.openCopy(wrapper.vm.selectedRole);
    expect(wrapper.vm.modalMode).toBe('copy');
    expect(wrapper.vm.metaForm.name).toBe('Копия: Арендатор');
    expect(wrapper.vm.metaForm.code).toBe('tenant_copy');
    expect(wrapper.vm.metaForm.description).toBe('Права арендатора');
    expect(wrapper.vm.copySourceGroupIds).toEqual([10, 11]);
  });

  it('submitMeta в режиме copy создаёт роль и переносит дефолтные группы источника', async () => {
    const wrapper = await mountReady();
    await wrapper.vm.selectRole(ROLES[0]);
    wrapper.vm.openCopy(wrapper.vm.selectedRole);
    await wrapper.vm.submitMeta();
    await flushPromises();

    expect(createRole).toHaveBeenCalledWith({
      name: 'Копия: Арендатор',
      code: 'tenant_copy',
      description: 'Права арендатора',
    });
    expect(setRoleDefaultGroups).toHaveBeenCalledWith(99, [10, 11]);
    expect(notifyMock).toHaveBeenCalledWith(
      expect.objectContaining({ bold: 'Копия: Арендатор', suffix: ' создана как копия' }),
    );
    // Модалка закрылась
    expect(wrapper.vm.showMetaModal).toBe(false);
  });

  it('копия роли без групп не дёргает setRoleDefaultGroups', async () => {
    const wrapper = await mountReady();
    await wrapper.vm.selectRole(ROLES[1]); // Охранник, групп нет
    wrapper.vm.openCopy(wrapper.vm.selectedRole);
    expect(wrapper.vm.copySourceGroupIds).toEqual([]);
    await wrapper.vm.submitMeta();
    await flushPromises();

    expect(createRole).toHaveBeenCalledTimes(1);
    expect(setRoleDefaultGroups).not.toHaveBeenCalled();
  });

  it('создание новой роли не переносит группы (setRoleDefaultGroups не вызывается)', async () => {
    const wrapper = await mountReady();
    wrapper.vm.openCreate();
    wrapper.vm.metaForm = { name: 'Куратор', code: 'curator', description: '' };
    await wrapper.vm.submitMeta();
    await flushPromises();

    expect(createRole).toHaveBeenCalledWith({ name: 'Куратор', code: 'curator', description: null });
    expect(setRoleDefaultGroups).not.toHaveBeenCalled();
    expect(wrapper.vm.modalMode).toBe('create');
  });

  it('saveSelected сохраняет только мету роли (updateRole), группы не трогает', async () => {
    const wrapper = await mountReady();
    await wrapper.findAll('[data-testid="role-row"]')[0].trigger('click');
    await flushPromises();
    await wrapper.find('[data-testid="role-detail-name"]').setValue('Арендатор+');
    await wrapper.find('[data-testid="role-save"]').trigger('click');
    await flushPromises();

    expect(updateRole).toHaveBeenCalledWith(1, { name: 'Арендатор+', description: 'Права арендатора' });
    expect(setRoleDefaultGroups).not.toHaveBeenCalled();
    expect(setRolePermissions).not.toHaveBeenCalled();
  });

  it('handleSavePerms шлёт только изменённое: точечные права и группы раздельно', async () => {
    const wrapper = await mountReady();
    await wrapper.vm.selectRole(ROLES[0]); // группы [10,11], гранты ['page.center']

    // Меняются только точечные права -> зовётся setRolePermissions, группы не трогаются.
    await wrapper.vm.handleSavePerms({ directKeys: ['page.center', 'page.cars'], groupIds: [10, 11] });
    await flushPromises();
    expect(setRolePermissions).toHaveBeenCalledWith(1, ['page.center', 'page.cars']);
    expect(setRoleDefaultGroups).not.toHaveBeenCalled();
  });

  it('handleSavePerms при изменении только групп не трогает точечные права (инвариант)', async () => {
    const wrapper = await mountReady();
    await wrapper.vm.selectRole(ROLES[0]); // группы [10,11], гранты ['page.center']

    // Добавили группу, точечные права те же -> setRolePermissions НЕ вызывается.
    await wrapper.vm.handleSavePerms({ directKeys: ['page.center'], groupIds: [10, 11, 12] });
    await flushPromises();
    expect(setRoleDefaultGroups).toHaveBeenCalledWith(1, [10, 11, 12]);
    expect(setRolePermissions).not.toHaveBeenCalled();
  });

  it('submitMeta показывает ошибку из envelope при неуспешном создании', async () => {
    createRole.mockResolvedValueOnce({ message: 'Код уже занят' });
    const wrapper = await mountReady();
    wrapper.vm.openCreate();
    wrapper.vm.metaForm = { name: 'Дубль', code: 'tenant', description: '' };
    await wrapper.vm.submitMeta();
    await flushPromises();

    expect(wrapper.vm.metaError).toBe('Код уже занят');
    expect(setRoleDefaultGroups).not.toHaveBeenCalled();
    expect(wrapper.vm.showMetaModal).toBe(true); // не закрылась
  });

  it('поиск роли находит по вводу в EN-раскладке (common util вместо плоского includes)', async () => {
    const wrapper = await mountReady();
    // "fhtylfnjh" на физических клавишах = "арендатор" (роль "Арендатор").
    wrapper.vm.searchQuery = 'fhtylfnjh';
    await flushPromises();

    const rows = wrapper.findAll('[data-testid="role-row"]');
    expect(rows).toHaveLength(1);
    expect(rows[0].text()).toContain('Арендатор');
  });

  it('performDelete удаляет роль и уведомляет', async () => {
    const wrapper = await mountReady();
    await wrapper.vm.selectRole(ROLES[0]);
    wrapper.vm.deleteConfirm = { id: 1, name: 'Арендатор' };
    await wrapper.vm.performDelete();
    await flushPromises();

    expect(deleteRole).toHaveBeenCalledWith(1);
    expect(notifyMock).toHaveBeenCalledWith(
      expect.objectContaining({ bold: 'Арендатор', suffix: ' удалена' }),
    );
  });
});
