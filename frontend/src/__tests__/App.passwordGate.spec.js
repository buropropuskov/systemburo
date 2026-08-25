import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { reactive } from 'vue';
import { shallowMount, flushPromises } from '@vue/test-utils';

// #1911: пока система требует задать свой пароль вместо присланного письмом,
// приложение не монтирует ни оболочку (шапка/меню), ни страницу, ни тур - поверх
// стоит несъёмное окно смены пароля. Серверный гейт всё равно отвечает отказом на
// всё, кроме самой смены, и без окна человек видел бы пустой экран.

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
  required: false,
  refresh: vi.fn(),
  reset: vi.fn(),
});
const passwordChangeState = reactive({ required: true, reset: vi.fn() });
const onboardingState = reactive({ reset: vi.fn() });

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({}) }),
}));
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }));
vi.mock('@/stores/permissions', () => ({ usePermissionsStore: () => permissionsState }));
vi.mock('@/stores/pdConsent', () => ({ usePDConsentStore: () => consentState }));
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
import ChangePasswordModal from '@/components/ChangePasswordModal.vue';

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

function passwordModal(wrapper) {
  return wrapper.findComponent(ChangePasswordModal);
}

describe('App.vue - гейт обязательной смены пароля (#1911)', () => {
  beforeEach(() => {
    authState.token = 'tok';
    authState.isAuthenticated = true;
    authState.isSuperAdmin = false;
    permissionsState.banned = false;
    consentState.required = false;
    passwordChangeState.required = true;
    passwordChangeState.reset.mockClear();
    onboardingState.reset.mockClear();
  });

  // Сторы общие для файла: не снятый инстанс продолжает следить за ними и
  // реагировать на подготовку следующего кейса.
  afterEach(() => {
    wrappers.forEach((w) => w.unmount());
    wrappers = [];
  });

  it('смена требуется: окно открыто и несъёмно, оболочка, страница и тур не монтируются', async () => {
    const wrapper = mountApp();
    await flushPromises();

    expect(passwordModal(wrapper).props('show')).toBe(true);
    expect(passwordModal(wrapper).props('mandatory')).toBe(true);
    expect(wrapper.findComponent(NavMenu).exists()).toBe(false);
    expect(wrapper.findComponent(TheHeader).exists()).toBe(false);
    expect(wrapper.findComponent(OnboardingTour).exists()).toBe(false);
    expect(wrapper.html()).not.toContain('router-view');
  });

  it('требования нет: оболочка, страница и тур на месте, окна нет', async () => {
    passwordChangeState.required = false;
    const wrapper = mountApp();
    await flushPromises();

    expect(passwordModal(wrapper).props('show')).toBe(false);
    expect(wrapper.findComponent(NavMenu).exists()).toBe(true);
    expect(wrapper.findComponent(TheHeader).exists()).toBe(true);
    expect(wrapper.html()).toContain('router-view');
  });

  // Супер-администратор проходит гейт согласия как аварийную дверь, но здесь
  // исключения нет ни на сервере, ни во фронте: пароль из письма опасен независимо
  // от прав.
  it('супер-администратора гейт закрывает наравне со всеми', async () => {
    authState.isSuperAdmin = true;
    const wrapper = mountApp();
    await flushPromises();

    expect(passwordModal(wrapper).props('show')).toBe(true);
  });

  it('забаненному показывается блокировка, а не смена пароля', async () => {
    permissionsState.banned = true;
    const wrapper = mountApp();
    await flushPromises();

    expect(passwordModal(wrapper).props('show')).toBe(false);
  });

  it('на входе, странице ошибки и техработах окно не показываем', async () => {
    for (const path of ['/', '/500', '/maintenance']) {
      const wrapper = mountApp(path);
      await flushPromises();
      expect(passwordModal(wrapper).props('show')).toBe(false);
    }
  });

  it('гость окна не получает', async () => {
    authState.token = null;
    authState.isAuthenticated = false;
    const wrapper = mountApp('/');
    await flushPromises();

    expect(passwordModal(wrapper).props('show')).toBe(false);
  });

  // Окно само чистит сессию и уводит на вход: не сняв требование, оно всплыло бы
  // поверх формы входа.
  it('после смены пароля требование снимается', async () => {
    const wrapper = mountApp();
    await flushPromises();

    passwordModal(wrapper).vm.$emit('changed');
    await flushPromises();

    expect(passwordChangeState.reset).toHaveBeenCalledTimes(1);
  });

  it('выход из окна сбрасывает требование рядом со сбросом тура', async () => {
    const wrapper = mountApp();
    await flushPromises();

    await wrapper.vm.logout();

    expect(passwordChangeState.reset).toHaveBeenCalledTimes(1);
    expect(onboardingState.reset).toHaveBeenCalledTimes(1);
  });
});
