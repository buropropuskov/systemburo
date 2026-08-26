import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import TopList from '../TopList.vue';

const ITEMS = [
  { label: 'Дебаркадер №1', value: 9 },
  { label: 'Ворота 2', value: 3 },
];

describe('TopList', () => {
  it('рендерит заголовок, подпись и строки с рангами и значениями', () => {
    const wrapper = mount(TopList, {
      props: { title: 'Места разгрузки', subtitle: 'по въездам машин', items: ITEMS },
    });

    expect(wrapper.find('.top__title').text()).toBe('Места разгрузки');
    expect(wrapper.find('.top__sub').text()).toBe('по въездам машин');

    const rows = wrapper.findAll('.top__row');
    expect(rows).toHaveLength(2);
    expect(rows[0].find('.top__rank').text()).toBe('1');
    expect(rows[0].find('.top__name').text()).toBe('Дебаркадер №1');
    expect(rows[0].find('.top__val').text()).toBe('9');
    expect(rows[1].find('.top__rank').text()).toBe('2');
  });

  it('длина бара пропорциональна лидеру: у максимума scaleX(1), минимум не ниже 0.04', () => {
    const wrapper = mount(TopList, {
      props: { title: 'Топ', items: [{ label: 'A', value: 100 }, { label: 'B', value: 1 }] },
    });

    const fills = wrapper.findAll('.top__bar-fill');
    expect(fills[0].attributes('style')).toContain('scaleX(1)');
    // 1/100 = 1% -> поднимается до минимума 4% (scaleX 0.04), чтобы полоска была видна.
    expect(fills[1].attributes('style')).toContain('scaleX(0.04)');
  });

  it('пустой список показывает плейсхолдер вместо строк', () => {
    const wrapper = mount(TopList, {
      props: { title: 'Организации', items: [] },
    });

    expect(wrapper.find('.top__empty').text()).toContain('Нет данных');
    expect(wrapper.find('.top__row').exists()).toBe(false);
  });
});
