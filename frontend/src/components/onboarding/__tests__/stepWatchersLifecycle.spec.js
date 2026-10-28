import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createStepWatchers } from '../stepWatchers';
let observers, watchers, driver, index, generation, busy;
const step = { advanceWhen: '#opened' };
beforeEach(() => {
  vi.useFakeTimers(); observers = []; index = 5; generation = 1; busy = false;
  vi.stubGlobal('MutationObserver', class {
    constructor(callback) { this.callback = callback; this.observe = vi.fn(); this.disconnect = vi.fn(); observers.push(this); }
  });
  driver = { obNext: vi.fn() };
  watchers = createStepWatchers({ getDriver: () => busy ? null : driver, getGen: () => generation,
    getStep: () => step, getIndex: () => index });
});
afterEach(() => {
  watchers.stopAll(); document.body.replaceChildren();
  vi.restoreAllMocks(); vi.unstubAllGlobals(); vi.useRealTimers();
});
function opened() { const panel = document.createElement('div'); panel.id = 'opened'; document.body.append(panel); }
describe('advance watcher lifecycle ownership', () => {
  it('owns one observer and no layout timers', () => {
    watchers.watchStep(5); expect(observers).toHaveLength(1); expect(vi.getTimerCount()).toBe(0);
    watchers.stopAll(); expect(observers[0].disconnect).toHaveBeenCalledTimes(1);
  });
  it('a queued advance callback after stop cannot affect the driver', () => {
    watchers.watchStep(5); const queued = observers[0].callback;
    watchers.stopAll(); opened(); queued(); expect(driver.obNext).not.toHaveBeenCalled();
  });
  it('an old callback cannot consume a new watcher at the same index', () => {
    watchers.watchStep(5); const queued = observers[0].callback;
    watchers.watchStep(5); const current = observers[1]; opened(); queued();
    expect(current.disconnect).not.toHaveBeenCalled(); expect(driver.obNext).not.toHaveBeenCalled();
    current.callback(); expect(driver.obNext).toHaveBeenCalledTimes(1);
  });
  it('a consumed callback cannot advance twice', () => {
    watchers.watchStep(5); const queued = observers[0].callback; opened(); queued(); queued();
    expect(driver.obNext).toHaveBeenCalledTimes(1);
  });
  it.each(['index', 'generation'])('ignores callbacks after a changed %s', changed => {
    watchers.watchStep(5); opened(); if (changed === 'index') index = 6; else generation = 2;
    observers[0].callback(); expect(driver.obNext).not.toHaveBeenCalled();
  });
  it('does not mistake an already present node for a fresh user action', () => {
    opened(); watchers.watchStep(5); expect(observers).toHaveLength(0); expect(driver.obNext).not.toHaveBeenCalled();
  });
  it('does not advance while B navigation owns preparation', () => {
    watchers.watchStep(5); opened(); busy = true; observers[0].callback();
    expect(driver.obNext).not.toHaveBeenCalled(); expect(observers[0].disconnect).not.toHaveBeenCalled();
    busy = false; observers[0].callback(); expect(driver.obNext).toHaveBeenCalledTimes(1);
  });
  it('needs neither an observer nor timers when the step has no advance condition', () => {
    watchers.stopAll(); watchers = createStepWatchers({ getDriver: () => driver, getGen: () => 1,
      getStep: () => ({}), getIndex: () => 5 });
    watchers.watchStep(5); expect(observers).toHaveLength(0); expect(vi.getTimerCount()).toBe(0);
  });
});
