import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createTourGeometry } from '../tourGeometry';

let geometry, scope, target, rect, frames, nextFrame, observers, deps, candidate, current;
function element(parent = document.body) {
  const el = document.createElement('div'); parent.append(el);
  el.getBoundingClientRect = () => ({ top: rect.top, left: 10, width: 100, height: 40 });
  return el;
}
function flush() {
  const queued = [...frames.values()]; frames.clear(); queued.forEach(callback => callback());
}
function commit(overrides = {}) {
  geometry.commit({ target, scope, index: 4, stepId: 'step', isCurrent: () => current, ...overrides });
}
beforeEach(() => {
  vi.useFakeTimers(); vi.setSystemTime(new Date('2026-01-15T12:00:00Z'));
  frames = new Map(); nextFrame = 0; observers = []; rect = { top: 10 }; current = true;
  vi.stubGlobal('requestAnimationFrame', vi.fn(callback => { frames.set(++nextFrame, callback); return nextFrame; }));
  vi.stubGlobal('cancelAnimationFrame', vi.fn(id => frames.delete(id)));
  const Observer = class {
    constructor(callback) { this.callback = callback; this.observe = vi.fn(); this.unobserve = vi.fn(); this.disconnect = vi.fn(); observers.push(this); }
  };
  vi.stubGlobal('ResizeObserver', Observer); vi.stubGlobal('MutationObserver', Observer);
  scope = element(); target = element(scope); candidate = target;
  deps = { getDriver: () => ({ refresh: deps.refresh }), refresh: vi.fn(),
    resolveTarget: vi.fn(() => candidate), scrollTarget: vi.fn(),
    applyShape: vi.fn(), correctPopover: vi.fn(), requestRetarget: vi.fn() };
  geometry = createTourGeometry(deps);
});
afterEach(() => {
  geometry.dispose(); document.body.replaceChildren(); vi.unstubAllGlobals(); vi.restoreAllMocks(); vi.useRealTimers();
});

