import { mount } from '@vue/test-utils';
import {
  describe, it, expect, vi,
} from 'vitest';
import { nextTick } from 'vue';
import BaseDropdown from '../BaseDropdown.vue';

// Escape - четвёртый способ закрыть окно проекта (крестик, затемнение, Escape, свайп).
// У дропдауна его не было: нажатие уходило мимо списка, а внутри модалки закрывало
// саму модалку - человек терял форму вместо того, чтобы свернуть список.

const OPTIONS = [{ id: 1, name: 'Ромашка' }, { id: 2, name: 'Восток' }];

function mountDropdown(props = {}) {
  return mount(BaseDropdown, {
    props: { options: OPTIONS, ...props },
    attachTo: document.body,
  });
}

function pressEscape() {
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
}

describe('BaseDropdown - закрытие по Escape', () => {
  it('Escape закрывает открытое меню и сбрасывает поиск', async () => {
    const wrapper = mountDropdown({ searchable: true });
    await wrapper.get('.base-dropdown__button').trigger('click');
    wrapper.vm.searchQuery = 'ромашка';
    await nextTick();
    expect(wrapper.find('.base-dropdown__menu').exists()).toBe(true);

    pressEscape();
    await nextTick();

    expect(wrapper.vm.isOpen).toBe(false);
    expect(wrapper.vm.searchQuery).toBe('');
    expect(wrapper.find('.base-dropdown__menu').exists()).toBe(false);
    wrapper.unmount();
  });

  it('при открытом меню Escape не идёт дальше - модалка под списком не закрывается', async () => {
    const outer = vi.fn();
    document.addEventListener('keydown', outer);
    const wrapper = mountDropdown();
    await wrapper.get('.base-dropdown__button').trigger('click');

    pressEscape();
    await nextTick();

    expect(wrapper.vm.isOpen).toBe(false);
    expect(outer).not.toHaveBeenCalled();

    document.removeEventListener('keydown', outer);
    wrapper.unmount();
  });

  it('при закрытом меню Escape проходит насквозь - закрывать окна ему никто не мешает', async () => {
    const outer = vi.fn();
    document.addEventListener('keydown', outer);
    const wrapper = mountDropdown();

    pressEscape();
    await nextTick();

    expect(outer).toHaveBeenCalledTimes(1);

    document.removeEventListener('keydown', outer);
    wrapper.unmount();
  });

  it('слушатель снимается вместе с закрытием меню, а не копится', async () => {
    const addSpy = vi.spyOn(document, 'addEventListener');
    const removeSpy = vi.spyOn(document, 'removeEventListener');
    const wrapper = mountDropdown();

    await wrapper.get('.base-dropdown__button').trigger('click');
    expect(addSpy.mock.calls.some(([type, h]) => type === 'keydown' && h === wrapper.vm.handleEscape)).toBe(true);

    await wrapper.get('.base-dropdown__button').trigger('click');
    expect(removeSpy.mock.calls.some(([type, h]) => type === 'keydown' && h === wrapper.vm.handleEscape)).toBe(true);

    addSpy.mockRestore();
    removeSpy.mockRestore();
    wrapper.unmount();
  });
});
