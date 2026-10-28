import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createTourGeometry } from '../tourGeometry';

let geometry, target, candidate, top, frames, observers, deps, activeIndex;
const originalHeight = window.innerHeight;
function makeTarget() {
  const el = document.createElement('div'); document.body.append(el);
  el.getBoundingClientRect = () => ({ top, bottom: top + 100, left: 20, width: 300, height: 100 });
  return el;
}
function frame() { const callbacks = [...frames.values()]; frames.clear(); callbacks.forEach(callback => callback()); }
async function settle() { for (let i = 0; i < 5; i++) await Promise.resolve(); }
function commit(index = 5) {
  activeIndex = index;
  geometry.commit({ target, scope: document.body, index, stepId: 'detail-status', isCurrent: () => activeIndex === index });
}
beforeEach(() => {
  vi.useFakeTimers(); vi.setSystemTime(new Date('2026-01-15T12:00:00Z'));
  window.innerHeight = 493; frames = new Map(); observers = []; let next = 0; top = 120;
  vi.stubGlobal('requestAnimationFrame', vi.fn(callback => { frames.set(++next, callback); return next; }));
  vi.stubGlobal('cancelAnimationFrame', vi.fn(id => frames.delete(id)));
  const Observer = class {
    constructor(callback) { this.callback = callback; this.observe = vi.fn(); this.unobserve = vi.fn(); this.disconnect = vi.fn(); observers.push(this); }
  };
  vi.stubGlobal('ResizeObserver', Observer); vi.stubGlobal('MutationObserver', Observer);
  target = makeTarget(); candidate = target;
  deps = { getDriver: () => ({ refresh: deps.refresh }), refresh: vi.fn(),
    resolveTarget: () => candidate, shouldScroll: el => el.getBoundingClientRect().bottom > window.innerHeight,
    scrollTarget: vi.fn(() => { top = 150; }), requestRetarget: vi.fn(), onError: vi.fn() };
  geometry = createTourGeometry(deps);
});
afterEach(() => {
  geometry.dispose(); document.body.replaceChildren(); window.innerHeight = originalHeight;
  vi.unstubAllGlobals(); vi.restoreAllMocks(); vi.useRealTimers();
});

