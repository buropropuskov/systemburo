import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import NavTableLinks from '../NavTableLinks.vue';

describe('NavTableLinks', () => {
  it('preserves flat/wrapped table labels, selection and navigation payloads', async () => {
    const wrapper = mount(NavTableLinks, { props: { expanded: true, current: 'post', groups: [{
      key: 'people', label: 'Люди', tables: [
        { id: 1, name: 'kpp', display_name: 'КПП' },
        { table: { id: 2, name: 'post', display_name: 'Пост' } },
      ],
    }] } });
    const buttons = wrapper.findAll('button');
    expect(buttons.map(button => button.text())).toEqual(['КПП', 'Пост']);
    expect(buttons[0].classes()).not.toContain('active');
    expect(buttons[1].classes()).toContain('active');
    await buttons[1].trigger('click');
    expect(wrapper.emitted('navigate')).toEqual([['post']]);
    expect(wrapper.attributes()).not.toHaveProperty('inert');
    wrapper.unmount();
  });
  it('keeps unnamed records inactive and disabled records non-interactive', () => {
    const wrapper = mount(NavTableLinks, { props: { disabled: true, groups: [{ key: 'other', tables: [{ id: 1 }] }] } });
    expect(wrapper.get('button').text()).toBe('Без названия');
    expect(wrapper.get('button').classes()).not.toContain('active');
    expect(wrapper.get('button').element.disabled).toBe(true);
    expect(wrapper.attributes()).toHaveProperty('inert');
    wrapper.unmount();
  });
});
