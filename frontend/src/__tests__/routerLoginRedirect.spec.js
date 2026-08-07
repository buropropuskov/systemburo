import { describe, it, expect, vi, beforeEach } from 'vitest';
import { reactive } from 'vue';

// #974: push-уведомление приводит человека на защищённый адрес спустя дни,
// когда сессия давно протухла. Гард раньше делал next('/') без цели - после
// входа человек всегда попадал на дефолтную ленту, а не на заявку из
// уведомления. Замок проверяет, что адрес сохраняется в query и переживает
// навигацию до формы входа.

const refresh = vi.fn().mockResolvedValue(undefined);
const authState = reactive({ isAuthenticated: false, isSuperAdmin: false, userTypeCode: 'user', token: null });

vi.mock('@/stores/pdConsent', () => ({ usePDConsentStore: () => ({ refresh }) }));
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }));
vi.mock('@/stores/maintenance', () => ({ useMaintenanceStore: () => ({ enabled: false }) }));
vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ hasPermission: () => true, fetchPermissions: vi.fn(), isStale: false, loaded: true }),
}));
vi.mock('@/utils/dirtyTracker', () => ({ confirmIfAnyDirty: () => Promise.resolve(true) }));

import router from '../router';

describe('router: возврат на защищённый адрес после входа (#974)', () => {
  beforeEach(() => {
    refresh.mockClear();
    authState.isAuthenticated = false;
  });

  it('сохраняет исходный адрес в query.redirect при уводе на вход', async () => {
    await router.push('/table/cars').catch(() => {});

    expect(router.currentRoute.value.path).toBe('/');
    expect(router.currentRoute.value.query.redirect).toBe('/table/cars');
  });

  it('сохраняет query исходного адреса тоже', async () => {
    await router.push('/personal-cabinet?tab=cars').catch(() => {});

    expect(router.currentRoute.value.query.redirect).toBe('/personal-cabinet?tab=cars');
  });

  it('на вход без защищённого перехода (обычный заход) query.redirect не появляется', async () => {
    authState.isAuthenticated = false;
    await router.push('/').catch(() => {});

    expect(router.currentRoute.value.query.redirect).toBeUndefined();
  });
});