describe('legacy layout requirements now owned by geometry', () => {
  it('does not scroll a ready visible target', () => {
    commit(); frame(); expect(deps.scrollTarget).not.toHaveBeenCalled(); expect(deps.refresh).toHaveBeenCalledTimes(1);
  });
  it('corrects later layout growth once and refreshes only after the scroll settles', async () => {
    commit(); frame(); let finish;
    deps.scrollTarget.mockImplementation(() => new Promise(resolve => { finish = () => { top = 150; resolve(); }; }));
    top = 831; observers[0].callback(); frame(); await settle();
    expect(deps.scrollTarget).toHaveBeenCalledTimes(1); expect(deps.refresh).toHaveBeenCalledTimes(1);
    finish(); await settle(); frame(); expect(deps.refresh).toHaveBeenCalledTimes(2);
    expect(top).toBe(150); expect(frames.size).toBe(0);
  });
  it.each(['wheel', 'touchmove'])('does not fight manual %s when content grows', async name => {
    commit(); frame(); target.dispatchEvent(new Event(name, { bubbles: true }));
    top = 900; observers[0].callback(); frame(); await settle(); frame();
    expect(deps.scrollTarget).not.toHaveBeenCalled(); expect(top).toBe(900);
  });
  it('aborts the inflight correction on stop and prevents its late refresh', async () => {
    let signal;
    deps.scrollTarget.mockImplementation((_, __, innerSignal) => new Promise(resolve => {
      signal = innerSignal;
      const timer = setTimeout(() => { top = 150; resolve(); }, 100);
      signal.addEventListener('abort', () => { clearTimeout(timer); resolve(); }, { once: true });
    }));
    top = 831; commit(); frame(); await settle(); expect(signal.aborted).toBe(false);
    geometry.stop(); expect(signal.aborted).toBe(true); expect(vi.getTimerCount()).toBe(0);
    await settle(); frame(); expect(deps.refresh).not.toHaveBeenCalled(); expect(top).toBe(831);
  });
  it('does not loop forever when a target remains unreachable after correction', async () => {
    deps.scrollTarget.mockImplementation(() => {}); top = 831; commit(); frame(); await settle(); frame();
    for (let i = 0; i < 3; i++) { geometry.invalidate('layout'); frame(); await settle(); }
    expect(deps.scrollTarget).toHaveBeenCalledTimes(1); expect(deps.refresh).toHaveBeenCalledTimes(1);
    expect(frames.size).toBe(0); expect(vi.getTimerCount()).toBe(0);
  });
  it('replacement and a new step invalidate old queued geometry callbacks', () => {
    commit(); const stale = [...frames.values()][0]; const oldObserver = observers[0];
    target.remove(); candidate = makeTarget(); target = candidate; commit(6);
    stale(); oldObserver.callback(); frame();
    expect(deps.refresh).toHaveBeenCalledTimes(1); expect(deps.requestRetarget).not.toHaveBeenCalled();
  });
  it('a center step has no scroll target and does not create correction timers', () => {
    target = null; candidate = null; commit(); frame();
    expect(deps.scrollTarget).not.toHaveBeenCalled(); expect(deps.refresh).toHaveBeenCalledTimes(1);
    expect(vi.getTimerCount()).toBe(0);
  });
  function scopedCommit() {
    const scope = document.createElement('section'); document.body.append(scope); scope.append(target);
    const sibling = document.createElement('div'); document.body.append(sibling);
    activeIndex = 5;
    geometry.commit({ target, scope, index: 5, stepId: 'detail-status', isCurrent: () => activeIndex === 5 });
    frame(); return sibling;
  }
  it.each(['wheel', 'touchmove'])('respects manual %s on a sibling outside the target scope', async name => {
    const sibling = scopedCommit();
    const event = new Event(name, { bubbles: true, cancelable: true }); sibling.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(false);
    top = 950; observers[0].callback(); frame(); await settle(); frame();
    expect(deps.scrollTarget).not.toHaveBeenCalled(); expect(top).toBe(950);
    expect(deps.refresh).toHaveBeenCalledTimes(2);
  });
  it('treats PageDown on a non-editable sibling as manual scroll intent', async () => {
    const sibling = scopedCommit();
    sibling.dispatchEvent(new KeyboardEvent('keydown', { key: 'PageDown', bubbles: true }));
    top = 950; geometry.invalidate('layout'); frame(); await settle(); frame();
    expect(deps.scrollTarget).not.toHaveBeenCalled(); expect(top).toBe(950);
  });
  it('manual wheel aborts an inflight correction without cancelling the active geometry lease', async () => {
    const sibling = scopedCommit(); let signal;
    deps.scrollTarget.mockImplementation((_, __, innerSignal) => new Promise((resolve, reject) => {
      signal = innerSignal;
      const timer = setTimeout(() => { top = 150; resolve(); }, 100);
      signal.addEventListener('abort', () => {
        clearTimeout(timer); reject(new DOMException('cancelled', 'AbortError'));
      }, { once: true });
    }));
    top = 831; geometry.invalidate('layout'); frame(); await settle();
    expect(signal.aborted).toBe(false); expect(vi.getTimerCount()).toBe(1);
    sibling.dispatchEvent(new Event('wheel', { bubbles: true }));
    expect(signal.aborted).toBe(true); expect(vi.getTimerCount()).toBe(0);
    await settle(); frame(); await vi.advanceTimersByTimeAsync(100);
    expect(top).toBe(831); expect(deps.onError).not.toHaveBeenCalled();
    top = 1050; observers[0].callback(); frame(); await settle(); frame();
    expect(deps.scrollTarget).toHaveBeenCalledTimes(1); expect(top).toBe(1050);
    expect(deps.refresh).toHaveBeenCalledTimes(3);
    observers.forEach(observer => expect(observer.disconnect).not.toHaveBeenCalled());
  });
});
