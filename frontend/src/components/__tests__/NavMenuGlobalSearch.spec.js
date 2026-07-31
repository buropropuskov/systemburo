import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NavMenu from '../NavMenu.vue';
import { useAuthStore } from '@/stores/auth';

// Поле «Поиск...» в рельсе раньше фильтровало пункты меню на месте. Теперь оно открывает
// окно сквозного поиска, где разделы идут первой группой: прежняя возможность осталась, к
// ней добавились данные. Поле сделано нередактируемым намеренно -- ввод идёт в окне, и
// два поля с разным поведением в одной панели путали бы.

vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn() }));
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({
  getUnreadCount: vi.fn().mockResolvedValue({ count: 0 }),
}));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }));

let bus;

function mountNav() {
  bus = { on: vi.fn(), off: vi.fn(), emit: vi.fn() };
  return mount(NavMenu, {
    global: {
      mocks: {
        $bus: bus,
        $router: { push: vi.fn() },
        $route: { path: '/news', params: {} },
      },
    },
  });
}

let wrapper;

beforeEach(() => {
  setActivePinia(createPinia());
  useAuthStore.mockReturnValue({
    isAuthenticated: true,
    canViewAccessibleAttachments: false,
    user: { username: 'tester' },
  });
});

afterEach(() => {
  wrapper?.unmount();
  vi.clearAllMocks();
});

describe('NavMenu: поле поиска открывает сквозной поиск', () => {
  it('клик по полю просит открыть окно поиска', async () => {
    wrapper = mountNav();

    await wrapper.find('.nav-search').trigger('mousedown');

    expect(bus.emit).toHaveBeenCalledWith('global-search:open');
  });

  it('фокус с клавиатуры тоже открывает окно', async () => {
    wrapper = mountNav();

    await wrapper.find('.nav-search').trigger('focus');

    expect(bus.emit).toHaveBeenCalledWith('global-search:open');
  });

  it('поле не редактируется на месте: ввод идёт в окне поиска', () => {
    wrapper = mountNav();

    expect(wrapper.find('.nav-search').attributes('readonly')).toBeDefined();
  });
});
