import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';

// Chart.js рисует на холсте, которого в jsdom нет. Подменяем сам конструктор
// и запоминаем конфигурацию: проверяем собранный конфиг (тип, данные, подписи,
// обработчики подсказки и оси), а не отрисовку.
const { rendered } = vi.hoisted(() => ({ rendered: { last: null, built: 0, destroyed: 0 } }));
vi.mock('chart.js', () => {
  class ChartStub {
    constructor(canvas, config) {
      rendered.last = config;
      rendered.built += 1;
      this.canvas = canvas;
    }

    destroy() {
      rendered.destroyed += 1;
    }
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

import AnalyticsAreaChart from '../AnalyticsAreaChart.vue';

const DATA = [
  { timestamp: '2026-06-15', count: 4 },
  { timestamp: '2026-06-16', count: 9 },
];

/** Монтирует компонент и отдаёт конфигурацию, ушедшую в Chart.js. */
async function build(props) {
  rendered.last = null;
  rendered.built = 0;
  rendered.destroyed = 0;
  const wrapper = mount(AnalyticsAreaChart, { props, attachTo: document.body });
  await wrapper.vm.$nextTick();
  return { wrapper, config: rendered.last };
}

describe('AnalyticsAreaChart', () => {
  it('строит area-серию: значения, палитра проекта, градиентная заливка', async () => {
    const { config } = await build({ data: DATA, color: '#4F5BDF', seriesName: 'Динамика заявок' });

    expect(config).not.toBeNull();
    expect(config.type).toBe('line');
    const ds = config.data.datasets[0];
    expect(ds.label).toBe('Динамика заявок');
    expect(ds.data).toEqual([4, 9]);
    expect(ds.borderColor).toBe('#4F5BDF');
    // Заливка area строится градиентом по области графика, поэтому это функция,
    // а не цвет: до первой разметки области нет и заливать нечего.
    expect(ds.fill).toBe(true);
    expect(typeof ds.backgroundColor).toBe('function');
    expect(ds.backgroundColor({ chart: {} })).toBe('transparent');
    // Сглаживание кривой - тот же вид, что был у прежнего движка.
    expect(ds.tension).toBeGreaterThan(0);
    expect(config.options.plugins.legend.display).toBe(false);
  });

  it('градиент строится в цвете ряда от насыщенного верха к прозрачному низу', async () => {
    const { config } = await build({ data: DATA, color: '#4F5BDF' });
    const stops = [];
    const chart = {
      chartArea: { top: 0, bottom: 100 },
      ctx: {
        createLinearGradient: () => ({ addColorStop: (at, color) => stops.push([at, color]) }),
      },
    };
    config.data.datasets[0].backgroundColor({ chart });
    expect(stops).toEqual([
      [0, 'rgba(79, 91, 223, 0.32)'],
      [1, 'rgba(79, 91, 223, 0.02)'],
    ]);
  });

  it('точка под курсором не сливается с линией своего же цвета', async () => {
    const { config } = await build({ data: DATA, color: '#4F5BDF' });
    const ds = config.data.datasets[0];

    // Точка появляется только под курсором: в покое линия идёт без узлов.
    expect(ds.pointRadius).toBe(0);
    expect(ds.pointHoverRadius).toBeGreaterThan(0);
    // Белое кольцо отбивает точку от линии: без него Chart.js красит её цветом
    // ряда и заливкой области, и на собственном графике точку не видно.
    expect(ds.pointHoverBorderColor).toBe('#ffffff');
    expect(ds.pointHoverBorderWidth).toBeGreaterThan(0);
    // Сердцевина в цвете ряда - точка читается как узел своей линии.
    expect(ds.pointHoverBackgroundColor).toBe('#4F5BDF');
  });

  it('подсказка встаёт рядом с точкой и не накрывает её', async () => {
    const { config } = await build({ data: DATA });
    const { tooltip } = config.options.plugins;
    // 'nearest' + отступ каретки: по центру ряда и вплотную подсказка ложилась
    // на саму точку, ради которой наводятся.
    expect(tooltip.position).toBe('nearest');
    expect(tooltip.caretPadding).toBeGreaterThanOrEqual(10);
    // Точка на верхней отметке шкалы иначе срезается краем холста.
    expect(config.options.layout.padding.top).toBeGreaterThan(0);
  });

  it('под курсором ведёт вертикаль к оси X', async () => {
    const { config } = await build({ data: DATA });
    expect(config.plugins.map((p) => p.id)).toContain('crosshair');
  });

  it('метки оси X — дд.мм без сдвига таймзоны', async () => {
    const { config } = await build({ data: DATA });
    // '2026-06-15' -> '15.06' (день не съезжает на 14.06 из-за UTC-полуночи)
    expect(config.data.labels).toEqual(['15.06', '16.06']);
  });

  it('тултип склоняет единицу по числу и показывает полную дату', async () => {
    const { config } = await build({ data: DATA, unitForms: ['заявка', 'заявки', 'заявок'] });
    const { callbacks } = config.options.plugins.tooltip;
    expect(callbacks.label({ raw: 1 })).toBe('1 заявка');
    expect(callbacks.label({ raw: 2 })).toBe('2 заявки');
    expect(callbacks.label({ raw: 5 })).toBe('5 заявок');
    // полная дата по индексу точки
    expect(callbacks.title([{ dataIndex: 0 }])).toBe('15.06.2026');
  });

  it('пустые данные: показывает заглушку, график не рендерит', async () => {
    const { wrapper, config } = await build({ data: [] });
    expect(wrapper.text()).toContain('Нет данных');
    expect(config).toBeNull();
  });

  it('точка без значения остаётся разрывом, а не нулём', async () => {
    // «Нет данных» у производной метрики (этап никто не прошёл) != 0: точка на
    // дне шкалы читалась бы как реальное значение «прошло мгновенно».
    const { config } = await build({
      data: [
        { timestamp: '2026-06-15', count: 4 },
        { timestamp: '2026-06-16', count: null },
        { timestamp: '2026-06-17', count: 0 },
      ],
    });
    // 0 — честное значение и остаётся нулём, null — разрыв.
    expect(config.data.datasets[0].data).toEqual([4, null, 0]);
    // Разрыв не затягивается прямой: иначе линия показала бы значение там,
    // где данных нет.
    expect(config.data.datasets[0].spanGaps).toBe(false);
  });

  it('ни одной точки со значением: заглушка, а не пустая сетка', async () => {
    const { wrapper, config } = await build({
      data: [
        { timestamp: '2026-06-15', count: null },
        { timestamp: '2026-06-16', count: null },
      ],
    });
    expect(wrapper.text()).toContain('Нет данных');
    expect(config).toBeNull();
  });

  it('ряд из одних нулей — данные: ноль это значение, а не его отсутствие', async () => {
    const { config } = await build({ data: [{ timestamp: '2026-06-15', count: 0 }] });
    expect(config).not.toBeNull();
    expect(config.data.datasets[0].data).toEqual([0]);
  });

  it('valueType=duration: ось и тултип рисуют длительность, а не сырые секунды', async () => {
    const { config } = await build({ data: DATA, valueType: 'duration' });
    expect(config.options.scales.y.ticks.callback(8100)).toBe('2 ч 15 мин');
    expect(config.options.plugins.tooltip.callbacks.label({ raw: 259200 })).toBe('3 сут');
  });

  it('без valueType длительности нет: ось остаётся числом с единицей', async () => {
    const { config } = await build({ data: DATA, unitForms: ['заявка', 'заявки', 'заявок'] });
    // Разделитель разрядов берём у самой локали: он неразрывный и зависит от ICU.
    const n = (8100).toLocaleString('ru-RU');
    expect(config.options.scales.y.ticks.callback(8100)).toBe(n);
    expect(config.options.plugins.tooltip.callbacks.label({ raw: 8100 })).toBe(`${n} заявок`);
  });

  it('перестроение по новым данным разрушает прежний экземпляр', async () => {
    // Chart.js держит экземпляр в реестре по холсту: не разрушив прежний, при
    // смене данных получаешь «Canvas is already in use» и лишнего слушателя
    // изменения размера на каждую смену фильтра.
    const { wrapper } = await build({ data: DATA });
    expect(rendered.built).toBe(1);

    await wrapper.setProps({ data: [{ timestamp: '2026-06-15', count: 1 }] });
    await wrapper.vm.$nextTick();
    expect(rendered.last.data.datasets[0].data).toEqual([1]);
    expect(rendered.built).toBe(2);
    expect(rendered.destroyed).toBe(1);
  });

  it('размонтирование разрушает экземпляр', async () => {
    const { wrapper } = await build({ data: DATA });
    wrapper.unmount();
    expect(rendered.destroyed).toBe(1);
  });
});
