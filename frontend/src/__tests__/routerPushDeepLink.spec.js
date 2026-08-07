import { describe, it, expect, vi, beforeEach } from 'vitest';
import { reactive } from 'vue';

// #974: клик по push-уведомлению вёл ВСЕХ на /personal-cabinet?open=<id>, но
// эту заявку видит там только автор - согласующий/администратор (page.center)
// открывает чужие заявки в Центре (/center?open=<id>), и получал пустой личный
// кабинет вместо заявки, ради которой пришло уведомление. Service worker прав
// не знает (живёт вне вкладки/Pinia), поэтому шлёт нейтральный
// /?open_application=<id>, а этот гард решает маршрут - тем же кодом, что клик
// по карточке (useNotificationNavigation.resolveApplicationRoute).

const refresh = vi.fn().mockResolvedValue(undefined);
const authState = reactive({ isAuthenticated: true, isSuperAdmin: false, userTypeCode: 'user', token: 'tok' });

// granted переключается ВНУТРИ fetchPermissions - так замок ловит гард, который
// решает маршрут ДО await (а не после): если бы guard читал hasPermission до
// ожидания fetchPermissions, тест 'дожидается прав' поймал бы неверный /personal-cabinet.
const permState = reactive({ granted: false, isStale: true });
const fetchPermissions = vi.fn(async () => {
  permState.granted = true;
  permState.isStale = false;
});

vi.mock('@/stores/pdConsent', () => ({ usePDConsentStore: () => ({ refresh }) }));
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }));
vi.mock('@/stores/maintenance', () => ({ useMaintenanceStore: () => ({ enabled: false }) }));
vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({
    hasPermission: (key) => key === 'page.center' && permState.granted,
    fetchPermissions,
    get isStale() { return permState.isStale; },
    loaded: true,
    banned: false,
  }),
}));
vi.mock('@/utils/dirtyTracker', () => ({ confirmIfAnyDirty: () => Promise.resolve(true) }));

import router from '../router';

describe('router: адрес push-уведомления о заявке (#974)', () => {
  beforeEach(() => {
    refresh.mockClear();
    fetchPermissions.mockClear();
    authState.isAuthenticated = true;
    permState.granted = false;
    permState.isStale = true;
  });

  it('с правом page.center уводит в Центр, параметр пропадает из адреса', async () => {
    permState.granted = true;
    permState.isStale = false;

    await router.push('/?open_application=42').catch(() => {});

    expect(router.currentRoute.value.path).toBe('/center');
    expect(router.currentRoute.value.query.open).toBe('42');
    expect(router.currentRoute.value.query.open_application).toBeUndefined();
  });

  it('без права page.center уводит в личный кабинет с тем же id', async () => {
    permState.granted = false;
    permState.isStale = false;

    await router.push('/?open_application=42').catch(() => {});

    expect(router.currentRoute.value.path).toBe('/personal-cabinet');
    expect(router.currentRoute.value.query.open).toBe('42');
  });

  it('права ещё не загружены - гард дожидается fetchPermissions перед решением маршрута', async () => {
    permState.isStale = true;
    permState.granted = false; // станет true только ВНУТРИ fetchPermissions

    await router.push('/?open_application=7').catch(() => {});

    expect(fetchPermissions).toHaveBeenCalled();
    expect(router.currentRoute.value.path).toBe('/center'); // не /personal-cabinet - грант подъехал вовремя
  });

  it('гостя уводит на вход, адрес сохраняется в query.redirect (переиспользует #974 механизм)', async () => {
    authState.isAuthenticated = false;

    await router.push('/?open_application=42').catch(() => {});

    expect(router.currentRoute.value.path).toBe('/');
    expect(router.currentRoute.value.query.redirect).toBe('/?open_application=42');
    // Без прав решать пока нечего - гард не должен трогать permissions-стор для гостя.
    expect(fetchPermissions).not.toHaveBeenCalled();
  });

  it('обычный заход на / без open_application не задет - прежнее поведение', async () => {
    authState.isAuthenticated = true;
    await router.push('/').catch(() => {});

    expect(router.currentRoute.value.path).toBe('/news');
  });
});
