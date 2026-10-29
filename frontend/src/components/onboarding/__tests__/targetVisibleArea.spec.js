import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ensureInView, isTargetInView } from '../targetWaits';

function target(rect, parent = document.body) {
  const el = document.createElement('div'); parent.append(el);
  el.getBoundingClientRect = () => ({ ...rect, right: rect.left + rect.width, bottom: rect.top + rect.height });
  el.scrollIntoView = vi.fn();
  return el;
}
beforeEach(() => {
  vi.useFakeTimers();
  vi.stubGlobal('innerWidth', 1024); vi.stubGlobal('innerHeight', 768);
  vi.stubGlobal('requestAnimationFrame', cb => setTimeout(cb, 16));
  vi.stubGlobal('cancelAnimationFrame', clearTimeout);
});
afterEach(() => { document.body.replaceChildren(); vi.unstubAllGlobals(); vi.useRealTimers(); });
describe('actual target viewing area', () => {
  it('rejects targets below visual viewport despite fitting layout viewport', () => {
    vi.stubGlobal('visualViewport', { width: 1024, height: 420, offsetLeft: 0, offsetTop: 0 });
    expect(isTargetInView(target({ top: 500, left: 20, width: 200, height: 100 }))).toBe(false);
  });
  it('rejects targets outside X and respects visual viewport offsets', () => {
    expect(isTargetInView(target({ top: 100, left: 1100, width: 200, height: 100 }))).toBe(false);
    vi.stubGlobal('visualViewport', { width: 500, height: 400, offsetLeft: 100, offsetTop: 150 });
    expect(isTargetInView(target({ top: 100, left: 120, width: 100, height: 50 }))).toBe(false);
    expect(isTargetInView(target({ top: 160, left: 120, width: 100, height: 50 }))).toBe(true);
  });
  it('intersects clipping scroll ancestors, independently per axis', () => {
    const parent = target({ top: 100, left: 20, width: 400, height: 200 }); parent.style.overflowX = 'auto'; parent.style.overflowY = 'auto';
    expect(isTargetInView(target({ top: 260, left: 40, width: 100, height: 80 }, parent))).toBe(false);
    expect(isTargetInView(target({ top: 120, left: 40, width: 100, height: 80 }, parent))).toBe(true);
    parent.style.overflowY = 'visible'; parent.style.overflowX = 'hidden';
    expect(isTargetInView(target({ top: 600, left: 40, width: 100, height: 80 }, parent))).toBe(true);
    expect(isTargetInView(target({ top: 120, left: 400, width: 100, height: 80 }, parent))).toBe(false);
  });
  it('accepts meaningful top of oversized block but not its middle or horizontally clipped label', () => {
    const parent = target({ top: 100, left: 20, width: 400, height: 200 }); parent.style.overflowX = 'auto'; parent.style.overflowY = 'auto';
    expect(isTargetInView(target({ top: 110, left: 40, width: 200, height: 500 }, parent))).toBe(true);
    expect(isTargetInView(target({ top: -100, left: 40, width: 200, height: 500 }, parent))).toBe(false);
    expect(isTargetInView(target({ top: 110, left: 400, width: 200, height: 500 }, parent))).toBe(false);
  });
  it('rejects impossible bounded scroll instead of reporting readiness', async () => {
    const el = target({ top: 900, left: 20, width: 200, height: 100 });
    const pending = ensureInView(el);
    const result = expect(pending).rejects.toMatchObject({ reason: 'target-unreachable' });
    await vi.advanceTimersByTimeAsync(1300); await result;
    expect(el.scrollIntoView.mock.calls.length).toBeLessThan(5);
  });
  it('returns false on cancellation without rejection or later scrolling', async () => {
    const el = target({ top: 900, left: 20, width: 200, height: 100 });
    const controller = new AbortController(); const pending = ensureInView(el, undefined, controller.signal);
    controller.abort(); const count = el.scrollIntoView.mock.calls.length;
    expect(await pending).toBe(false); await vi.advanceTimersByTimeAsync(2000);
    expect(el.scrollIntoView).toHaveBeenCalledTimes(count);
  });
  it('makes one visual-viewport correction in the real nested scrollport then settles', async () => {
    const parent = target({ top: 100, left: 20, width: 400, height: 200 });
    parent.style.overflowY = 'auto';
    Object.defineProperties(parent, { scrollHeight: { value: 1000 }, clientHeight: { value: 200 } });
    const rect = { top: 350, left: 40, width: 100, height: 50 };
    const el = target(rect, parent);
    parent.scrollBy = vi.fn(({ top }) => { rect.top -= top; });
    const pending = ensureInView(el);
    await vi.advanceTimersByTimeAsync(64);
    expect(await pending).toBe(true);
    expect(parent.scrollBy).toHaveBeenCalledExactlyOnceWith({ left: 0, top: 100, behavior: 'instant' });
    expect(vi.getTimerCount()).toBe(0);
  });
});
