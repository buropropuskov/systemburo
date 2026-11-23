import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { useUiStore } from '@/stores/ui';
import PostRemovalAction from '../PostRemovalAction.vue';

enableAutoUnmount(afterEach);
const posts = [4, 7, 9].map(id => ({ id, name: `КПП №${id}` }));
let confirm;
beforeEach(() => {
  setActivePinia(createPinia());
  confirm = vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);
});
function render(admin = false, extra = {}) {
  return mount(PostRemovalAction, { props: { tableId: 4, label: 'Учебная запись', tables: posts, admin, ...extra }, global: { stubs: { teleport: true, transition: false } } });
}
describe('PostRemovalAction', () => {
  it('one-post operator can only remove the current post even with several bindings', async () => {
    const wrapper = render();
    expect(wrapper.find('[data-testid="row-remove-trigger"]').exists()).toBe(false);
    await wrapper.get('button').trigger('click');
    await flushPromises();
    expect(wrapper.emitted('remove')).toEqual([[{ tableIds: [4], all: false }]]);
    expect(confirm.mock.calls[0][0].message).toContain('КПП №4');
    expect(confirm.mock.calls[0][0].message).not.toContain('КПП №7');
  });
  it('administrator confirms the exact complete set', async () => {
    const wrapper = render(true);
    await wrapper.get('[data-testid="row-remove-trigger"]').trigger('click');
    await wrapper.get('[data-testid="row-remove-all"]').trigger('click');
    await flushPromises();
    expect(wrapper.emitted('remove')).toEqual([[{ tableIds: [4, 7, 9], all: true }]]);
    expect(confirm.mock.calls[0][0].message).toContain('КПП №4, КПП №7, КПП №9');
    expect(confirm.mock.calls[0][0].message).toContain('запись перестанет действовать');
  });
  it('choosing two posts keeps the third out of the request', async () => {
    const wrapper = render(true);
    await wrapper.get('[data-testid="row-remove-trigger"]').trigger('click');
    await wrapper.get('[data-testid="row-remove-selected"]').trigger('click');
    await flushPromises();
    const modal = wrapper.get('[role="dialog"]');
    await modal.findAll('.passage__item')[1].trigger('click');
    await modal.get('.lk-button--primary').trigger('click');
    await flushPromises();
    expect(wrapper.emitted('remove')).toEqual([[{ tableIds: [4, 7], all: false }]]);
  });
  it('cancelling the confirmation emits no removal', async () => {
    confirm.mockResolvedValue(false);
    const wrapper = render();
    await wrapper.get('button').trigger('click');
    await flushPromises();
    expect(wrapper.emitted('remove')).toBeUndefined();
  });
  it('disabled and missing-current-post states do not submit', async () => {
    for (const extra of [{ disabled: true }, { tableId: null }]) {
      const wrapper = render(false, extra);
      expect(wrapper.get('button').attributes('disabled')).toBeDefined();
      await wrapper.get('button').trigger('click');
      expect(wrapper.emitted('remove')).toBeUndefined();
    }
    expect(confirm).not.toHaveBeenCalled();
  });
  it('Escape closes only the post menu and returns focus', async () => {
    const wrapper = render(true);
    await wrapper.get('[data-testid="row-remove-trigger"]').trigger('click');
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await flushPromises();
    expect(wrapper.find('[data-testid="row-remove-all"]').exists()).toBe(false);
    expect(wrapper.emitted('remove')).toBeUndefined();
  });
});
