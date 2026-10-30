import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cabinetApplicationSteps } from '../cabinetApplicationSteps';
import { createStepPreparation, waitSelectorOf } from '../stepPreparation';

vi.mock('../reveal', () => ({ applyReveal: vi.fn(async () => false) }));

function owner(state, blank = 'empty') {
  const node = document.createElement('div');
  Object.assign(node.dataset, { obOwner: 'cabinet-list', obState: state, obBlank: blank });
  document.body.append(node);
  return node;
}

function prepare(initial = false) {
  const step = cabinetApplicationSteps.find(step => step.id === 'cabinet-download');
  const waitForElement = vi.fn(async selector => document.querySelector(selector));
  const controller = new AbortController();
  const execute = createStepPreparation({
    store: { steps: [step] }, watchers: { stopAll: vi.fn() }, beforePrepare: vi.fn(),
    applyDemoAttachment: () => false, waitSelectorOf, waitForElement,
    ensureInView: vi.fn(), demoFallback: 'information',
  });
  return { waitForElement, run: execute(0, { isCurrent: () => true, signal: controller.signal }, { initial }) };
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => { document.body.replaceChildren(); vi.clearAllTimers(); vi.useRealTimers(); });
async function finish(assertion, milliseconds = 5000) {
  assertion.catch(() => {}); // Attach before advancing fake time; the assertion is still awaited below.
  await vi.advanceTimersByTimeAsync(milliseconds);
  await assertion;
}

describe('cabinet download with a settled visible list', () => {
  it.each([false, true])('keeps the lesson without waiting for a known absent blank, initial=%s', async initial => {
    owner('ready');
    const probe = prepare(initial);
    const result = expect(probe.run).resolves.toBe('information');
    await finish(result);
    expect(probe.waitForElement).not.toHaveBeenCalled();
  });

  it('does not call a still-loading list empty', async () => {
    const node = owner('loading');
    const probe = prepare();
    let settled = false;
    probe.run.then(() => { settled = true; }, () => { settled = true; });
    await vi.advanceTimersByTimeAsync(100);
    expect(settled).toBe(false);
    node.dataset.obState = 'ready';
    const result = expect(probe.run).resolves.toBe('information');
    await finish(result, 100);
  });

  it.each(['error', 'forbidden'])('preserves recovery for %s instead of claiming information success', async state => {
    owner(state);
    const result = expect(prepare().run).rejects.toMatchObject({ reason: state });
    await finish(result);
  });

  it('keeps a real available target anchored', async () => {
    owner('ready', 'ready');
    const button = document.createElement('button');
    button.dataset.testid = 'ob-application-download';
    button.getBoundingClientRect = () => ({ top: 20, left: 20, width: 100, height: 40 });
    document.body.append(button);
    const probe = prepare(true);
    const result = expect(probe.run).resolves.toBe(true);
    await finish(result);
    expect(probe.waitForElement).toHaveBeenCalled();
  });

  it('does not hide a missing target when the loaded list says a blank is available', async () => {
    owner('ready', 'ready');
    await finish(expect(prepare(true).run).rejects.toMatchObject({ reason: 'missing-target' }));
  });

  it('does not treat an unknown availability as a settled absence', async () => {
    const node = owner('ready');
    delete node.dataset.obBlank;
    await finish(expect(prepare(true).run).rejects.toMatchObject({ reason: 'timeout' }));
  });
});
