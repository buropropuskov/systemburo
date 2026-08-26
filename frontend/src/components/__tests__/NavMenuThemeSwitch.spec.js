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

  it('показывает тумблер с подписью, выключенный на светлой теме', async () => {
    wrapper = mountNav();
    await flushPromises();

    expect(toggle().exists()).toBe(true);
    expect(toggle().text()).toContain('Тёмная тема');
    expect(toggle().attributes('aria-checked')).toBe('false');
    // Тем осталось две, поэтому списка выбора в меню больше нет.
    expect(wrapper.findAll('.theme-item')).toHaveLength(0);
  });

  it('клик включает тёмную тему и сохраняет выбор', async () => {
    wrapper = mountNav();
    await flushPromises();

    await toggle().trigger('click');
    await flushPromises();

    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
    expect(localStorage.getItem('app-theme')).toBe('dark');
    expect(saveTheme).toHaveBeenCalledWith('dark');
    expect(toggle().attributes('aria-checked')).toBe('true');
  });

  it('повторный клик возвращает светлую тему', async () => {
    wrapper = mountNav();
    await flushPromises();

    await toggle().trigger('click');
    await flushPromises();
    await toggle().trigger('click');
    await flushPromises();

    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
    expect(saveTheme).toHaveBeenLastCalledWith('light');
    expect(toggle().attributes('aria-checked')).toBe('false');
  });

  it('иконка пункта - луна', async () => {
    wrapper = mountNav();
    await flushPromises();

    // Палитра тут ни о чём не говорит: пункт называется «Тёмная тема».
    expect(toggle().findComponent({ name: 'NavIcon' }).props('name')).toBe('moon');
    expect(navIcons.moon, 'иконки moon нет в реестре').toBeTruthy();
  });

  it('ползунок не попадает под гашение переходов смены темы', async () => {
    // html.theme-switching гасит ВСЕ transition на два кадра, чтобы цвета встали
    // одним кадром. Без исключения ползунок телепортируется (замер: 0
    // промежуточных положений против 12 с исключением).
    const css = readFileSync(resolve(__dirname, '../../assets/tokens.css'), 'utf8');
    const rule = css.match(/html\.theme-switching\s+\.nav-theme-switch[^{]*\{([^}]*)\}/);
    expect(rule, 'нет исключения для тумблера темы').not.toBeNull();
    expect(rule[1]).toMatch(/transition:[^;]*transform[^;]*!important/);
  });

  it('в drawer на телефоне ползунок видим и стоит справа', () => {
    // Индикатор проявляется правилом .nav-menu.expanded, а drawer этот класс
    // никогда не получает (expandMenu гейтит разворот на мобилке), поэтому
    // мобильный медиа-блок обязан показать ползунок сам.
    const sfc = readFileSync(resolve(__dirname, '../NavMenu.vue'), 'utf8');
    const start = sfc.indexOf('@media (max-width: 768px)');
    expect(start, 'нет мобильного медиа-блока').toBeGreaterThan(-1);

    let depth = 0;
    let end = sfc.length;
    for (let i = sfc.indexOf('{', start); i < sfc.length; i += 1) {
      if (sfc[i] === '{') depth += 1;
      if (sfc[i] === '}') {
        depth -= 1;
        if (depth === 0) { end = i; break; }
      }
    }
    const mobile = sfc.slice(start, end);

    const shown = mobile.match(/\.nav-menu \.nav-theme-switch[^{]*\{([^}]*)\}/);
    expect(shown, 'ползунок темы остаётся скрытым в drawer').not.toBeNull();
    expect(shown[1]).toMatch(/opacity:\s*1\s*!important/);

    const placed = mobile.match(/\.nav-menu \.nav-item--theme[^{]*\{([^}]*)\}/);
    expect(placed, 'строка темы не разведена по краям').not.toBeNull();
    expect(placed[1]).toMatch(/justify-content:\s*space-between/);
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
