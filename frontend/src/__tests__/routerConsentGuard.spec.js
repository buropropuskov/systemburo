import { describe, it, expect, vi, beforeEach } from 'vitest';
import { reactive } from 'vue';

// #1567: навигационный гард обязан дождаться состояния гейта согласия ДО того,
// как смонтируется защищённая страница. Без этого ожидания страница успевает
// запросить свои данные, все запросы получают отказ, а окно согласия встаёт
// поверх уже нарисованного интерфейса (поймано приёмкой на стенде).
//
// Правка бьёт по КАЖДОЙ навигации всех пользователей, поэтому замок нужен в
// обычном прогоне: e2e-спека гейта включается только явным флагом и в CI молчит.

const refresh = vi.fn().mockResolvedValue(undefined);
const authState = reactive({ isAuthenticated: true, isSuperAdmin: false, userTypeCode: 'user', token: 'tok' });

vi.mock('@/stores/pdConsent', () => ({ usePDConsentStore: () => ({ refresh }) }));
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }));
vi.mock('@/stores/maintenance', () => ({ useMaintenanceStore: () => ({ enabled: false }) }));
vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ hasPermission: () => true, fetchPermissions: vi.fn(), isStale: false, loaded: true }),
}));
vi.mock('@/utils/dirtyTracker', () => ({ confirmIfAnyDirty: () => Promise.resolve(true) }));

import router from '../router';

describe('router: ожидание состояния гейта согласия (#1567)', () => {
  beforeEach(() => {
    refresh.mockClear();
    authState.isAuthenticated = true;
  });

  it('перед защищённым маршрутом состояние гейта запрашивается', async () => {
    await router.push('/news').catch(() => {});
    expect(refresh).toHaveBeenCalled();
  });

  it('на странице входа состояние гейта не запрашивается', async () => {
    authState.isAuthenticated = false;
    refresh.mockClear();

    await router.push('/').catch(() => {});

    expect(refresh).not.toHaveBeenCalled();
  });

  it('гостя уводит на вход, не дожидаясь гейта', async () => {
    authState.isAuthenticated = false;
    refresh.mockClear();

    await router.push('/news').catch(() => {});

    expect(refresh).not.toHaveBeenCalled();
    expect(router.currentRoute.value.path).toBe('/');
  });

  it('навигация не залипает, когда состояние гейта не читается', async () => {
    // Стор глушит сетевую ошибку сам и резолвится - гард обязан пропустить, иначе
    // недоступный гейт запер бы весь интерфейс.
    refresh.mockResolvedValueOnce(undefined);
    await router.push('/personal-cabinet').catch(() => {});
    expect(router.currentRoute.value.path).toBe('/personal-cabinet');
  });
});
