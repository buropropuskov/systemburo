import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';

// Столбики обращений по суткам (#2125 S9e). Chart.js рисует на холсте, которого
// в jsdom нет: подменяем конструктор и проверяем собранную конфигурацию - ряды,
// подписи осей и обработчики подсказки.
const { rendered } = vi.hoisted(() => ({ rendered: { last: null } }));
vi.mock('chart.js', () => {
  class ChartStub {
    constructor(canvas, config) {
      rendered.last = config;
      this.canvas = canvas;
    }

    destroy() {}
  }
  ChartStub.register = () => {};
  return {
    Chart: ChartStub,
    ArcElement: {},
    BarController: {},
    BarElement: {},
    CategoryScale: {},
    DoughnutController: {},
    Filler: {},
    Legend: {},
    LineController: {},
    LineElement: {},
    LinearScale: {},
    PointElement: {},
    Tooltip: {},
  };
});

import DailyRequestsChart from '../DailyRequestsChart.vue';

// Разряды разделяет неразрывный пробел: так их ставит toLocaleString('ru-RU'),
// на котором стоит форматирование чисел раздела.
const NBSP = '\u00A0';

const POINTS = [
  { day: '2026-08-10', requests: 1000, errors: 10 },
  { day: '2026-08-11', requests: null, errors: null },
  { day: '2026-08-12', requests: 500, errors: 0 },
];

/** Ширина экрана для useNarrowScreen: порог тот же, что у мобильных @media. */
function setViewport(narrow) {
  window.matchMedia = (query) => ({
    matches: narrow && query.includes('max-width'),
    media: query,
    addEventListener() {},
    removeEventListener() {},
    addListener() {},
    removeListener() {},
  });
}

/** Монтирует график и отдаёт конфигурацию, ушедшую в Chart.js. */
async function build(points = POINTS) {
  rendered.last = null;
  const wrapper = mount(DailyRequestsChart, { props: { points }, attachTo: document.body });
  await wrapper.vm.$nextTick();
  await wrapper.vm.$nextTick();
  return { wrapper, config: rendered.last };
}

/**
 * Пункты подсказки над столбиком - так, как их собирает сам Chart.js в режиме
 * `index`: столбик высотой null он пропущенным НЕ считает (это делает только
 * точка линии), поэтому в список входят оба ряда, а отсев остаётся за `filter`
 * из настроек. Ручная фильтрация здесь дала бы ложно-зелёный тест: подсказка
 * на сутках без записей показывала бы нули, а спека этого не заметила.
 */
function tooltipItems(config, index) {
  const { filter } = config.options.plugins.tooltip;
  return config.data.datasets
    .map((ds, datasetIndex) => ({ dataset: ds, datasetIndex, dataIndex: index, raw: ds.data[index] }))
    .filter(item => filter(item));
}

