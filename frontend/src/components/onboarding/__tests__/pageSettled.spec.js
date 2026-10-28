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

describe('scoped page readiness', () => {
  function createScope() {
    const scope = document.createElement('section');
    document.body.append(scope);
    return scope;
  }

  it('ignores a busy loader outside the requested scope without polling', async () => {
    const scope = createScope();
    expect(isPageBusy()).toBe(true);
    expect(isPageBusy(scope)).toBe(false);
    await expect(waitForPageSettled(500, undefined, scope)).resolves.toBe(true);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('includes a busy scope element itself', () => {
    expect(isPageBusy(loader)).toBe(true);
  });

  it('includes descendant loaders even when aria-hidden or inert', () => {
    const scope = createScope(); scope.append(loader);
    loader.setAttribute('aria-hidden', 'true');
    loader.setAttribute('inert', '');
    expect(isPageBusy(scope)).toBe(true);
  });

  it.each(['display', 'visibility', 'opacity', 'hidden'])('ignores CSS-hidden loaders (%s)', property => {
    const scope = createScope(); scope.append(loader);
    if (property === 'hidden') scope.setAttribute('hidden', '');
    else scope.style[property] = { display: 'none', visibility: 'hidden', opacity: '0' }[property];
    expect(isPageBusy(scope)).toBe(false);
  });

  it.each(['abort', 'ready', 'timeout'])('cleans scoped waiting after %s', async completion => {
    const scope = createScope(); scope.append(loader);
    const outside = document.createElement('div'); outside.className = 'loader-spinner';
    outside.getBoundingClientRect = () => ({ width: 20, height: 20 });
    document.body.append(outside);
    const controller = new AbortController();
    const removed = vi.spyOn(controller.signal, 'removeEventListener');
    const pending = waitForPageSettled(500, controller.signal, scope);
    expect(vi.getTimerCount()).toBe(1);
    if (completion === 'abort') controller.abort();
    if (completion === 'ready') loader.remove();
    if (completion !== 'abort') await vi.advanceTimersByTimeAsync(completion === 'ready' ? 100 : 500);
    await expect(pending).resolves.toBe(completion === 'ready');
    expect(isPageBusy()).toBe(true);
    expect(vi.getTimerCount()).toBe(0);
    expect(removed).toHaveBeenCalledWith('abort', expect.any(Function));
  });

  it('returns ready for an absent SSR scope', async () => {
    expect(isPageBusy(null)).toBe(false);
    await expect(waitForPageSettled(500, undefined, null)).resolves.toBe(true);
    vi.stubGlobal('document', undefined);
    try {
      expect(isPageBusy()).toBe(false);
      await expect(waitForPageSettled()).resolves.toBe(true);
    } finally { vi.unstubAllGlobals(); }
    expect(vi.getTimerCount()).toBe(0);
  });
});
afterEach(() => { document.body.replaceChildren(); vi.restoreAllMocks(); vi.useRealTimers(); });

describe('page readiness cancellation and cleanup', () => {
  it('waits for a busy ancestor even when its child control is already rendered', async () => {
    const owner = document.createElement('section');
    owner.setAttribute('aria-busy', 'true');
    owner.getBoundingClientRect = () => ({ width: 100, height: 100 });
    const target = document.createElement('button');
    owner.append(target); document.body.append(owner);
    const pending = waitForPageSettled(500, undefined, target);
    expect(vi.getTimerCount()).toBe(1);
    owner.setAttribute('aria-busy', 'false');
    await vi.advanceTimersByTimeAsync(100);
    await expect(pending).resolves.toBe(true);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('re-resolves a replaced owner instead of waiting on its detached loader', async () => {
    const owner = document.createElement('section');
    owner.id = 'owner'; owner.append(loader); document.body.append(owner);
    const pending = waitForPageSettled(500, undefined, () => document.querySelector('#owner'));
    const replacement = document.createElement('section'); replacement.id = 'owner';
    owner.replaceWith(replacement);
    await vi.advanceTimersByTimeAsync(100);
    await expect(pending).resolves.toBe(true);
    expect(vi.getTimerCount()).toBe(0);
  });

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
