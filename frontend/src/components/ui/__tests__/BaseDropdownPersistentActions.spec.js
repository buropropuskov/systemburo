import { afterEach, describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { defineComponent, h, nextTick, onMounted, ref } from 'vue';
import BaseDropdown from '../BaseDropdown.vue';

let wrapper;
const listeners = [];
afterEach(() => {
  wrapper?.unmount(); wrapper = null;
  for (const listener of listeners.splice(0)) document.removeEventListener('keydown', listener);
  document.body.replaceChildren();
});

function statefulSlot() {
  const mounted = vi.fn();
  const Component = defineComponent({ setup() {
    const value = ref(''); onMounted(mounted);
    return () => h('input', { 'data-testid': 'persistent-action', value: value.value,
      onInput: event => { value.value = event.target.value; } });
  } });
  return { mounted, content: () => h(Component) };
}
function escape() { document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); }

describe('persistent responsive action content', () => {
  it('preserves a single action instance and its input after closing and reopening', async () => {
    const slot = statefulSlot();
    wrapper = mount(BaseDropdown, { props: { options: [], keepMounted: true, teleport: true },
      slots: { content: slot.content }, attachTo: document.body });
    const input = document.querySelector('[data-testid=persistent-action]');
    expect(input).not.toBe(null);
    await wrapper.get('.base-dropdown__button').trigger('click');
    input.value = 'Сохранённое значение'; input.dispatchEvent(new Event('input', { bubbles: true }));
    escape(); await nextTick();
    expect(wrapper.vm.isOpen).toBe(false);
    await wrapper.get('.base-dropdown__button').trigger('click');
    expect(document.querySelector('[data-testid=persistent-action]')).toBe(input);
    expect(input.value).toBe('Сохранённое значение');
    expect(slot.mounted).toHaveBeenCalledTimes(1);
  });

  it('moves the same content between inline layout and the menu and restores safe focus', async () => {
    const slot = statefulSlot();
    wrapper = mount(BaseDropdown, { props: { options: [], inline: true, keepMounted: true, teleport: true },
      slots: { content: slot.content }, attachTo: document.body });
    const input = document.querySelector('[data-testid=persistent-action]');
    expect(input).not.toBe(null);
    expect(wrapper.find('.base-dropdown__button').exists()).toBe(false);
    input.focus();
    await wrapper.setProps({ inline: false }); await nextTick();
    expect(document.activeElement).toBe(wrapper.get('.base-dropdown__button').element);
    expect(document.querySelector('[data-testid=persistent-action]')).toBe(input);
    await wrapper.setProps({ inline: true });
    expect(document.querySelector('[data-testid=persistent-action]')).toBe(input);
    expect(slot.mounted).toHaveBeenCalledTimes(1);
  });

  it('closes only the nested menu on the first Escape and closes descendants with their owner', async () => {
    const behind = vi.fn(); listeners.push(behind); document.addEventListener('keydown', behind);
    wrapper = mount(BaseDropdown, { props: { options: [], keepMounted: true, teleport: true },
      slots: { content: () => h(BaseDropdown, { options: [{ id: 1, name: 'Вариант' }], teleport: true }) },
      attachTo: document.body });
    const child = wrapper.findAllComponents(BaseDropdown).find(item => item.vm !== wrapper.vm);
    expect(child).toBeTruthy();
    await wrapper.get('.base-dropdown__button').trigger('click');
    await child.get('.base-dropdown__button').trigger('click');
    escape(); await nextTick();
    expect(child.vm.isOpen).toBe(false); expect(wrapper.vm.isOpen).toBe(true);
    expect(behind).not.toHaveBeenCalled();
    await child.get('.base-dropdown__button').trigger('click');
    wrapper.vm.close(); await nextTick(); await nextTick();
    expect(child.vm.isOpen).toBe(false); expect(wrapper.vm.isOpen).toBe(false);
  });
});
