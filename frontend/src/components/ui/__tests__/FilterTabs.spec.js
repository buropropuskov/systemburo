import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import FilterTabs from '../FilterTabs.vue';

const allTabs = [
  { key: 'all', label: 'Все' },
  { key: 'active', label: 'Активные' },
  { key: 'hidden', label: 'Скрытые', visible: false },
  { key: 'archived', label: 'Архив', visible: true },
];

function mountTabs(props = {}) {
  return mount(FilterTabs, {
    props: { tabs: allTabs, modelValue: 'all', ...props },
  });
}

describe('FilterTabs', () => {
  it('renders only visible tabs', () => {
    const wrapper = mountTabs();
    const buttons = wrapper.findAll('.filter-tab');
    expect(buttons).toHaveLength(3);
    expect(buttons.map(b => b.text())).toEqual(['Все', 'Активные', 'Архив']);
  });

  it('does not render tabs with visible=false', () => {
    const wrapper = mountTabs();
    expect(wrapper.text()).not.toContain('Скрытые');
  });

  it('active tab has correct class', () => {
    const wrapper = mountTabs({ modelValue: 'active' });
    const buttons = wrapper.findAll('.filter-tab');
    const activeButton = buttons.find(b => b.text() === 'Активные');
    expect(activeButton.classes()).toContain('filter-tab--active');
  });

  it('non-active tabs do not have active class', () => {
    const wrapper = mountTabs({ modelValue: 'all' });
    const buttons = wrapper.findAll('.filter-tab');
    const inactiveButton = buttons.find(b => b.text() === 'Активные');
    expect(inactiveButton.classes()).not.toContain('filter-tab--active');
  });

  it('click emits update:modelValue with tab key', async () => {
    const wrapper = mountTabs();
    const buttons = wrapper.findAll('.filter-tab');
    const activeButton = buttons.find(b => b.text() === 'Активные');
    await activeButton.trigger('click');
    expect(wrapper.emitted('update:modelValue')).toEqual([['active']]);
  });

  it('renders all tabs when none have visible property', () => {
    const simpleTabs = [
      { key: 'a', label: 'Tab A' },
      { key: 'b', label: 'Tab B' },
    ];
    const wrapper = mount(FilterTabs, {
      props: { tabs: simpleTabs, modelValue: 'a' },
    });
    expect(wrapper.findAll('.filter-tab')).toHaveLength(2);
  });
});

/**
 * Правило владельца (тег #55678): ряд пилюль, который не помещается в строку,
 * сворачивается в выпадающий список, а не переносится на второй ряд. Перенос крал
 * высоту и рвал шапку - замер, с которого правило появилось: четыре фильтра «Моих
 * сотрудников» это 807px при контейнере 728 на ширине 768.
 *
 * jsdom не считает раскладку, поэтому ширины подставляем сами: `offsetWidth` пилюль
 * и `clientWidth` ряда.
 */
describe('FilterTabs - сворачивание в список', () => {
  const tabs = [
    { key: 'organization', label: 'Сотрудники организации' },
    { key: 'company', label: 'Сотрудники компании' },
    { key: 'user', label: 'Мои сотрудники' },
    { key: 'all_system', label: 'Все сотрудники системы' },
  ];

  const mountWith = async (pillWidth, rowWidth) => {
    const wrapper = mount(FilterTabs, { props: { tabs, modelValue: 'user' } });
    const row = wrapper.vm.$refs.row;
    Object.defineProperty(row, 'clientWidth', { value: rowWidth, configurable: true });
    [...row.children].forEach((el) => {
      Object.defineProperty(el, 'offsetWidth', { value: pillWidth, configurable: true });
    });
    wrapper.vm.measure();
    await wrapper.vm.$nextTick();
    return wrapper;
  };

  it('помещаются - остаются пилюлями, списка нет', async () => {
    const wrapper = await mountWith(150, 900);
    expect(wrapper.vm.collapsed).toBe(false);
    expect(wrapper.find('[data-testid="filter-tabs-select"]').exists()).toBe(false);
  });

  it('не помещаются - вместо ряда появляется список', async () => {
    const wrapper = await mountWith(210, 728);
    expect(wrapper.vm.collapsed).toBe(true);
    expect(wrapper.find('[data-testid="filter-tabs-select"]').exists()).toBe(true);
  });

  it('в списке те же пункты и в том же порядке', async () => {
    const wrapper = await mountWith(210, 728);
    expect(wrapper.vm.dropdownOptions).toEqual(tabs.map((t) => ({ value: t.key, label: t.label })));
  });
});
