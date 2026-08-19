import { describe, it, expect, vi, beforeEach } from 'vitest';
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

import AnalyticsBarChart from '../AnalyticsBarChart.vue';

const DATA = [
  { label: '08:00', value: 3 },
  { label: '09:00', value: 7 },
];

/**
 * Ширина экрана для useNarrowScreen: порог тот же, что у мобильных @media.
 * jsdom своего matchMedia не имеет, поэтому подставляем его сами.
 */
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

/** Монтирует компонент и отдаёт конфигурацию, ушедшую в Chart.js. */
async function build(props) {
  rendered.last = null;
  rendered.built = 0;
  rendered.destroyed = 0;
  const wrapper = mount(AnalyticsBarChart, { props, attachTo: document.body });
  await wrapper.vm.$nextTick();
  await wrapper.vm.$nextTick();
  return { wrapper, config: rendered.last };
}

describe('AnalyticsBarChart', () => {
  beforeEach(() => setViewport(false));

  it('строит bar-серию: значения, категории, палитра проекта', async () => {
    const { config } = await build({ data: DATA, color: '#4F5BDF', seriesName: 'Проезды машин' });

    expect(config).not.toBeNull();
    expect(config.type).toBe('bar');
    const ds = config.data.datasets[0];
    expect(ds.label).toBe('Проезды машин');
    expect(ds.data).toEqual([3, 7]);
    expect(ds.backgroundColor).toBe('#4F5BDF');
    expect(config.data.labels).toEqual(['08:00', '09:00']);
    expect(config.options.plugins.legend.display).toBe(false);
    // Десктоп подписи не усекает: обработчик живёт только на узком экране,
    // а безусловное усечение резало бы и короткие «00:00».
    //
    // Проверяем именно ОТСУТСТВИЕ ключа, а не значение undefined. Chart.js
    // накладывает переданные настройки на свои по наличию ключа: `callback:
    // undefined` затирает штатный обработчик категориальной оси, и вместо
    // «08:00» на оси появляются номера делений. Проверка на undefined проходила
    // в обоих случаях и этот баг пропустила - поймали глазами в браузере.
    expect(Object.hasOwn(config.options.scales.x.ticks, 'callback')).toBe(false);
  });

  it('столбцы скруглены сверху и разведены зазором', async () => {
    const { config } = await build({ data: DATA });
    const ds = config.data.datasets[0];
    // Скругление только у верхушки: снизу столбец стоит на оси.
    expect(ds.borderRadius).toEqual({ topLeft: 4, topRight: 4, bottomLeft: 0, bottomRight: 0 });
    // Ширина столбца - около 62% слота, как было до перехода на Chart.js.
    expect(ds.categoryPercentage * ds.barPercentage).toBeCloseTo(0.62, 2);
  });

  it('столбец под курсором светлеет, а не остаётся прежним', async () => {
    const { config } = await build({ data: DATA, color: '#4F5BDF' });
    const ds = config.data.datasets[0];
    // Отклик на наведение был у прежнего движка; без него не видно, к какому
    // столбцу относится всплывшая подсказка.
    expect(ds.hoverBackgroundColor).not.toBe(ds.backgroundColor);
    expect(ds.hoverBackgroundColor).toBe('rgb(93, 104, 226)');
  });

  it('на узком экране сокращает число подписей оси X, бары не трогает', async () => {
    // 24 часовых бара с 12 подписями сливаются на 390px в «00:0002:00...» -
    // на узком экране число подписей падает, но категории (бары) остаются все.
    setViewport(true);
    const hours = Array.from(
      { length: 24 },
      (_, h) => ({ label: `${String(h).padStart(2, '0')}:00`, value: h }),
    );
    const { config } = await build({ data: hours });
    expect(config.data.labels).toHaveLength(24);
    expect(config.options.scales.x.ticks.maxTicksLimit).toBe(6);

    // На телефоне длинные категориальные подписи разреза (организация, место)
    // усекаются с многоточием, короткие числовые и статусы проходят как есть.
    const fmt = config.options.scales.x.ticks.callback;
    const scale = { getLabelForValue: (v) => ['08:00', 'Завершено', 'ООО «Производственно-строительное объединение»'][v] };
    expect(fmt.call(scale, 0)).toBe('08:00');
    expect(fmt.call(scale, 1)).toBe('Завершено');
    expect(fmt.call(scale, 2)).toBe('ООО «Производ…');
  });

  it('на широком экране подписей больше и они целые', async () => {
    const hours = Array.from(
      { length: 24 },
      (_, h) => ({ label: `${String(h).padStart(2, '0')}:00`, value: h }),
    );
    const { config } = await build({ data: hours });
    expect(config.options.scales.x.ticks.maxTicksLimit).toBe(12);
    expect(Object.hasOwn(config.options.scales.x.ticks, 'callback')).toBe(false);
  });

  it('тултип склоняет единицу по числу', async () => {
    const { config } = await build({ data: DATA, unitForms: ['проезд', 'проезда', 'проездов'] });
    const { label } = config.options.plugins.tooltip.callbacks;
    expect(label({ raw: 1 })).toBe('1 проезд');
    expect(label({ raw: 2 })).toBe('2 проезда');
    expect(label({ raw: 5 })).toBe('5 проездов');
  });

  it('пустые данные: показывает заглушку, график не рендерит', async () => {
    const { wrapper, config } = await build({ data: [] });
    expect(wrapper.text()).toContain('Нет данных');
    expect(config).toBeNull();
  });

  it('столбец без значения пропускается, а не рисуется нулевым', async () => {
    // «Нет данных» у производной метрики (этап никто не прошёл) != 0: столбец
    // нулевой высоты читался бы как реальное значение «прошло мгновенно».
    const { config } = await build({
      data: [
        { label: 'ООО А', value: 7 },
        { label: 'ООО Б', value: null },
        { label: 'ООО В', value: 0 },
      ],
    });
    // 0 — честное значение и остаётся нулём, null — пропуск.
    expect(config.data.datasets[0].data).toEqual([7, null, 0]);
  });

  it('ни одного столбца со значением: заглушка, а не пустая сетка', async () => {
    // Мультиметрика: строки есть по другой метрике, а выбранный этап не прошёл никто.
    const { wrapper, config } = await build({
      data: [{ label: 'ООО А', value: null }, { label: 'ООО Б', value: null }],
    });
    expect(wrapper.text()).toContain('Нет данных');
    expect(config).toBeNull();
  });

  it('ряд из одних нулей — данные: ноль это значение, а не его отсутствие', async () => {
    const { config } = await build({ data: [{ label: 'ООО А', value: 0 }] });
    expect(config).not.toBeNull();
    expect(config.data.datasets[0].data).toEqual([0]);
  });

  it('valueType=duration: ось и тултип рисуют длительность, а не сырые секунды', async () => {
    const { config } = await build({ data: DATA, valueType: 'duration' });
    expect(config.options.scales.y.ticks.callback(8100)).toBe('2 ч 15 мин');
    expect(config.options.plugins.tooltip.callbacks.label({ raw: 259200 })).toBe('3 сут');
  });

  it('без valueType длительности нет: ось остаётся числом с единицей', async () => {
    const { config } = await build({ data: DATA, unitForms: ['проезд', 'проезда', 'проездов'] });
    // Разделитель разрядов берём у самой локали: он неразрывный и зависит от ICU.
    const n = (8100).toLocaleString('ru-RU');
    expect(config.options.scales.y.ticks.callback(8100)).toBe(n);
    expect(config.options.plugins.tooltip.callbacks.label({ raw: 8100 })).toBe(`${n} проездов`);
  });

  it('перестроение по новым данным разрушает прежний экземпляр', async () => {
    const { wrapper } = await build({ data: DATA });
    expect(rendered.built).toBe(1);
    await wrapper.setProps({ data: [{ label: '10:00', value: 1 }] });
    await wrapper.vm.$nextTick();
    expect(rendered.last.data.datasets[0].data).toEqual([1]);
    expect(rendered.destroyed).toBe(1);
  });
});
