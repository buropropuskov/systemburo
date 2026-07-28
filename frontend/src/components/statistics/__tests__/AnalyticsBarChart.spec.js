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
    // Десктоп подписи не усекает: форматтер живёт только в мобильном брейкпоинте,
    // а безусловный trim резал бы и короткие «00:00» до точки (проверено на 390).
    expect(opts.xaxis.labels.trim).toBeUndefined();
    expect(opts.xaxis.labels.formatter).toBeUndefined();
  });

  it('на узком экране сокращает число подписей оси X, бары не трогает', () => {
    // 24 часовых бара с 12 тиками сливаются на 390px в «00:0002:00...» -
    // responsive-брейкпоинт снижает tickAmount, но категории (бары) остаются все.
    rendered.last = null;
    const hours = Array.from({ length: 24 }, (_, h) => ({ label: `${String(h).padStart(2, '0')}:00`, value: h }));
    mount(AnalyticsBarChart, { props: { data: hours } });
    const opts = rendered.last.options;
    expect(opts.xaxis.categories).toHaveLength(24);
    const narrow = opts.responsive.find((r) => r.breakpoint === 768);
    expect(narrow).toBeTruthy();
    expect(narrow.options.xaxis.tickAmount).toBe(6);
    // На телефоне длинные категориальные подписи разреза (организация/место)
    // усекаются с многоточием, короткие числовые/статусы проходят как есть.
    const fmt = narrow.options.xaxis.labels.formatter;
    expect(fmt('08:00')).toBe('08:00');
    expect(fmt('Завершено')).toBe('Завершено');
    expect(fmt('ООО «Производственно-строительное объединение»')).toBe('ООО «Производ…');
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

  it('столбец без значения пропускается, а не рисуется нулевым', () => {
    // «Нет данных» у производной метрики (этап никто не прошёл) != 0: столбец
    // нулевой высоты читался бы как реальное значение «прошло мгновенно».
    rendered.last = null;
    mount(AnalyticsBarChart, {
      props: {
        data: [
          { label: 'ООО А', value: 7 },
          { label: 'ООО Б', value: null },
          { label: 'ООО В', value: 0 },
        ],
      },
    });
    // 0 — честное значение и остаётся нулём, null — пропуск.
    expect(rendered.last.series[0].data).toEqual([7, null, 0]);
  });

  it('ни одного столбца со значением: заглушка, а не пустая сетка', () => {
    // Мультиметрика: строки есть по другой метрике, а выбранный этап не прошёл никто.
    rendered.last = null;
    const wrapper = mount(AnalyticsBarChart, {
      props: { data: [{ label: 'ООО А', value: null }, { label: 'ООО Б', value: null }] },
    });
    expect(wrapper.text()).toContain('Нет данных');
    expect(rendered.last).toBeNull();
  });

  it('ряд из одних нулей — данные: ноль это значение, а не его отсутствие', () => {
    rendered.last = null;
    mount(AnalyticsBarChart, { props: { data: [{ label: 'ООО А', value: 0 }] } });
    expect(rendered.last).not.toBeNull();
    expect(rendered.last.series[0].data).toEqual([0]);
  });

  it('valueType=duration: ось и тултип рисуют длительность, а не сырые секунды', () => {
    rendered.last = null;
    mount(AnalyticsBarChart, { props: { data: DATA, valueType: 'duration' } });
    const opts = rendered.last.options;
    expect(opts.yaxis.labels.formatter(8100)).toBe('2 ч 15 мин');
    expect(opts.tooltip.y.formatter(259200)).toBe('3 сут');
  });

  it('без valueType длительности нет: ось остаётся числом с единицей', () => {
    rendered.last = null;
    mount(AnalyticsBarChart, { props: { data: DATA, unitForms: ['проезд', 'проезда', 'проездов'] } });
    const opts = rendered.last.options;
    // Разделитель разрядов берём у самой локали: он неразрывный и зависит от ICU.
    const n = (8100).toLocaleString('ru-RU');
    expect(opts.yaxis.labels.formatter(8100)).toBe(n);
    expect(opts.tooltip.y.formatter(8100)).toBe(`${n} проездов`);
  });
});
