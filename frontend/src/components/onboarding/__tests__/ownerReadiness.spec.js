import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { readStepOwner, retryStepOwner, waitForStepOwner } from '../ownerReadiness';

function owner(key, state, parent = document.body) {
  const node = document.createElement('div');
  node.dataset.obOwner = key; node.dataset.obState = state;
  node.getBoundingClientRect = () => ({ width: 100, height: 30 });
  parent.append(node);
  return node;
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => { document.body.replaceChildren(); vi.clearAllTimers(); vi.useRealTimers(); });

describe('explicit onboarding owner outcomes', () => {
  it('reads and retries a unique failed owner even when its empty region has no layout height', async () => {
    const node = owner('accessible-list', 'error');
    node.getBoundingClientRect = () => ({ width: 100, height: 0 });
    const refresh = vi.fn(event => { event.preventDefault(); node.dataset.obState = 'loading'; });
    node.addEventListener('onboarding-retry', refresh);
    await expect(waitForStepOwner('accessible-list')).resolves.toEqual({ kind: 'error', element: node });
    expect(retryStepOwner('accessible-list')).toBe(true); expect(refresh).toHaveBeenCalledOnce();
    expect(node.dataset.obState).toBe('loading');
  });

  it('does not choose arbitrarily between two hidden responsive owners', async () => {
    owner('fact-list', 'error').style.display = 'none';
    owner('fact-list', 'ready').style.display = 'none';
    expect(readStepOwner('fact-list')).toBe(null);
    const pending = waitForStepOwner('fact-list', { timeout: 100 });
    await vi.advanceTimersByTimeAsync(100);
    await expect(pending).resolves.toEqual({ kind: 'timeout' });
  });
  it.each(['ready', 'empty', 'error', 'forbidden'])('preserves %s instead of equating it with loaded data', async state => {
    const node = owner('fact-list', state);
    await expect(waitForStepOwner('fact-list')).resolves.toEqual({ kind: state, element: node });
    expect(vi.getTimerCount()).toBe(0);
  });

  it('does not mistake a missing owner for successful empty data', async () => {
    const pending = waitForStepOwner('accessible-list', { timeout: 100 });
    await vi.advanceTimersByTimeAsync(100);
    await expect(pending).resolves.toEqual({ kind: 'timeout' });
    expect(vi.getTimerCount()).toBe(0);
  });

  it('waits for the requested owner and ignores unrelated ready or busy sections', async () => {
    owner('accessible-detail', 'ready');
    const list = owner('accessible-list', 'loading');
    const pending = waitForStepOwner('accessible-list');
    await vi.advanceTimersByTimeAsync(100);
    list.dataset.obState = 'empty';
    await vi.advanceTimersByTimeAsync(50);
    await expect(pending).resolves.toEqual({ kind: 'empty', element: list });
  });

  it('rebinds a replaced owner, excluding hidden responsive copies', async () => {
    const hidden = owner('fact-list', 'ready'); hidden.style.display = 'none';
    const old = owner('fact-list', 'loading');
    const pending = waitForStepOwner('fact-list'); old.remove();
    const current = owner('fact-list', 'error');
    await vi.advanceTimersByTimeAsync(50);
    await expect(pending).resolves.toEqual({ kind: 'error', element: current });
  });

  it('cancels without late readiness or outstanding timers', async () => {
    const node = owner('fact-list', 'loading'), controller = new AbortController();
    const pending = waitForStepOwner('fact-list', { signal: controller.signal });
    controller.abort(); node.dataset.obState = 'ready';
    await expect(pending).resolves.toEqual({ kind: 'cancelled' });
    expect(vi.getTimerCount()).toBe(0);
  });

  it('supports static steps and the owner itself as a scoped root', async () => {
    await expect(waitForStepOwner(null)).resolves.toEqual({ kind: 'unmanaged' });
    const root = owner('accessible-detail', 'ready');
    expect(readStepOwner('accessible-detail', () => root)?.element).toBe(root);
  });
});
