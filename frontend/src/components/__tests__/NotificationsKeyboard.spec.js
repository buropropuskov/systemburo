import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { nextTick } from 'vue';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(async () => ({ ok: true, json: async () => [] })),
  apiRequestRaw: vi.fn(),
}));
vi.mock('@/services/eventStream', () => ({
  default: { connect: vi.fn(), disconnect: vi.fn(), subscribe: () => vi.fn(), onStatus: () => vi.fn() },
}));
import { apiRequest, apiRequestRaw } from '@/api/client';
import { useOnboardingStore } from '@/stores/onboarding';
import { resetModalStack } from '@/utils/modalStack';
import UserNotifications from '../UserNotifications.vue';
import UserNotificationsInline from '../UserNotificationsInline.vue';

const wrappers = [];
let mediaChange;
const originalMatchMedia = window.matchMedia;
beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  vi.spyOn(HTMLElement.prototype, 'getClientRects').mockReturnValue([{}]);
  window.matchMedia = vi.fn(() => ({
    matches: true, addEventListener: (_name, callback) => { mediaChange = callback; },
    removeEventListener: vi.fn(),
  }));
  apiRequestRaw.mockResolvedValue({
    ok: true,
    json: async () => ({ success: true, data: [{ id: 1, is_read: true, title: 'Сообщение', message: 'Текст', created_at: '2030-01-01T12:00:00Z' }], meta: { total: 1, unread_count: 0 } }),
  });
});
afterEach(() => {
  wrappers.reverse().forEach(w => w.unmount());
  wrappers.length = 0;
  window.matchMedia = originalMatchMedia;
  resetModalStack();
  document.body.replaceChildren();
  vi.restoreAllMocks();
});
async function mountPanel(Component = UserNotifications) {
  const trigger = document.createElement('button');
  document.body.append(trigger);
  trigger.focus();
  const wrapper = mount(Component, {
    props: Component === UserNotifications ? { show: false } : {},
    attachTo: document.body,
    global: { mocks: { $router: { push: vi.fn(async () => {}) } } },
  });
  wrappers.push(wrapper);
  await flushPromises();
  if (Component === UserNotifications) await wrapper.setProps({ show: true });
  await flushPromises();
  return { wrapper, trigger };
}
function tab(shiftKey = false) {
  document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey, bubbles: true, cancelable: true }));
}
describe('Notification opening and mobile focus', () => {
  it.each([UserNotifications, UserNotificationsInline])('uses a separate native opening button in $name', async Component => {
    const { wrapper } = await mountPanel(Component);
    const button = document.querySelector('.notification-item__content, .notification-content');
    expect(button.tagName).toBe('BUTTON');
    expect(button.type).toBe('button');
    expect(button.querySelector('button')).toBeNull();
    button.focus();
    button.click();
    await flushPromises();
    expect(document.querySelector('[data-testid="notif-detail-dialog"]')).not.toBeNull();
    expect(apiRequest).not.toHaveBeenCalled();
    wrapper.vm.showDetailModal = false;
    await nextTick();
    expect(document.activeElement).toBe(button);
  });
  it('keeps deletion separate from opening', async () => {
    const { wrapper } = await mountPanel(UserNotificationsInline);
    const remove = vi.spyOn(wrapper.vm, 'deleteNotification').mockImplementation(() => {});
    document.querySelector('.delete-btn').click();
    await nextTick();
    expect(remove).toHaveBeenCalledOnce();
    expect(wrapper.vm.showDetailModal).toBe(false);
  });
  it('marks an unread notification once when its opening button is activated', async () => {
    const { wrapper } = await mountPanel();
    wrapper.vm.notifications[0].is_read = false;
    await nextTick();
    const button = document.querySelector('.notification-item__content');
    button.focus();
    button.click();
    await flushPromises();
    const reads = apiRequest.mock.calls.filter(([path]) => path === '/notifications/1/read');
    expect(reads).toHaveLength(1);
    expect(reads[0][1].method).toBe('PUT');
    expect(wrapper.vm.notifications[0].is_read).toBe(true);
    expect(wrapper.vm.showDetailModal).toBe(true);
  });
  it('wraps Tab in the sheet and restores its opener after closing', async () => {
    const { wrapper, trigger } = await mountPanel();
    const panel = document.querySelector('.notifications');
    expect(document.activeElement).toBe(panel);
    expect(panel.getAttribute('role')).toBe('dialog');
    const buttons = [...panel.querySelectorAll('button')].filter(el => !el.disabled);
    tab(true);
    expect(document.activeElement).toBe(buttons.at(-1));
    tab();
    expect(document.activeElement).toBe(buttons[0]);
    await wrapper.setProps({ show: false });
    expect(document.activeElement).toBe(trigger);
  });
  it('lets Escape close only the child and returns to the notification button', async () => {
    const { wrapper } = await mountPanel();
    const button = document.querySelector('.notification-item__content');
    button.focus();
    button.click();
    await flushPromises();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await flushPromises();
    expect(wrapper.emitted('close')).toBeUndefined();
    expect(wrapper.vm.showDetailModal).toBe(false);
    expect(document.activeElement).toBe(button);
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(wrapper.emitted('close')).toHaveLength(1);
  });
  it('releases the sheet focus when resizing to desktop without trapping the dropdown', async () => {
    const { trigger } = await mountPanel();
    mediaChange({ matches: false });
    await nextTick();
    expect(document.activeElement).toBe(trigger);
    expect(document.querySelector('.notifications').hasAttribute('aria-modal')).toBe(false);
    const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true });
    trigger.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(false);
  });
  it('does not move focus when the tour owns the notification panel', async () => {
    useOnboardingStore().revealOpen = 'notifications';
    const { trigger } = await mountPanel();
    expect(document.activeElement).toBe(trigger);
  });
  it('focuses the sheet again when an open dropdown returns from desktop to mobile', async () => {
    const { trigger } = await mountPanel();
    mediaChange({ matches: false });
    await flushPromises();
    expect(document.activeElement).toBe(trigger);
    mediaChange({ matches: true });
    await flushPromises();
    expect(document.activeElement).toBe(document.querySelector('.notifications'));
  });
});
