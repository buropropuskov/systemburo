import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

// chart.js/auto не установлен в worktree и требует реального canvas — мокаем
// конструктор, чтобы проверить именно buildConfig (тип/данные/палитра), а не
// рендер в jsdom. instances в hoisted: фабрика vi.mock поднимается над импортами.
const { instances } = vi.hoisted(() => ({ instances: [] }));
vi.mock('chart.js/auto', () => ({
  Chart: class {
    constructor(canvas, config) {
      this.canvas = canvas;
      this.config = config;
      instances.push(this);
    }

    destroy() {}
  },
}));

import ReportChart from '../ReportChart.vue';

const ROWS = [{ label: 'A', value: 5 }, { label: 'B', value: 3 }];

describe('ReportChart', () => {
  it('строит bar: метки/значения из строк, палитра по бакетам', async () => {
    instances.length = 0;
    mount(ReportChart, { props: { rows: ROWS, type: 'bar', unit: 'шт' } });
    await flushPromises();

    expect(instances).toHaveLength(1);
    const cfg = instances[0].config;
    expect(cfg.type).toBe('bar');
    expect(cfg.data.labels).toEqual(['A', 'B']);
    expect(cfg.data.datasets[0].data).toEqual([5, 3]);
    expect(cfg.data.datasets[0].backgroundColor).toHaveLength(2);
  });

  it('строит line для временного разреза с заливкой', async () => {
    instances.length = 0;
    mount(ReportChart, { props: { rows: ROWS, type: 'line', unit: '' } });
    await flushPromises();

    const cfg = instances[0].config;
    expect(cfg.type).toBe('line');
    expect(cfg.data.datasets[0].fill).toBe(true);
  });

  it('строит pie с массивом цветов по числу сегментов', async () => {
    instances.length = 0;
    mount(ReportChart, { props: { rows: ROWS, type: 'pie' } });
    await flushPromises();

    const cfg = instances[0].config;
    expect(cfg.type).toBe('pie');
    expect(cfg.data.datasets[0].backgroundColor).toHaveLength(2);
  });

  it('пересоздаёт график при смене типа, уничтожая предыдущий', async () => {
    instances.length = 0;
    const wrapper = mount(ReportChart, { props: { rows: ROWS, type: 'bar' } });
    await flushPromises();
    const spy = vi.spyOn(instances[0], 'destroy');

    await wrapper.setProps({ type: 'pie' });
    await flushPromises();

    expect(spy).toHaveBeenCalled();
    expect(instances).toHaveLength(2);
    expect(instances[1].config.type).toBe('pie');
  });
});
