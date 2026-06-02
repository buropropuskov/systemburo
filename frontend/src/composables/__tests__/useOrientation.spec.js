import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { defineComponent, h } from 'vue';
import { useOrientation } from '../useOrientation';

describe('useOrientation', () => {
  let originalMatchMedia;
  let originalInnerWidth;
  let mqlListeners;

  beforeEach(() => {
    originalMatchMedia = window.matchMedia;
    originalInnerWidth = window.innerWidth;
    mqlListeners = [];
  });

  afterEach(() => {
    if (originalMatchMedia) {
      window.matchMedia = originalMatchMedia;
    } else {
      delete window.matchMedia;
    }
    Object.defineProperty(window, 'innerWidth', {
      value: originalInnerWidth,
      writable: true,
      configurable: true,
    });
  });

  function mockMatchMedia(portrait) {
    window.matchMedia = vi.fn().mockImplementation(query => ({
      matches: query.includes('portrait') ? portrait : false,
      media: query,
      addEventListener: (ev, cb) => mqlListeners.push(cb),
      removeEventListener: (ev, cb) => {
        const i = mqlListeners.indexOf(cb);
        if (i >= 0) mqlListeners.splice(i, 1);
      },
      addListener: () => {},
      removeListener: () => {},
    }));
  }

  function mountHarness() {
    const harness = defineComponent({
      setup() {
        const { isPortrait, isCompact } = useOrientation();
        return { isPortrait, isCompact };
      },
      render() {
        return h('div', `${this.isPortrait}-${this.isCompact}`);
      },
    });
    return mount(harness);
  }

  it('isPortrait=true и isCompact=true на узком вертикальном экране', () => {
    mockMatchMedia(true);
    Object.defineProperty(window, 'innerWidth', { value: 800, writable: true, configurable: true });
    const w = mountHarness();
    expect(w.vm.isPortrait).toBe(true);
    expect(w.vm.isCompact).toBe(true);
  });

  it('isCompact=false если экран портретный но широкий (>=1100)', () => {
    mockMatchMedia(true);
    Object.defineProperty(window, 'innerWidth', { value: 1200, writable: true, configurable: true });
    const w = mountHarness();
    expect(w.vm.isPortrait).toBe(true);
    expect(w.vm.isCompact).toBe(false);
  });

  it('isPortrait=false в горизонтальной ориентации', () => {
    mockMatchMedia(false);
    Object.defineProperty(window, 'innerWidth', { value: 1920, writable: true, configurable: true });
    const w = mountHarness();
    expect(w.vm.isPortrait).toBe(false);
    expect(w.vm.isCompact).toBe(false);
  });

  it('не падает в окружении без matchMedia', () => {
    delete window.matchMedia;
    expect(() => mountHarness()).not.toThrow();
  });
});
