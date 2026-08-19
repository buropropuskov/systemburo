import { describe, it, expect, vi, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { computed, defineComponent, ref } from 'vue';

// Chart.js рисует на холсте, которого в jsdom нет: подменяем конструктор и
// считаем перерисовки - проверяем обвязку, а не отрисовку.
const { instances } = vi.hoisted(() => ({ instances: { list: [] } }));
vi.mock('chart.js', () => {
  class ChartStub {
    constructor(canvas, config) {
      this.canvas = canvas;
      this.config = config;
      this.updates = 0;
      this.destroyed = false;
      instances.list.push(this);
    }

    update() {
      this.updates += 1;
    }

    destroy() {
      this.destroyed = true;
    }
  }
  ChartStub.register = () => {};
  return {
    Chart: ChartStub,
    ArcElement: {}, BarController: {}, BarElement: {}, CategoryScale: {},
    DoughnutController: {}, Filler: {}, Legend: {}, LineController: {},
    LineElement: {}, LinearScale: {}, PointElement: {}, Tooltip: {},
  };
});

import { cssVariable, themeColor, useChartCanvas } from '../useChartCanvas';

const Host = defineComponent({
  setup() {
    const canvas = ref(null);
    useChartCanvas(canvas, computed(() => ({ type: 'line', data: {}, options: {} })));
    return { canvas };
  },
  template: '<canvas ref="canvas" />',
});

/** Монтирует холст с графиком и отдаёт последний созданный экземпляр. */
async function build() {
  instances.list = [];
  const wrapper = mount(Host, { attachTo: document.body });
  await wrapper.vm.$nextTick();
  return { wrapper, chart: instances.list.at(-1) };
}

afterEach(() => {
  document.documentElement.removeAttribute('data-theme');
  vi.restoreAllMocks();
});

/** Подменяет чтение стилей значениями темы. */
function stubTheme(values) {
  vi.spyOn(window, 'getComputedStyle').mockReturnValue({
    getPropertyValue: (name) => values[name] ?? '',
  });
}

describe('themeColor', () => {
  it('берёт цвет из переменной темы у холста графика', () => {
    stubTheme({ '--border': ' #3a3f4a ' });
    const el = document.createElement('canvas');
    expect(themeColor('--border', '#eef0f7')({ chart: { canvas: el } })).toBe('#3a3f4a');
  });

  it('без темы отдаёт запасной цвет, а не пустую строку', () => {
    stubTheme({});
    const el = document.createElement('canvas');
    // Пустая строка ушла бы в Chart.js цветом и рисовала бы чёрным.
    expect(themeColor('--border', '#eef0f7')({ chart: { canvas: el } })).toBe('#eef0f7');
    // Холста ещё нет (первый проход разметки) - тоже запасной цвет.
    expect(themeColor('--border', '#eef0f7')({})).toBe('#eef0f7');
    expect(cssVariable(null, '--text', '#333333')).toBe('#333333');
  });
});

describe('useChartCanvas', () => {
  it('смена темы страницы перерисовывает график', async () => {
    const { chart } = await build();
    expect(chart.updates).toBe(0);

    document.documentElement.setAttribute('data-theme', 'dark');
    // MutationObserver отрабатывает микрозадачей.
    await Promise.resolve();
    expect(chart.updates).toBe(1);
  });

  it('размонтирование разрушает график и снимает слежение за темой', async () => {
    const { wrapper, chart } = await build();
    wrapper.unmount();
    expect(chart.destroyed).toBe(true);

    document.documentElement.setAttribute('data-theme', 'dark');
    await Promise.resolve();
    expect(chart.updates).toBe(0);
  });
});
