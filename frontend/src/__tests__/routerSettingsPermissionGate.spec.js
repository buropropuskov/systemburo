import { describe, it, expect, vi, beforeEach } from 'vitest';
import { reactive } from 'vue';

// #7: /admin/settings гейтится точечным ключом каталога прав page.admin.settings
// (не super-only) - администраторы получают его через adminAll, а конкретному
// администратору доступ можно точечно отобрать личным deny-override. Раньше
// бэкенд требовал именно супер-права (checkSuper в settings_service.go), и фронт
// держал отдельный requiresSuperAdmin-гейт поверх permission - это отменено,
// доступ решает только сам ключ права, как у любой другой страницы.

const authState = reactive({ isAuthenticated: true, isSuperAdmin: false, userTypeCode: 'user', token: 'tok' });
let settingsAllowed = true;

vi.mock('@/stores/pdConsent', () => ({ usePDConsentStore: () => ({ refresh: vi.fn().mockResolvedValue(undefined) }) }));
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }));
vi.mock('@/stores/maintenance', () => ({ useMaintenanceStore: () => ({ enabled: false }) }));
vi.mock('@/stores/permissions', () => ({
  // Остальные ключи разрешены всегда - разграничивает тест именно page.admin.settings,
  // а не общий доступ в Админку.
  usePermissionsStore: () => ({
    hasPermission: (key) => (key === 'page.admin.settings' ? settingsAllowed : true),
    fetchPermissions: vi.fn(),
    isStale: false,
    loaded: true,
    banned: false,
  }),
}));
vi.mock('@/utils/dirtyTracker', () => ({ confirmIfAnyDirty: () => Promise.resolve(true) }));

import router from '../router';

describe('router: гейт /admin/settings правом page.admin.settings (#7)', () => {
  beforeEach(async () => {
    authState.isAuthenticated = true;
    authState.isSuperAdmin = false;
    settingsAllowed = true;
    // Сброс на нейтральный маршрут перед каждым тестом - иначе повторный push на
    // тот же '/admin/settings' считается дублирующей навигацией и гард не
    // перевызывается, маскируя смену settingsAllowed между тестами.
    await router.push('/personal-cabinet').catch(() => {});
  });

  it('администратор с правом page.admin.settings проходит на страницу настроек', async () => {
    settingsAllowed = true;
    await router.push('/admin/settings').catch(() => {});

    expect(router.currentRoute.value.name).toBe('AdminSettings');
  });

  it('администратор с личным deny-override уходит на Forbidden', async () => {
    settingsAllowed = false;
    await router.push('/admin/settings').catch(() => {});

    expect(router.currentRoute.value.name).toBe('Forbidden');
    expect(router.currentRoute.value.query.permission).toBe('page.admin.settings');
  });

  it('супер-админ проходит на страницу настроек', async () => {
    authState.isSuperAdmin = true;
    settingsAllowed = true;
    await router.push('/admin/settings').catch(() => {});

    expect(router.currentRoute.value.name).toBe('AdminSettings');
  });
});
