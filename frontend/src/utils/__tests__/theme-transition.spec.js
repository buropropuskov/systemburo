import { describe, it, expect, vi, afterEach } from 'vitest';
import { applyTheme } from '../theme';

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  document.documentElement.className = '';
  document.documentElement.removeAttribute('data-theme');
});

describe('instant theme changes', () => {
  it.each(['light', 'dark', 'glass-light', 'glass-dark'])('applies %s synchronously', (id) => {
    applyTheme(id);
    expect(document.documentElement.dataset.theme).toBe(id);
    expect(document.documentElement.classList.contains('theme-switching')).toBe(true);
    expect(document.documentElement.classList.contains('theme-colors-changing')).toBe(false);
  });

  it('keeps the last rapid choice without an animation queue', () => {
    applyTheme('glass-dark');
    applyTheme('light');
    applyTheme('glass-light');
    expect(document.documentElement.dataset.theme).toBe('glass-light');
    expect(document.documentElement.classList.contains('theme-colors-changing')).toBe(false);
  });

  it('ignores legacy animation options and clears a stale animation class', () => {
    document.documentElement.classList.add('theme-colors-changing');
    applyTheme('glass-dark', { animate: true });
    expect(document.documentElement.dataset.theme).toBe('glass-dark');
    expect(document.documentElement.classList.contains('theme-colors-changing')).toBe(false);
    expect(document.documentElement.classList.contains('theme-switching')).toBe(true);
  });
});
