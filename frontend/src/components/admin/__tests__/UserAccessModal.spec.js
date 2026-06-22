import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

import UserAccessModal from '../UserAccessModal.vue';
import {
  getUserEffectivePermissions,
  setUserAdmin,
  setUserRole,
  assignGroupToUser,
  updateUserPermissions,
  banUser,
} from '@/api/permissions';

const CATALOG = [
  { key: 'page.center', display_name: 'Центр заявок', category: 'Навигация' },
  { key: 'page.cars', display_name: 'Автомобили', category: 'Навигация' },
  { key: 'page.statistics', display_name: 'Аналитика', category: 'Навигация' },
  { key: 'action.grant.admin', display_name: 'Выдача прав администратора', category: 'Администрирование', super_only: true },
];

const ROLES = [{ id: 1, name: 'Пользователь', code: 'user' }, { id: 2, name: 'Охранник', code: 'guard' }];
const GROUPS = [
  { id: 5, name: 'Аналитика', keys: ['page.statistics'] },
  { id: 6, name: 'Все таблицы', keys: [] },
];

const notify = vi.fn();
const confirm = vi.fn(async () => true);

vi.mock('@/api/permissions', () => ({
  getPermissionCatalog: vi.fn(async () => CATALOG),
  getUserEffectivePermissions: vi.fn(async () => ({
    mode: 'normal',
    permissions: [{ key: 'page.center', value: 'allow', source: 'role' }],
    denied: [],
    banned: false,
    ban_reason: '',
  })),
  getUserPermissions: vi.fn(async () => []),
  updateUserPermissions: vi.fn(async () => ({})),
  listRoles: vi.fn(async () => ROLES),
  listPermissionGroups: vi.fn(async () => GROUPS),
  assignGroupToUser: vi.fn(async () => ({})),
  unassignGroupFromUser: vi.fn(async () => ({})),
  setUserAdmin: vi.fn(async () => ({})),
  setUserRole: vi.fn(async () => ({})),
  banUser: vi.fn(async () => ({})),
  unbanUser: vi.fn(async () => ({})),
}));

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(async () => ({ ok: true, json: async () => [] })),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}));

vi.mock('@/stores/ui', () => ({
  useUiStore: () => ({ confirm }),
}));

const USER = {
  id: 42,
  username: 'ivanov',
  user_type: 'Арендатор',
  organization: 'ООО «Логистик»',
  role_id: 1,
  is_admin: false,
  is_super_admin: false,
  is_banned: false,
  last_name: 'Иванов',
  first_name: 'Иван',
  middle_name: 'Иванович',
};

async function mountModal(user = USER) {
  const wrapper = mount(UserAccessModal, {
    props: { user },
    global: { stubs: { teleport: true, LoaderSpinner: true } },
  });
  await flushPromises();
  return wrapper;
}

describe('UserAccessModal (#187 Фаза 3, две колонки)', () => {
  beforeEach(() => {
    notify.mockClear();
    confirm.mockClear();
    [setUserAdmin, setUserRole, assignGroupToUser, updateUserPermissions, banUser].forEach((f) => f.mockClear());
    getUserEffectivePermissions.mockResolvedValue({
      mode: 'normal',
      permissions: [{ key: 'page.center', value: 'allow', source: 'role' }],
      denied: [],
      banned: false,
      ban_reason: '',
    });
  });

  it('рендерит шапку, две колонки и эффективные права с источником', async () => {
    const wrapper = await mountModal();

    expect(wrapper.find('.access-modal__title').text()).toContain('Иванов Иван Иванович');
    expect(wrapper.find('.access-modal__sub').text()).toContain('@ivanov');
    expect(wrapper.find('.access-modal__sub').text()).toContain('Арендатор');
    expect(wrapper.find('.col-left').exists()).toBe(true);
    expect(wrapper.find('.col-right').exists()).toBe(true);

    // Право от роли -- включено с бейджем «роль»; не выданное -- выключено.
    expect(wrapper.vm.stateByKey['page.center']).toMatchObject({ on: true, source: 'role' });
    expect(wrapper.vm.stateByKey['page.cars']).toMatchObject({ on: false, source: null });
    // super-only заблокировано для не-супера.
    expect(wrapper.vm.stateByKey['action.grant.admin'].locked).toBe(true);
  });

  it('включение флага Администратор делает все не-super права on и сохраняется через setUserAdmin', async () => {
    const wrapper = await mountModal();
    await wrapper.find('[data-testid="admin-toggle"]').trigger('click');

    expect(wrapper.vm.localIsAdmin).toBe(true);
    expect(wrapper.vm.stateByKey['page.cars']).toMatchObject({ on: true, source: 'admin' });
    // super-only остаётся выключенным даже у админа.
    expect(wrapper.vm.stateByKey['action.grant.admin'].on).toBe(false);

    await wrapper.find('[data-testid="save-button"]').trigger('click');
    await flushPromises();
    expect(setUserAdmin).toHaveBeenCalledWith(42, true);
  });

  it('тумблер права создаёт личный override и сохраняется через updateUserPermissions', async () => {
    const wrapper = await mountModal();
    wrapper.vm.onToggleKey('page.cars');
    await wrapper.vm.$nextTick();

    expect(wrapper.vm.stateByKey['page.cars']).toMatchObject({ on: true, source: 'override' });

    await wrapper.find('[data-testid="save-button"]').trigger('click');
    await flushPromises();
    expect(updateUserPermissions).toHaveBeenCalledWith(42, { permissions: [{ key: 'page.cars', value: 'allow' }] });
  });

  it('смена роли и добавление группы уходят в setUserRole и assignGroupToUser', async () => {
    const wrapper = await mountModal();
    wrapper.vm.form.role_id = 2;
    wrapper.vm.addGroup(5);
    await wrapper.vm.$nextTick();

    await wrapper.find('[data-testid="save-button"]').trigger('click');
    await flushPromises();

    expect(setUserRole).toHaveBeenCalledWith(42, 2);
    expect(assignGroupToUser).toHaveBeenCalledWith(42, 5);
  });

  it('без изменений save не дёргает мутирующие эндпоинты', async () => {
    const wrapper = await mountModal();
    await wrapper.find('[data-testid="save-button"]').trigger('click');
    await flushPromises();

    expect(setUserAdmin).not.toHaveBeenCalled();
    expect(setUserRole).not.toHaveBeenCalled();
    expect(updateUserPermissions).not.toHaveBeenCalled();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'права доступа' }));
  });

  it('блокировка -- отдельное немедленное действие с причиной, не часть save', async () => {
    const wrapper = await mountModal();
    wrapper.vm.banReasonInput = 'нарушение пропускного режима';
    await wrapper.find('[data-testid="ban-button"]').trigger('click');
    await flushPromises();

    expect(confirm).toHaveBeenCalled();
    expect(banUser).toHaveBeenCalledWith(42, 'нарушение пропускного режима');
    expect(wrapper.emitted('updated')).toHaveLength(1);
  });

  it('для супер-админа флаг и блокировка заблокированы, права readonly-on', async () => {
    getUserEffectivePermissions.mockResolvedValue({
      mode: 'super', permissions: [], denied: [], banned: false, ban_reason: '',
    });
    const wrapper = await mountModal({ ...USER, is_super_admin: true });

    expect(wrapper.find('[data-testid="admin-toggle"]').attributes('disabled')).toBeDefined();
    expect(wrapper.find('[data-testid="ban-button"]').attributes('disabled')).toBeDefined();
    expect(wrapper.vm.stateByKey['page.cars']).toMatchObject({ on: true, locked: true });
  });
});
