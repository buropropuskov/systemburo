import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { reactive } from 'vue';
import { shallowMount, flushPromises } from '@vue/test-utils';

/**
 * Раскрытие `reveal.open: 'search-panel'`: панель сквозного поиска живёт в корне
 * приложения и открывается только по действию пользователя. Тур поднимает сигнал
 * в сторе, App открывает панель и закрывает за собой ровно то, что открыл.
 */

const permissionsState = reactive({
  banned: false,
  fetchPermissions: vi.fn(),
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
const onboardingState = reactive({ revealOpen: null, reset: vi.fn() });
const passwordChangeState = reactive({ required: false, reset: vi.fn() });

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({}) }),
}));
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }));
vi.mock('@/stores/permissions', () => ({ usePermissionsStore: () => permissionsState }));
vi.mock('@/stores/pdConsent', () => ({ usePDConsentStore: () => consentState }));
// Гейт смены пароля (#1911) читает свой стор из тех же computed; спека монтируется
// без Pinia, поэтому стор мокаем - смена пароля тут не проверяется.
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

let wrappers = [];

function mountApp() {
  const wrapper = shallowMount(App, {
    global: {
      mocks: { $route: { path: '/news', name: 'News' }, $router: { push: vi.fn().mockResolvedValue() } },
      stubs: { 'router-view': true, RouterView: true, transition: true },
    },
  });
  wrappers.push(wrapper);
  return wrapper;
}

describe('App.vue - раскрытие панели поиска для онбординга', () => {
  beforeEach(() => {
    onboardingState.revealOpen = null;
    authState.isAuthenticated = true;
    permissionsState.banned = false;
  });

  afterEach(() => {
    wrappers.forEach((w) => w.unmount());
    wrappers = [];
  });

  it('сигнал search-panel открывает панель', async () => {
    const wrapper = mountApp();
    expect(wrapper.vm.searchOpen).toBe(false);

    onboardingState.revealOpen = 'search-panel';
    await flushPromises();

    expect(wrapper.vm.searchOpen).toBe(true);
  });

  it('гашение сигнала закрывает панель, открытую туром', async () => {
    const wrapper = mountApp();
    onboardingState.revealOpen = 'search-panel';
    await flushPromises();
    expect(wrapper.vm.searchOpen).toBe(true);

    onboardingState.revealOpen = null;
    await flushPromises();
    expect(wrapper.vm.searchOpen).toBe(false);
  });

  it('панель, открытую пользователем, тур не закрывает', async () => {
    const wrapper = mountApp();
    wrapper.vm.openGlobalSearch();
    await flushPromises();
    expect(wrapper.vm.searchOpen).toBe(true);

    onboardingState.revealOpen = 'search-panel';
    await flushPromises();
    onboardingState.revealOpen = null;
    await flushPromises();

    expect(wrapper.vm.searchOpen).toBe(true);
  });

  it('чужая цель раскрытия панель не открывает', async () => {
    const wrapper = mountApp();
    onboardingState.revealOpen = 'admin-column';
    await flushPromises();

    expect(wrapper.vm.searchOpen).toBe(false);
  });
});
