import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

import { useAuthStore } from '@/stores/auth';
import { usePermissionsStore } from '@/stores/permissions';
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
  { key: 'table.kpp_4.view', display_name: 'КПП №4: Доступ к таблице', category: 'Таблицы' },
  { key: 'table.kpp_4.entry', display_name: 'КПП №4: Отметка въезда/входа', category: 'Таблицы' },
  { key: 'table.kpp_4.export', display_name: 'КПП №4: Экспорт', category: 'Таблицы' },
];

const ROLES = [
  { id: 1, name: 'Пользователь', code: 'user', direct_grants: ['page.center'], default_groups: [] },
  { id: 2, name: 'Охранник', code: 'guard', direct_grants: [], default_groups: [] },
];
const GROUPS = [
  { id: 5, name: 'Аналитика', keys: ['page.statistics'] },
  { id: 6, name: 'Все таблицы', keys: [] },
];

const notify = vi.fn();
const confirm = vi.fn(async () => true);

vi.mock('@/api/permissions', () => ({
  getMyPermissions: vi.fn(async () => ({ mode: 'normal', permissions: [], denied: [] })),
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
  tryRestoreSession: vi.fn(async () => false),
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

/** JWT-подобный токен: стор читает is_super_admin из payload. */
function tokenWith(payload) {
  const body = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600, ...payload }));
  return `header.${body}.signature`;
}

/**
 * Кем открыто окно. Без аргумента -- администратор: именно он открывает окно по
 * permission.audit.manage и именно у него стор отвечает «да» на super-only ключи.
 */
