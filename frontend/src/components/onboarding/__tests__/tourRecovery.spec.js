import { beforeEach, describe, expect, it, vi } from 'vitest';
const mocks = vi.hoisted(() => ({ driver: vi.fn(), attach: vi.fn(), detach: vi.fn() }));
vi.mock('driver.js', () => ({ driver: mocks.driver }));
vi.mock('../popoverZoom', () => ({ createPopoverZoomFix: () => ({ attach: mocks.attach, detach: mocks.detach }) }));
import { createRecoveryDriver } from '../tourRecovery';

let config;
beforeEach(() => {
  vi.clearAllMocks();
  mocks.driver.mockImplementation(value => { config = value; return { destroy: vi.fn(() => config.onDestroyed()) }; });
});

describe('recovery is not a completed learning step', () => {
  it('offers retry without a Done label, progress, target or normal index callbacks', () => {
    const retry = vi.fn(), close = vi.fn();
    const instance = createRecoveryDriver({ retry, close, reducedMotion: true });
    expect(config.nextBtnText).toBe('Повторить'); expect(config.doneBtnText).toBe('Повторить');
    expect(config.showProgress).toBe(false); expect(config.steps[0].element).toBeUndefined();
    expect(config.showButtons).toEqual(['next', 'close']); expect(config.animate).toBe(false);
    instance.obNext(); expect(retry).toHaveBeenCalledOnce();
    config.onDestroyStarted(); expect(close).toHaveBeenCalledOnce();
  });

  it('returns only to the provided committed step, never to a numeric neighbour', () => {
    const back = vi.fn();
    const instance = createRecoveryDriver({ retry: vi.fn(), back, close: vi.fn() });
    expect(config.showButtons).toContain('previous');
    const previousButton = document.createElement('button'); previousButton.disabled = true;
    previousButton.classList.add('driver-popover-btn-disabled');
    config.onPopoverRender({ wrapper: document.createElement('div'), previousButton });
    expect(previousButton.disabled).toBe(false);
    expect(previousButton.classList.contains('driver-popover-btn-disabled')).toBe(false);
    instance.obPrev(); expect(back).toHaveBeenCalledOnce();
  });

  it('releases zoom and invalidates stale controls on public destroy', () => {
    const retry = vi.fn(), onDestroyed = vi.fn();
    const instance = createRecoveryDriver({ retry, close: vi.fn(), onDestroyed });
    const wrapper = document.createElement('div'); config.onPopoverRender({ wrapper });
    expect(mocks.attach).toHaveBeenCalledWith(wrapper);
    instance.destroy(); instance.obNext(); config.onNextClick();
    expect(mocks.detach).toHaveBeenCalled(); expect(onDestroyed).toHaveBeenCalledOnce();
    expect(retry).not.toHaveBeenCalled();
  });
});
