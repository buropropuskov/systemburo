import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createStepTransition } from '../stepTransition';
const fixture = vi.hoisted(() => ({ deps: null, geometry: null, drawer: false, narrow: false, reveal: vi.fn() }));
vi.mock('../tourGeometry', () => ({ createTourGeometry: deps => { fixture.deps = deps; return fixture.geometry; } }));
vi.mock('../reveal', () => ({
  applyReveal: (...args) => fixture.reveal(...args),
  isNavDrawerOpen: () => fixture.drawer,
  resolveReveal: (steps, index) => ({ mobile: steps[index]?.reveal?.mobile || null }),
}));
vi.mock('@/constants/breakpoints', () => ({ isDrawerViewport: () => fixture.narrow }));
import { createTourGeometryBinding } from '../tourGeometryBinding';

let binding, transitions, store, target, driver, wait, generation;
beforeEach(() => {
  fixture.drawer = true; fixture.narrow = true; fixture.reveal.mockReset();
  fixture.reveal.mockImplementation(async () => { fixture.drawer = true; });
  target = document.createElement('nav'); target.id = 'rotation-target'; document.body.append(target);
  target.getBoundingClientRect = () => ({ top: 0, left: fixture.drawer ? 0 : -280, width: 280, height: 720 });
  store = { isActive: true, currentIndex: 0, activeTourKey: 'user', steps: [{ id: 'nav', element: '#rotation-target', reveal: { mobile: 'nav' } }], setIndex: vi.fn(), markSkipped: vi.fn() };
  generation = 1;
  fixture.geometry = { prepare: vi.fn(async frame => frame.target), stop: vi.fn(), commit: vi.fn(), dispose: vi.fn(), invalidate: vi.fn() };
  transitions = createStepTransition({ isActive: () => store.isActive });
  driver = { obGetGlobalIndex: () => 0, obGoTo: vi.fn(), getState: () => ({}) };
  wait = vi.fn(async () => target);
  binding = createTourGeometryBinding({ store, getDriver: () => driver, getGeneration: () => generation, isBusy: transitions.isBusy, runTransition: transitions.run, waitForElement: wait, ensureInView: vi.fn() });
});
afterEach(() => { transitions.cancel(); binding.dispose(); document.body.replaceChildren(); });
const frame = () => ({ target, index: 0, stepId: 'nav' });
describe('active navigation reveal after drawer breakpoint rotation', () => {
  it('reopens the same step after 390/768/1024 -> desktop -> 1280, before measurement', async () => {
    for (const width of [390, 768, 1024]) {
      fixture.narrow = true;
      expect(fixture.deps.beforeMeasure(frame(), new Set(['viewport']))).toBe(true);
      expect(fixture.reveal).not.toHaveBeenCalled();
      expect(width).toBeLessThan(1367);
    }
    fixture.narrow = false; fixture.drawer = false;
    expect(fixture.deps.beforeMeasure(frame(), new Set(['viewport']))).toBe(true);
    fixture.narrow = true;
    expect(fixture.deps.beforeMeasure(frame(), new Set(['viewport']))).toBe(false);
    await vi.waitFor(() => expect(driver.obGoTo).toHaveBeenCalledExactlyOnceWith(0, false));
    expect(fixture.reveal).toHaveBeenCalledWith(store.steps, 0, expect.objectContaining({ closeOthers: false, signal: expect.any(AbortSignal) }));
    expect(wait).toHaveBeenCalledWith('#rotation-target', 4000, expect.any(AbortSignal));
    expect(fixture.geometry.prepare).toHaveBeenCalledWith(expect.objectContaining({ target, stepId: 'nav' }), expect.any(Object));
    expect(store.setIndex).not.toHaveBeenCalled(); expect(store.markSkipped).not.toHaveBeenCalled();
  });
  it('does not claim or close an already user-open drawer', () => {
    expect(fixture.deps.beforeMeasure(frame(), new Set(['viewport']))).toBe(true);
    expect(fixture.reveal).not.toHaveBeenCalled(); expect(driver.obGoTo).not.toHaveBeenCalled();
  });
  it('does not restore drawers for unrelated steps or non-viewport changes', () => {
    fixture.drawer = false;
    expect(fixture.deps.beforeMeasure(frame(), new Set(['mutation']))).toBe(true);
    store.steps[0].reveal = {};
    expect(fixture.deps.beforeMeasure(frame(), new Set(['viewport']))).toBe(true);
    expect(fixture.reveal).not.toHaveBeenCalled();
  });
  it('cannot commit after cancellation during reveal', async () => {
    let resolve;
    fixture.reveal.mockImplementation(() => new Promise(done => { resolve = done; }));
    fixture.drawer = false;
    fixture.deps.beforeMeasure(frame(), new Set(['viewport']));
    transitions.cancel(); generation++; resolve();
    await Promise.resolve(); await Promise.resolve();
    expect(wait).not.toHaveBeenCalled(); expect(driver.obGoTo).not.toHaveBeenCalled();
  });
});
