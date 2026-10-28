import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createStepPreparation } from '../stepPreparation';
import { waitForElement } from '../targetWaits';

vi.mock('../reveal', () => ({ applyReveal: vi.fn(async () => false) }));

function box(id, parent = document.body, className = '') {
  const element = document.createElement('div');
  element.id = id; element.className = className;
  element.getBoundingClientRect = () => ({ top: 100, left: 20, width: 80, height: 32 });
  parent.append(element);
  return element;
}

function prepare(step, prepareTarget, options = {}) {
  const controller = new AbortController();
  const ensureInView = vi.fn(async () => {});
  const execute = createStepPreparation({
    store: { steps: [step] }, watchers: { stopAll: vi.fn() },
    beforePrepare: vi.fn(), applyDemoAttachment: () => false,
    waitSelectorOf: value => value.waitFor || value.element,
    waitForElement, ensureInView, demoFallback: 'demo',
    ...(prepareTarget ? { prepareTarget } : {}),
  });
  const pending = execute(0, { signal: controller.signal, isCurrent: () => !controller.signal.aborted }, options);
  return { pending, controller, ensureInView };
}

beforeEach(() => { vi.useFakeTimers(); vi.setSystemTime(new Date('2026-01-15T12:00:00Z')); });
afterEach(() => { document.body.replaceChildren(); vi.clearAllTimers(); vi.useRealTimers(); vi.restoreAllMocks(); });

describe('step preparation uses the visible target readiness scope', () => {
  it.each([false, true])('blocks a missing mandatory anchor after its deadline (initial=%s)', async initial => {
    const run = prepare({ id: 'required', element: '#missing' }, undefined, { initial });
    const result = expect(run.pending).rejects.toMatchObject({ stepId: 'required', reason: 'missing-target' });
    await vi.advanceTimersByTimeAsync(10000); await result;
    expect(run.ensureInView).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('keeps an intentional description-only step without a target deadline', async () => {
    const run = prepare({ id: 'description', element: null }, undefined, { initial: true });
    await expect(run.pending).resolves.toBe(true);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('uses only an explicit demo when its anchor is missing', async () => {
    const run = prepare({ id: 'example', element: '#missing', demo: 'application' });
    await vi.advanceTimersByTimeAsync(10000);
    await expect(run.pending).resolves.toBe('demo');
  });

  it('skips a missing optional continuation without treating it as shown', async () => {
    const run = prepare({ id: 'optional', element: '#missing', optional: true });
    await vi.advanceTimersByTimeAsync(1000);
    await expect(run.pending).resolves.toBe(false);
  });

  it('blocks an initial optional anchor rather than letting the host show an unanchored step', async () => {
    const run = prepare({ id: 'optional-first', element: '#missing', optional: true }, undefined, { initial: true });
    const result = expect(run.pending).rejects.toMatchObject({ reason: 'missing-target' });
    await vi.advanceTimersByTimeAsync(10000); await result;
  });

  it('rejects a target removed during scroll preparation', async () => {
    const target = box('target');
    const run = prepare({ id: 'scroll-target', element: '#target' }, async () => { target.remove(); return null; });
    const result = expect(run.pending).rejects.toMatchObject({ stepId: 'scroll-target', reason: 'missing-target' });
    await vi.advanceTimersByTimeAsync(120); await result;
  });

  it('rechecks the explicit owner after target preparation', async () => {
    const target = box('target'); target.dataset.obOwner = 'fact-list'; target.dataset.obState = 'ready';
    const run = prepare({ id: 'changed-owner', element: '#target', dataOwner: 'fact-list' }, async () => {
      target.dataset.obState = 'error'; return target;
    });
    const result = expect(run.pending).rejects.toMatchObject({ stepId: 'changed-owner', reason: 'owner-changed' });
    await vi.advanceTimersByTimeAsync(120); await result;
  });
  it.each(['error', 'forbidden'])('blocks an explicit %s outcome without skipping or scrolling', async state => {
    const owner = box('owner'); owner.dataset.obOwner = 'fact-list'; owner.dataset.obState = state;
    const run = prepare({ id: 'guard-target', element: '#owner', dataOwner: 'fact-list', optional: true });
    await expect(run.pending).rejects.toMatchObject({ stepId: 'guard-target', reason: state });
    expect(run.ensureInView).not.toHaveBeenCalled(); expect(vi.getTimerCount()).toBe(0);
  });

  it.each([true, false])('uses known empty data immediately (optional=%s), without a target deadline', async optional => {
    const owner = box('owner'); owner.dataset.obOwner = 'fact-list'; owner.dataset.obState = 'empty';
    const run = prepare({ id: 'empty-target', element: '#missing', dataOwner: 'fact-list', optional });
    await expect(run.pending).resolves.toBe(optional ? false : 'demo');
    expect(vi.getTimerCount()).toBe(0); expect(run.ensureInView).not.toHaveBeenCalled();
  });

  it('does not convert a loading deadline into an optional skip', async () => {
    const owner = box('owner'); owner.dataset.obOwner = 'fact-list'; owner.dataset.obState = 'loading';
    const run = prepare({ id: 'guard-target', element: '#owner', dataOwner: 'fact-list', optional: true });
    const result = expect(run.pending).rejects.toMatchObject({ stepId: 'guard-target', reason: 'timeout' });
    await vi.advanceTimersByTimeAsync(5000); await result;
    expect(run.ensureInView).not.toHaveBeenCalled(); expect(vi.getTimerCount()).toBe(0);
  });

  it('does not wait for an unrelated page loader or scroll a hidden desktop copy', async () => {
    const hidden = box('desktop'); hidden.dataset.tour = 'action'; hidden.style.display = 'none';
    const visible = box('mobile'); visible.dataset.tour = 'action';
    box('unrelated', document.body, 'loader-spinner');
    const run = prepare({ element: '[data-tour="action"]' });
    await vi.advanceTimersByTimeAsync(120);
    await expect(run.pending).resolves.toBe(true);
    expect(run.ensureInView).toHaveBeenCalledWith(visible, undefined, run.controller.signal);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('waits for the explicit data owner and scrolls its distinct semantic target', async () => {
    const owner = box('owner'), target = box('target', owner);
    const loader = box('loader', owner, 'loader-spinner');
    const run = prepare({ element: '#target', waitFor: '#owner', dataReady: '#owner' });
    await vi.advanceTimersByTimeAsync(500);
    expect(run.ensureInView).not.toHaveBeenCalled();
    loader.remove();
    await vi.advanceTimersByTimeAsync(340);
    await expect(run.pending).resolves.toBe(true);
    expect(run.ensureInView).toHaveBeenCalledWith(target, undefined, run.controller.signal);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('cancels during scoped loading without committing or scrolling the old target', async () => {
    const owner = box('owner'); box('loader', owner, 'loader-spinner');
    const run = prepare({ element: '#owner' });
    await vi.advanceTimersByTimeAsync(100);
    run.controller.abort();
    await expect(run.pending).resolves.toBeUndefined();
    expect(run.ensureInView).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
  });
});
