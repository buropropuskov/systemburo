import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NavMenu from '../NavMenu.vue';
import { useAuthStore } from '@/stores/auth';
import { getMyPermissions } from '@/api/permissions';

// #1097 W4.1: drawer открывается свайпом вправо. Кромку экрана жест не занимает -
// там живёт системный «Назад» телефонов с жестовой навигацией.

vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn() }));
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({
  getUnreadCount: vi.fn().mockResolvedValue({ count: 0 }),
}));
vi.mock('@/api/feedback', () => ({ getFeedbackStats: vi.fn().mockResolvedValue({ unread: 0 }) }));
vi.mock('@/api/permissions', () => ({ getMyPermissions: vi.fn() }));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }));

function mountNav() {
  return mount(NavMenu, {
    attachTo: document.body,
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
        $route: { path: '/news', params: {} },
      },
    },
  });
}

function fire(type, x, y = 300) {
  const event = new Event(type, { bubbles: true, cancelable: true });
  event.touches = [{ clientX: x, clientY: y }];
  document.body.dispatchEvent(event);
}

let wrapper;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  useAuthStore.mockReturnValue({ isSuperAdmin: true, canViewAccessibleAttachments: false });
  getMyPermissions.mockResolvedValue({ mode: 'super', permissions: [], denied: [] });
  // Композабл гейтит жест по matchMedia - на узком экране он включён.
  window.matchMedia = vi.fn().mockReturnValue({
    matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn(),
  });
});

afterEach(() => {
  wrapper?.unmount();
  document.body.classList.remove('nav-drawer-open');
  document.body.style.overflow = '';
});

describe('NavMenu: открытие drawer свайпом (W4.1)', () => {
  it('свайп вправо открывает drawer и блокирует прокрутку фона', async () => {
    wrapper = mountNav();
    await flushPromises();

    fire('touchstart', 60);
    fire('touchmove', 220);
    fire('touchend', 220);
    await flushPromises();

    expect(wrapper.find('.nav-menu').classes()).toContain('nav-menu--mobile-open');
    expect(document.body.classList.contains('nav-drawer-open')).toBe(true);
  });

  it('свайп от кромки экрана drawer не открывает - жест принадлежит системе', async () => {
    wrapper = mountNav();
    await flushPromises();

    fire('touchstart', 8);
    fire('touchmove', 220);
    fire('touchend', 220);
    await flushPromises();

    expect(wrapper.find('.nav-menu').classes()).not.toContain('nav-menu--mobile-open');
  });

  it('панель идёт за пальцем: до отпускания она сдвинута, а не открыта', async () => {
    wrapper = mountNav();
    await flushPromises();

    fire('touchstart', 60);
    fire('touchmove', 130);
    await flushPromises();

    const nav = wrapper.find('.nav-menu');
    expect(nav.classes()).toContain('nav-menu--dragging');
    // min(0px, ...) - страховка узкого экрана: там панель шириной 85vw доезжает до
    // края раньше, чем палец пройдёт номинальные 280px.
    expect(nav.attributes('style')).toContain('translateX(min(0px, calc(-100% + 70px)))');
    expect(nav.classes()).not.toContain('nav-menu--mobile-open');
    // Затемнение густеет вместе с вытянутой панелью.
    expect(wrapper.find('.nav-menu__backdrop').exists()).toBe(true);

    fire('touchend', 130);
    await flushPromises();
    // Порог не пройден - панель вернулась, inline-сдвиг снят.
    expect(wrapper.find('.nav-menu').attributes('style')).toBeUndefined();
    expect(wrapper.find('.nav-menu').classes()).not.toContain('nav-menu--mobile-open');
  });

  it('при открытой модалке свайп не перехватывается', async () => {
    wrapper = mountNav();
    await flushPromises();
    document.body.style.overflow = 'hidden';

    fire('touchstart', 60);
    fire('touchmove', 260);
    fire('touchend', 260);
    await flushPromises();

    expect(wrapper.find('.nav-menu').classes()).not.toContain('nav-menu--mobile-open');
  });
});
