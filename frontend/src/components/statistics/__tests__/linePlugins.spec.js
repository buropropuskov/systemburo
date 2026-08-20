import { describe, it, expect } from 'vitest';
import { crosshairPlugin } from '../linePlugins';

/** Холст, запоминающий проведённые отрезки. */
function fakeChart({ active = [], area = { top: 10, bottom: 210 } } = {}) {
  const segments = [];
  let cursor = null;
  const ctx = {
    save: () => {}, restore: () => {}, beginPath: () => {}, stroke: () => {},
    setLineDash: () => {},
    moveTo: (x, y) => { cursor = { x, y }; },
    lineTo: (x, y) => { segments.push({ from: cursor, to: { x, y } }); },
  };
  return {
    ctx,
    canvas: null,
    chartArea: area,
    getActiveElements: () => active,
    segments,
  };
}

describe('crosshairPlugin', () => {
  it('ведёт вертикаль через точку, оставляя вокруг неё просвет', () => {
    const chart = fakeChart({ active: [{ element: { x: 120, y: 90 } }] });
    crosshairPlugin.afterDatasetsDraw(chart);

    // Два отрезка: сверху до точки и от точки вниз - точка не перечёркнута.
    expect(chart.segments).toHaveLength(2);
    const [upper, lower] = chart.segments;
    expect(upper.from).toEqual({ x: 120, y: 10 });
    expect(lower.to).toEqual({ x: 120, y: 210 });
    // Просвет вокруг точки больше её радиуса с обводкой (6 + 3).
    expect(90 - upper.to.y).toBeGreaterThan(9);
    expect(lower.from.y - 90).toBeGreaterThan(9);
    // Обе половины идут по одной вертикали - по x самой точки.
    expect([upper.from.x, upper.to.x, lower.from.x, lower.to.x]).toEqual([120, 120, 120, 120]);
  });

  it('без наведения не рисует ничего', () => {
    const chart = fakeChart({ active: [] });
    crosshairPlugin.afterDatasetsDraw(chart);
    expect(chart.segments).toHaveLength(0);
  });

  it('без размеченной области графика молчит, а не падает', () => {
    const chart = fakeChart({ active: [{ element: { x: 5, y: 5 } }], area: null });
    expect(() => crosshairPlugin.afterDatasetsDraw(chart)).not.toThrow();
    expect(chart.segments).toHaveLength(0);
  });
});
