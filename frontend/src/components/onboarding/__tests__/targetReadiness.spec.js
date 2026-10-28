import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { waitForElement, ensureInView } from '../targetWaits.js';

// jsdom does not lay out CSS. Rect/offsetParent below model the actual distinctions:
// display:none has no box; visibility:hidden/opacity:0 keep a box; fixed has no offsetParent.
function target({ display = 'block', visibility = 'visible', opacity = '1', fixed = false, top = 100, height = 32 } = {}) {
  const element = document.createElement('button');
  element.dataset.testid = 'diagnostic-target';
  Object.assign(element.style, { display, visibility, opacity, position: fixed ? 'fixed' : 'absolute', top: top + 'px', left: '20px', width: '80px', height: height + 'px' });
  Object.defineProperty(element, 'offsetParent', { configurable: true, get: () => getComputedStyle(element).display === 'none' || fixed ? null : document.body });
  element.getBoundingClientRect = () => {
    const style = getComputedStyle(element);
    const hidden = style.display === 'none';
    const x = hidden ? 0 : parseFloat(style.left), y = hidden ? 0 : parseFloat(style.top);
    const width = hidden ? 0 : parseFloat(style.width), actualHeight = hidden ? 0 : parseFloat(style.height);
    return { x, y, left: x, top: y, width, height: actualHeight, right: x + width, bottom: y + actualHeight };
  };
  element.scrollIntoView = vi.fn(() => {
    // Browser page scrolling cannot relocate a fixed control.
    if (!fixed) element.style.top = Math.round((window.innerHeight - height) / 2) + 'px';
  });
  document.body.append(element);
  return element;
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date('2026-01-15T12:00:00Z'));
  vi.stubGlobal('innerHeight', 844);
  vi.stubGlobal('innerWidth', 390);
  vi.stubGlobal('requestAnimationFrame', callback => setTimeout(() => callback(performance.now()), 16));
  vi.stubGlobal('cancelAnimationFrame', id => clearTimeout(id));
});
afterEach(() => {
  document.body.replaceChildren();
  vi.clearAllTimers(); vi.restoreAllMocks(); vi.unstubAllGlobals(); vi.useRealTimers();
});

describe('visible targets and scrolling', () => {
  it('returns the second concrete HTMLElement when the first desktop copy is display:none', async () => {
    const desktop = target({ display: 'none' }), mobile = target();
    expect(desktop.getBoundingClientRect().width).toBe(0);
    expect(mobile.getBoundingClientRect().width).toBe(80);
    const pending = waitForElement('[data-testid="diagnostic-target"]', 480);
    await vi.advanceTimersByTimeAsync(480);
    expect(await pending).toBe(mobile);
    expect(vi.getTimerCount()).toBe(0);
  });

  it.each([{ visibility: 'hidden' }, { opacity: '0' }])('does not accept a layout box hidden by CSS %j', async style => {
    const element = target(style);
    expect(element.getBoundingClientRect().height).toBe(32);
    expect(element.offsetParent).toBe(document.body);
    const pending = waitForElement('[data-testid="diagnostic-target"]', 480);
    await vi.advanceTimersByTimeAsync(480);
    expect(await pending).toBeNull();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('does not spend 1200ms scrolling a visible fixed control at top12', async () => {
    const element = target({ fixed: true, top: 12 });
    expect(element.offsetParent).toBeNull();
    const started = Date.now(); let settledMs = null;
    const pending = ensureInView(element).then(() => { settledMs = Date.now() - started; });
    await vi.advanceTimersByTimeAsync(1232); await pending;
    expect({ settledMs, scrollCalls: element.scrollIntoView.mock.calls.length }).toEqual({ settledMs: 0, scrollCalls: 0 });
    expect(vi.getTimerCount()).toBe(0);
  });
});

describe('normal controls: existing ready/cancel behavior must remain', () => {
  it('resolves the actual visible HTMLElement after stable measurement', async () => {
    const element = target();
    const pending = waitForElement('[data-testid="diagnostic-target"]', 480);
    await vi.advanceTimersByTimeAsync(120);
    expect(await pending).toBe(element);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('returns null for a missing target at the controlled deadline', async () => {
    const pending = waitForElement('#absent', 480);
    await vi.advanceTimersByTimeAsync(480);
    expect(await pending).toBeNull(); expect(vi.getTimerCount()).toBe(0);
  });

  it('pre-aborted target lookup returns null without a timer', async () => {
    target(); const controller = new AbortController(); controller.abort();
    await expect(waitForElement('[data-testid="diagnostic-target"]', 480, controller.signal)).resolves.toBeNull();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('abort during target lookup clears the pending interval and listener', async () => {
    const controller = new AbortController(), remove = vi.spyOn(controller.signal, 'removeEventListener');
    const pending = waitForElement('#absent', 480, controller.signal);
    controller.abort(); await expect(pending).resolves.toBeNull();
    expect(vi.getTimerCount()).toBe(0); expect(remove).toHaveBeenCalledWith('abort', expect.any(Function));
  });

  it('a normal visible target does not scroll or schedule frames', async () => {
    const element = target(); await ensureInView(element);
    expect(element.scrollIntoView).not.toHaveBeenCalled(); expect(vi.getTimerCount()).toBe(0);
  });

  it('an ordinary below-fold target is scrolled into view then releases its frame', async () => {
    const element = target({ top: 1200 }); const pending = ensureInView(element);
    await vi.advanceTimersByTimeAsync(16); await pending;
    expect(element.scrollIntoView).toHaveBeenCalledWith({ block: 'center', inline: 'nearest', behavior: 'instant' });
    expect(element.getBoundingClientRect().top).toBe(406); expect(vi.getTimerCount()).toBe(0);
  });

  it('pre-aborted scroll does not touch even a fixed target', async () => {
    const controller = new AbortController(); controller.abort(); const element = target({ fixed: true, top: 12 });
    await ensureInView(element, undefined, controller.signal);
    expect(element.scrollIntoView).not.toHaveBeenCalled(); expect(vi.getTimerCount()).toBe(0);
  });

  it('abort during an unreachable scroll cancels RAF and listener', async () => {
    const element = target({ fixed: true, top: 900 }), controller = new AbortController();
    const remove = vi.spyOn(controller.signal, 'removeEventListener');
    const pending = ensureInView(element, undefined, controller.signal);
    expect(vi.getTimerCount()).toBe(1);
    controller.abort(); await pending;
    expect(vi.getTimerCount()).toBe(0); expect(remove).toHaveBeenCalledWith('abort', expect.any(Function));
  });
});
