import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { onboardingSteps } from '../onboardingSteps';
import { createStepPreparation } from '../stepPreparation';
import { waitForElement } from '../targetWaits';

vi.mock('../reveal', () => ({ applyReveal: vi.fn(async () => false) }));

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date('2026-01-15T12:00:00Z'));
});
afterEach(() => {
  document.body.replaceChildren();
  vi.clearAllTimers();
  vi.useRealTimers();
});

describe('conditional explanations retain their lesson when a block is absent', () => {
  it.each(['detail-status-section', 'createapp-custom'])('%s does not disappear after a target timeout', async id => {
    const step = onboardingSteps.find(item => item.id === id);
    const wait = vi.fn(waitForElement);
    const prepareTarget = vi.fn();
    const controller = new AbortController();
    const prepare = createStepPreparation({
      store: { steps: [step] }, watchers: { stopAll: vi.fn() },
      beforePrepare: vi.fn(), applyDemoAttachment: () => false,
      waitSelectorOf: item => item.waitFor || item.element,
      waitForElement: wait, ensureInView: vi.fn(), prepareTarget,
      demoFallback: 'demo',
    });
    const pending = prepare(0, { signal: controller.signal, isCurrent: () => true });
    await vi.advanceTimersByTimeAsync(10000);

    expect(await pending).toBe(true);
    expect(wait).not.toHaveBeenCalled();
    expect(prepareTarget).not.toHaveBeenCalled();
    expect(step.description).toMatch(/когда|если|после/i);
    expect(vi.getTimerCount()).toBe(0);
  });
});
