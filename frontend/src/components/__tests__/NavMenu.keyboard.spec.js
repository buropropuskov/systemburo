import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NavMenu from '../NavMenu.vue';
import { useAuthStore } from '@/stores/auth';
import { getMyPermissions } from '@/api/permissions';
import eventBus from '@/eventBus';

vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn() }));
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false }) }));
vi.mock('@/api/applications', () => ({ getUnreadCount: vi.fn().mockResolvedValue({ count: 0 }) }));
vi.mock('@/api/feedback', () => ({ getFeedbackStats: vi.fn().mockResolvedValue({ unread: 0 }) }));
vi.mock('@/api/permissions', () => ({ getMyPermissions: vi.fn() }));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }));

let wrapper;
const push = vi.fn();
const table = { id: 1, name: 'kpp_4', display_name: 'КПП №4', table_type: 'cars' };
async function openNav() {
  wrapper = mount(NavMenu, {
    attachTo: document.body,
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push, replace: vi.fn().mockResolvedValue() },
        $route: { path: '/news', params: {} },
      },
      stubs: { FeedbackModal: true },
    },
  });
  await flushPromises();
  await wrapper.setData({ systemTables: [table] });
}
beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  useAuthStore.mockReturnValue({ isSuperAdmin: true, canViewAccessibleAttachments: true });
  getMyPermissions.mockResolvedValue({ mode: 'super', permissions: [], denied: [] });
});
afterEach(() => { wrapper?.unmount(); vi.useRealTimers(); vi.restoreAllMocks(); });

// JSDOM has no real input modality; native browser checks cover :focus-visible.
function keyboardFocus(element) {
  const matches = element.matches.bind(element);
  vi.spyOn(element, 'matches').mockImplementation(selector => selector === ':focus-visible' || matches(selector));
  element.focus();
}

describe('NavMenu: native keyboard actions', () => {
  it('exposes all visible actions as non-submit buttons and keeps the existing route handler', async () => {
    await openNav();
    for (const action of wrapper.findAll('.nav-item, button.dropdown-item')) {
      expect(action.element.tagName).toBe('BUTTON');
      expect(action.attributes('type')).toBe('button');
    }
    await wrapper.get('[data-testid="nav-link-employees"]').trigger('click');
    expect(push).toHaveBeenCalledExactlyOnceWith('/employeesview');
  });
  it('removes collapsed table choices from keyboard navigation and reports expansion', async () => {
    await openNav();
    const toggle = wrapper.get('[data-testid="nav-link-tables"]');
    expect(toggle.attributes('aria-expanded')).toBe('false');
    expect(wrapper.get('.dropdown-below').attributes()).toHaveProperty('inert');
    await wrapper.setData({ isExpanded: true });
    await toggle.trigger('click');
    expect(toggle.attributes('aria-expanded')).toBe('true');
    expect(wrapper.get('.dropdown-below').attributes()).not.toHaveProperty('inert');
  });
  it('retains the rail and table choices while focus stays inside it', async () => {
    await openNav();
    vi.useFakeTimers();
    const nav = wrapper.get('nav');
    keyboardFocus(wrapper.get('[data-testid="nav-link-tables"]').element);
    await wrapper.vm.$nextTick();
    await wrapper.get('[data-testid="nav-link-tables"]').trigger('click');
    await nav.trigger('mouseleave');
    await vi.advanceTimersByTimeAsync(200);
    expect(wrapper.vm.railExpanded).toBe(true);
    expect(wrapper.vm.dropdowns.tables).toBe(true);
  });
  it('does not turn pointer-disabled banned actions into keyboard actions', async () => {
    await openNav();
    await wrapper.setData({ isBanned: true });
    expect(wrapper.get('[data-testid="nav-link-center"]').element.disabled).toBe(true);
    expect(wrapper.get('[data-testid="nav-link-cabinet"]').element.disabled).toBe(false);
  });
  it('collapses after pointer departure when a clicked button lacks keyboard focus', async () => {
    await openNav();
    vi.useFakeTimers();
    await wrapper.setData({ isExpanded: true });
    const button = wrapper.get('[data-testid="nav-link-news"]').element;
    const matches = button.matches.bind(button);
    const modality = vi.spyOn(button, 'matches').mockImplementation(selector => selector === ':focus-visible' ? false : matches(selector));
    button.focus();
    await wrapper.get('nav').trigger('mouseleave');
    await vi.advanceTimersByTimeAsync(200);
    expect(wrapper.vm.railExpanded).toBe(false);
    modality.mockRestore();
  });
  it('keeps teleported theme controls open while they contain keyboard focus', async () => {
    await openNav();
    await wrapper.setData({ isExpanded: true });
    await wrapper.get('[data-testid="nav-theme-toggle"]').trigger('click');
    await flushPromises();
    const input = document.querySelector('.theme-options input');
    keyboardFocus(input);
    wrapper.vm.$refs.navigation.matches = () => false;
    document.querySelector('.theme-options').matches = () => false;
    wrapper.vm.collapseMenu();
    await new Promise(resolve => setTimeout(resolve, 180));
    expect(wrapper.vm.isExpanded).toBe(true);
    expect(document.querySelector('.theme-options')).not.toBeNull();
  });
  it('makes administrative entries native buttons with their existing destination', async () => {
    await openNav();
    const trigger = wrapper.get('[data-testid="nav-link-admin"]');
    expect(trigger.attributes('aria-expanded')).toBe('false');
    await trigger.trigger('click');
    expect(trigger.attributes('aria-expanded')).toBe('true');
    const item = wrapper.get('.admin-link');
    expect(item.element.tagName).toBe('BUTTON');
    expect(item.attributes('type')).toBe('button');
    await item.trigger('click');
    expect(push).toHaveBeenCalledOnce();
  });
  it('moves focus into a manually opened drawer', async () => {
    await openNav();
    const outside = document.createElement('button');
    document.body.append(outside);
    outside.focus();
    wrapper.vm.openMobile();
    await flushPromises();
    expect(wrapper.get('.nav-root').element.contains(document.activeElement)).toBe(true);
    outside.remove();
  });
  it('does not take focus when the tour requests a drawer', async () => {
    await openNav();
    const outside = document.createElement('button');
    document.body.append(outside);
    outside.focus();
    const request = { element: wrapper.get('nav').element, token: 1, open: true };
    eventBus.emit('mobile-nav-set', request);
    await flushPromises();
    expect(request.handled).toBe(true);
    expect(document.activeElement).toBe(outside);
    outside.remove();
  });
});
