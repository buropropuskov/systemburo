import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NavMenu from '../NavMenu.vue';
import { useAuthStore } from '@/stores/auth';

// FE-S2: пункт "Доступные мне" в секции ЗАЯВКИ виден только охраннику
// (canViewAccessibleAttachments) и супер-админу; обычный пользователь его не видит.
// Гейтинг идёт через v-show, поэтому проверяем видимость, а не наличие в DOM.

vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn() }));
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({
  getUnreadCount: vi.fn().mockResolvedValue({ count: 0 }),
}));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }));

const TESTID = '[data-testid="nav-link-accessible-attachments"]';

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
});

afterEach(() => {
  wrapper?.unmount();
});

describe('NavMenu: гейтинг пункта "Доступные мне" (FE-S2)', () => {
  it('охранник видит пункт', () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false, canViewAccessibleAttachments: true });
    wrapper = mountNav();
    const item = wrapper.find(TESTID);
    expect(item.exists()).toBe(true);
    expect(item.isVisible()).toBe(true);
  });

  it('супер-админ видит пункт', () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: true, canViewAccessibleAttachments: true });
    wrapper = mountNav();
    expect(wrapper.find(TESTID).isVisible()).toBe(true);
  });

  // Гибрид page.available (срез 6b): обычный юзер (не охранник, не супер) с грантом
  // page.available получает canViewAccessibleAttachments=true и видит пункт. NavMenu
  // следует getter'у независимо от источника доступа (тип vs грант роли/группы).
  it('обычный пользователь с грантом page.available видит пункт', () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false, canViewAccessibleAttachments: true });
    wrapper = mountNav();
    const item = wrapper.find(TESTID);
    expect(item.exists()).toBe(true);
    expect(item.isVisible()).toBe(true);
  });

  it('обычный пользователь не видит пункт', () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false, canViewAccessibleAttachments: false });
    wrapper = mountNav();
    const item = wrapper.find(TESTID);
    expect(item.exists()).toBe(true);
    expect(item.isVisible()).toBe(false);
  });

  // sectionVisible.requests: при поиске не оставляем осиротевший заголовок ЗАЯВКИ.
  // Совпадение только по "Доступные мне" поднимает секцию лишь тем, кто пункт видит.
  it('поиск "Доступные" не показывает секцию ЗАЯВКИ обычному пользователю', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false, canViewAccessibleAttachments: false });
    wrapper = mountNav();
    wrapper.vm.searchQuery = 'Доступные';
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.sectionVisible.requests).toBe(false);
  });

  it('поиск "Доступные" показывает секцию ЗАЯВКИ охраннику', async () => {
    useAuthStore.mockReturnValue({ isSuperAdmin: false, canViewAccessibleAttachments: true });
    wrapper = mountNav();
    wrapper.vm.searchQuery = 'Доступные';
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.sectionVisible.requests).toBe(true);
  });
});
