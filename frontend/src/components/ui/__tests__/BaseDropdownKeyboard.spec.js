import { afterEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { h } from 'vue';
import BaseDropdown from '../BaseDropdown.vue';

const options = [{ id: 1, name: 'Альфа' }, { id: 2, name: 'Бета' }, { id: 3, name: 'Вега' }];
let wrapper;
const items = () => [...document.querySelectorAll('.base-dropdown__item')];
const press = async (node, key) => {
  if (node !== document.activeElement) node.focus();
  node.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
  await flushPromises();
};
function create(props = {}, slots = {}) {
  wrapper = mount(BaseDropdown, { attachTo: document.body, props: { options, teleport: true, ...props }, slots });
  return wrapper.get('button').element;
}
afterEach(() => { wrapper?.unmount(); vi.restoreAllMocks(); });

describe('BaseDropdown keyboard', () => {
  it('opens on ArrowDown at the selected option without changing the value', async () => {
    const trigger = create({ modelValue: 2 });
    trigger.focus();
    await press(trigger, 'ArrowDown');
    expect(document.activeElement).toBe(items()[1]);
    expect(trigger.getAttribute('aria-expanded')).toBe('true');
    expect(items()[1].getAttribute('aria-selected')).toBe('true');
    expect(items().every(item => item.tabIndex === -1)).toBe(true);
    expect(document.querySelector('[role="listbox"]').getAttribute('tabindex')).toBe('-1');
    expect(wrapper.emitted('update:modelValue')).toBeUndefined();
  });

  it('opens on ArrowUp at the last option if nothing is selected', async () => {
    const trigger = create();
    await press(trigger, 'ArrowUp');
    expect(document.activeElement).toBe(items()[2]);
  });

  it('moves focus with arrows, Home and End without selecting', async () => {
    const trigger = create();
    await press(trigger, 'ArrowDown');
    await press(document.activeElement, 'End');
    expect(document.activeElement).toBe(items()[2]);
    await press(document.activeElement, 'Home');
    expect(document.activeElement).toBe(items()[0]);
    await press(document.activeElement, 'ArrowDown');
    expect(document.activeElement).toBe(items()[1]);
    await press(document.activeElement, 'ArrowUp');
    expect(document.activeElement).toBe(items()[0]);
    expect(wrapper.emitted('update:modelValue')).toBeUndefined();
  });

  it('uses a native button to select once and returns focus to the trigger', async () => {
    const trigger = create();
    await press(trigger, 'Enter');
    expect(document.activeElement.tagName).toBe('BUTTON');
    document.activeElement.click();
    await flushPromises();
    expect(wrapper.emitted('update:modelValue')).toEqual([[1]]);
    expect(wrapper.vm.isOpen).toBe(false);
    expect(document.activeElement).toBe(trigger);
  });

  it('keeps multi-selection open and does not select when moving focus', async () => {
    const trigger = create({ multiple: true, modelValue: [1] });
    await press(trigger, 'ArrowDown');
    await press(document.activeElement, 'ArrowDown');
    document.activeElement.click();
    await flushPromises();
    expect(wrapper.emitted('update:modelValue')).toEqual([[[1, 2]]]);
    expect(wrapper.vm.isOpen).toBe(true);
    expect(document.activeElement).toBe(items()[1]);
  });

  it('keeps search typing and moves to the filtered option on ArrowDown', async () => {
    const trigger = create({ searchable: true });
    trigger.click();
    await flushPromises();
    const input = document.querySelector('.base-dropdown__search-input');
    input.focus();
    input.value = 'Бет';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    await flushPromises();
    expect(document.activeElement).toBe(input);
    await press(input, 'ArrowDown');
    expect(document.activeElement).toBe(items()[0]);
    expect(document.activeElement.textContent).toContain('Бета');
    expect(input.value).toBe('Бет');
  });

  it('closes on Tab without consuming the native Tab movement', async () => {
    const trigger = create();
    await press(trigger, 'ArrowDown');
    const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true });
    document.activeElement.dispatchEvent(event);
    await flushPromises();
    expect(event.defaultPrevented).toBe(false);
    expect(wrapper.vm.isOpen).toBe(false);
    expect(document.activeElement).toBe(trigger);
  });

  it('does not open when disabled and safely handles an empty list', async () => {
    const trigger = create({ disabled: true, options: [] });
    await press(trigger, 'ArrowDown');
    expect(wrapper.vm.isOpen).toBe(false);
    await wrapper.setProps({ disabled: false });
    trigger.focus();
    await press(trigger, 'ArrowDown');
    expect(wrapper.vm.isOpen).toBe(true);
    expect(document.activeElement).toBe(trigger);
  });

  it('keeps focus in the control when an asynchronous reload removes all options', async () => {
    const trigger = create({ searchable: true });
    await press(trigger, 'ArrowDown');
    expect(document.activeElement).toBe(items()[0]);
    await wrapper.setProps({ options: [] });
    await flushPromises();
    expect(document.activeElement).toBe(document.querySelector('.base-dropdown__search-input'));
    expect(wrapper.emitted('update:modelValue')).toBeUndefined();
  });

  it('supports a custom trigger and preserves custom action content', async () => {
    const trigger = create({}, {
      trigger: ({ toggle }) => h('button', { class: 'custom-trigger', 'aria-haspopup': 'listbox', onClick: toggle }, 'Открыть'),
    });
    await press(trigger, 'ArrowDown');
    expect(document.activeElement).toBe(items()[0]);
    await press(document.activeElement, 'Escape');
    expect(document.activeElement).toBe(trigger);
    wrapper.unmount();
    create({}, { content: '<button class="custom-action">Действие</button>' });
    await wrapper.get('button').trigger('click');
    expect(document.querySelector('[role="listbox"]')).toBeNull();
    const action = document.querySelector('.custom-action');
    action.focus();
    await press(action, 'ArrowDown');
    expect(document.activeElement).toBe(action);
    await press(action, 'Tab');
    expect(wrapper.vm.isOpen).toBe(true);
  });

  it('finds an option by typing without changing selection', async () => {
    const trigger = create();
    await press(trigger, 'ArrowDown');
    await press(document.activeElement, 'В');
    expect(document.activeElement.textContent).toContain('Вега');
    expect(wrapper.emitted('update:modelValue')).toBeUndefined();
  });

  it('matches a typed prefix and expires it against a controlled clock', async () => {
    const clock = vi.spyOn(Date, 'now').mockReturnValue(1000);
    const trigger = create({ options: [
      { id: 1, name: 'Альфа' }, { id: 2, name: 'Аркадий' }, { id: 3, name: 'Антон' },
    ] });
    await press(trigger, 'ArrowDown');
    await press(document.activeElement, 'А');
    expect(document.activeElement.textContent).toContain('Аркадий');
    await press(document.activeElement, 'н');
    expect(document.activeElement.textContent).toContain('Антон');
    clock.mockReturnValue(2000);
    await press(document.activeElement, 'А');
    expect(document.activeElement.textContent).toContain('Альфа');
    await press(document.activeElement, 'р');
    expect(document.activeElement.textContent).toContain('Аркадий');
    expect(wrapper.emitted('update:modelValue')).toBeUndefined();
  });

  it('keeps inline options mounted after selecting and handles Escape separately', async () => {
    create({ inline: true });
    items()[0].focus();
    await press(document.activeElement, 'ArrowDown');
    expect(document.activeElement).toBe(items()[1]);
    document.activeElement.click();
    await flushPromises();
    expect(wrapper.emitted('update:modelValue')).toEqual([[2]]);
    expect(items()).toHaveLength(3);
  });

  it('leaves native activation of a custom primary action to its owner', async () => {
    const activate = vi.fn();
    const trigger = create({}, {
      trigger: () => h('button', { onClick: activate }, 'Пройти сначала'),
    });
    trigger.focus();
    const enter = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true });
    trigger.dispatchEvent(enter);
    await flushPromises();
    expect(enter.defaultPrevented).toBe(false);
    expect(wrapper.vm.isOpen).toBe(false);
    trigger.click();
    expect(activate).toHaveBeenCalledTimes(1);
    await press(trigger, 'ArrowDown');
    expect(wrapper.vm.isOpen).toBe(false);
  });

  it('waits for the custom owner before focusing an asynchronously opened popup', async () => {
    let ready;
    const gate = new Promise(resolve => { ready = resolve; });
    const trigger = create({}, {
      trigger: ({ toggle }) => h('button', {
        'aria-haspopup': 'listbox', onClick: async () => { await gate; toggle(); },
      }, 'Подготовить выбор'),
    });
    await press(trigger, 'ArrowDown');
    expect(wrapper.vm.isOpen).toBe(false);
    expect(document.activeElement).toBe(trigger);
    ready();
    await flushPromises();
    expect(wrapper.vm.isOpen).toBe(true);
    expect(document.activeElement).toBe(items()[0]);
  });

  it('does not steal focus if the user left while the owner was preparing', async () => {
    let ready;
    const gate = new Promise(resolve => { ready = resolve; });
    const trigger = create({}, {
      trigger: ({ toggle }) => h('button', {
        'aria-haspopup': 'listbox', onClick: async () => { await gate; toggle(); },
      }, 'Подготовить выбор'),
    });
    const elsewhere = document.createElement('button');
    document.body.append(elsewhere);
    try {
      await press(trigger, 'ArrowDown');
      elsewhere.focus();
      ready();
      await flushPromises();
      expect(document.activeElement).toBe(elsewhere);
    } finally { elsewhere.remove(); }
  });

  it('keeps a parent action menu open when a nested list handles keys', async () => {
    wrapper = mount({
      components: { BaseDropdown },
      data: () => ({ options }),
      template: '<BaseDropdown :options="[]"><template #content><BaseDropdown :options="options" /></template></BaseDropdown>',
    }, { attachTo: document.body });
    const parent = wrapper.findAllComponents(BaseDropdown)[0];
    await parent.get('button').trigger('click');
    const child = wrapper.findAllComponents(BaseDropdown)[1];
    const trigger = child.get('button').element;
    await press(trigger, 'ArrowDown');
    expect(document.activeElement).toBe(items()[0]);
    await press(document.activeElement, 'Escape');
    expect(child.vm.isOpen).toBe(false);
    expect(parent.vm.isOpen).toBe(true);
    expect(document.activeElement).toBe(trigger);
  });
});
