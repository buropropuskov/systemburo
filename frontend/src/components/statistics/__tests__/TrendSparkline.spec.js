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

  it('штрих ровный при сжатии, SVG заполняет контейнер на всех браузерах', () => {
    // preserveAspectRatio=none сжимает viewBox неравномерно и плющит толщину линии;
    // non-scaling-stroke держит штрих ровным. width/height=100% (а не 120x32) надёжно
    // клампятся по CSS-контейнеру на мобильных браузерах.
    const wrapper = mount(TrendSparkline, { props: { series: [40, 12], direction: 'down' } });
    const svg = wrapper.find('svg.spark');
    expect(svg.attributes('width')).toBe('100%');
    expect(svg.attributes('height')).toBe('100%');
    expect(wrapper.find('polyline').attributes('vector-effect')).toBe('non-scaling-stroke');
  });
});
