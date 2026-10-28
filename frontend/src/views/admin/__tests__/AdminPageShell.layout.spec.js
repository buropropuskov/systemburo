import { mount, flushPromises } from '@vue/test-utils';
import { afterEach, describe, expect, it, vi } from 'vitest';
import Shell from '../AdminPageShell.vue';
import { notifyPageLayout } from '@/utils/pageLayout';

vi.mock('@/utils/viewportScale', () => ({ getViewportZoom: () => 1 }));

let wrapper;
afterEach(() => { wrapper?.unmount(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

describe('route layout completion', () => {
  it('remeasures after the previous route leaves, without viewport/header resize', async () => {
    let top = 856;
    vi.stubGlobal('innerHeight', 900);
    vi.stubGlobal('matchMedia', () => ({ matches: false }));
    vi.spyOn(Element.prototype, 'getBoundingClientRect').mockImplementation(() => ({ top }));
    wrapper = mount(Shell);
    await flushPromises();
    expect(wrapper.element.style.height).toBe('44px');
    top = 60;
    notifyPageLayout();
    expect(wrapper.element.style.height).toBe('840px');
    notifyPageLayout();
    expect(wrapper.element.style.height).toBe('840px');
    wrapper.unmount(); wrapper = null;
    top = 100;
    notifyPageLayout();
  });

  it('preserves direct load and removes desktop height on phone rotation', async () => {
    let mobile = false;
    vi.stubGlobal('innerHeight', 900);
    vi.stubGlobal('matchMedia', () => ({ matches: mobile }));
    vi.spyOn(Element.prototype, 'getBoundingClientRect').mockReturnValue({ top: 60 });
    wrapper = mount(Shell);
    await flushPromises();
    expect(wrapper.element.style.height).toBe('840px');
    mobile = true;
    window.dispatchEvent(new Event('resize'));
    expect(wrapper.element.style.height).toBe('');
    notifyPageLayout();
    expect(wrapper.element.style.height).toBe('');
  });
});
