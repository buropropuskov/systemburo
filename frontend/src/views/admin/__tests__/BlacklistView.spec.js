import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import BlacklistView from '../BlacklistView.vue';

// Вкладки теперь рендерят заголовок + FilterTabs через слот #header-left, поэтому
// стабы пробрасывают слот. Обе вкладки смонтированы (v-show) - слот рендерится дважды.
const stubs = {
  VehicleBlacklistTab: { name: 'VehicleBlacklistTab', template: '<div class="veh-tab"><slot name="header-left" /></div>' },
  PersonBlacklistTab: { name: 'PersonBlacklistTab', template: '<div class="per-tab"><slot name="header-left" /></div>' },
  FilterTabs: {
    name: 'FilterTabs',
    props: ['tabs', 'modelValue'],
    template: '<div class="tabs">{{ tabs.map(t => `${t.label}=${t.count}`).join("|") }}</div>',
  },
};

describe('BlacklistView', () => {
  it('строит ярлыки вкладок с отдельными счётчиками', () => {
    const wrapper = mount(BlacklistView, { global: { stubs } });
    const tabsText = wrapper.findAll('.tabs')[0].text();
    expect(tabsText).toContain('Машины=0');
    expect(tabsText).toContain('Люди=0');
  });

  it('рендерит обе вкладки (v-show) и заголовок в шапке', () => {
    const wrapper = mount(BlacklistView, { global: { stubs } });
    expect(wrapper.findAll('.bl-page-title')[0].text()).toBe('Чёрный список');
    expect(wrapper.find('.veh-tab').exists()).toBe(true);
    expect(wrapper.find('.per-tab').exists()).toBe(true);
  });

  it('счётчик реактивно обновляется', async () => {
    const wrapper = mount(BlacklistView, { global: { stubs } });
    wrapper.vm.vehicleCount = 5;
    wrapper.vm.personCount = 3;
    await wrapper.vm.$nextTick();
    const tabsText = wrapper.findAll('.tabs')[0].text();
    expect(tabsText).toContain('Машины=5');
    expect(tabsText).toContain('Люди=3');
  });
});
