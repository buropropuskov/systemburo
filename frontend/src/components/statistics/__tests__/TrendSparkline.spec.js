import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import TrendSparkline from '../TrendSparkline.vue';

const pointsOf = (props) => mount(TrendSparkline, { props }).find('polyline').attributes('points');

describe('TrendSparkline', () => {
  it('строит polyline по числу точек ряда', () => {
    const pts = pointsOf({ series: [1, 5, 3], direction: 'up' }).trim().split(/\s+/);
    expect(pts).toHaveLength(3);
  });

  it('пустой ряд — пустая линия', () => {
    expect(pointsOf({ series: [] })).toBe('');
  });

  it('одна точка — горизонтальная линия из двух концов', () => {
    const pts = pointsOf({ series: [7] }).trim().split(/\s+/);
    expect(pts).toHaveLength(2);
  });

  it('цвет линии зависит от направления', () => {
    const up = mount(TrendSparkline, { props: { series: [1, 2], direction: 'up' } });
    const down = mount(TrendSparkline, { props: { series: [1, 2], direction: 'down' } });
    expect(up.find('polyline').attributes('stroke')).not.toBe(
      down.find('polyline').attributes('stroke'),
    );
  });
});
