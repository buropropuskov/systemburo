import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import ReportStepper from '../ReportStepper.vue';

const STEPS = [
  { label: '1 · Что считаем', state: 'done' },
  { label: '2 · По чему разбиваем', state: 'current' },
  { label: '3 · Фильтры', state: 'upcoming' },
  { label: '4 · Период', state: 'upcoming' },
];

describe('ReportStepper', () => {
  it('рисует точку, подпись и соединитель для каждого шага', () => {
    const wrapper = mount(ReportStepper, { props: { steps: STEPS } });
    expect(wrapper.findAll('.step')).toHaveLength(4);
    // соединителей на один меньше, чем шагов.
    expect(wrapper.findAll('.step-line')).toHaveLength(3);
    expect(wrapper.text()).toContain('Что считаем');
    expect(wrapper.text()).toContain('Период');
  });

  it('done-шаг показывает галочку, upcoming — номер; классы состояний навешаны', () => {
    const wrapper = mount(ReportStepper, { props: { steps: STEPS } });
    const steps = wrapper.findAll('.step');
    expect(steps[0].classes()).toContain('step--done');
    expect(steps[0].find('svg').exists()).toBe(true); // галочка
    expect(steps[1].classes()).toContain('step--current');
    expect(steps[2].find('svg').exists()).toBe(false); // номер, не галочка
    expect(steps[3].find('.step-dot').text()).toBe('4');
    // соединитель после done-шага залит.
    expect(wrapper.findAll('.step-line')[0].classes()).toContain('step-line--filled');
  });
});
