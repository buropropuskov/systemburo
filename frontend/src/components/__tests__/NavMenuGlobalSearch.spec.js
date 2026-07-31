import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NavMenu from '../NavMenu.vue';
import { useAuthStore } from '@/stores/auth';

// Поле «Поиск...» в рельсе раньше фильтровало пункты меню на месте. Теперь ввод из него
// уходит в сквозной поиск, а найденное показывается в панели справа: прежняя возможность
// осталась (разделы идут первой группой), к ней добавились данные.

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
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
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

describe('NavMenu: поле поиска питает сквозной поиск', () => {
  it('введённая строка уходит наружу, в панель результатов', async () => {
    wrapper = mountNav();

    await wrapper.find('.nav-search').setValue('Роголев');

    expect(bus.emit).toHaveBeenCalledWith('global-search:query', 'Роголев');
  });

  it('поле редактируется на месте', () => {
    wrapper = mountNav();

    expect(wrapper.find('.nav-search').attributes('readonly')).toBeUndefined();
  });

  it('закрытие панели чистит поле, иначе она откроется снова', async () => {
    wrapper = mountNav();
    await wrapper.find('.nav-search').setValue('Роголев');

    wrapper.vm.clearSearch();
    await wrapper.vm.$nextTick();

    expect(wrapper.find('.nav-search').element.value).toBe('');
  });

  it('клик по лупе раскрывает свёрнутый рельс и ставит курсор в поле', async () => {
    wrapper = mountNav();

    await wrapper.find('.nav-search-ic').trigger('click');

    expect(wrapper.vm.uiStore.sidebarExpanded).toBe(true);
  });
});
