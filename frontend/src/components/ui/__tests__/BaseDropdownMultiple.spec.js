import { mount } from '@vue/test-utils';
import { describe, it, expect } from 'vitest';
import BaseDropdown from '../BaseDropdown.vue';

// Мультивыбор BaseDropdown (#1398): фильтры Центра выбирают несколько организаций,
// компаний, мест разгрузки и проходов одним дропдауном. Ключевое отличие от
// одиночного режима - выбор ТОГЛИТ значение и не закрывает меню.

const OPTIONS = [
  { id: 1, name: 'Организация А' },
  { id: 2, name: 'Организация Б' },
  { id: 3, name: 'Организация В' },
];

function mountMulti(props = {}) {
  return mount(BaseDropdown, {
    props: {
      options: OPTIONS,
      multiple: true,
      modelValue: [],
      placeholder: 'Все организации',
      summaryLabel: 'Организация',
      ...props,
    },
  });
}

const items = (w) => w.findAll('.base-dropdown__item');
const buttonText = (w) => w.find('.base-dropdown__text').text();

describe('BaseDropdown multiple', () => {
  it('клик по пункту добавляет значение и НЕ закрывает меню', async () => {
    const w = mountMulti();
    await w.find('.base-dropdown__button').trigger('click');
    expect(w.vm.isOpen).toBe(true);

    await items(w)[1].trigger('click');

    expect(w.emitted('update:modelValue')[0][0]).toEqual([2]);
    expect(w.vm.isOpen).toBe(true);
  });

  it('повторный клик снимает значение', async () => {
    const w = mountMulti({ modelValue: [1, 2] });
    await w.find('.base-dropdown__button').trigger('click');

    await items(w)[0].trigger('click');

    expect(w.emitted('update:modelValue')[0][0]).toEqual([2]);
  });

  it('эмитит новый массив, не мутируя проп', async () => {
    const value = [1];
    const w = mountMulti({ modelValue: value });
    await w.find('.base-dropdown__button').trigger('click');

    await items(w)[1].trigger('click');

    expect(w.emitted('update:modelValue')[0][0]).toEqual([1, 2]);
    expect(value).toEqual([1]);
  });

  it('подпись кнопки: placeholder - имя единственного - счётчик', async () => {
    const w = mountMulti();
    expect(buttonText(w)).toBe('Все организации');

    await w.setProps({ modelValue: [2] });
    expect(buttonText(w)).toBe('Организация Б');

    await w.setProps({ modelValue: [2, 3] });
    expect(buttonText(w)).toBe('Организация: 2');
  });

  it('выбранное значение без своей опции (справочник ещё грузится) даёт счётчик, а не пустоту', () => {
    const w = mountMulti({ modelValue: [99] });
    expect(buttonText(w)).toBe('Организация: 1');
  });

  it('пункт помечен выбранным и несёт чекбокс', async () => {
    const w = mountMulti({ modelValue: [3] });
    await w.find('.base-dropdown__button').trigger('click');

    const rows = items(w);
    expect(rows[2].classes()).toContain('base-dropdown__item--selected');
    expect(rows[0].classes()).not.toContain('base-dropdown__item--selected');
    expect(rows[2].find('.base-dropdown__check--on').exists()).toBe(true);
    expect(rows[0].find('.base-dropdown__check--on').exists()).toBe(false);
  });

  it('«Сбросить выбор» есть только при непустом выборе и эмитит пустой массив', async () => {
    const empty = mountMulti();
    await empty.find('.base-dropdown__button').trigger('click');
    expect(empty.find('[data-testid="base-dropdown-clear"]').exists()).toBe(false);

    const w = mountMulti({ modelValue: [1, 2] });
    await w.find('.base-dropdown__button').trigger('click');
    await w.find('[data-testid="base-dropdown-clear"]').trigger('click');

    expect(w.emitted('update:modelValue')[0][0]).toEqual([]);
  });

  it('поиск фильтрует пункты, выбор из отфильтрованного списка работает', async () => {
    const w = mountMulti({ searchable: true });
    await w.find('.base-dropdown__button').trigger('click');
    await w.find('.base-dropdown__search-input').setValue('Б');

    const rows = items(w);
    expect(rows).toHaveLength(1);
    await rows[0].trigger('click');
    expect(w.emitted('update:modelValue')[0][0]).toEqual([2]);
  });

  it('одиночный режим не затронут: выбор эмитит скаляр и закрывает меню', async () => {
    const w = mount(BaseDropdown, {
      props: { options: OPTIONS, modelValue: null, placeholder: 'Выберите' },
    });
    await w.find('.base-dropdown__button').trigger('click');
    await items(w)[1].trigger('click');

    expect(w.emitted('update:modelValue')[0][0]).toBe(2);
    expect(w.vm.isOpen).toBe(false);
    expect(w.find('.base-dropdown__check').exists()).toBe(false);
  });
});
