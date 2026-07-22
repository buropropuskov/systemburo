import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import AnimatedCounter from '../AnimatedCounter.vue';

// Кадры вызываем вручную с заданным временем - иначе тест зависел бы от реального rAF.
let queue = [];

async function runFrame(at) {
  const pending = queue;
  queue = [];
  pending.forEach((cb) => cb(at));
  await flushPromises();
}

describe('AnimatedCounter', () => {
  beforeEach(() => {
    queue = [];
    vi.stubGlobal('requestAnimationFrame', (cb) => { queue.push(cb); return queue.length; });
    vi.stubGlobal('cancelAnimationFrame', () => { queue = []; });
    window.matchMedia = vi.fn(() => ({ matches: false }));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('первое значение показывает сразу, без пересчёта от нуля', () => {
    const wrapper = mount(AnimatedCounter, { props: { value: 7 } });
    expect(wrapper.find('.animated-counter__value').text()).toBe('7');
    expect(queue).toHaveLength(0);
  });

  it('при смене значения проходит промежуточные шаги и доходит до нового', async () => {
    const wrapper = mount(AnimatedCounter, { props: { value: 7, duration: 300 } });
    await wrapper.setProps({ value: 12 });

    await runFrame(0); // якорный кадр - фиксирует стартовый timestamp
    await runFrame(150); // середина анимации
    const mid = Number(wrapper.find('.animated-counter__value').text());
    expect(mid).toBeGreaterThan(7);
    expect(mid).toBeLessThanOrEqual(12);

    await runFrame(300); // конец
    expect(wrapper.find('.animated-counter__value').text()).toBe('12');
    expect(queue).toHaveLength(0);
  });

  it('убывание тоже анимируется и приходит ровно к новому значению', async () => {
    const wrapper = mount(AnimatedCounter, { props: { value: 10, duration: 300 } });
    await wrapper.setProps({ value: 8 });

    await runFrame(0);
    await runFrame(100);
    expect(Number(wrapper.find('.animated-counter__value').text())).toBeLessThan(10);

    await runFrame(400); // время вышло - фиксируем целевое
    expect(wrapper.find('.animated-counter__value').text()).toBe('8');
  });

  it('новое значение в середине анимации отменяет прежний пересчёт и ведёт к нему', async () => {
    const wrapper = mount(AnimatedCounter, { props: { value: 0, duration: 300 } });
    await wrapper.setProps({ value: 50 });
    await runFrame(0);
    await runFrame(150);

    await wrapper.setProps({ value: 3 });
    await runFrame(200); // якорный кадр новой анимации
    await runFrame(600);

    expect(wrapper.find('.animated-counter__value').text()).toBe('3');
  });

  it('при prefers-reduced-motion меняет значение мгновенно, без кадров', async () => {
    window.matchMedia = vi.fn(() => ({ matches: true }));
    const wrapper = mount(AnimatedCounter, { props: { value: 1 } });
    await wrapper.setProps({ value: 9 });

    expect(wrapper.find('.animated-counter__value').text()).toBe('9');
    expect(queue).toHaveLength(0);
  });

  it('резервирует ширину под разрядность, чтобы 9 -> 10 не двигало соседей', () => {
    const wrapper = mount(AnimatedCounter, { props: { value: 9, minDigits: 3 } });
    expect(wrapper.find('.animated-counter__reserve').text()).toBe('000');
    expect(wrapper.find('.animated-counter__value').text()).toBe('9');
  });

  it('резерв растёт под значение шире минимальной разрядности', async () => {
    const wrapper = mount(AnimatedCounter, { props: { value: 5, minDigits: 2, duration: 0 } });
    expect(wrapper.find('.animated-counter__reserve').text()).toBe('00');

    await wrapper.setProps({ value: 1234 });
    expect(wrapper.find('.animated-counter__reserve').text()).toBe('0000');
  });

  it('снятие с монтирования отменяет незавершённый пересчёт', async () => {
    const wrapper = mount(AnimatedCounter, { props: { value: 1, duration: 300 } });
    await wrapper.setProps({ value: 40 });
    expect(queue.length).toBeGreaterThan(0);

    wrapper.unmount();
    expect(queue).toHaveLength(0);
  });
});
