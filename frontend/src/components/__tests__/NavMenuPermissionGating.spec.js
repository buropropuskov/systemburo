import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NavMenu from '../NavMenu.vue';
import { useAuthStore } from '@/stores/auth';
import { getMyPermissions } from '@/api/permissions';

// Гейтинг навигации по правам (#187 Фаза 2): "нет доступа -> нет вкладки".
// Стор прав заполняется через мок getMyPermissions; auth-режим - через useAuthStore.

vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn() }));
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({ getUnreadCount: vi.fn().mockResolvedValue({ count: 0 }) }));
vi.mock('@/api/feedback', () => ({ getFeedbackStats: vi.fn().mockResolvedValue({ unread: 0 }) }));
vi.mock('@/api/permissions', () => ({ getMyPermissions: vi.fn() }));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }));

// Базовая роль "Пользователь" (грантовый набор из migrate.go::baseRoleGrants,
// видимые в навигации page.*). page.center/page.statistics в неё НЕ входят.
const BASE_KEYS = ['page.new_application', 'page.employees', 'page.cars', 'page.news', 'page.personal_cabinet'];

function permResponse(mode, keys = [], denied = []) {
  return { mode, permissions: keys.map((key) => ({ key, value: 'allow', source: 'role' })), denied, banned: false };
}

function mountNav() {
  return mount(NavMenu, {
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
        $route: { path: '/news', params: {} },
      },
    },
  });
}

let wrapper;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
});

afterEach(() => {
  wrapper?.unmount();
});

// v-show оставляет узел в DOM (display:none) -> проверяем isVisible(); v-if (admin)
// узел удаляет -> exists().
const visible = (w, testid) => {
  const el = w.find(`[data-testid="${testid}"]`);
  return el.exists() && el.isVisible();
};

describe('NavMenu: гейтинг навигации по правам', () => {
  it('super-admin видит все вкладки, включая Администрирование и Центр', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: true, canViewAccessibleAttachments: true });
    getMyPermissions.mockResolvedValue(permResponse('super', []));
    wrapper = mountNav();
    await flushPromises();

    expect(visible(wrapper, 'nav-link-center')).toBe(true);
    expect(visible(wrapper, 'nav-link-cars')).toBe(true);
    expect(visible(wrapper, 'nav-link-employees')).toBe(true);
    expect(visible(wrapper, 'nav-link-analytics')).toBe(true);
    // Отчёты - свой пункт под аналитикой: раздел не находили ни в меню, ни поиском (#2297).
    expect(visible(wrapper, 'nav-link-reports')).toBe(true);
    expect(wrapper.find('[data-testid="nav-link-admin"]').exists()).toBe(true);
  });

  it('обычный юзер с базовой ролью: свои вкладки видны, Центр/Аналитика/Админка скрыты', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false, canViewAccessibleAttachments: false });
    getMyPermissions.mockResolvedValue(permResponse('normal', BASE_KEYS));
    wrapper = mountNav();
    await flushPromises();

    // Доступное по базовой роли - видно.
    expect(visible(wrapper, 'nav-link-cars')).toBe(true);
    expect(visible(wrapper, 'nav-link-employees')).toBe(true);
    expect(visible(wrapper, 'nav-link-news')).toBe(true);
    expect(visible(wrapper, 'nav-link-cabinet')).toBe(true);
    expect(visible(wrapper, 'nav-link-new-application')).toBe(true);

    // Нет права - нет вкладки.
    expect(visible(wrapper, 'nav-link-center')).toBe(false);       // page.center не в базовой роли
    expect(visible(wrapper, 'nav-link-analytics')).toBe(false);    // page.statistics не в базовой роли
    expect(wrapper.find('[data-testid="nav-link-admin"]').exists()).toBe(false);
  });

  it('юзер без прав: вкладок нет, выход остаётся', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false, canViewAccessibleAttachments: false });
    getMyPermissions.mockResolvedValue(permResponse('normal', []));
    wrapper = mountNav();
    await flushPromises();

    expect(visible(wrapper, 'nav-link-cars')).toBe(false);
    expect(visible(wrapper, 'nav-link-center')).toBe(false);
    expect(visible(wrapper, 'nav-link-news')).toBe(false);
    expect(wrapper.find('[data-testid="nav-link-admin"]').exists()).toBe(false);
    // Выход не требует права.
    expect(visible(wrapper, 'nav-button-logout')).toBe(true);
  });
});

