import { describe, it, expect, vi, beforeEach } from 'vitest';
import { reactive } from 'vue';
import { shallowMount, flushPromises } from '@vue/test-utils';

// #840: App.vue подписывается на адресный scope user:<id> и по сигналу
// user.banned/user.unbanned мгновенно перезапрашивает права (fetchPermissions с
// force - иначе свежий 30с-кэш коротит вызов в no-op), не дожидаясь навигации/
// опроса. Подписка реактивна к токену: логин/logout/истечение сессии/смена юзера.

const fetchPermissions = vi.fn();
// reactive: computed banScopeUserId должен реагировать на смену token/userPayload,
// иначе watch не сработает (как в реальном Pinia-сторе).
const authState = reactive({
  token: 'tok',
  userPayload: { user_id: 42 },
  isAuthenticated: true,
  isSuperAdmin: false,
  loadUserTypeCode: vi.fn(),
  setTokens: vi.fn(),
  clearTokens: vi.fn(),
});

vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({}) }) }));
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }));
vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ banned: false, fetchPermissions, clearPermissions: vi.fn() }),
}));
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => ({ reset: vi.fn() }) }));
// Гейт согласия (#1567) читает свой стор из computed App.vue; спека монтируется
// без Pinia, поэтому стор мокаем - согласие тут не проверяется.
vi.mock('@/stores/pdConsent', () => ({
  usePDConsentStore: () => ({ resolved: false, required: false, refresh: vi.fn(), reset: vi.fn() }),
}));
// Гейт смены пароля (#1911) - по той же причине, что и согласие выше.
vi.mock('@/stores/passwordChange', () => ({
  usePasswordChangeStore: () => ({ required: false, reset: vi.fn() }),
}));
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
import eventStream from '@/services/eventStream';

function mountApp() {
  return shallowMount(App, {
    global: {
      mocks: { $route: { path: '/news', name: 'News' }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } },
      stubs: { 'router-view': true, RouterView: true, transition: true },
    },
  });
}

describe('App.vue - real-time бан (#840)', () => {
  beforeEach(() => {
    authState.token = 'tok';
    authState.userPayload = { user_id: 42 };
    fetchPermissions.mockReset();
    eventStream.connect.mockClear();
    eventStream.disconnect.mockClear();
    eventStream.subscribe.mockClear().mockImplementation(() => vi.fn());
  });

  it('при наличии токена подключается и подписывается на user:<id>', async () => {
    mountApp();
    await flushPromises();
    expect(eventStream.connect).toHaveBeenCalledTimes(1);
    expect(eventStream.subscribe).toHaveBeenCalledWith('user:42', expect.any(Function));
  });

  it('колбэк сигнала бана форсит перезапрос прав (fetchPermissions(true), игнор кэша)', async () => {
    mountApp();
    await flushPromises();
    const cb = eventStream.subscribe.mock.calls.find((c) => c[0] === 'user:42')[1];
    fetchPermissions.mockClear();
    cb();
    expect(fetchPermissions).toHaveBeenCalledTimes(1);
    // force=true обязателен: без него свежий 30с-кэш прав вернёт no-op и бан не всплывёт.
    expect(fetchPermissions).toHaveBeenCalledWith(true);
  });

  it('без токена не подписывается (гость на логине)', async () => {
    authState.token = null;
    mountApp();
    await flushPromises();
    expect(eventStream.subscribe).not.toHaveBeenCalled();
    expect(eventStream.connect).not.toHaveBeenCalled();
  });

  it('при unmount отписывается и отключает eventStream', async () => {
    const unsub = vi.fn();
    eventStream.subscribe.mockImplementation(() => unsub);
    const wrapper = mountApp();
    await flushPromises();

    wrapper.unmount();
    expect(unsub).toHaveBeenCalledTimes(1);
    expect(eventStream.disconnect).toHaveBeenCalledTimes(1);
  });

  it('истечение сессии (сброс токена в обход logout) снимает подписку, релогин другим юзером ставит новый scope', async () => {
    const unsub42 = vi.fn();
    eventStream.subscribe.mockImplementation((scope) => (scope === 'user:42' ? unsub42 : vi.fn()));
    mountApp();
    await flushPromises();
    expect(eventStream.subscribe).toHaveBeenCalledWith('user:42', expect.any(Function));

    // client.js на провале refresh обнуляет токен НЕ через logout() - подписка обязана сняться реактивно.
    authState.token = null;
    await flushPromises();
    expect(unsub42).toHaveBeenCalledTimes(1);

    // В той же вкладке логинится другой пользователь - real-time бан должен заработать на новом scope.
    authState.token = 'tok2';
    authState.userPayload = { user_id: 99 };
    await flushPromises();
    expect(eventStream.subscribe).toHaveBeenCalledWith('user:99', expect.any(Function));
  });
});
