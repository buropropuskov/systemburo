import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createStepTransition } from '../stepTransition';

const capture = vi.hoisted(() => ({ deps: null, geometry: null }));
vi.mock('../tourGeometry', () => ({ createTourGeometry: deps => { capture.deps = deps; return capture.geometry; } }));
import { createTourGeometryBinding } from '../tourGeometryBinding';

let binding, store, target, driver, generation, busy, transitions, waitForElement, onError;
beforeEach(() => {
  target = document.createElement('button'); target.id = 'binding-target'; document.body.append(target);
  target.getBoundingClientRect = () => ({ top: 20, bottom: 60, left: 20, right: 120, width: 100, height: 40 });
  store = { isActive: true, activeTourKey: 'user', currentIndex: 4,
    steps: Array.from({ length: 5 }, (_, i) => ({ id: 'step-' + i, element: '#binding-target' })),
    setIndex: vi.fn(), markSkipped: vi.fn() };
  generation = 1; busy = false; transitions = createStepTransition({ isActive: () => store.isActive });
  driver = { obGetGlobalIndex: () => 4, getActiveElement: () => target, getState: () => ({}), obGoTo: vi.fn() };
  capture.geometry = { prepare: vi.fn(async frame => frame.target), commit: vi.fn(),
    invalidate: vi.fn(), stop: vi.fn(), dispose: vi.fn() };
  waitForElement = vi.fn(async () => target);
  onError = vi.fn();
  binding = createTourGeometryBinding({ store, getDriver: () => driver, getGeneration: () => generation,
    isBusy: () => busy || transitions.isBusy(), runTransition: transitions.run,
    waitForElement: (...args) => waitForElement(...args), ensureInView: vi.fn(), onError });
});
afterEach(() => {
  transitions.cancel(); binding.dispose(); document.body.replaceChildren(); vi.restoreAllMocks();
});
function request(targetValue = target) {
  return capture.deps.requestRetarget({ target: targetValue, index: 4, stepId: 'step-4', epoch: 99 });
}
function unchangedNavigation() {
  expect(store.currentIndex).toBe(4); expect(store.setIndex).not.toHaveBeenCalled(); expect(store.markSkipped).not.toHaveBeenCalled();
}

describe('geometry binding transition boundaries', () => {
  it('reports a current retarget failure without an unhandled rejected request', async () => {
    const error = new Error('Synthetic scroll failure');
    capture.geometry.prepare.mockRejectedValue(error);
    await expect(request()).resolves.toBeUndefined();
    expect(onError).toHaveBeenCalledExactlyOnceWith(error);
    expect(driver.obGoTo).not.toHaveBeenCalled();
  });
  it('ignores a late rejection from an already cancelled retarget', async () => {
    let reject;
    capture.geometry.prepare.mockImplementation(() => new Promise((_, fail) => { reject = fail; }));
    const pending = request(); transitions.cancel(); reject(new Error('Cancelled synthetic request'));
    await pending; expect(onError).not.toHaveBeenCalled(); expect(driver.obGoTo).not.toHaveBeenCalled();
  });
  it('commits only the actual global Driver index and resolves its current DOM target', () => {
    binding.commit(3); expect(capture.geometry.commit).not.toHaveBeenCalled();
    binding.commit(4);
    const frame = capture.geometry.commit.mock.calls[0][0];
    expect(frame).toEqual(expect.objectContaining({ index: 4, stepId: 'step-4', target, scope: document.body }));
    expect(frame.isCurrent()).toBe(true);
    expect(capture.deps.resolveTarget(frame)).toBe(target);
    target.hidden = true; expect(capture.deps.resolveTarget(frame)).toBeNull();
    unchangedNavigation();
  });
  it.each(['tour', 'generation', 'driver', 'index', 'step', 'inactive'])('invalidates the committed lease after a changed %s', changed => {
    binding.commit(4); const frame = capture.geometry.commit.mock.calls[0][0];
    if (changed === 'tour') store.activeTourKey = 'admin';
    if (changed === 'generation') generation++;
    if (changed === 'driver') driver = { ...driver };
    if (changed === 'index') store.currentIndex = 3;
    if (changed === 'step') store.steps[4] = { id: 'other-step' };
    if (changed === 'inactive') store.isActive = false;
    expect(frame.isCurrent()).toBe(false);
  });
  it('handles a missing anchored target as same-index fallback under B without skip/index mutation', async () => {
    await request(null);
    expect(capture.geometry.stop).toHaveBeenCalledTimes(1);
    expect(capture.geometry.prepare).not.toHaveBeenCalled();
    expect(driver.obGoTo).toHaveBeenCalledWith(4, true);
    unchangedNavigation();
  });
  it('prepares a replacement and rebinds the same index once', async () => {
    await request();
    expect(capture.geometry.prepare).toHaveBeenCalledWith(expect.objectContaining({ target, index: 4, stepId: 'step-4' }),
      expect.objectContaining({ signal: expect.any(AbortSignal), isCurrent: expect.any(Function) }));
    expect(driver.obGoTo).toHaveBeenCalledExactlyOnceWith(4, false); unchangedNavigation();
  });
  it('never commits the old target when geometry preparation returns null', async () => {
    capture.geometry.prepare.mockResolvedValue(null); await request();
    expect(driver.obGoTo).not.toHaveBeenCalled(); unchangedNavigation();
  });
  it('ignores requests and commits while another B transition is busy', async () => {
    let finish; const pending = transitions.run(() => new Promise(resolve => { finish = resolve; }));
    request(); binding.commit(4);
    expect(capture.geometry.stop).not.toHaveBeenCalled(); expect(capture.geometry.commit).not.toHaveBeenCalled();
    expect(driver.obGoTo).not.toHaveBeenCalled(); finish(); await pending; unchangedNavigation();
  });
  it.each(['tour', 'generation', 'driver', 'cancel'])('does not move after deferred refresh and changed %s', async changed => {
    let finish; waitForElement.mockImplementation(() => new Promise(resolve => { finish = resolve; }));
    const originalDriver = driver, pending = binding.refresh(4);
    expect(waitForElement).toHaveBeenCalledWith('#binding-target', 4000, expect.any(AbortSignal));
    if (changed === 'tour') store.activeTourKey = 'admin';
    if (changed === 'generation') generation++;
    if (changed === 'driver') driver = { ...driver, obGoTo: vi.fn() };
    if (changed === 'cancel') transitions.cancel();
    finish(target); await pending;
    expect(originalDriver.obGoTo).not.toHaveBeenCalled(); expect(driver.obGoTo).not.toHaveBeenCalled();
    expect(capture.geometry.prepare).not.toHaveBeenCalled(); unchangedNavigation();
  });
  it('refresh waits then resolves and prepares the actual replacement DOM node', async () => {
    const replacement = document.createElement('button'); replacement.id = 'binding-target';
    replacement.getBoundingClientRect = target.getBoundingClientRect;
    target.remove(); document.body.append(replacement);
    await binding.refresh(4);
    expect(capture.geometry.prepare).toHaveBeenCalledWith(expect.objectContaining({ target: replacement }), expect.any(Object));
    expect(driver.obGoTo).toHaveBeenCalledWith(4, false); unchangedNavigation();
  });
});
