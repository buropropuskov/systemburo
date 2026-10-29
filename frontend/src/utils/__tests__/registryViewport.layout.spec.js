import { afterEach, describe, expect, it, vi } from 'vitest';
import { applyRegistryViewport, attachRegistryViewport, detachRegistryViewport } from '../registryViewport';
import { notifyPageLayout } from '../pageLayout';

vi.mock('@/utils/viewportScale', () => ({ getViewportZoom: () => 1 }));

let vm;
afterEach(() => { if (vm) detachRegistryViewport(vm); vm = null; vi.unstubAllGlobals(); });

describe('registry route layout', () => {
  it('recovers a height measured while the previous route still occupied the viewport', () => {
    vi.stubGlobal('innerWidth', 1440);
    vi.stubGlobal('innerHeight', 900);
    let top = 950;
    const root = document.createElement('div');
    root.getBoundingClientRect = () => ({ top });
    vm = { $refs: { root }, $nextTick: callback => callback() };
    vm._applyHeight = () => applyRegistryViewport(vm, 'sentinel', () => {});
    attachRegistryViewport(vm);
    expect(root.style.height).toBe('0px');
    top = 60;
    notifyPageLayout();
    expect(root.style.height).toBe('840px');
    detachRegistryViewport(vm);
    top = 100;
    notifyPageLayout();
    expect(root.style.height).toBe('840px');
  });

  it('keeps natural page scrolling on tablet when a route finishes after rotation', () => {
    vi.stubGlobal('innerWidth', 1440);
    vi.stubGlobal('innerHeight', 900);
    const root = document.createElement('div');
    root.getBoundingClientRect = () => ({ top: 60 });
    vm = { $refs: { root }, $nextTick: callback => callback() };
    vm._applyHeight = () => applyRegistryViewport(vm, 'sentinel', () => {});
    attachRegistryViewport(vm);
    expect(root.style.height).toBe('840px');
    vi.stubGlobal('innerWidth', 768);
    window.dispatchEvent(new Event('resize'));
    expect(root.style.height).toBe('');
    notifyPageLayout();
    expect(root.style.height).toBe('');
  });
});