function openedBy({ superAdmin = false, allow = [] } = {}) {
  const perms = usePermissionsStore();
  useAuthStore().token = superAdmin ? tokenWith({ is_super_admin: true }) : tokenWith({});
  perms.mode = superAdmin ? 'super' : (allow.length ? 'normal' : 'admin');
  perms.effective = Object.fromEntries(allow.map((k) => [k, { value: 'allow', source: 'role' }]));
}

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
    setActivePinia(createPinia());
    openedBy({ superAdmin: true });
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

  it('смена роли сразу пересчитывает наследованные тумблеры (#867 UX)', async () => {
    const wrapper = await mountModal();
    // Роль 1 (Пользователь) грантит page.center -> тумблер включён.
    expect(wrapper.vm.stateByKey['page.center']).toMatchObject({ on: true, source: 'role' });
    // Меняем роль на 2 (Охранник, без грантов) через дропдаун - тумблер обновляется
    // сразу, без переоткрытия (как в браузере: v-model -> watch -> recomputeInherited).
    wrapper.findComponent('[data-testid="role-select"]').vm.$emit('update:model-value', 2);
    await flushPromises();
    expect(wrapper.vm.stateByKey['page.center']).toMatchObject({ on: false, source: null });
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

  it('права таблицы собраны во второй уровень со счётчиком', async () => {
    const wrapper = await mountModal();
    const group = wrapper.get('[data-table="kpp_4"]');
    expect(group.get('.ep-group__name').text()).toBe('КПП №4');
    expect(group.get('.ep-group__count').text()).toBe('0 из 3');
    expect(group.get('.ep-group__toggle').attributes('aria-expanded')).toBe('false');
  });

  it('поиск сужает дерево и раскрывает найденное внутри таблицы', async () => {
    const wrapper = await mountModal();
    const search = wrapper.get('[data-testid="user-permissions-search"]');

    await search.setValue('экспорт');
    expect(wrapper.find('[data-key="page.cars"]').exists()).toBe(false);
    expect(wrapper.find('[data-key="table.kpp_4.export"]').exists()).toBe(true);
    // Совпадение внутри свёрнутой таблицы раскрывается принудительно.
    expect(wrapper.get('[data-table="kpp_4"]').get('.ep-group__toggle').attributes('aria-expanded')).toBe('true');

    await search.setValue('');
    expect(wrapper.find('[data-key="page.cars"]').exists()).toBe(true);
  });

  it('поиск не теряет override спрятанного права при сохранении', async () => {
    const wrapper = await mountModal();
    wrapper.vm.onToggleKey('page.cars');
    await wrapper.vm.$nextTick();

    // Право ушло из выдачи поиска, но состояние считается по полному каталогу.
    await wrapper.get('[data-testid="user-permissions-search"]').setValue('экспорт');
    expect(wrapper.find('[data-key="page.cars"]').exists()).toBe(false);
    expect(wrapper.vm.stateByKey['page.cars']).toMatchObject({ on: true, source: 'override' });

    await wrapper.find('[data-testid="save-button"]').trigger('click');
    await flushPromises();
    expect(updateUserPermissions).toHaveBeenCalledWith(42, { permissions: [{ key: 'page.cars', value: 'allow' }] });
  });

  it('для супер-админа флаг и блокировка заблокированы, права readonly-on', async () => {
    getUserEffectivePermissions.mockResolvedValue({
      mode: 'super', permissions: [], denied: [], banned: false, ban_reason: '',
    });
    const wrapper = await mountModal({ ...USER, is_super_admin: true });

    expect(wrapper.find('[data-testid="admin-toggle"]').attributes('disabled')).toBeDefined();
    const banBtn = wrapper.find('[data-testid="ban-button"]');
    expect(banBtn.attributes('disabled')).toBeDefined();
    // Короткая надпись, влезающая в кнопку; полный текст -- в title-тултипе.
    expect(banBtn.text()).toBe('Невозможно');
    expect(banBtn.attributes('title')).toBe('Супер-администратора заблокировать нельзя');
    expect(wrapper.vm.stateByKey['page.cars']).toMatchObject({ on: true, locked: true });
    // Замок по целевому пользователю держится и у того, кому выдавать признак можно.
    expect(wrapper.get('[data-testid="admin-toggle-lock-reason"]').text())
      .toBe('У супер-администратора и так все права');
  });

  // action.grant.admin -- super-only, и стор в режиме admin отвечает на такие ключи
  // «да»: до #1983 тумблер выглядел рабочим, а отказ приходил на сохранении.
  describe('тумблер Администратор гейтится action.grant.admin (#1983)', () => {
    it('администратор без права: тумблер виден, но не двигается, и рядом сказано почему', async () => {
      openedBy();
      const wrapper = await mountModal();
      const toggle = wrapper.get('[data-testid="admin-toggle"]');

      expect(toggle.attributes('disabled')).toBeDefined();
      expect(toggle.attributes('title')).toBe('Выдать признак может только Системный администратор');
      expect(wrapper.get('[data-testid="admin-toggle-lock-reason"]').text())
        .toBe('Выдать признак может только Системный администратор');

      await toggle.trigger('click');
      expect(wrapper.vm.localIsAdmin).toBe(false);

      await wrapper.get('[data-testid="save-button"]').trigger('click');
      await flushPromises();
      expect(setUserAdmin).not.toHaveBeenCalled();
    });

    it('обычный пользователь с управлением правами: тумблер тоже закрыт', async () => {
      openedBy({ allow: ['permission.audit.manage'] });
      const wrapper = await mountModal();

      expect(wrapper.get('[data-testid="admin-toggle"]').attributes('disabled')).toBeDefined();
      expect(wrapper.find('[data-testid="admin-toggle-lock-reason"]').exists()).toBe(true);
    });

    it('системный администратор: тумблер двигается, пояснения нет', async () => {
      openedBy({ superAdmin: true });
      const wrapper = await mountModal();
      const toggle = wrapper.get('[data-testid="admin-toggle"]');

      expect(toggle.attributes('disabled')).toBeUndefined();
      expect(wrapper.find('[data-testid="admin-toggle-lock-reason"]').exists()).toBe(false);

      await toggle.trigger('click');
      expect(wrapper.vm.localIsAdmin).toBe(true);

      await wrapper.get('[data-testid="save-button"]').trigger('click');
      await flushPromises();
      expect(setUserAdmin).toHaveBeenCalledWith(42, true);
    });
  });
});
