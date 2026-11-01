import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { nextTick } from 'vue';
import NotificationDetailModal from '../NotificationDetailModal.vue';
import { resetModalStack, setModalOpen, releaseModal } from '@/utils/modalStack';

const wrappers = [];
beforeEach(() => vi.spyOn(HTMLElement.prototype, 'getClientRects').mockReturnValue([{}]));
afterEach(() => {
  wrappers.reverse().forEach(w => w.unmount());
  wrappers.length = 0;
  resetModalStack();
  document.body.replaceChildren();
  vi.restoreAllMocks();
});
function open() {
  const trigger = document.createElement('button');
  document.body.append(trigger);
  trigger.focus();
  const wrapper = mount(NotificationDetailModal, {
    props: { show: true, notification: { title: 'Сообщение', message: 'Текст' } },
    attachTo: document.body,
  });
  wrappers.push(wrapper);
  return { wrapper, trigger };
}
function tab(shiftKey = false) {
  document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey, bubbles: true, cancelable: true }));
}
describe('Notification details focus', () => {
  it('focuses the container, wraps both Tab directions and returns to its trigger', async () => {
    const { wrapper, trigger } = open();
    await nextTick();
    const dialog = document.querySelector('[data-testid="notif-detail-dialog"]');
    const buttons = [...dialog.querySelectorAll('button')];
    expect(document.activeElement).toBe(dialog);
    tab(true);
    expect(document.activeElement).toBe(buttons.at(-1));
    tab();
    expect(document.activeElement).toBe(buttons[0]);
    await wrapper.setProps({ show: false });
    expect(document.activeElement).toBe(trigger);
  });
  it('returns focus when the open component is removed', async () => {
    const { wrapper, trigger } = open();
    await nextTick();
    wrapper.unmount();
    wrappers.pop();
    expect(document.activeElement).toBe(trigger);
  });
  it('does not steal focus from a higher modal or the onboarding tour', async () => {
    const upper = {};
    setModalOpen(upper, true, 12502);
    const { wrapper, trigger } = open();
    await nextTick();
    expect(document.activeElement).toBe(trigger);
    await wrapper.setProps({ show: false });
    expect(document.activeElement).toBe(trigger);
    releaseModal(upper);
  });
});
