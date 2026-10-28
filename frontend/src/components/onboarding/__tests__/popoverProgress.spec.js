import { afterEach, describe, expect, it, vi } from 'vitest';
import { buildProgressBlock } from '../popoverProgress';

afterEach(() => { document.body.innerHTML = ''; vi.restoreAllMocks(); });

describe('step list selection', () => {
  it('closes even when selecting the current step and no transition is needed', () => {
    const onJump = vi.fn();
    vi.spyOn(window, 'requestAnimationFrame').mockReturnValue(1);
    const steps = [{ id: 'first', route: '/news', title: 'Текущий шаг' }];
    const block = buildProgressBlock({ steps, skipped: [], globalIndex: 1, total: 1, currentGlobal: 0, onJump });
    document.body.appendChild(block);
    const counter = block.querySelector('[data-testid="ob-step-counter"]');
    counter.click();
    expect(block.classList.contains('is-open')).toBe(true);
    expect(counter.getAttribute('aria-expanded')).toBe('true');
    block.querySelector('.is-current').click();
    expect(onJump).toHaveBeenCalledExactlyOnceWith(0);
    expect(block.classList.contains('is-open')).toBe(false);
    expect(counter.getAttribute('aria-expanded')).toBe('false');
    expect(document.activeElement).toBe(counter);
  });
});
