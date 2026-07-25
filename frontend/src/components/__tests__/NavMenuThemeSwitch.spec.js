import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NavMenu from '../NavMenu.vue';
import { useAuthStore } from '@/stores/auth';
import { getMyPermissions } from '@/api/permissions';
import { saveTheme } from '@/api/theme';
import { THEMES } from '@/utils/theme';

// #1415: выбор темы живёт в навигационном меню, секция «ПОЛЬЗОВАТЕЛЬ».

vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn() }));
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({ getUnreadCount: vi.fn().mockResolvedValue({ count: 0 }) }));
vi.mock('@/api/feedback', () => ({ getFeedbackStats: vi.fn().mockResolvedValue({ unread: 0 }) }));
vi.mock('@/api/permissions', () => ({ getMyPermissions: vi.fn() }));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }));
vi.mock('@/api/theme', () => ({
  getTheme: vi.fn().mockResolvedValue({ theme: null }),
  saveTheme: vi.fn().mockResolvedValue({ message: 'ok' }),
}));

function mountNav() {
  return mount(NavMenu, {
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn() },
        $route: { path: '/news', params: {} },
      },
      stubs: { FeedbackModal: true },
    },
  });
}

let wrapper;

beforeEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute('data-theme');
  setActivePinia(createPinia());
  vi.clearAllMocks();
  saveTheme.mockResolvedValue({ message: 'ok' });
  useAuthStore.mockReturnValue({ isSuperAdmin: true, canViewAccessibleAttachments: false });
  getMyPermissions.mockResolvedValue({ mode: 'super', permissions: [], denied: [] });
});

afterEach(() => {
  wrapper?.unmount();
});

describe('NavMenu: переключатель темы (#1415)', () => {
  it('показывает все темы реестра с названиями и кружками палитры', async () => {
    wrapper = mountNav();
    await flushPromises();

    const items = wrapper.findAll('.theme-item');
    expect(items).toHaveLength(THEMES.length);
    expect(items.map((el) => el.text())).toEqual(THEMES.map((t) => t.name));
    expect(wrapper.findAll('.theme-dot')).toHaveLength(THEMES.length);
  });

  it('клик по теме переключает data-theme на <html> и сохраняет выбор', async () => {
    wrapper = mountNav();
    await flushPromises();

    await wrapper.find('[data-testid="nav-theme-dark"]').trigger('click');
    await flushPromises();

    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
    expect(localStorage.getItem('app-theme')).toBe('dark');
    expect(saveTheme).toHaveBeenCalledWith('dark');
  });

  it('активным подсвечен текущий пункт, и подсветка переезжает за выбором', async () => {
    wrapper = mountNav();
    await flushPromises();

    const active = () => wrapper.findAll('.theme-item.active').map((el) => el.text());
    expect(active()).toEqual(['Светлая']);

    await wrapper.find('[data-testid="nav-theme-official-blue"]').trigger('click');
    await flushPromises();

    expect(active()).toEqual(['Официальная']);
  });

  it('дропдаун раскрывается по клику на пункт «Оформление»', async () => {
    wrapper = mountNav();
    await flushPromises();

    expect(wrapper.vm.dropdowns.themes).toBe(false);
    await wrapper.find('[data-testid="nav-link-theme"]').trigger('click');
    expect(wrapper.vm.dropdowns.themes).toBe(true);
  });

  it('пункт находится поиском по рельсу и не пропадает у юзера без прав', async () => {
    getMyPermissions.mockResolvedValue({ mode: 'user', permissions: [], denied: [] });
    wrapper = mountNav();
    await flushPromises();
    await wrapper.setData({ searchQuery: 'оформл' });

    expect(wrapper.vm.sectionVisible.user).toBe(true);
    expect(wrapper.find('[data-testid="nav-link-theme"]').isVisible()).toBe(true);
  });
});
