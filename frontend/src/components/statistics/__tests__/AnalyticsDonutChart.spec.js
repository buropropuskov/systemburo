import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';

// apexcharts требует реального SVG/измерений — мокаем vue3-apexcharts стабом,
// который запоминает последние props, чтобы проверять собранный конфиг (тип,
// серию, метки, форматтеры), а не рендер в jsdom.
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

import AnalyticsDonutChart from '../AnalyticsDonutChart.vue';

const DATA = [
  { label: 'Автозаявки', value: 12 },
  { label: 'Проведение работ', value: 8 },
];

describe('AnalyticsDonutChart', () => {
  it('строит donut: серия значений, метки сегментов, палитра проекта', () => {
    rendered.last = null;
    mount(AnalyticsDonutChart, {
      props: { data: DATA, unitForms: ['вложение', 'вложения', 'вложений'] },
    });

    expect(rendered.last).not.toBeNull();
    expect(rendered.last.type).toBe('donut');
    expect(rendered.last.series).toEqual([12, 8]);
    expect(rendered.last.options.labels).toEqual(['Автозаявки', 'Проведение работ']);
    expect(rendered.last.options.colors[0]).toBe('#4F5BDF');
  });

  it('отбрасывает нулевые сегменты, чтобы не искажать кольцо', () => {
    rendered.last = null;
    mount(AnalyticsDonutChart, {
      props: { data: [{ label: 'A', value: 5 }, { label: 'B', value: 0 }, { label: 'C', value: 3 }] },
    });
    expect(rendered.last.series).toEqual([5, 3]);
    expect(rendered.last.options.labels).toEqual(['A', 'C']);
  });

  it('тултип склоняет единицу по числу сегмента', () => {
    rendered.last = null;
    mount(AnalyticsDonutChart, {
      props: { data: DATA, unitForms: ['проезд', 'проезда', 'проездов'] },
    });
    const tip = rendered.last.options.tooltip;
    expect(tip.y.formatter(1)).toBe('1 проезд');
    expect(tip.y.formatter(2)).toBe('2 проезда');
    expect(tip.y.formatter(5)).toBe('5 проездов');
  });

  it('тултип с isFloat: дробное значение с запятой, склонение по округлению', () => {
    rendered.last = null;
    mount(AnalyticsDonutChart, {
      props: { data: [{ label: 'A', value: 1.7 }], isFloat: true, unitForms: ['проезд', 'проезда', 'проездов'] },
    });
    expect(rendered.last.options.tooltip.y.formatter(1.7)).toBe('1,7 проезда');
  });

  it('центр кольца суммирует сегменты', () => {
    rendered.last = null;
    mount(AnalyticsDonutChart, { props: { data: DATA } });
    const total = rendered.last.options.plotOptions.pie.donut.labels.total;
    expect(total.formatter({ globals: { seriesTotals: [12, 8] } })).toBe('20');
  });

  it('пустые данные: показывает заглушку, график не рендерит', () => {
    rendered.last = null;
    const wrapper = mount(AnalyticsDonutChart, { props: { data: [] } });
    expect(wrapper.text()).toContain('Нет данных');
    expect(rendered.last).toBeNull();
  });
});
