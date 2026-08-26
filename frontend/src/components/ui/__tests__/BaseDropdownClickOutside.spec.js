import { mount } from '@vue/test-utils';
import {
  describe, it, expect, vi,
} from 'vitest';
import { nextTick } from 'vue';
import BaseDropdown from '../BaseDropdown.vue';

// Кейсы перенесены со спеки прежнего фильтра-компонента, который заменил этот дропдаун:
// под teleport меню живёт в body, то есть ВНЕ корня компонента, и наивная проверка
// «клик не в $el -> закрыть» гасит меню на первом же клике по пункту.

const OPTIONS = [{ id: 1, name: 'Ромашка' }, { id: 2, name: 'Восток' }];

function mountOpened(props = {}) {
  const wrapper = mount(BaseDropdown, {
    props: { options: OPTIONS, teleport: true, ...props },
    attachTo: document.body,
  });
  wrapper.vm.isOpen = true;
  return wrapper;
}

describe('BaseDropdown - клик вне меню под teleport', () => {
  it('клик по телепортированному меню не закрывает, клик снаружи закрывает', async () => {
    const wrapper = mountOpened();
    await nextTick();

    const menu = wrapper.vm.$refs.menu;
    expect(menu).toBeTruthy();
    // Меню действительно вне корня компонента - иначе кейс не про то.
    expect(wrapper.element.contains(menu)).toBe(false);

    wrapper.vm.handleClickOutside({ target: menu });
    expect(wrapper.vm.isOpen).toBe(true);

    wrapper.vm.handleClickOutside({ target: document.body });
    expect(wrapper.vm.isOpen).toBe(false);

    wrapper.unmount();
  });

  it('клик по кнопке-триггеру не закрывает через обработчик (закрытие - дело toggle)', async () => {
    const wrapper = mountOpened();
    await nextTick();

    const button = wrapper.get('.base-dropdown__button').element;
    wrapper.vm.handleClickOutside({ target: button });

    expect(wrapper.vm.isOpen).toBe(true);
    wrapper.unmount();
  });

  it('закрытие снаружи сбрасывает поисковый запрос', async () => {
    const wrapper = mountOpened({ searchable: true });
    await nextTick();
    wrapper.vm.searchQuery = 'ромашка';

    wrapper.vm.handleClickOutside({ target: document.body });

    expect(wrapper.vm.isOpen).toBe(false);
    expect(wrapper.vm.searchQuery).toBe('');
    wrapper.unmount();
  });

  it('слушатель повешен в capture-фазе (#1132: @click.stop на .base-modal глушит bubble)', () => {
    const addSpy = vi.spyOn(document, 'addEventListener');
    const wrapper = mount(BaseDropdown, { props: { options: OPTIONS } });

    const call = addSpy.mock.calls.find(
      ([type, handler]) => type === 'click' && handler === wrapper.vm.handleClickOutside,
    );
    expect(call).toBeTruthy();
    expect(call[2]).toBe(true);

    addSpy.mockRestore();
    wrapper.unmount();
  });
});
