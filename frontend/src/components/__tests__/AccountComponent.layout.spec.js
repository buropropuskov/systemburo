import { shallowMount, flushPromises } from '@vue/test-utils';
import { afterEach, describe, expect, it, vi } from 'vitest';
import Account from '../AccountComponent.vue';
import { notifyPageLayout } from '@/utils/pageLayout';

vi.mock('@/utils/viewportScale', () => ({ getViewportZoom: () => 1 }));

let wrapper;
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

async function mountDashboard(top) {
  vi.stubGlobal('innerHeight', 900);
  vi.stubGlobal('innerWidth', 1440);
  vi.spyOn(Account.methods, 'fetchUserData').mockImplementation(() => {});
  vi.spyOn(Element.prototype, 'getBoundingClientRect').mockImplementation(() => ({ top: top.value }));
  wrapper = shallowMount(Account);
  await flushPromises();
}

describe('cabinet route layout completion', () => {
  it('recovers its height after the outgoing route leaves without resizing the viewport', async () => {
    const top = { value: 900 };
    await mountDashboard(top);
    expect(wrapper.element.style.height).toBe('0px');
    top.value = 60;
    notifyPageLayout();
    expect(wrapper.element.style.height).toBe('825px');
    notifyPageLayout();
    expect(wrapper.element.style.height).toBe('825px');
    wrapper.unmount(); wrapper = null;
    expect(() => notifyPageLayout()).not.toThrow();
  });

  it('keeps natural height on tablet after rotation and subsequent route layout events', async () => {
    await mountDashboard({ value: 60 });
    expect(wrapper.element.style.height).toBe('825px');
    vi.stubGlobal('innerWidth', 768);
    window.dispatchEvent(new Event('resize'));
    expect(wrapper.element.style.height).toBe('');
    notifyPageLayout();
    expect(wrapper.element.style.height).toBe('');
  });
});
