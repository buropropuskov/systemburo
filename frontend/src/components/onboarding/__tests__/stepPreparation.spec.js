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

function prepare(step) {
  const controller = new AbortController();
  const ensureInView = vi.fn(async () => {});
  const execute = createStepPreparation({
    store: { steps: [step] }, watchers: { stopAll: vi.fn() },
    beforePrepare: vi.fn(), applyDemoAttachment: () => false,
    waitSelectorOf: value => value.waitFor || value.element,
    waitForElement, ensureInView, demoFallback: 'demo',
  });
  const pending = execute(0, { signal: controller.signal, isCurrent: () => !controller.signal.aborted });
  return { pending, controller, ensureInView };
}

beforeEach(() => { vi.useFakeTimers(); vi.setSystemTime(new Date('2026-01-15T12:00:00Z')); });
afterEach(() => { document.body.replaceChildren(); vi.clearAllTimers(); vi.useRealTimers(); vi.restoreAllMocks(); });

describe('step preparation uses the visible target readiness scope', () => {
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
