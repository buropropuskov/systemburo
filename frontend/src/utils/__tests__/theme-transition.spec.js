import { describe, it, expect, vi, afterEach } from 'vitest';
import { applyTheme } from '../theme';
afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); document.documentElement.className = ''; });
describe('theme color transition', () => {
  it('animates deliberate changes and restarts the common deadline on rapid choices', () => {
    vi.useFakeTimers();
    vi.stubGlobal('matchMedia', () => ({ matches: false }));
    applyTheme('glass-dark', { animate: true });
    expect(document.documentElement.classList.contains('theme-colors-changing')).toBe(true);
    vi.advanceTimersByTime(200);
    applyTheme('glass-light', { animate: true });
    vi.advanceTimersByTime(150);
    expect(document.documentElement.dataset.theme).toBe('glass-light');
    expect(document.documentElement.classList.contains('theme-colors-changing')).toBe(true);
    vi.advanceTimersByTime(150);
    expect(document.documentElement.classList.contains('theme-colors-changing')).toBe(false);
  });
  it('does not animate startup or reduced-motion', () => {
    applyTheme('glass-dark');
    expect(document.documentElement.classList.contains('theme-colors-changing')).toBe(false);
    vi.stubGlobal('matchMedia', () => ({ matches: true }));
    applyTheme('glass-light', { animate: true });
    expect(document.documentElement.classList.contains('theme-colors-changing')).toBe(false);
  });
});
