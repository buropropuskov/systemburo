import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import NewsAndReview from '../NewsAndReview.vue';
import { resetModalStack, setModalOpen, releaseModal } from '@/utils/modalStack';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn(async url => ({ ok: true, json: async () => url === '/news'
  ? [{ id: 1, title: 'Режим работы', created_at: '2026-10-01T09:00:00Z', full_text: '<a href="#details">Подробнее</a>' }]
  : null })) }));
vi.mock('@/services/eventStream', () => ({ default: { connect: vi.fn(), disconnect: vi.fn(), subscribe: vi.fn(() => vi.fn()) } }));
let wrapper;
beforeEach(() => {
  setActivePinia(createPinia());
  vi.spyOn(HTMLElement.prototype, 'getClientRects').mockReturnValue([{}]);
});
afterEach(() => { wrapper?.unmount(); resetModalStack(); document.body.replaceChildren(); vi.restoreAllMocks(); });
async function openNews() {
  wrapper = shallowMount(NewsAndReview, { attachTo: document.body, global: { stubs: { teleport: false } } });
  await flushPromises(); const trigger = wrapper.get('.news-details-button'); trigger.element.focus(); await trigger.trigger('click');
  return trigger.element;
}
function key(key, shiftKey = false) { window.dispatchEvent(new KeyboardEvent('keydown', { key, shiftKey, cancelable: true })); }
describe('news reader focus', () => {
  it('focuses its dialog, cycles through the article and returns to read-more', async () => {
    const trigger = await openNews(), dialog = document.querySelector('.modal-content');
    expect(document.activeElement).toBe(dialog); expect(dialog.getAttribute('role')).toBe('dialog');
    key('Tab'); expect(document.activeElement.textContent).toBe('Подробнее');
    key('Tab'); expect(document.activeElement.classList.contains('close-btn')).toBe(true);
    key('Tab'); expect(document.activeElement.textContent).toBe('Подробнее');
    key('Escape'); await wrapper.vm.$nextTick(); expect(document.activeElement).toBe(trigger);
  });
  it('does not handle the keyboard while a higher dialog owns it', async () => {
    await openNews(); const higher = {}, target = document.createElement('button'); document.body.append(target); target.focus(); setModalOpen(higher, true, 2000);
    key('Tab'); key('Escape'); expect(wrapper.vm.showNewsDetailsModal).toBe(true); expect(document.activeElement).toBe(target);
    releaseModal(higher); key('Escape'); await wrapper.vm.$nextTick(); expect(wrapper.vm.showNewsDetailsModal).toBe(false);
  });
});
