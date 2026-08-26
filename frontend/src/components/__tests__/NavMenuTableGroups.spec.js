import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NavMenu from '../NavMenu.vue';
import { useAuthStore } from '@/stores/auth';
import { getMyPermissions } from '@/api/permissions';

// #1307: таблицы в меню разложены по типу - на нескольких постах общий список
// превращался в кучу без ориентиров.

vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn() }));
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({ getUnreadCount: vi.fn().mockResolvedValue({ count: 0 }) }));
vi.mock('@/api/feedback', () => ({ getFeedbackStats: vi.fn().mockResolvedValue({ unread: 0 }) }));
vi.mock('@/api/permissions', () => ({ getMyPermissions: vi.fn() }));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }));

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

const TABLES = [
  { id: 1, name: 'kpp_4', display_name: 'КПП №4', table_type: 'cars' },
  { id: 2, name: 'post_72', display_name: 'ПОСТ №72', table_type: 'people' },
  { id: 3, name: 'post_72_auto', display_name: 'ПОСТ №72 (АВТО)', table_type: 'cars' },
];

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

describe('NavMenu: список таблиц по типам (#1307)', () => {
  it('машины и люди идут отдельными группами в заданном порядке', async () => {
    wrapper = mountNav();
    await flushPromises();
    await wrapper.setData({ systemTables: TABLES });

    const groups = wrapper.vm.groupedTables;
    expect(groups.map((g) => g.label)).toEqual(['Автомобили', 'Люди']);
    expect(groups[0].tables.map((t) => t.display_name)).toEqual(['КПП №4', 'ПОСТ №72 (АВТО)']);
    expect(groups[1].tables.map((t) => t.display_name)).toEqual(['ПОСТ №72']);

    // Скоуп по списку таблиц: в меню есть второй дропдаун (выбор темы, #1415).
    const rendered = wrapper.findAll('[data-testid="nav-tables-list"] > *').map((el) => el.text());
    expect(rendered).toEqual(['Автомобили', 'КПП №4', 'ПОСТ №72 (АВТО)', 'Люди', 'ПОСТ №72']);
  });

  it('при одном типе подписи не показываются', async () => {
    wrapper = mountNav();
    await flushPromises();
    await wrapper.setData({ systemTables: TABLES.filter((t) => t.table_type === 'cars') });

    expect(wrapper.vm.groupedTables).toHaveLength(1);
    expect(wrapper.findAll('.dropdown-group-title')).toHaveLength(0);
    expect(wrapper.findAll('[data-testid="nav-tables-list"] .dropdown-item').map((el) => el.text()))
      .toEqual(['КПП №4', 'ПОСТ №72 (АВТО)']);
  });

  it('таблица с неизвестным типом попадает в «Прочие», а не теряется', async () => {
    wrapper = mountNav();
    await flushPromises();
    await wrapper.setData({
      systemTables: [...TABLES, { id: 4, name: 'misc', display_name: 'Разное', table_type: '' }],
    });

    const groups = wrapper.vm.groupedTables;
    expect(groups.map((g) => g.label)).toEqual(['Автомобили', 'Люди', 'Прочие']);
    expect(groups[2].tables.map((t) => t.display_name)).toEqual(['Разное']);
  });
});
