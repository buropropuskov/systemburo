import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';

// apexcharts требует реального SVG/измерений — мокаем vue3-apexcharts стабом,
// который запоминает последние props, чтобы проверять собранный конфиг (тип,
// серию, категории, форматтер тултипа), а не рендер в jsdom.
const { rendered } = vi.hoisted(() => ({ rendered: { last: null } }));
vi.mock('vue3-apexcharts', () => ({
  default: {
    name: 'ApexStub',
    props: ['type', 'height', 'options', 'series'],
    render() {
      rendered.last = { type: this.type, options: this.options, series: this.series };
      return null;
    },
  },
}));

import AnalyticsBarChart from '../AnalyticsBarChart.vue';

const DATA = [
  { label: '08:00', value: 3 },
  { label: '09:00', value: 7 },
];

describe('AnalyticsBarChart', () => {
  it('строит bar-серию: значения, категории, палитра проекта', () => {
    rendered.last = null;
    mount(AnalyticsBarChart, {
      props: { data: DATA, color: '#4F5BDF', seriesName: 'Проезды машин' },
    });

    expect(rendered.last).not.toBeNull();
    expect(rendered.last.type).toBe('bar');
    expect(rendered.last.series[0].name).toBe('Проезды машин');
    expect(rendered.last.series[0].data).toEqual([3, 7]);

    const opts = rendered.last.options;
    expect(opts.colors).toEqual(['#4F5BDF']);
    expect(opts.xaxis.categories).toEqual(['08:00', '09:00']);
    expect(opts.legend.show).toBe(false);
  });

  it('тултип склоняет единицу по числу', () => {
    rendered.last = null;
    mount(AnalyticsBarChart, {
      props: { data: DATA, unitForms: ['проезд', 'проезда', 'проездов'] },
    });
    const tip = rendered.last.options.tooltip;
    expect(tip.y.formatter(1)).toBe('1 проезд');
    expect(tip.y.formatter(2)).toBe('2 проезда');
    expect(tip.y.formatter(5)).toBe('5 проездов');
  });

  it('пустые данные: показывает заглушку, график не рендерит', () => {
    rendered.last = null;
    const wrapper = mount(AnalyticsBarChart, { props: { data: [] } });
    expect(wrapper.text()).toContain('Нет данных');
    expect(rendered.last).toBeNull();
  });
});
