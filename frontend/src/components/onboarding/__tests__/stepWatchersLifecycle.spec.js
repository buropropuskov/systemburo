import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createStepWatchers } from '../stepWatchers';

let observers, watchers, driver, target;
const step = { element: '#target', advanceWhen: '#opened' };
beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date('2026-01-15T12:00:00Z'));
  observers = [];
  vi.stubGlobal('MutationObserver', class {
    constructor(callback) { this.callback = callback; this.observe = vi.fn(); this.disconnect = vi.fn(); observers.push(this); }
  });
  target = document.createElement('div'); target.id = 'target';
  target.scrollIntoView = vi.fn();
  target.getBoundingClientRect = () => ({ top: 2000, bottom: 2100, width: 100, height: 100 });
  document.body.append(target);
  driver = { obNext: vi.fn(), obRetarget: vi.fn(), refresh: vi.fn(), getActiveElement: () => target };
  watchers = createStepWatchers({ getDriver: () => driver, getGen: () => 1, getStep: () => step, getIndex: () => 5 });
});
afterEach(() => {
  watchers.stopAll();
  document.body.replaceChildren();
  vi.restoreAllMocks(); vi.unstubAllGlobals(); vi.useRealTimers();
});

function opened() { const panel = document.createElement('div'); panel.id = 'opened'; document.body.append(panel); }
function replaceTarget() {
  const replacement = document.createElement('div'); replacement.id = 'target';
  target.remove(); document.body.append(replacement);
}

describe('step watcher lifecycle ownership', () => {
  it('disconnects all observers and clears initial timers on stop', () => {
    watchers.watchStep(5);
    expect(observers).toHaveLength(2);
    expect(vi.getTimerCount()).toBe(2);
    watchers.stopAll();
    observers.forEach(observer => expect(observer.disconnect).toHaveBeenCalledTimes(1));
    expect(vi.getTimerCount()).toBe(0);
  });

  it.each(['advance', 'retarget'])('a queued %s callback after stop cannot affect the current driver', kind => {
    watchers.watchStep(5);
    const queued = observers[kind === 'advance' ? 0 : 1].callback;
    watchers.stopAll();
    if (kind === 'advance') opened(); else replaceTarget();
    queued();
    expect(driver.obNext).not.toHaveBeenCalled();
    expect(driver.obRetarget).not.toHaveBeenCalled();
  });

  it.each(['advance', 'retarget'])('an old queued %s cannot disconnect a new watcher of the same index', kind => {
    watchers.watchStep(5);
    const queued = observers[kind === 'advance' ? 0 : 1].callback;
    watchers.watchStep(5);
    const current = observers[kind === 'advance' ? 2 : 3];
    if (kind === 'advance') opened(); else replaceTarget();
    queued();
    expect(current.disconnect).not.toHaveBeenCalled();
    expect(driver.obNext).not.toHaveBeenCalled();
    expect(driver.obRetarget).not.toHaveBeenCalled();
    current.callback();
    expect(kind === 'advance' ? driver.obNext : driver.obRetarget).toHaveBeenCalledTimes(1);
  });

  it.each(['advance', 'retarget'])('a consumed %s observer ignores an already queued second callback', kind => {
    watchers.watchStep(5);
    const queued = observers[kind === 'advance' ? 0 : 1].callback;
    if (kind === 'advance') opened(); else replaceTarget();
    queued(); queued();
    expect(kind === 'advance' ? driver.obNext : driver.obRetarget).toHaveBeenCalledTimes(1);
  });

  it('a queued visibility callback cannot scroll after stop even with the same generation and index', () => {
    const scheduled = vi.spyOn(globalThis, 'setTimeout');
    watchers.watchStep(5);
    const queued = scheduled.mock.calls.find(([, delay]) => delay === 350)[0];
    watchers.stopAll(); queued();
    expect(target.scrollIntoView).not.toHaveBeenCalled();
    expect(driver.refresh).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('stop clears the nested refresh scheduled by a visibility scroll', async () => {
    watchers.watchStep(5);
    await vi.advanceTimersByTimeAsync(350);
    expect(target.scrollIntoView).toHaveBeenCalledTimes(1);
    watchers.stopAll();
    expect(vi.getTimerCount()).toBe(0);
    await vi.advanceTimersByTimeAsync(1000);
    expect(driver.refresh).not.toHaveBeenCalled();
  });

  it('a captured nested refresh cannot run after a new watcher of the same index', async () => {
    const scheduled = vi.spyOn(globalThis, 'setTimeout');
    watchers.watchStep(5);
    await vi.advanceTimersByTimeAsync(350);
    const queued = scheduled.mock.calls.find(([, delay]) => delay === 400)[0];
    watchers.watchStep(5);
    queued();
    expect(driver.refresh).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(2);
  });

  it('a current visibility chain still scrolls then refreshes at the original delays', async () => {
    watchers.watchStep(5);
    await vi.advanceTimersByTimeAsync(349);
    expect(target.scrollIntoView).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(target.scrollIntoView).toHaveBeenCalledWith({ block: 'center', inline: 'nearest' });
    await vi.advanceTimersByTimeAsync(399);
    expect(driver.refresh).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(driver.refresh).toHaveBeenCalledTimes(1);
  });
});