describe('DailyRequestsChart', () => {
  beforeEach(() => setViewport(false));

  it('делит столбик на успешные и ошибочные, складывая их в число запросов дня', async () => {
    const { config } = await build();
    const [success, errors] = config.data.datasets;

    expect(config.type).toBe('bar');
    expect(success.label).toBe('Успешных');
    expect(success.data).toEqual([990, null, 500]);
    expect(errors.label).toBe('С ошибкой');
    expect(errors.data).toEqual([10, null, null]);
    expect(config.options.scales.x.stacked).toBe(true);
    expect(config.options.scales.y.stacked).toBe(true);
  });

  it('сутки без ошибок оставляет пустыми, чтобы минимальная высота их не пометила', async () => {
    const { config } = await build();
    const errors = config.data.datasets[1];

    // Сегмент ошибок вытягивается до трёх пикселей, иначе одна ошибка на тысячу
    // запросов не видна вовсе. Ноль обязан остаться пустым: иначе день без
    // ошибок получил бы ту же полоску, что и день с ошибкой.
    expect(errors.minBarLength).toBe(3);
    expect(errors.data[2]).toBeNull();
  });

  it('день без записей не рисует столбика ни одним рядом', async () => {
    const { config } = await build();

    expect(config.data.datasets[0].data[1]).toBeNull();
    expect(config.data.datasets[1].data[1]).toBeNull();
  });

  it('ось подписывает сутками, подсказка называет дату целиком', async () => {
    const { config } = await build();

    expect(config.data.labels).toEqual(['10.08', '11.08', '12.08']);
    const title = config.options.plugins.tooltip.callbacks.title(tooltipItems(config, 0));
    expect(title).toBe('10.08.2026');
  });

  it('подсказка показывает числа обоих рядов, итог дня и долю ошибок', async () => {
    const { config } = await build();
    const { label, footer } = config.options.plugins.tooltip.callbacks;
    const items = tooltipItems(config, 0);

    expect(items.map(label)).toEqual(['Успешных: 990', 'С ошибкой: 10']);
    expect(footer(items)).toBe(`Всего 1${NBSP}000, доля ошибок 1.00%`);
  });

  it('в день без ошибок подсказка честно показывает нулевую долю', async () => {
    const { config } = await build();
    const items = tooltipItems(config, 2);

    expect(items.map(config.options.plugins.tooltip.callbacks.label)).toEqual(['Успешных: 500']);
    expect(config.options.plugins.tooltip.callbacks.footer(items)).toBe('Всего 500, доля ошибок 0.00%');
  });

  it('на сутках без записей объясняет словами, а не показывает нули', async () => {
    const { config } = await build();
    const { label, footer } = config.options.plugins.tooltip.callbacks;
    const items = tooltipItems(config, 1);

    // День, за который записей нет вовсе, - это не «ноль запросов»: столбика
    // нет, и подсказка обязана сказать то же самое.
    expect(items.map(label)).toEqual(['Записей за эти сутки нет']);
    expect(footer(items)).toBe('');
  });

  it('верх столбика скругляет тот ряд, который в этот день сверху', async () => {
    const { config } = await build();
    const [success, errors] = config.data.datasets;
    const corners = { topLeft: 4, topRight: 4, bottomLeft: 0, bottomRight: 0 };

    // Под красной шапкой скруглять синий нельзя - между сегментами появляется ямка.
    expect(success.borderRadius({ dataIndex: 0 })).toBe(0);
    expect(success.borderRadius({ dataIndex: 2 })).toEqual(corners);
    expect(errors.borderRadius).toEqual(corners);
  });

  it('два ряда различимы не только цветом: легенда на месте', async () => {
    const { config } = await build();

    expect(config.options.plugins.legend.display).not.toBe(false);
    expect(config.data.datasets.map(ds => ds.label)).toEqual(['Успешных', 'С ошибкой']);
  });

  it('цвета берутся у темы страницы, а столбик под курсором светлеет', async () => {
    const { config } = await build();
    const [success, errors] = config.data.datasets;
    const noTheme = { chart: { canvas: null } };

    // Тёмная тема переопределяет --accent и --danger, поэтому цвет читается с
    // холста в момент отрисовки, а не прибивается числом.
    expect(success.backgroundColor(noTheme)).toBe('#4F5BDF');
    expect(errors.backgroundColor(noTheme)).toBe('#dc3545');
    expect(success.hoverBackgroundColor(noTheme)).toBe('rgb(100, 111, 227)');
    expect(errors.hoverBackgroundColor(noTheme)).not.toBe(errors.backgroundColor(noTheme));
  });

  it('на узком экране подписей дат меньше, а столбики остаются все', async () => {
    setViewport(true);
    const month = Array.from({ length: 31 }, (_, i) => ({
      day: `2026-08-${String(i + 1).padStart(2, '0')}`, requests: 10 + i, errors: 0,
    }));
    const { config } = await build(month);

    expect(config.options.scales.x.ticks.maxTicksLimit).toBe(6);
    expect(config.data.labels).toHaveLength(31);
    expect(config.data.datasets[0].data).toHaveLength(31);
  });

  it('называет содержимое голосом экранного диктора', async () => {
    const { wrapper } = await build();

    expect(wrapper.get('canvas').attributes('aria-label'))
      .toBe(`Обращения по суткам: дней с записями 2, запросов 1${NBSP}500, из них с ошибкой 10`);
  });
});
