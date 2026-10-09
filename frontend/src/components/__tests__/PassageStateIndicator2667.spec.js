import { describe, expect, it } from 'vitest';
import { mount } from '@vue/test-utils';
import PassageStateIndicator from '../PassageStateIndicator.vue';

describe('compact passage indicator #2667', () => {
  it('provides an explicit keyboard/click explanation rather than hover-only text', async () => {
    const wrapper = mount(PassageStateIndicator, { props: { expired: true, passageState: { open: true, entry_time_known: true } } });
    expect(wrapper.find('button').attributes('aria-label')).toContain('новый вход запрещён');
    expect(wrapper.find('[role="status"]').exists()).toBe(false);
    await wrapper.find('button').trigger('click');
    expect(wrapper.find('[role="status"]').text()).toContain('Срок истёк');
    await wrapper.find('button').trigger('keydown', { key: 'Escape' });
    expect(wrapper.find('[role="status"]').exists()).toBe(false);
    wrapper.unmount();
  });
  it('distinguishes an unknown legacy entry from a confirmed 48-hour duration', () => {
    const wrapper = mount(PassageStateIndicator, { props: { passageState: { open: true, entry_time_known: false, needs_attention: false } } });
    expect(wrapper.find('button').attributes('aria-label')).toContain('время входа неизвестно');
    expect(wrapper.text()).not.toContain('48 часов');
    wrapper.unmount();
  });
  it('shows grace information, resets open explanation after state changes, and hides regular valid rows', async () => {
    const wrapper = mount(PassageStateIndicator, { props: { expired: true, passageState: { open: false, in_exit_grace: true } } });
    expect(wrapper.find('button').attributes('aria-label')).toContain('5 минут');
    await wrapper.find('button').trigger('click');
    await wrapper.setProps({ expired: false, passageState: { open: true, entry_time_known: true, needs_attention: false } });
    expect(wrapper.find('button').exists()).toBe(false);
    wrapper.unmount();
  });
});
