import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NavMenu from '../NavMenu.vue';
import { useAuthStore } from '@/stores/auth';
import { useOnboardingStore } from '@/stores/onboarding';
import { getMyPermissions } from '@/api/permissions';

/**
 * Раскрытие `reveal.open: 'admin-column'`: вторая колонка Админки появляется только
 * по клику пользователя, поэтому тур поднимает сигнал в сторе, а рельс открывает
 * колонку сам и закрывает за собой ровно то, что открыл.
 */

vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn() }));
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({
  getUnreadCount: vi.fn().mockResolvedValue({ count: 0 }),
}));
vi.mock('@/api/feedback', () => ({ getFeedbackStats: vi.fn().mockResolvedValue({ unread: 0 }) }));
vi.mock('@/api/permissions', () => ({ getMyPermissions: vi.fn() }));
vi.mock('@/api/onboarding', () => ({
  getOnboardingStatus: vi.fn().mockResolvedValue({ completed: {} }),
  markOnboardingComplete: vi.fn().mockResolvedValue({ message: 'ok' }),
  getSecurityFactRoute: vi.fn().mockResolvedValue(null),
}));
vi.mock('@/api/approvers', () => ({
  getMyApprovalRole: vi.fn().mockResolvedValue({ is_approver: false, is_reviewer: false }),
}));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }));

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
  useAuthStore.mockReturnValue({ isSuperAdmin: true, canViewAccessibleAttachments: false });
  getMyPermissions.mockResolvedValue({ mode: 'super', permissions: [], denied: [] });
});

afterEach(() => {
  wrapper?.unmount();
});

describe('NavMenu - раскрытие колонки Админки для онбординга', () => {
  it('сигнал admin-column открывает колонку', async () => {
    wrapper = mountNav();
    await flushPromises();
    expect(wrapper.find('[data-testid="admin-column"]').exists()).toBe(false);

    useOnboardingStore().setRevealOpen('admin-column');
    await flushPromises();

    expect(wrapper.vm.adminOpen).toBe(true);
    expect(wrapper.find('[data-testid="admin-column"]').exists()).toBe(true);
  });

  it('гашение сигнала закрывает колонку, открытую туром', async () => {
    wrapper = mountNav();
    await flushPromises();

    const store = useOnboardingStore();
    store.setRevealOpen('admin-column');
    await flushPromises();
    expect(wrapper.vm.adminOpen).toBe(true);

    store.setRevealOpen(null);
    await flushPromises();
    expect(wrapper.vm.adminOpen).toBe(false);
  });

  it('колонку, открытую пользователем, тур не закрывает', async () => {
    wrapper = mountNav();
    await flushPromises();

    wrapper.vm.toggleAdmin();
    await flushPromises();
    expect(wrapper.vm.adminOpen).toBe(true);

    const store = useOnboardingStore();
    store.setRevealOpen('admin-column');
    await flushPromises();
    store.setRevealOpen(null);
    await flushPromises();

    expect(wrapper.vm.adminOpen).toBe(true);
  });

  it('чужая цель раскрытия колонку не открывает', async () => {
    wrapper = mountNav();
    await flushPromises();

    useOnboardingStore().setRevealOpen('search-panel');
    await flushPromises();

    expect(wrapper.vm.adminOpen).toBe(false);
  });

  it('пользователь закрыл колонку сам - последующее гашение сигнала её не трогает', async () => {
    wrapper = mountNav();
    await flushPromises();

    const store = useOnboardingStore();
    store.setRevealOpen('admin-column');
    await flushPromises();

    wrapper.vm.closeAdmin();
    wrapper.vm.toggleAdmin();
    await flushPromises();
    expect(wrapper.vm.adminOpen).toBe(true);

    store.setRevealOpen(null);
    await flushPromises();
    expect(wrapper.vm.adminOpen).toBe(true);
  });
});