describe('geometry leases', () => {
  it('coalesces invalidations and does not refresh unchanged observer/style feedback', () => {
    commit(); geometry.invalidate(); geometry.invalidate(); expect(frames.size).toBe(1);
    flush(); expect(deps.refresh).toHaveBeenCalledTimes(1);
    observers.forEach(observer => observer.callback()); flush();
    expect(deps.refresh).toHaveBeenCalledTimes(1); expect(frames.size).toBe(0);
  });
  it('refreshes actual same-target geometry changes without retarget', () => {
    commit(); flush(); rect.top = 35; observers[0].callback(); flush();
    expect(deps.refresh).toHaveBeenCalledTimes(2); expect(deps.requestRetarget).not.toHaveBeenCalled();
  });
  it('ignores animated popover dimensions including wrapping but refreshes changed content', () => {
    const popover = element(); let left = 10, width = 380, height = 260;
    popover.getBoundingClientRect = () => ({ top: 400, left, width, height });
    deps.getDriver = () => ({ refresh: deps.refresh, getState: () => ({ popover: { wrapper: popover } }) });
    geometry.dispose(); geometry = createTourGeometry(deps); commit(); flush();
    for (let frame = 0; frame < 20; frame += 1) {
      left -= 0.25; width += 0.25; height += 2; observers[0].callback(); flush();
    }
    expect(deps.refresh).toHaveBeenCalledTimes(1);
    popover.textContent = 'Additional instructions'; observers[0].callback(); flush();
    expect(deps.refresh).toHaveBeenCalledTimes(2);
  });
  it('repositions when a popover image loads without changing the text', () => {
    const popover = element(), image = document.createElement('img'); popover.append(image);
    let imageWidth = 0; Object.defineProperty(image, 'naturalWidth', { get: () => imageWidth });
    deps.getDriver = () => ({ refresh: deps.refresh, getState: () => ({ popover: { wrapper: popover } }) });
    geometry.dispose(); geometry = createTourGeometry(deps); commit(); flush();
    imageWidth = 200; observers[0].callback(); flush();
    expect(deps.refresh).toHaveBeenCalledTimes(2);
  });
  it('keeps font invalidation until an in-flight scroll correction completes', async () => {
    const fonts = new EventTarget(), previous = Object.getOwnPropertyDescriptor(document, 'fonts');
    Object.defineProperty(document, 'fonts', { configurable: true, value: fonts });
    let finish, needsScroll = false;
    deps.shouldScroll = () => needsScroll;
    deps.scrollTarget.mockImplementation(() => new Promise(resolve => { finish = resolve; }));
    geometry.dispose(); geometry = createTourGeometry(deps);
    try {
      commit(); flush(); needsScroll = true; geometry.invalidate(); flush(); await Promise.resolve();
      fonts.dispatchEvent(new Event('loadingdone')); flush();
      expect(deps.refresh).toHaveBeenCalledTimes(1);
      needsScroll = false; finish();
      for (let turn = 0; turn < 5; turn += 1) await Promise.resolve();
      flush(); expect(deps.refresh).toHaveBeenCalledTimes(2);
    } finally {
      if (previous) Object.defineProperty(document, 'fonts', previous);
      else delete document.fonts;
    }
  });
  it.each(['hidden', 'removed'])('requests a replacement once for a %s target without writing stale geometry', state => {
    commit(); flush(); if (state === 'hidden') target.hidden = true; else target.remove();
    candidate = element(scope); observers[1].callback(); flush(); geometry.invalidate(); flush();
    expect(deps.requestRetarget).toHaveBeenCalledTimes(1);
    expect(deps.requestRetarget).toHaveBeenCalledWith(expect.objectContaining({ target: candidate, index: 4, stepId: 'step' }));
    expect(deps.refresh).toHaveBeenCalledTimes(1);
  });
  it('notifies missing anchored targets once without stale writes and reports recovery', () => {
    commit(); flush(); target.hidden = true; geometry.invalidate(); flush();
    geometry.invalidate(); flush();
    expect(deps.requestRetarget).toHaveBeenCalledTimes(1);
    expect(deps.requestRetarget).toHaveBeenCalledWith(expect.objectContaining({ target: null, reason: 'target-missing', index: 4 }));
    target.remove(); candidate = null; geometry.invalidate(); flush();
    expect(deps.refresh).toHaveBeenCalledTimes(1); expect(deps.requestRetarget).toHaveBeenCalledTimes(1);
    candidate = element(scope); geometry.invalidate(); flush();
    expect(deps.requestRetarget).toHaveBeenLastCalledWith(expect.objectContaining({ target: candidate, reason: 'target-recovered' }));
  });
  it('reports recovery when the same hidden element becomes visible again', () => {
    commit(); flush(); target.hidden = true; geometry.invalidate(); flush();
    target.hidden = false; geometry.invalidate(); flush();
    expect(deps.requestRetarget).toHaveBeenLastCalledWith(expect.objectContaining({ target, reason: 'target-recovered' }));
    expect(deps.requestRetarget).toHaveBeenCalledTimes(2);
  });
  it('cancels queued observers and RAF even after recommit of the same index', () => {
    commit(); const oldFrame = [...frames.values()][0]; const oldObserver = observers[0];
    commit(); oldFrame(); oldObserver.callback(); flush();
    expect(deps.refresh).toHaveBeenCalledTimes(1);
    expect(oldObserver.disconnect).toHaveBeenCalledTimes(1);
    const queued = [...frames.values()]; geometry.stop(); queued.forEach(callback => callback());
    expect(frames.size).toBe(0);
  });
  it('uses a separate active lease and abort cleans all subscriptions', () => {
    const controller = new AbortController(); commit({ signal: controller.signal }); flush();
    controller.abort(); rect.top = 80; observers.forEach(observer => observer.callback());
    window.dispatchEvent(new Event('resize')); flush();
    expect(deps.refresh).toHaveBeenCalledTimes(1); expect(frames.size).toBe(0);
    observers.forEach(observer => expect(observer.disconnect).toHaveBeenCalledTimes(1));
  });
  it.each(['aborted', 'not-current'])('ignores a stale %s commit without stopping the current lease', state => {
    commit(); flush(); const previous = [...observers];
    const controller = new AbortController(); if (state === 'aborted') controller.abort();
    commit({ signal: controller.signal, isCurrent: () => state !== 'not-current' });
    previous.forEach(observer => expect(observer.disconnect).not.toHaveBeenCalled());
    rect.top = 75; previous[0].callback(); flush(); expect(deps.refresh).toHaveBeenCalledTimes(2);
  });
  it('refreshes visual viewport changes while keeping the current index', () => {
    const viewport = new EventTarget(); Object.assign(viewport, { width: 390, height: 844, offsetTop: 0, offsetLeft: 0 });
    Object.defineProperty(window, 'visualViewport', { configurable: true, value: viewport });
    try {
      commit(); flush(); viewport.height = 500; viewport.offsetTop = 20;
      viewport.dispatchEvent(new Event('resize')); viewport.dispatchEvent(new Event('scroll')); flush();
      expect(deps.refresh).toHaveBeenCalledTimes(2); expect(deps.requestRetarget).not.toHaveBeenCalled();
    } finally { delete window.visualViewport; }
  });
  it('never fights manual touch/wheel or programmatic scroll with forced scrolling', () => {
    commit(); flush(); scope.dispatchEvent(new Event('touchmove')); scope.dispatchEvent(new Event('wheel'));
    scope.dispatchEvent(new Event('scroll')); rect.top = 2000; geometry.invalidate(); flush();
    expect(deps.scrollTarget).not.toHaveBeenCalled(); expect(deps.refresh).toHaveBeenCalledTimes(2);
  });
  it('bounds transform frames and cancels them on stop', () => {
    commit(); flush(); const event = new Event('transitionrun', { bubbles: true });
    Object.defineProperty(event, 'propertyName', { value: 'transform' }); target.dispatchEvent(event);
    flush(); expect(frames.size).toBe(1); vi.advanceTimersByTime(1201); flush(); expect(frames.size).toBe(0);
    target.dispatchEvent(event); flush(); geometry.stop(); expect(frames.size).toBe(0); expect(vi.getTimerCount()).toBe(0);
  });
  function transition(node, type, property) {
    const event = new Event(type, { bubbles: true });
    Object.defineProperty(event, 'propertyName', { value: property }); node.dispatchEvent(event);
  }
  it('does not end an ancestor transform when descendant opacity ends', () => {
    commit(); flush(); transition(scope, 'transitionrun', 'transform'); flush();
    vi.advanceTimersByTime(200); transition(target, 'transitionend', 'opacity'); flush();
    expect(frames.size).toBe(1);
    transition(scope, 'transitionend', 'transform'); flush(); expect(frames.size).toBe(0);
  });
  it('tracks two transform targets and properties independently within one bounded window', () => {
    commit(); flush(); transition(scope, 'transitionrun', 'transform');
    transition(target, 'transitionrun', 'translate'); flush();
    transition(scope, 'transitionend', 'transform'); flush(); expect(frames.size).toBe(1);
    transition(target, 'transitioncancel', 'translate'); flush(); expect(frames.size).toBe(0);
  });
  it('supports center popovers with no target', () => {
    candidate = null; commit({ target: null }); flush();
    expect(deps.applyShape).toHaveBeenCalledWith(expect.any(Object), null); expect(deps.refresh).toHaveBeenCalledTimes(1);
    expect(deps.requestRetarget).not.toHaveBeenCalled();
  });
  it('observes the public Driver state popover and releases it on stop', () => {
    const popover = element();
    deps.getDriver = () => ({ refresh: deps.refresh, getState: () => ({ popover: { wrapper: popover } }) });
    geometry.dispose(); geometry = createTourGeometry(deps);
    commit(); flush();
    expect(observers[0].observe).toHaveBeenCalledWith(popover);
    expect(deps.correctPopover).toHaveBeenCalledWith(popover);
    geometry.stop(); expect(observers[0].disconnect).toHaveBeenCalledTimes(1);
  });
  it('guards prepare before and after its injected scroll promise', async () => {
    let finish; deps.scrollTarget.mockImplementation(() => new Promise(resolve => { finish = resolve; }));
    const controller = new AbortController();
    const pending = geometry.prepare({ target, scrollTo: 'start' }, { signal: controller.signal, isCurrent: () => current });
    const innerSignal = deps.scrollTarget.mock.calls[0][2];
    expect(innerSignal).not.toBe(controller.signal);
    expect(deps.scrollTarget).toHaveBeenCalledWith(target, 'start', innerSignal);
    expect(innerSignal.aborted).toBe(false);
    controller.abort(); expect(innerSignal.aborted).toBe(true); finish(); await expect(pending).resolves.toBeNull();
    await expect(geometry.prepare({ target }, { signal: controller.signal, isCurrent: () => current })).resolves.toBeNull();
    expect(deps.scrollTarget).toHaveBeenCalledTimes(1);
  });
  it('returns a prepared target only while its source operation is current', async () => {
    const operation = { isCurrent: () => current };
    await expect(geometry.prepare({ target }, operation)).resolves.toBe(target);
    current = false; await expect(geometry.prepare({ target }, operation)).resolves.toBeNull();
    expect(deps.scrollTarget).toHaveBeenCalledTimes(1);
  });
  it.each(['stop', 'dispose'])('aborts the actual pending scroll effect on %s before it can write', async method => {
    const write = vi.fn(); let signal;
    deps.scrollTarget.mockImplementation((_, __, innerSignal) => new Promise(resolve => {
      signal = innerSignal;
      const timer = setTimeout(() => { write(); resolve(); }, 100);
      innerSignal.addEventListener('abort', () => { clearTimeout(timer); resolve(); }, { once: true });
    }));
    const external = new AbortController(); const removed = vi.spyOn(external.signal, 'removeEventListener');
    const pending = geometry.prepare({ target }, { signal: external.signal, isCurrent: () => true });
    geometry[method](); expect(signal.aborted).toBe(true); expect(vi.getTimerCount()).toBe(0);
    await expect(pending).resolves.toBeNull(); await vi.advanceTimersByTimeAsync(100);
    expect(write).not.toHaveBeenCalled(); expect(removed).toHaveBeenCalledWith('abort', expect.any(Function));
  });
  it.each(['resolve', 'reject'])('removes the prepare external listener after %s', async result => {
    const external = new AbortController(); const removed = vi.spyOn(external.signal, 'removeEventListener');
    deps.scrollTarget.mockImplementation(() => result === 'resolve' ? Promise.resolve() : Promise.reject(new Error('scroll failed')));
    const pending = geometry.prepare({ target }, { signal: external.signal, isCurrent: () => true });
    if (result === 'resolve') await expect(pending).resolves.toBe(target);
    else await expect(pending).rejects.toThrow('scroll failed');
    expect(removed).toHaveBeenCalledWith('abort', expect.any(Function));
  });
  it('preserves a new commit synchronously created by an old scroll abort listener', async () => {
    deps.scrollTarget.mockImplementation((_, __, signal) => new Promise(resolve => {
      signal.addEventListener('abort', () => { commit({ stepId: 'new-from-stop' }); resolve(); }, { once: true });
    }));
    const pending = geometry.prepare({ target }, { isCurrent: () => true });
    geometry.stop(); await pending; flush();
    expect(deps.refresh).toHaveBeenCalledTimes(1);
    observers.forEach(observer => expect(observer.disconnect).not.toHaveBeenCalled());
  });
  it('does not abort a new preparation created inside the old preparation abort listener', async () => {
    let newer, newSignal, finishNew;
    deps.scrollTarget.mockImplementationOnce((_, __, signal) => new Promise(resolve => {
      signal.addEventListener('abort', () => {
        newer = geometry.prepare({ target }, { isCurrent: () => true }); resolve();
      }, { once: true });
    })).mockImplementationOnce((_, __, signal) => {
      newSignal = signal; return new Promise(resolve => { finishNew = resolve; });
    });
    const old = geometry.prepare({ target }, { isCurrent: () => true });
    geometry.stop(); expect(newSignal.aborted).toBe(false); await old;
    finishNew(); await expect(newer).resolves.toBe(target);
  });
  it.each(['dispose', 'newer-commit'])('does not overwrite reentrant %s during commit cleanup', async action => {
    deps.scrollTarget.mockImplementation((_, __, signal) => new Promise(resolve => {
      signal.addEventListener('abort', () => {
        if (action === 'dispose') geometry.dispose();
        else commit({ index: 9, stepId: 'newer' });
        resolve();
      }, { once: true });
    }));
    const old = geometry.prepare({ target }, { isCurrent: () => true });
    commit({ index: 4, stepId: 'stale-outer' }); await old; flush();
    if (action === 'dispose') {
      expect(deps.refresh).not.toHaveBeenCalled(); expect(frames.size).toBe(0);
    } else {
      target.hidden = true; geometry.invalidate(); flush();
      expect(deps.requestRetarget).toHaveBeenCalledWith(expect.objectContaining({ index: 9, stepId: 'newer' }));
    }
  });
});
