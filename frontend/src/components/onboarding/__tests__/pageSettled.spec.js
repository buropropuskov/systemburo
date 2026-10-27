import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { isPageBusy, waitForPageSettled } from '../pageSettled';

let loader;
beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date('2026-01-15T12:00:00Z'));
  loader = document.createElement('div');
  loader.className = 'loader-spinner';
  loader.getBoundingClientRect = () => ({ width: 20, height: 20 });
  document.body.append(loader);
});
afterEach(() => { document.body.replaceChildren(); vi.restoreAllMocks(); vi.useRealTimers(); });

describe('page readiness cancellation and cleanup', () => {
  it.each([true, false])('an already aborted signal resolves immediately without polling (busy=%s)', async busy => {
    if (!busy) loader.remove();
    const controller = new AbortController(); controller.abort();
    const listener = vi.spyOn(controller.signal, 'addEventListener');
    let result;
    const pending = waitForPageSettled(5000, controller.signal).then(value => { result = value; });
    await Promise.resolve();
    expect(result).toBe(false);
    expect(vi.getTimerCount()).toBe(0);
    expect(listener).not.toHaveBeenCalled();
    await pending;
  });

  it('a ready page completes immediately without scheduling work', async () => {
    loader.remove();
    await expect(waitForPageSettled()).resolves.toBe(true);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('a loader without dimensions is not considered busy', () => {
    loader.getBoundingClientRect = () => ({ width: 0, height: 0 });
    expect(isPageBusy()).toBe(false);
  });

  it.each(['abort', 'ready', 'timeout'])('cleans its interval and abort listener after %s', async completion => {
    const controller = new AbortController();
    const removed = vi.spyOn(controller.signal, 'removeEventListener');
    const pending = waitForPageSettled(500, controller.signal);
    expect(vi.getTimerCount()).toBe(1);
    if (completion === 'abort') controller.abort();
    if (completion === 'ready') loader.remove();
    if (completion !== 'abort') await vi.advanceTimersByTimeAsync(completion === 'ready' ? 100 : 500);
    await expect(pending).resolves.toBe(completion === 'ready');
    expect(vi.getTimerCount()).toBe(0);
    expect(removed).toHaveBeenCalledWith('abort', expect.any(Function));
    controller.abort();
    expect(vi.getTimerCount()).toBe(0);
  });
});
