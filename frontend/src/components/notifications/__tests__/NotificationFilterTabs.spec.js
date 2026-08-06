import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import NotificationFilterTabs from '../NotificationFilterTabs.vue';

describe('NotificationFilterTabs', () => {
  it('помечает активной кнопку по modelValue', () => {
    const wrapper = mount(NotificationFilterTabs, { props: { modelValue: 'unread' } });
    const buttons = wrapper.findAll('button');
    expect(buttons[0].classes()).not.toContain('active');
    expect(buttons[1].classes()).toContain('active');
  });

  it('клик по «Все»/«Непрочитанные» эмитит update:modelValue', async () => {
    const wrapper = mount(NotificationFilterTabs, { props: { modelValue: 'all' } });
    await wrapper.find('[data-testid="notif-filter-unread"]').trigger('click');
    expect(wrapper.emitted('update:modelValue')[0]).toEqual(['unread']);

    await wrapper.find('[data-testid="notif-filter-all"]').trigger('click');
    expect(wrapper.emitted('update:modelValue')[1]).toEqual(['all']);
  });
});
