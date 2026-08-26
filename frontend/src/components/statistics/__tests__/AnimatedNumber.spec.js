import { describe, it, expect, vi, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import AnimatedNumber from '../AnimatedNumber.vue';

afterEach(() => {
  vi.unstubAllGlobals();
  delete window.matchMedia;
});

describe('AnimatedNumber', () => {
  it('пустое значение рендерит прочерк', () => {
    expect(mount(AnimatedNumber, { props: { value: null } }).text()).toBe('—');
    expect(mount(AnimatedNumber, { props: { value: '' } }).text()).toBe('—');
  });

  it('число форматируется в ru-RU при первом рендере (без анимации появления)', () => {
    const wrapper = mount(AnimatedNumber, { props: { value: 12345 } });
    expect(wrapper.text()).toBe((12345).toLocaleString('ru-RU'));
  });

  it('строковое число форматируется, нечисловая строка — прочерк', () => {
    expect(mount(AnimatedNumber, { props: { value: '123' } }).text()).toBe('123');
    expect(mount(AnimatedNumber, { props: { value: 'abc' } }).text()).toBe('—');
  });

  it('matchMedia бросает исключение — анимация не падает, идёт обычный путь', async () => {
    window.matchMedia = vi.fn(() => { throw new Error('not supported'); });
    const raf = vi.fn();
    vi.stubGlobal('requestAnimationFrame', raf);

    const wrapper = mount(AnimatedNumber, { props: { value: 10 } });
    await wrapper.setProps({ value: 50 });

    // prefersReducedMotion поглотил ошибку -> запланирован count-up через rAF.
    expect(raf).toHaveBeenCalled();
  });

  it('duration<=0 меняет значение моментально', async () => {
    const wrapper = mount(AnimatedNumber, { props: { value: 10, duration: 0 } });
    await wrapper.setProps({ value: 99 });
    expect(wrapper.text()).toBe('99');
  });

  it('при prefers-reduced-motion значение меняется моментально, без rAF', async () => {
    window.matchMedia = vi.fn(() => ({ matches: true }));
    const raf = vi.fn();
    vi.stubGlobal('requestAnimationFrame', raf);

    const wrapper = mount(AnimatedNumber, { props: { value: 10 } });
    await wrapper.setProps({ value: 200 });

    expect(wrapper.text()).toBe('200');
    expect(raf).not.toHaveBeenCalled();
  });

  it('переход в прочерк и из прочерка — моментальный', async () => {
    const wrapper = mount(AnimatedNumber, { props: { value: 50 } });
    await wrapper.setProps({ value: null });
    expect(wrapper.text()).toBe('—');
    await wrapper.setProps({ value: 7 });
    expect(wrapper.text()).toBe('7');
  });

  it('при смене числа делает count-up от старого к новому и доходит до цели', async () => {
    // Управляемый rAF: захватываем колбэк и сами прогоняем кадры.
    let cb = null;
    const raf = vi.fn((fn) => { cb = fn; return 1; });
    vi.stubGlobal('requestAnimationFrame', raf);
    vi.stubGlobal('cancelAnimationFrame', vi.fn());

    const wrapper = mount(AnimatedNumber, { props: { value: 10, duration: 100 } });
    await wrapper.setProps({ value: 110 });

    // Старт анимации запланирован, но кадры ещё не прогнаны — значение прежнее.
    expect(raf).toHaveBeenCalled();
    expect(wrapper.text()).toBe('10');

    cb(0); // первый кадр якорит время, p=0
    await wrapper.vm.$nextTick();
    expect(wrapper.text()).toBe('10');

    cb(50); // середина — между старым и новым
    await wrapper.vm.$nextTick();
    const mid = Number(wrapper.text());
    expect(mid).toBeGreaterThan(10);
    expect(mid).toBeLessThan(110);

    cb(100); // финал — ровно цель
    await wrapper.vm.$nextTick();
    expect(wrapper.text()).toBe('110');
  });
});
