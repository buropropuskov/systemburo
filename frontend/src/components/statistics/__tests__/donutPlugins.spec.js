import { describe, it, expect } from 'vitest';
import { centerLabelPlugin, sliceLabelsPlugin } from '../donutPlugins';

const DEG = Math.PI / 180;

/**
 * Поддельный график с холстом-протоколом: плагины рисуют, а тест смотрит, что
 * именно и в каком месте они нарисовали.
 */
function fakeChart({ values, labels = [], arcs, hidden = [], active = [] }) {
  const drawn = [];
  const ctx = {
    font: '',
    fillStyle: '',
    textAlign: '',
    textBaseline: '',
    save() {},
    restore() {},
  };
  ctx.fillText = (text, x, y) => drawn.push({ text, x, y, font: ctx.font, fillStyle: ctx.fillStyle });

  const chart = {
    ctx,
    data: { labels, datasets: [{ data: values }] },
    options: { plugins: {} },
    getDatasetMeta: () => ({
      data: arcs.map((a, i) => ({
        startAngle: a.start,
        endAngle: a.end,
        x: a.x ?? 100,
        y: a.y ?? 100,
        tooltipPosition: () => ({ x: a.x ?? 10 * (i + 1), y: a.y ?? 20 * (i + 1) }),
      })),
    }),
    getDataVisibility: (i) => !hidden.includes(i),
    getActiveElements: () => active,
  };
  return { chart, drawn };
}

/** Три равные доли по 120 градусов. */
const THIRDS = [
  { start: 0, end: 120 * DEG },
  { start: 120 * DEG, end: 240 * DEG },
  { start: 240 * DEG, end: 360 * DEG },
];

describe('sliceLabelsPlugin', () => {
  it('подписывает долю каждого сегмента в процентах', () => {
    const { chart, drawn } = fakeChart({ values: [50, 25, 25], arcs: THIRDS });
    sliceLabelsPlugin.afterDatasetsDraw(chart);
    expect(drawn.map((d) => d.text)).toEqual(['50%', '25%', '25%']);
    expect(drawn[0].fillStyle).toBe('#ffffff');
  });

  it('подпись ставится в середину дуги, а не в центр кольца', () => {
    const { chart, drawn } = fakeChart({
      values: [1, 1],
      arcs: [
        { start: 0, end: 180 * DEG, x: 40, y: 60 },
        { start: 180 * DEG, end: 360 * DEG, x: 140, y: 60 },
      ],
    });
    sliceLabelsPlugin.afterDatasetsDraw(chart);
    expect(drawn.map((d) => [d.x, d.y])).toEqual([[40, 60], [140, 60]]);
  });

  it('тонкую долю не подписывает: текст не помещается в дугу и лезет на соседей', () => {
    const { chart, drawn } = fakeChart({
      values: [99, 1],
      arcs: [
        { start: 0, end: 356 * DEG },
        // 4 градуса — меньше порога читаемости.
        { start: 356 * DEG, end: 360 * DEG },
      ],
    });
    sliceLabelsPlugin.afterDatasetsDraw(chart);
    expect(drawn.map((d) => d.text)).toEqual(['99%']);
  });

  it('скрытый легендой сегмент не подписан, а доли пересчитаны без него', () => {
    // Скрытый сегмент остаётся в данных, и если считать проценты по всем,
    // видимые доли перестанут давать в сумме сто.
    const { chart, drawn } = fakeChart({
      values: [30, 30, 40],
      arcs: [
        { start: 0, end: 180 * DEG },
        { start: 180 * DEG, end: 360 * DEG },
        { start: 0, end: 0 },
      ],
      hidden: [2],
    });
    sliceLabelsPlugin.afterDatasetsDraw(chart);
    expect(drawn.map((d) => d.text)).toEqual(['50%', '50%']);
  });

  it('нет видимых значений — не рисует ничего вместо NaN%', () => {
    const { chart, drawn } = fakeChart({
      values: [10],
      arcs: [{ start: 0, end: 360 * DEG }],
      hidden: [0],
    });
    sliceLabelsPlugin.afterDatasetsDraw(chart);
    expect(drawn).toEqual([]);
  });
});

describe('centerLabelPlugin', () => {
  const center = () => centerLabelPlugin({ label: 'Всего', format: (v) => `${v} шт` });

  it('без наведения показывает подпись итога и сумму видимых сегментов', () => {
    const { chart, drawn } = fakeChart({
      values: [12, 8],
      labels: ['A', 'B'],
      arcs: [{ start: 0, end: 180 * DEG, x: 90, y: 70 }, { start: 180 * DEG, end: 360 * DEG }],
    });
    center().afterDatasetsDraw(chart);
    expect(drawn.map((d) => d.text)).toEqual(['Всего', '20 шт']);
    // Обе строки стоят в середине кольца, одна над другой.
    expect(drawn.map((d) => d.x)).toEqual([90, 90]);
    expect(drawn[0].y).toBeLessThan(drawn[1].y);
  });

  it('сумма считается по видимым: скрытый легендой сегмент в итог не идёт', () => {
    const { chart, drawn } = fakeChart({
      values: [12, 8],
      labels: ['A', 'B'],
      arcs: THIRDS.slice(0, 2),
      hidden: [1],
    });
    center().afterDatasetsDraw(chart);
    expect(drawn[1].text).toBe('12 шт');
  });

  it('под курсором показывает имя сегмента и его значение', () => {
    const { chart, drawn } = fakeChart({
      values: [12, 8],
      labels: ['Автозаявки', 'Работы'],
      arcs: THIRDS.slice(0, 2),
      active: [{ datasetIndex: 0, index: 1 }],
    });
    center().afterDatasetsDraw(chart);
    expect(drawn.map((d) => d.text)).toEqual(['Работы', '8 шт']);
  });

  it('без настроек берёт дефолты, а не падает', () => {
    // Плагин общий: подпись и формат нужны не всякому кольцу.
    const { chart, drawn } = fakeChart({
      values: [2, 3],
      arcs: THIRDS.slice(0, 2),
    });
    centerLabelPlugin().afterDatasetsDraw(chart);
    expect(drawn.map((d) => d.text)).toEqual(['', '5']);
  });

  it('кольца ещё нет - рисовать не по чему, и плагин молчит', () => {
    // Первый проход разметки идёт до построения дуг: без якоря середины
    // подпись поставить некуда.
    const { chart, drawn } = fakeChart({ values: [1], arcs: [] });
    center().afterDatasetsDraw(chart);
    expect(drawn).toEqual([]);
  });
});
