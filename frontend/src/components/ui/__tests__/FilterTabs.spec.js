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
