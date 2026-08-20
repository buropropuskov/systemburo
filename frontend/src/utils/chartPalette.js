/**
 * Палитра холста для графиков. Canvas не понимает CSS-переменные: цвет уходит
 * в `ctx.strokeStyle` строкой, поэтому значения темы читаются с корня документа
 * и перечитываются при её смене.
 */

/** Ключи темы, из которых собирается палитра графика. */
const PALETTE_TOKENS = {
  accent: '--accent',
  grid: '--border',
  label: '--text-muted',
  surface: '--surface'
};

/** Значения на случай, когда стили ещё не применены (первый кадр, тесты). */
const FALLBACK = {
  accent: '#4F5BDF',
  grid: '#e6e6e6',
  label: '#a2a2a2',
  surface: '#ffffff'
};

/**
 * Цвета текущей темы для холста.
 * @param {HTMLElement} [root] элемент, с которого читаются переменные
 * @returns {{accent: string, grid: string, label: string, surface: string}}
 */
export function readChartPalette(root = document.documentElement) {
  const styles = getComputedStyle(root);
  const palette = {};
  Object.entries(PALETTE_TOKENS).forEach(([key, token]) => {
    palette[key] = (styles.getPropertyValue(token) || '').trim() || FALLBACK[key];
  });
  return palette;
}

/**
 * Тот же цвет с прозрачностью. Шестизначный hex разбирается в rgba: склейка
 * hex + '30', которой пользовался градиент, на любом другом виде цвета (rgb,
 * color-mix, имя) давала бы мусор вместо заливки.
 * @param {string} color
 * @param {number} alpha 0..1
 * @returns {string}
 */
export function withAlpha(color, alpha) {
  const value = String(color || '').trim();
  const hex = /^#([0-9a-f]{6})$/i.exec(value);
  if (hex) {
    const n = parseInt(hex[1], 16);
    return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${alpha})`;
  }
  const short = /^#([0-9a-f]{3})$/i.exec(value);
  if (short) {
    const [r, g, b] = short[1].split('').map(c => parseInt(c + c, 16));
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
  }
  return `color-mix(in srgb, ${value || FALLBACK.accent} ${Math.round(alpha * 100)}%, transparent)`;
}
