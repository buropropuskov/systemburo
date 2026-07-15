import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';

// apexcharts требует реального SVG/измерений — мокаем vue3-apexcharts стабом,
// который запоминает последние props, чтобы проверять собранный конфиг (тип,
// серию, палитру, форматтеры), а не рендер в jsdom.
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

import AnalyticsAreaChart from '../AnalyticsAreaChart.vue';

const DATA = [
  { timestamp: '2026-06-15', count: 4 },
  { timestamp: '2026-06-16', count: 9 },
];

describe('AnalyticsAreaChart', () => {
  it('строит area-серию: значения, палитра проекта, градиентная заливка', () => {
    rendered.last = null;
    mount(AnalyticsAreaChart, {
      props: { data: DATA, color: '#4F5BDF', seriesName: 'Динамика заявок' },
    });

    expect(rendered.last).not.toBeNull();
    expect(rendered.last.type).toBe('area');
    expect(rendered.last.series[0].name).toBe('Динамика заявок');
    expect(rendered.last.series[0].data).toEqual([4, 9]);

    const opts = rendered.last.options;
    expect(opts.colors).toEqual(['#4F5BDF']);
    expect(opts.fill.type).toBe('gradient');
    expect(opts.stroke.curve).toBe('smooth');
    expect(opts.legend.show).toBe(false);
  });

  it('метки оси X — дд.мм без сдвига таймзоны', () => {
    rendered.last = null;
    mount(AnalyticsAreaChart, { props: { data: DATA } });
    // '2026-06-15' -> '15.06' (день не съезжает на 14.06 из-за UTC-полуночи)
    expect(rendered.last.options.xaxis.categories).toEqual(['15.06', '16.06']);
  });

  it('тултип склоняет единицу по числу и показывает полную дату', () => {
    rendered.last = null;
    mount(AnalyticsAreaChart, {
      props: { data: DATA, unitForms: ['заявка', 'заявки', 'заявок'] },
    });
    const tip = rendered.last.options.tooltip;
    expect(tip.y.formatter(1)).toBe('1 заявка');
    expect(tip.y.formatter(2)).toBe('2 заявки');
    expect(tip.y.formatter(5)).toBe('5 заявок');
    // полная дата по индексу точки
    expect(tip.x.formatter(null, { dataPointIndex: 0 })).toBe('15.06.2026');
  });

  it('пустые данные: показывает заглушку, график не рендерит', () => {
    rendered.last = null;
    const wrapper = mount(AnalyticsAreaChart, { props: { data: [] } });
    expect(wrapper.text()).toContain('Нет данных');
    expect(rendered.last).toBeNull();
  });

  it('точка без значения остаётся разрывом, а не нулём', () => {
    // «Нет данных» у производной метрики (этап никто не прошёл) != 0: ноль на дне
    // шкалы читался бы как реальное значение «прошло мгновенно».
    rendered.last = null;
    mount(AnalyticsAreaChart, {
      props: {
        data: [
          { timestamp: '2026-06-15', count: 4 },
          { timestamp: '2026-06-16', count: null },
          { timestamp: '2026-06-17', count: 0 },
        ],
      },
    });
    // 0 — честное значение и остаётся нулём, null — разрыв.
    expect(rendered.last.series[0].data).toEqual([4, null, 0]);
  });

  it('ни одной точки со значением: заглушка, а не пустая сетка', () => {
    // Мультиметрика: строки есть по другой метрике, а выбранный этап не прошёл никто.
    rendered.last = null;
    const wrapper = mount(AnalyticsAreaChart, {
      props: { data: [{ timestamp: '2026-06-15', count: null }, { timestamp: '2026-06-16', count: null }] },
    });
    expect(wrapper.text()).toContain('Нет данных');
    expect(rendered.last).toBeNull();
  });

  it('ряд из одних нулей — данные: ноль это значение, а не его отсутствие', () => {
    rendered.last = null;
    mount(AnalyticsAreaChart, { props: { data: [{ timestamp: '2026-06-15', count: 0 }] } });
    expect(rendered.last).not.toBeNull();
    expect(rendered.last.series[0].data).toEqual([0]);
  });

  it('valueType=duration: ось и тултип рисуют длительность, а не сырые секунды', () => {
    rendered.last = null;
    mount(AnalyticsAreaChart, {
      props: { data: DATA, valueType: 'duration' },
    });
    const opts = rendered.last.options;
    expect(opts.yaxis.labels.formatter(8100)).toBe('2 ч 15 мин');
    expect(opts.tooltip.y.formatter(259200)).toBe('3 сут');
  });

  it('без valueType длительности нет: ось остаётся числом с единицей', () => {
    rendered.last = null;
    mount(AnalyticsAreaChart, { props: { data: DATA, unitForms: ['заявка', 'заявки', 'заявок'] } });
    const opts = rendered.last.options;
    // Разделитель разрядов берём у самой локали: он неразрывный и зависит от ICU.
    const n = (8100).toLocaleString('ru-RU');
    expect(opts.yaxis.labels.formatter(8100)).toBe(n);
    expect(opts.tooltip.y.formatter(8100)).toBe(`${n} заявок`);
  });
});
