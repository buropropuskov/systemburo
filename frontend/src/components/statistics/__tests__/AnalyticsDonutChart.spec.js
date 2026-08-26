import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';

// Chart.js рисует на холсте, которого в jsdom нет. Подменяем сам конструктор
// и запоминаем конфигурацию: проверяем собранный конфиг (тип, доли, подписи,
// обработчики подсказки), а не отрисовку.
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

import AnalyticsDonutChart from '../AnalyticsDonutChart.vue';

const DATA = [
  { label: 'Автозаявки', value: 12 },
  { label: 'Проведение работ', value: 8 },
];

/** Монтирует компонент и отдаёт конфигурацию, ушедшую в Chart.js. */
async function build(props) {
  rendered.last = null;
  rendered.built = 0;
  rendered.destroyed = 0;
  const wrapper = mount(AnalyticsDonutChart, { props, attachTo: document.body });
  await wrapper.vm.$nextTick();
  return { wrapper, config: rendered.last };
}

describe('AnalyticsDonutChart', () => {
  it('строит кольцо: доли, метки сегментов, палитра проекта', async () => {
    const { config } = await build({
      data: DATA,
      unitForms: ['вложение', 'вложения', 'вложений'],
    });

    expect(config).not.toBeNull();
    expect(config.type).toBe('doughnut');
    expect(config.data.datasets[0].data).toEqual([12, 8]);
    expect(config.data.labels).toEqual(['Автозаявки', 'Проведение работ']);
    expect(config.data.datasets[0].backgroundColor[0]).toBe('#4F5BDF');
    // Именно кольцо, а не круг: вырез той же доли радиуса, что был раньше.
    expect(config.options.cutout).toBe('64%');
  });

  it('отбрасывает нулевые сегменты, чтобы не искажать кольцо', async () => {
    const { config } = await build({
      data: [{ label: 'A', value: 5 }, { label: 'B', value: 0 }, { label: 'C', value: 3 }],
    });
    expect(config.data.datasets[0].data).toEqual([5, 3]);
    expect(config.data.labels).toEqual(['A', 'C']);
  });

  it('сегментов больше, чем цветов: палитра идёт по кругу, а не обрывается', async () => {
    // Chart.js цвет по индексу берёт из массива как есть, поэтому раскладку
    // держим на своей стороне: девятому сегменту иначе не досталось бы цвета.
    const { config } = await build({
      data: Array.from({ length: 9 }, (_, i) => ({ label: `Т${i}`, value: i + 1 })),
    });
    const colors = config.data.datasets[0].backgroundColor;
    expect(colors).toHaveLength(9);
    expect(colors[8]).toBe(colors[0]);
  });

  it('тултип склоняет единицу по числу сегмента и называет сам сегмент', async () => {
    const { config } = await build({ data: DATA, unitForms: ['проезд', 'проезда', 'проездов'] });
    const { callbacks } = config.options.plugins.tooltip;
    expect(callbacks.label({ label: 'A', raw: 1 })).toBe(' A: 1 проезд');
    expect(callbacks.label({ label: 'A', raw: 2 })).toBe(' A: 2 проезда');
    expect(callbacks.label({ label: 'A', raw: 5 })).toBe(' A: 5 проездов');
    // Имя сегмента уже в строке значения — заголовок его не повторяет.
    expect(callbacks.title()).toBe('');
  });

  it('тултип с isFloat: дробное значение с запятой, склонение по округлению', async () => {
    const { config } = await build({
      data: [{ label: 'A', value: 1.7 }],
      isFloat: true,
      unitForms: ['проезд', 'проезда', 'проездов'],
    });
    expect(config.options.plugins.tooltip.callbacks.label({ label: 'A', raw: 1.7 }))
      .toBe(' A: 1,7 проезда');
  });

  it('центр кольца подписан итогом по сегментам', async () => {
    const { config } = await build({ data: DATA, totalLabel: 'Всего вложений' });
    // Сами подписи рисуют плагины: без них кольцо осталось бы немым.
    expect(config.plugins.map((p) => p.id)).toEqual(['sliceLabels', 'centerLabel']);

    const drawn = [];
    const ctx = { save() {}, restore() {}, fillText: (text) => drawn.push(text) };
    config.plugins[1].afterDatasetsDraw({
      ctx,
      data: config.data,
      getDatasetMeta: () => ({ data: [{ x: 50, y: 50 }] }),
      getDataVisibility: () => true,
      getActiveElements: () => [],
    });
    expect(drawn).toEqual(['Всего вложений', '20']);
  });

  it('в настройках графика нет функций: Chart.js вызвал бы их как вычисляемые', async () => {
    // Формат значения, положенный в options.plugins, Chart.js зовёт сам и
    // передаёт свой контекст вместо числа - кольцо падало на отрисовке, а мок
    // конструктора этого не показывал. Настройки плагинов идут замыканием.
    const { config } = await build({ data: DATA });
    // Места, где функция законна: обработчики подсказки и подписей делений оси
    // плюс цвета оформления - их Chart.js зовёт на каждой отрисовке и берёт
    // возвращённый цвет, так что тема успевает смениться. Понадобится ещё одно
    // (legend.onClick, animation.onComplete) - дописать путь сюда, а не снимать
    // замок: он стоит не против функций вообще, а против настроек, которые
    // Chart.js вызовет вместо того, чтобы прочитать.
    const allowed = [
      /^options\.plugins\.tooltip\.callbacks\./,
      /^options\.scales\.[^.]+\.ticks\.callback$/,
      /^options\.plugins\.legend\.labels\.color$/,
      /^options\.plugins\.tooltip\.borderColor$/,
    ];
    const functionsIn = (value, path = 'options') => {
      if (allowed.some((re) => re.test(path))) return [];
      if (typeof value === 'function') return [path];
      if (!value || typeof value !== 'object') return [];
      return Object.entries(value).flatMap(([key, inner]) => functionsIn(inner, `${path}.${key}`));
    };
    expect(functionsIn(config.options)).toEqual([]);
  });

  it('empty-ring: без данных рисует ободок-заглушку с нулём вместо текста', async () => {
    const { wrapper, config } = await build({ data: [], emptyRing: true });

    // Место графика не пустеет: холст на месте, заглушки текстом нет.
    expect(wrapper.text()).not.toContain('Нет данных');
    expect(config).not.toBeNull();
    const ds = config.data.datasets[0];
    // Единица - способ получить замкнутую дугу: нулевой сегмент Chart.js не рисует.
    expect(ds.data).toEqual([1]);
    // Цвет заглушки - вычисляемый (тема), а не массив: значение внутри массива
    // Chart.js вычисляемым не считает и красит кольцо чёрным.
    expect(typeof ds.backgroundColor).toBe('function');
    expect(ds.backgroundColor({})).toBe('#eef0f7');
    // Заглушка не притворяется данными: ни доли, ни легенды, ни подсказки.
    expect(config.options.plugins.sliceLabels.display).toBe(false);
    expect(config.options.plugins.legend.display).toBe(false);
    expect(config.options.plugins.tooltip.enabled).toBe(false);
  });

  it('empty-ring не трогает кольцо с данными', async () => {
    const { config } = await build({ data: DATA, emptyRing: true });
    expect(config.data.datasets[0].data).toEqual(DATA.map((d) => d.value));
    expect(config.options.plugins.legend.display).toBe(true);
    expect(config.options.plugins.tooltip.enabled).toBe(true);
  });

  it('без empty-ring пустые данные по-прежнему дают заглушку текстом', async () => {
    const { wrapper, config } = await build({ data: [] });
    expect(wrapper.text()).toContain('Нет данных');
    expect(config).toBeNull();
  });

  it('легенда стоит под кольцом', async () => {
    const { config } = await build({ data: DATA });
    expect(config.options.plugins.legend.position).toBe('bottom');
  });

  it('сегмент под курсором подсвечивается, а не остаётся в своём цвете', async () => {
    const { config } = await build({ data: DATA });
    const ds = config.data.datasets[0];
    expect(ds.hoverBackgroundColor[0]).not.toBe(ds.backgroundColor[0]);
  });

  it('пустые данные: показывает заглушку, график не рендерит', async () => {
    const { wrapper, config } = await build({ data: [] });
    expect(wrapper.text()).toContain('Нет данных');
    expect(config).toBeNull();
  });

  it('перестроение по новым данным разрушает прежний экземпляр', async () => {
    // Chart.js держит экземпляр в реестре по холсту: не разрушив прежний, при
    // смене данных получаешь «Canvas is already in use» и лишнего слушателя
    // изменения размера на каждую смену фильтра.
    const { wrapper } = await build({ data: DATA });
    expect(rendered.built).toBe(1);

    await wrapper.setProps({ data: [{ label: 'A', value: 1 }] });
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
