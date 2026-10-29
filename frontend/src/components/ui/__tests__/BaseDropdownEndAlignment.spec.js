import { afterEach, describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import BaseDropdown from '../BaseDropdown.vue';
vi.mock('@/utils/viewportScale', () => ({ getViewportZoom: () => 1 }));
let wrapper;
afterEach(() => { wrapper?.unmount(); vi.unstubAllGlobals(); });
async function position(left, menuAlign) {
  vi.stubGlobal('innerWidth', 768);
  wrapper = mount(BaseDropdown, { props: { options: [], teleport: true, menuMinWidth: 300, ...(menuAlign ? { menuAlign } : {}) } });
  wrapper.vm.$refs.dropdown.getBoundingClientRect = () => ({ left, top: 100, bottom: 130, width: 130, height: 30 });
  await wrapper.get('.base-dropdown__button').trigger('click');
  return Number.parseFloat(wrapper.vm.menuStyle.left);
}
describe('dropdown alignment to its owning trigger', () => {
  it('ends at the trigger edge rather than extending into the search rail', async () => {
    expect(await position(560, 'end')).toBe(390);
    expect(Number.parseFloat(wrapper.vm.menuStyle.width) + Number.parseFloat(wrapper.vm.menuStyle.left)).toBe(690);
  });
  it('keeps the end-aligned menu inside the left viewport edge', async () => {
    expect(await position(15, 'end')).toBe(8);
  });
  it('preserves the existing default alignment', async () => {
    expect(await position(560)).toBe(460);
  });
});
