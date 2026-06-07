import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import BlacklistView from '../BlacklistView.vue';

const stubs = {
  VehicleBlacklistTab: { name: 'VehicleBlacklistTab', template: '<div class="veh-tab" />' },
  PersonBlacklistTab: { name: 'PersonBlacklistTab', template: '<div class="per-tab" />' },
  FilterTabs: {
    name: 'FilterTabs',
    props: ['tabs', 'modelValue'],
    template: '<div class="tabs">{{ tabs.map(t => t.label).join("|") }}</div>',
  },
};

describe('BlacklistView', () => {
  it('строит ярлыки вкладок со счётчиками', () => {
    const wrapper = mount(BlacklistView, { global: { stubs } });
    expect(wrapper.find('.tabs').text()).toContain('Машины (0)');
    expect(wrapper.find('.tabs').text()).toContain('Люди (0)');
  });

  it('рендерит обе вкладки (v-show) и заголовок страницы', () => {
    const wrapper = mount(BlacklistView, { global: { stubs } });
    expect(wrapper.find('.page-title').text()).toBe('Чёрный список');
    expect(wrapper.find('.veh-tab').exists()).toBe(true);
    expect(wrapper.find('.per-tab').exists()).toBe(true);
  });

  it('ярлык реактивно обновляется при изменении счётчика', async () => {
    const wrapper = mount(BlacklistView, { global: { stubs } });
    wrapper.vm.vehicleCount = 5;
    wrapper.vm.personCount = 3;
    await wrapper.vm.$nextTick();
    expect(wrapper.find('.tabs').text()).toContain('Машины (5)');
    expect(wrapper.find('.tabs').text()).toContain('Люди (3)');
  });
});
