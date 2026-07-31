import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NavMenu from '../NavMenu.vue';
import { useAuthStore } from '@/stores/auth';
import { getFeedbackStats } from '@/api/feedback';
import { getMyPermissions } from '@/api/permissions';

// Бейдж с числом непрочитанных обращений у пункта "Администрирование".
// Гейт - право page.admin.feedback (как и видимость самого пункта), НЕ isSuperAdmin:
// обычный администратор (is_admin) обязан видеть счётчик. Эндпоинт /feedback/stats
// открыт тем же правом.

vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn() }));
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({
  getUnreadCount: vi.fn().mockResolvedValue({ count: 0 }),
}));
vi.mock('@/api/feedback', () => ({ getFeedbackStats: vi.fn() }));
vi.mock('@/api/permissions', () => ({ getMyPermissions: vi.fn() }));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }));

const BADGE = '[data-testid="nav-feedback-badge"]';

// Управляет эффективными правами через штатный fetchPermissions стора.
// mode 'admin'/'super' -> page.admin.feedback разрешён; 'normal' без allow -> запрещён.
function setPermissions({ mode = 'normal', allow = [] } = {}) {
  getMyPermissions.mockResolvedValue({
    mode,
    permissions: allow.map((key) => ({ key, value: 'allow' })),
    denied: [],
  });
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

describe('NavMenu: бейдж новых обращений', () => {
  it('администратор (не супер) с правом page.admin.feedback видит счётчик', async () => {
    // Регрессия: раньше гейт был isSuperAdmin и админ счётчик не видел.
    useAuthStore.mockReturnValue({ isSuperAdmin: false });
    setPermissions({ mode: 'admin' });
    getFeedbackStats.mockResolvedValue({ unread: 3 });
    wrapper = mountNav();
    await flushPromises();

    expect(getFeedbackStats).toHaveBeenCalled();
    const badge = wrapper.find(BADGE);
    expect(badge.exists()).toBe(true);
    expect(badge.text()).toBe('3');
  });

  it('супер-админ: бейдж показывает число непрочитанных обращений', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: true });
    setPermissions({ mode: 'super' });
    getFeedbackStats.mockResolvedValue({ unread: 3 });
    wrapper = mountNav();
    await flushPromises();

    expect(getFeedbackStats).toHaveBeenCalled();
    expect(wrapper.find(BADGE).text()).toBe('3');
  });

  it('больше 9 непрочитанных показывает "9+"', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false });
    setPermissions({ mode: 'admin' });
    getFeedbackStats.mockResolvedValue({ unread: 15 });
    wrapper = mountNav();
    await flushPromises();

    expect(wrapper.find(BADGE).text()).toBe('9+');
  });

  it('нет непрочитанных - бейджа нет', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false });
    setPermissions({ mode: 'admin' });
    getFeedbackStats.mockResolvedValue({ unread: 0 });
    wrapper = mountNav();
    await flushPromises();

    expect(wrapper.find(BADGE).exists()).toBe(false);
  });

  it('без права page.admin.feedback: stats не запрашивается и бейджа нет', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false });
    setPermissions({ mode: 'normal' });
    getFeedbackStats.mockResolvedValue({ unread: 5 });
    wrapper = mountNav();
    await flushPromises();

    expect(getFeedbackStats).not.toHaveBeenCalled();
    expect(wrapper.find(BADGE).exists()).toBe(false);
  });

  it('ошибка запроса stats не роняет навигацию, бейджа нет', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false });
    setPermissions({ mode: 'admin' });
    getFeedbackStats.mockRejectedValue(new Error('boom'));
    wrapper = mountNav();
    await flushPromises();

    expect(wrapper.find(BADGE).exists()).toBe(false);
    expect(wrapper.find('[data-testid="nav-link-admin"]').exists()).toBe(true);
  });

  it('ошибка повторного опроса сохраняет последнее значение бейджа, не мигает в 0', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false });
    setPermissions({ mode: 'admin' });
    getFeedbackStats.mockResolvedValue({ unread: 3 });
    wrapper = mountNav();
    await flushPromises();
    expect(wrapper.find(BADGE).text()).toBe('3');

    // Сетевой сбой при следующем опросе не должен обнулять бейдж.
    getFeedbackStats.mockRejectedValueOnce(new Error('boom'));
    await wrapper.vm.fetchNewFeedbackCount();
    await flushPromises();
    expect(wrapper.find(BADGE).exists()).toBe(true);
    expect(wrapper.find(BADGE).text()).toBe('3');
  });
});
