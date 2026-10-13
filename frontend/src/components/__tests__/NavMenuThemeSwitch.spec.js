import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NavMenu from '../NavMenu.vue';
import { useAuthStore } from '@/stores/auth';
import { getMyPermissions } from '@/api/permissions';
import { saveTheme } from '@/api/theme';
import { navIcons } from '@/components/icons/navIcons';

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
    attachTo: document.body,
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
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
  const toggle = () => wrapper.find('[data-testid="nav-theme-toggle"]');

  it('показывает один пункт Оформление без немедленной смены темы', async () => {
    wrapper = mountNav();
    await flushPromises();
    expect(toggle().text()).toBe('Оформление');
    expect(toggle().element.tagName).toBe('BUTTON');
    await toggle().trigger('click');
    expect(document.documentElement.dataset.theme).toBe('light');
    expect(saveTheme).not.toHaveBeenCalled();
    expect(document.querySelectorAll('.theme-option')).toHaveLength(2);
  });

  async function changeDark(checked) {
    const input = document.querySelector('[data-testid="theme-dark-toggle"] input');
    input.checked = checked;
    input.dispatchEvent(new Event('change', { bubbles: true }));
    await flushPromises();
  }

  it('настройка включает тёмную тему и сохраняет выбор', async () => {
    wrapper = mountNav();
    await flushPromises();
    await toggle().trigger('click');
    await changeDark(true);
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(localStorage.getItem('app-theme')).toBe('dark');
    expect(saveTheme).toHaveBeenCalledWith('dark');
    expect(document.querySelector('.base-dropdown__menu')).not.toBeNull();
  });

  it('повторный выбор возвращает светлую тему', async () => {
    wrapper = mountNav();
    await flushPromises();
    await toggle().trigger('click');
    await changeDark(true);
    await changeDark(false);
    expect(document.documentElement.dataset.theme).toBe('light');
    expect(saveTheme).toHaveBeenLastCalledWith('light');
  });

  it('использует иконку оформления из существующего реестра', async () => {
    wrapper = mountNav();
    await flushPromises();
    expect(toggle().findComponent({ name: 'NavIcon' }).props('name')).toBe('theme');
    expect(navIcons.theme).toBeTruthy();
  });

  it('переход темы не включает анимацию всех элементов', () => {
    const css = readFileSync(resolve(__dirname, '../../assets/tokens.css'), 'utf8');
    expect(css).not.toContain('html.theme-colors-changing');
    expect(css).toMatch(/html\.theme-switching[^{]*\{[\s\S]*?transition:\s*none\s*!important/);
  });

  it('подпись доступна в мобильном drawer и на тач-экране', () => {
    const sfc = readFileSync(resolve(__dirname, '../ui/ThemeControls.vue'), 'utf8');
    const media = sfc.slice(sfc.indexOf('@media (max-width: 1366px), (hover: none) and (pointer: coarse)'));
    expect(media).toContain('.nav-text { opacity: 1; }');
    expect(media).toContain('min-height: 44px');
  });

  it('пункт находится поиском по рельсу и не пропадает у юзера без прав', async () => {
    getMyPermissions.mockResolvedValue({ mode: 'user', permissions: [], denied: [] });
    wrapper = mountNav();
    await flushPromises();
    await wrapper.setData({ searchQuery: 'оформл' });

    expect(wrapper.vm.sectionVisible.user).toBe(true);
    expect(toggle().isVisible()).toBe(true);
  });
});

describe('theme panel navigation lifecycle', () => {
  it.each(['closeMobile', 'hideRail', 'closeAllDropdowns'])('closes settings through %s', async (method) => {
    wrapper = mountNav();
    await flushPromises();
    await wrapper.find('[data-testid="nav-theme-toggle"]').trigger('click');
    await flushPromises();
    expect(document.querySelector('.theme-options')).not.toBeNull();
    wrapper.vm[method]();
    await flushPromises();
    expect(document.querySelector('.theme-options')).toBeNull();
  });
  it('keeps navigation expanded while the pointer crosses to settings', async () => {
    wrapper = mountNav();
    await flushPromises();
    wrapper.vm.isExpanded = true;
    await wrapper.find('[data-testid="nav-theme-toggle"]').trigger('click');
    await flushPromises();
    const panel = document.querySelector('.theme-options');
    panel.matches = vi.fn(() => true);
    wrapper.vm.collapseMenu();
    await new Promise(resolve => setTimeout(resolve, 180));
    expect(wrapper.vm.isExpanded).toBe(true);
    panel.matches = vi.fn(() => false);
    wrapper.vm.collapseMenu();
    await new Promise(resolve => setTimeout(resolve, 180));
    expect(wrapper.vm.isExpanded).toBe(false);
    wrapper.vm.closeAllDropdowns();
    await flushPromises();
  });
});
