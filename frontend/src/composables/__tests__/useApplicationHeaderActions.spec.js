import { afterEach, describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { defineComponent, h, nextTick, reactive } from 'vue';
import { useApplicationHeaderActions } from '../useApplicationHeaderActions';

let wrapper;
afterEach(() => { wrapper?.unmount(); wrapper = null; vi.restoreAllMocks(); vi.unstubAllGlobals(); });
const nextFrame = () => new Promise(resolve => window.requestAnimationFrame(resolve));

async function fixture({ manual = false, phone = false } = {}) {
  let resize;
  let changeViewport;
  let width = 768;
  vi.stubGlobal('matchMedia', () => ({
    matches: phone,
    addEventListener: (_, callback) => { changeViewport = callback; },
    removeEventListener() {},
  }));
  vi.stubGlobal('ResizeObserver', class {
    constructor(callback) { resize = callback; }
    observe() {}
    disconnect() {}
  });
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockImplementation(() => width);
  const ui = reactive({ tourActive: false });
  const tour = reactive({ isActive: true, revealOpen: '', revealOpenRevision: 0,
    currentStepId: 'actions', currentIndex: 0, stepsRevision: 0,
    steps: [{ reveal: { open: 'application-actions' } }, { reveal: { open: 'first-application' } }] });
  const menu = reactive({ isOpen: manual, disabled: false,
    open: vi.fn(() => { menu.isOpen = true; }), close: vi.fn(() => { menu.isOpen = false; }) });
  const getTour = vi.fn(() => tour);
  let actions;
  wrapper = mount(defineComponent({ setup() {
    actions = useApplicationHeaderActions({ ui, getTour });
    actions.headerActionsMenu.value = menu;
    return () => h('header', { ref: actions.headerRef });
  } }));
  await nextTick();
  function notifyResize(value) { width = value; resize(); }
  return { ui, tour, menu, getTour, actions, notifyResize, changeViewport,
    async resize(value) {
      notifyResize(value);
      await nextFrame();
      await nextTick();
    } };
}

describe('application header action ownership', () => {
  it('keeps the actions visible on a phone and updates after rotation', async () => {
    const f = await fixture({ phone: true });
    await f.resize(390);
    expect(f.actions.headerActionsInline.value).toBe(true);
    f.changeViewport({ matches: false });
    await f.resize(840);
    expect(f.actions.headerActionsInline.value).toBe(false);
    f.changeViewport({ matches: true });
    await f.resize(390);
    expect(f.actions.headerActionsInline.value).toBe(true);
  });
  it('does not load the onboarding store for ordinary card use', async () => {
    const f = await fixture();
    expect(f.getTour).not.toHaveBeenCalled();
    expect(f.actions.headerActionsInline.value).toBe(false);
    await f.resize(1600);
    expect(f.actions.headerActionsInline.value).toBe(true);
  });

  it('measures the latest width in the next frame after resize notifications', async () => {
    const f = await fixture();
    f.notifyResize(1200);
    f.notifyResize(1600);
    expect(f.actions.headerActionsInline.value).toBe(false);
    await nextFrame();
    expect(f.actions.headerActionsInline.value).toBe(true);
  });

  it('retains the menu during step preparation and restores it after the next step commits', async () => {
    const f = await fixture();
    f.ui.tourActive = true; f.tour.revealOpen = 'application-actions'; await nextTick();
    expect(f.menu.isOpen).toBe(true);
    f.tour.revealOpen = 'first-application'; await nextTick();
    expect(f.menu.isOpen).toBe(true);
    f.tour.currentIndex = 1; f.tour.currentStepId = 'body'; await nextTick();
    expect(f.menu.isOpen).toBe(false);
  });

  it('reopens on Retry and closes an owned menu when the tour ends', async () => {
    const f = await fixture();
    f.ui.tourActive = true; f.tour.revealOpen = 'application-actions'; await nextTick();
    f.menu.close(); await nextTick();
    f.tour.revealOpenRevision++; await nextTick();
    expect(f.menu.open).toHaveBeenCalledTimes(2);
    f.ui.tourActive = false; await nextTick();
    expect(f.menu.isOpen).toBe(false);
  });

  it('does not close a menu opened by the user before the tour', async () => {
    const f = await fixture({ manual: true });
    f.ui.tourActive = true; f.tour.revealOpen = 'application-actions'; await nextTick();
    f.ui.tourActive = false; await nextTick();
    expect(f.menu.isOpen).toBe(true);
    expect(f.menu.close).not.toHaveBeenCalled();
  });

  it('waits for available actions and cannot open them after cancellation', async () => {
    const f = await fixture(); f.menu.disabled = true;
    f.ui.tourActive = true; f.tour.revealOpen = 'application-actions'; await nextTick();
    expect(f.menu.open).not.toHaveBeenCalled();
    f.ui.tourActive = false; await nextTick();
    f.menu.disabled = false; await nextTick();
    expect(f.menu.open).not.toHaveBeenCalled();
  });
});