describe('NavMenu: гранулярность Администрирования по правам', () => {
  const groupTitles = (vm) => vm.permittedAdminGroups.map((g) => g.title);
  const groupBy = (vm, title) => vm.permittedAdminGroups.find((g) => g.title === title);

  it('page.admin.directories: видна только группа «Справочники» (11 пунктов)', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false, canViewAccessibleAttachments: false });
    getMyPermissions.mockResolvedValue(permResponse('normal', ['page.admin.directories']));
    wrapper = mountNav();
    await flushPromises();

    expect(groupTitles(wrapper.vm)).toEqual(['Справочники']);
    expect(groupBy(wrapper.vm, 'Справочники').items).toHaveLength(11);
  });

  it('page.admin.monitoring: виден «Мониторинг запросов», справочники скрыты', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false, canViewAccessibleAttachments: false });
    getMyPermissions.mockResolvedValue(permResponse('normal', ['page.admin.monitoring']));
    wrapper = mountNav();
    await flushPromises();

    const audit = groupBy(wrapper.vm, 'Аудит и связь');
    expect(audit).toBeTruthy();
    expect(audit.items.map((i) => i.label)).toContain('Мониторинг запросов');
    expect(audit.items.map((i) => i.label)).not.toContain('Обратная связь'); // нужен page.admin.feedback
    expect(groupTitles(wrapper.vm)).not.toContain('Справочники');
  });

  it('page.admin.tables_constructor: виден только «Конструктор таблиц» в группе «Система»', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false, canViewAccessibleAttachments: false });
    getMyPermissions.mockResolvedValue(permResponse('normal', ['page.admin.tables_constructor']));
    wrapper = mountNav();
    await flushPromises();

    expect(groupTitles(wrapper.vm)).toEqual(['Система']);
    expect(groupBy(wrapper.vm, 'Система').items.map((i) => i.label)).toEqual(['Конструктор таблиц']);
  });

  it('page.admin, не супер, точечный грант без page.admin.settings: Обработка данных (Система) + Руководство (Справочники), Настройки скрыты', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false, canViewAccessibleAttachments: false });
    getMyPermissions.mockResolvedValue(permResponse('normal', ['page.admin']));
    wrapper = mountNav();
    await flushPromises();

    // page.admin покрывает baseline-пункты этого ключа: «Обработка данных» (Система) и
    // «Руководство» (Справочники, гейт page.admin - бэкенд requireAdmin). Сами
    // справочники (page.admin.directories) и конструктор/мониторинг не выданы.
    // «Настройки» (#7) гейтится отдельным ключом page.admin.settings - точечный грант
    // ['page.admin'] его не включает, поэтому пункт скрыт как любой невыданный раздел.
    expect(groupTitles(wrapper.vm)).toEqual(['Справочники', 'Система']);
    expect(groupBy(wrapper.vm, 'Справочники').items.map((i) => i.label)).toEqual(['Руководство']);
    expect(groupBy(wrapper.vm, 'Система').items.map((i) => i.label)).toEqual(['Обработка данных']);
  });

  it('mode=admin (adminAll) без deny-override: Настройки видны наравне с Обработкой данных (#7)', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false, canViewAccessibleAttachments: false });
    getMyPermissions.mockResolvedValue(permResponse('admin', []));
    wrapper = mountNav();
    await flushPromises();

    expect(groupBy(wrapper.vm, 'Система').items.map((i) => i.label)).toEqual(
      expect.arrayContaining(['Настройки', 'Обработка данных']),
    );
  });

  it('mode=admin с личным deny-override на page.admin.settings: Настройки скрыты, остальное видно (#7)', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false, canViewAccessibleAttachments: false });
    getMyPermissions.mockResolvedValue(permResponse('admin', [], ['page.admin.settings']));
    wrapper = mountNav();
    await flushPromises();

    const system = groupBy(wrapper.vm, 'Система').items.map((i) => i.label);
    expect(system).not.toContain('Настройки');
    expect(system).toContain('Обработка данных');
  });

  it('супер-админ: Настройки видны', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: true, canViewAccessibleAttachments: true });
    getMyPermissions.mockResolvedValue(permResponse('super', []));
    wrapper = mountNav();
    await flushPromises();

    expect(groupBy(wrapper.vm, 'Система').items.map((i) => i.label)).toEqual(
      expect.arrayContaining(['Настройки', 'Обработка данных']),
    );
  });
});
