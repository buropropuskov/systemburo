import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { reactive } from 'vue';
import { shallowMount, flushPromises } from '@vue/test-utils';

// #1567: пока согласие на обработку ПД не дано, приложение не монтирует ни
// оболочку (шапка/меню), ни страницу, ни онбординг-тур - поверх стоит
// неснимаемое окно согласия. Исключения: супер-администратор, забаненный (ему
// показывается блокировка) и страницы вне оболочки (вход, ошибка, техработы).

const fetchPermissions = vi.fn();
const permissionsState = reactive({
  banned: false,
  fetchPermissions,
  clearPermissions: vi.fn(),
});
const authState = reactive({
  token: 'tok',
  userPayload: { user_id: 42 },
  isAuthenticated: true,
  isSuperAdmin: false,
  loadUserTypeCode: vi.fn(),
  setTokens: vi.fn(),
  clearTokens: vi.fn(),
});
const consentState = reactive({
  resolved: true,
  required: true,
  refresh: vi.fn(),
  reset: vi.fn(),
});
const onboardingState = reactive({ reset: vi.fn() });
const passwordChangeState = reactive({ required: false, reset: vi.fn() });

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({}) }),
}));
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }));
vi.mock('@/stores/permissions', () => ({ usePermissionsStore: () => permissionsState }));
vi.mock('@/stores/pdConsent', () => ({ usePDConsentStore: () => consentState }));
// Гейт смены пароля (#1911) сидит в тех же вычисляемых свойствах App: без мока
// стор пошёл бы за настоящей pinia, которую этот файл не поднимает.
vi.mock('@/stores/passwordChange', () => ({ usePasswordChangeStore: () => passwordChangeState }));
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => onboardingState }));
vi.mock('@/stores/theme', () => ({ useThemeStore: () => ({ syncFromServer: vi.fn() }) }));
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

import App from '@/App.vue';
import NavMenu from '@/components/NavMenu.vue';
import TheHeader from '@/components/TheHeader/TheHeader.vue';
import OnboardingTour from '@/components/onboarding/OnboardingTour.vue';
import PDConsentOverlay from '@/components/PDConsentOverlay.vue';

let wrappers = [];

function mountApp(path = '/news') {
  const wrapper = shallowMount(App, {
    global: {
      // push возвращает промис: watch(isBanned) навешивает на него .catch, и
      // синхронный мок валит монтирование.
      mocks: { $route: { path, name: 'News' }, $router: { push: vi.fn().mockResolvedValue() } },
      stubs: { 'router-view': true, RouterView: true, transition: true },
    },
  });
  wrappers.push(wrapper);
  return wrapper;
}

function consentOverlayActive(wrapper) {
  return wrapper.findComponent(PDConsentOverlay).props('active');
}

describe('App.vue - гейт согласия на обработку ПД (#1567)', () => {
  beforeEach(() => {
    authState.token = 'tok';
    authState.isAuthenticated = true;
    authState.isSuperAdmin = false;
    permissionsState.banned = false;
    consentState.resolved = true;
    consentState.required = true;
    consentState.refresh.mockClear();
    consentState.reset.mockClear();
    onboardingState.reset.mockClear();
    fetchPermissions.mockClear();
  });

  // Сторы общие для файла: не снятый инстанс продолжает следить за ними и
  // реагировать на подготовку следующего кейса.
  afterEach(() => {
    wrappers.forEach((w) => w.unmount());
    wrappers = [];
  });

  it('согласие требуется: окно активно, оболочка, страница и тур не монтируются', async () => {
    const wrapper = mountApp();
    await flushPromises();

    expect(consentOverlayActive(wrapper)).toBe(true);
    expect(wrapper.findComponent(NavMenu).exists()).toBe(false);
    expect(wrapper.findComponent(TheHeader).exists()).toBe(false);
    expect(wrapper.findComponent(OnboardingTour).exists()).toBe(false);
    expect(wrapper.html()).not.toContain('router-view');
  });

  it('согласие дано: оболочка, страница и тур на месте, окна нет', async () => {
    consentState.required = false;
    const wrapper = mountApp();
    await flushPromises();

    expect(consentOverlayActive(wrapper)).toBe(false);
    expect(wrapper.findComponent(NavMenu).exists()).toBe(true);
    expect(wrapper.findComponent(TheHeader).exists()).toBe(true);
    expect(wrapper.findComponent(OnboardingTour).exists()).toBe(true);
    expect(wrapper.html()).toContain('router-view');
  });

  it('супер-администратор гейтом не закрывается - иначе битую настройку не починить', async () => {
    authState.isSuperAdmin = true;
    const wrapper = mountApp();
    await flushPromises();

    expect(consentOverlayActive(wrapper)).toBe(false);
    expect(wrapper.findComponent(NavMenu).exists()).toBe(true);
  });

  it('забаненному показывается блокировка, а не согласие', async () => {
    permissionsState.banned = true;
    const wrapper = mountApp();
    await flushPromises();

    expect(consentOverlayActive(wrapper)).toBe(false);
  });

  it('на странице входа окно не показываем (после setTokens маршрут ещё /)', async () => {
    const wrapper = mountApp('/');
    await flushPromises();

    expect(consentOverlayActive(wrapper)).toBe(false);
    expect(wrapper.html()).toContain('router-view');
  });

  it('на странице ошибки и техработ окно не показываем', async () => {
    for (const path of ['/500', '/maintenance']) {
      const wrapper = mountApp(path);
      await flushPromises();
      expect(consentOverlayActive(wrapper)).toBe(false);
    }
  });

  it('состояние гейта неизвестно (сеть упала) - окно не показываем', async () => {
    consentState.resolved = false;
    const wrapper = mountApp();
    await flushPromises();

    expect(consentOverlayActive(wrapper)).toBe(false);
    expect(wrapper.findComponent(NavMenu).exists()).toBe(true);
  });

  it('гость не получает окна и запроса состояния', async () => {
    authState.token = null;
    authState.isAuthenticated = false;
    const wrapper = mountApp('/');
    await flushPromises();

    expect(consentOverlayActive(wrapper)).toBe(false);
    expect(consentState.refresh).not.toHaveBeenCalled();
  });

  it('при восстановленной сессии запрашивает состояние гейта', async () => {
    mountApp();
    await flushPromises();

    expect(consentState.refresh).toHaveBeenCalledTimes(1);
  });

  it('после успешного логина перечитывает состояние с force', async () => {
    const wrapper = mountApp('/');
    await flushPromises();
    consentState.refresh.mockClear();

    wrapper.vm.handleSuccessfulLogin({ token: 'tok2' });
    await flushPromises();

    // force обязателен: в этой вкладке уже мог отвечать гейт другого юзера.
    expect(consentState.refresh).toHaveBeenCalledWith(true);
  });

  it('выход сбрасывает состояние согласия рядом со сбросом тура', async () => {
    const wrapper = mountApp();
    await flushPromises();

    await wrapper.vm.logout();

    expect(consentState.reset).toHaveBeenCalledTimes(1);
    expect(onboardingState.reset).toHaveBeenCalledTimes(1);
  });
});
