import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { createTourGeometry } from '../tourGeometry';
import { raiseActiveHighlight } from '../stageShape';

let geometry, target, frames, observers, driver;
function flush() { const callbacks = [...frames.values()]; frames.clear(); callbacks.forEach(callback => callback()); }
beforeEach(() => {
  frames = new Map(); observers = []; let id = 0;
  vi.stubGlobal('requestAnimationFrame', callback => { frames.set(++id, callback); return id; });
  vi.stubGlobal('cancelAnimationFrame', key => frames.delete(key));
  vi.stubGlobal('MutationObserver', class {
    constructor(callback) { observers.push(callback); }
    observe() {} disconnect() {}
  });
  target = document.createElement('nav'); target.className = 'nav-menu'; document.body.append(target);
  target.getBoundingClientRect = () => ({ top: 0, left: 0, width: 280, height: 844 });
  driver = { getActiveElement: () => target, refresh: vi.fn() };
  geometry = createTourGeometry({ getDriver: () => driver, resolveTarget: () => target,
    restoreHighlight: raiseActiveHighlight });
  geometry.commit({ target, scope: target, index: 14, stepId: 'nav-rail' });
  flush();
});
afterEach(() => { geometry.dispose(); document.body.replaceChildren(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });

it('restores the same target after a Vue class patch without restarting the step', () => {
  expect(target.classList.contains('driver-active-element')).toBe(true);
  target.className = 'nav-menu expanded nav-menu--mobile-open';
  observers[0](); flush();
  expect(target.classList.contains('driver-active-element')).toBe(true);
  expect(target.classList.contains('ob-highlighted')).toBe(true);
  expect(driver.refresh).toHaveBeenCalledTimes(1);
  observers[0](); flush();
  expect(driver.refresh).toHaveBeenCalledTimes(1);
  expect(frames.size).toBe(0);
});

it('does not restore classes from an old queued callback after stop', () => {
  const queued = observers[0]; geometry.stop(); target.className = 'nav-menu';
  queued(); flush();
  expect(target.classList.contains('driver-active-element')).toBe(false);
  expect(driver.refresh).toHaveBeenCalledTimes(1);
});
