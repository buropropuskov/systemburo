import { afterEach, describe, expect, it, vi } from 'vitest';
import { defineComponent, getCurrentInstance } from 'vue';
import { mount } from '@vue/test-utils';
import bus from '@/eventBus';
import { useMobileDrawer } from '../useMobileDrawer';

let token = 0;
let activeWrapper;
const wrappers = [];
function mounted() {
  const close = vi.fn();
  const Theme = defineComponent({ setup(_, { expose }) { expose({ close }); return () => null; } });
  const wrapper = mount(defineComponent({ components: { Theme }, data: () => ({ mobileOpen: false }),
    setup() { return useMobileDrawer(getCurrentInstance()); }, template: '<nav ref="navigation"></nav><Theme ref="themeControls" />' }));
  activeWrapper = wrapper;
  wrappers.push(wrapper); return { wrapper, close };
}
function request(open, id = ++token, restore = false, element = activeWrapper.vm.$refs.navigation, previousToken) {
  const event = { element, token: id, previousToken, open, restore, handled: false }; bus.emit('mobile-nav-set', event);
  return event;
}
afterEach(() => {
  wrappers.splice(0).forEach(wrapper => { if (!wrapper.vm.$.isUnmounted) wrapper.unmount(); });
  document.body.classList.remove('nav-drawer-open'); vi.restoreAllMocks();
});
describe('mobile drawer lifecycle and event protocol', () => {
  it('synchronously sets tour state and restores a closed drawer without a double toggle', () => {
    const { wrapper, close } = mounted(); const id = ++token;
    expect(wrapper.vm.mobileOpen).toBe(false);
    expect(request(true, id).handled).toBe(true); request(true, id);
    expect(wrapper.vm.mobileOpen).toBe(true); expect(document.body.classList.contains('nav-drawer-open')).toBe(true);
    request(false, id, true); expect(wrapper.vm.mobileOpen).toBe(false); expect(close).toHaveBeenCalledOnce();
  });
  it('leaves user preopened state intact, including temporary tour close and restore', () => {
    const { wrapper } = mounted(); wrapper.vm.openMobile(); const id = ++token;
    request(true, id); request(false, id, true); expect(wrapper.vm.mobileOpen).toBe(true);
    const next = ++token; request(false, next); expect(wrapper.vm.mobileOpen).toBe(false);
    request(false, next, true); expect(wrapper.vm.mobileOpen).toBe(true);
  });
  it('manual close and reopen cancel restoration even when the final boolean matches', () => {
    const { wrapper } = mounted(); const id = ++token; request(true, id);
    bus.emit('mobile-nav-toggle'); bus.emit('mobile-nav-toggle'); request(false, id, true);
    expect(wrapper.vm.mobileOpen).toBe(true);
  });
  it.each(['route', 'breakpoint'])('%s close through the setter revokes the old tour lease', () => {
    const { wrapper } = mounted(); wrapper.vm.openMobile(); const id = ++token; request(false, id);
    wrapper.vm.setMobileOpen(false); request(false, id, true); expect(wrapper.vm.mobileOpen).toBe(false);
  });
  it('idempotent manual setters repair body lock and close theme controls', () => {
    const { wrapper, close } = mounted(); wrapper.vm.openMobile(); document.body.classList.remove('nav-drawer-open');
    wrapper.vm.setMobileOpen(true); expect(document.body.classList.contains('nav-drawer-open')).toBe(true);
    wrapper.vm.setMobileOpen(false); document.body.classList.add('nav-drawer-open'); wrapper.vm.setMobileOpen(false);
    expect(document.body.classList.contains('nav-drawer-open')).toBe(false); expect(close).toHaveBeenCalledTimes(2);
  });
  it('unmount removes subscriptions, its lock, and rejects old requests after remount', () => {
    const { wrapper } = mounted(); const oldElement = wrapper.vm.$refs.navigation;
    const id = ++token; request(true, id); wrapper.unmount();
    expect(document.body.classList.contains('nav-drawer-open')).toBe(false);
    expect(request(false, id, true, oldElement).handled).toBe(false);
    const next = mounted().wrapper; expect(request(true, id, false, oldElement).handled).toBe(false);
    expect(next.vm.mobileOpen).toBe(false);
    request(true, ++token); expect(next.vm.mobileOpen).toBe(true);
  });
  it('preserves the snapshot as new tokens explicitly succeed the previous request', () => {
    const { wrapper } = mounted(); const first = ++token, second = ++token;
    request(true, first); request(true, second, false, wrapper.vm.$refs.navigation, first);
    request(false, second, true); expect(wrapper.vm.mobileOpen).toBe(false);
  });
  it('an old component unmount cannot remove a newer component body lock', () => {
    const first = mounted().wrapper; first.vm.openMobile(); const second = mounted().wrapper; second.vm.openMobile();
    first.unmount(); expect(document.body.classList.contains('nav-drawer-open')).toBe(true);
  });
  it.each(['first', 'second'])('closing the %s open instance preserves the other instance lock', closed => {
    const first = mounted().wrapper; first.vm.openMobile();
    const second = mounted().wrapper; second.vm.openMobile();
    const closing = closed === 'first' ? first : second;
    const remaining = closed === 'first' ? second : first;
    closing.vm.setMobileOpen(false);
    expect(document.body.classList.contains('nav-drawer-open')).toBe(true);
    expect(remaining.vm.mobileOpen).toBe(true);
    remaining.vm.setMobileOpen(false);
    expect(document.body.classList.contains('nav-drawer-open')).toBe(false);
  });
  it('unmount removes only its registration and the last open instance releases the lock', () => {
    const first = mounted().wrapper; first.vm.openMobile();
    const second = mounted().wrapper; second.vm.openMobile();
    first.unmount(); expect(document.body.classList.contains('nav-drawer-open')).toBe(true);
    second.vm.setMobileOpen(false); expect(document.body.classList.contains('nav-drawer-open')).toBe(false);
  });
  it('unmount of an instance with no registration leaves a preexisting body lock alone', () => {
    document.body.classList.add('nav-drawer-open'); const { wrapper } = mounted();
    wrapper.unmount(); expect(document.body.classList.contains('nav-drawer-open')).toBe(true);
  });
});
