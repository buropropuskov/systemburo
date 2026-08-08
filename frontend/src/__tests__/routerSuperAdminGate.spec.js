import { describe, it, expect, vi, beforeEach } from 'vitest';
import { reactive } from 'vue';

// #7: /admin/settings гейтится permission 'page.admin', который выдан и обычным
// администраторам, а бэкенд под настройками требует именно супер-права (checkSuper
// в settings_service.go). Без отдельного фронтового гейта админ без супер-права
// проходил бы route-гард и упирался в мёртвый error-state на самой странице.
// Замок проверяет: супер проходит, обычный админ с тем же page.admin - уходит
// на Forbidden, как и при любой другой нехватке прав в этом роутере.

const authState = reactive({ isAuthenticated: true, isSuperAdmin: false, userTypeCode: 'user', token: 'tok' });

vi.mock('@/stores/pdConsent', () => ({ usePDConsentStore: () => ({ refresh: vi.fn().mockResolvedValue(undefined) }) }));
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }));
vi.mock('@/stores/maintenance', () => ({ useMaintenanceStore: () => ({ enabled: false }) }));
vi.mock('@/stores/permissions', () => ({
  // page.admin разрешён всегда (как у обычного админа) - разграничивает тест
  // именно факт супер-права, а не сам permission-ключ.
  usePermissionsStore: () => ({ hasPermission: () => true, fetchPermissions: vi.fn(), isStale: false, loaded: true, banned: false }),
}));
vi.mock('@/utils/dirtyTracker', () => ({ confirmIfAnyDirty: () => Promise.resolve(true) }));

import router from '../router';

describe('router: супер-гейт /admin/settings (#7)', () => {
  beforeEach(async () => {
    authState.isAuthenticated = true;
    // Сброс на нейтральный маршрут перед каждым тестом - иначе повторный push на
    // тот же '/admin/settings' считается дублирующей навигацией и гард не
    // перевызывается, маскируя смену isSuperAdmin между тестами.
    await router.push('/personal-cabinet').catch(() => {});
  });

  it('супер-админ проходит на страницу настроек', async () => {
    authState.isSuperAdmin = true;
    await router.push('/admin/settings').catch(() => {});

    expect(router.currentRoute.value.name).toBe('AdminSettings');
  });

  it('администратор без супер-права с тем же page.admin уходит на Forbidden', async () => {
    authState.isSuperAdmin = false;
    await router.push('/admin/settings').catch(() => {});

    expect(router.currentRoute.value.name).toBe('Forbidden');
    expect(router.currentRoute.value.query.permission).toBe('page.admin');
  });
});
