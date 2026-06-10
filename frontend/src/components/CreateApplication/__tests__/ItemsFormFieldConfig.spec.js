import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import ItemsForm from '../ItemsForm.vue';
import CustomFieldsSection from '../CustomFieldsSection.vue';

// H-8 (#529): ItemsForm уважает field-config; CustomFieldsSection показывает ошибку
// при is_required=true + submitted=true + пустом значении.

describe('ItemsForm - потребление field-config (#529)', () => {
  it('без конфига: оба столбца видимы, звёздочки обязательности на месте', () => {
    const w = mount(ItemsForm);
    expect(w.find('.name-header').exists()).toBe(true);
    expect(w.find('.quantity-header').exists()).toBe(true);
    expect(w.findAll('.required')).toHaveLength(2);
  });

  it('fieldVisible: нет строки -> true; visible:false -> false', () => {
    const w = mount(ItemsForm, {
      props: { fieldConfig: { foo: { visible: false, required: true } } },
    });
    expect(w.vm.fieldVisible('missing')).toBe(true);
    expect(w.vm.fieldVisible('foo')).toBe(false);
  });

  it('fieldRequired: нет строки -> true; required:false -> false', () => {
    const w = mount(ItemsForm, {
      props: { fieldConfig: { foo: { visible: true, required: false } } },
    });
    expect(w.vm.fieldRequired('missing')).toBe(true);
    expect(w.vm.fieldRequired('foo')).toBe(false);
  });

  it('конфиг скрывает столбец item_name при visible=false', () => {
    const w = mount(ItemsForm, {
      props: { fieldConfig: { item_name: { visible: false, required: true } } },
    });
    expect(w.find('.name-header').exists()).toBe(false);
    expect(w.find('.name-cell').exists()).toBe(false);
    expect(w.find('.quantity-header').exists()).toBe(true);
  });

  it('конфиг скрывает столбец quantity при visible=false', () => {
    const w = mount(ItemsForm, {
      props: { fieldConfig: { quantity: { visible: false, required: true } } },
    });
    expect(w.find('.quantity-header').exists()).toBe(false);
    expect(w.find('.quantity-cell').exists()).toBe(false);
    expect(w.find('.name-header').exists()).toBe(true);
  });

  it('конфиг убирает звёздочку item_name при required=false', () => {
    const w = mount(ItemsForm, {
      props: { fieldConfig: { item_name: { visible: true, required: false } } },
    });
    // только одна звёздочка - у quantity
    expect(w.findAll('.required')).toHaveLength(1);
  });

  it('конфиг убирает звёздочку quantity при required=false', () => {
    const w = mount(ItemsForm, {
      props: { fieldConfig: { quantity: { visible: true, required: false } } },
    });
    // только одна звёздочка - у item_name
    expect(w.findAll('.required')).toHaveLength(1);
  });

  it('canAddItems: без конфига - false при пустом item_name', () => {
    const w = mount(ItemsForm);
    expect(w.vm.canAddItems).toBe(false);
  });

  it('canAddItems: скрытый item_name не блокирует добавление при filled quantity', async () => {
    const w = mount(ItemsForm, {
      props: { fieldConfig: { item_name: { visible: false, required: true } } },
    });
    // item_name скрыт - quantity=1 (дефолт), должно быть разрешено
    expect(w.vm.canAddItems).toBe(true);
  });

  it('canAddItems: скрытый quantity не блокирует добавление при filled item_name', async () => {
    const w = mount(ItemsForm, {
      props: { fieldConfig: { quantity: { visible: false, required: true } } },
    });
    // устанавливаем через editItem чтобы триггернуть реактивность
    w.vm.editItem({ id: null, itemName: 'Тестовое наименование', quantity: null });
    await w.vm.$nextTick();
    expect(w.vm.canAddItems).toBe(true);
  });

  it('canAddItems: required=false снимает блок даже при пустом item_name', async () => {
    const w = mount(ItemsForm, {
      props: { fieldConfig: { item_name: { visible: true, required: false } } },
    });
    // item_name пустой, но required=false - quantity=1 (дефолт)
    expect(w.vm.canAddItems).toBe(true);
  });

  it('addItems эмитит при скрытом item_name, а не молчит (red H-8)', async () => {
    const w = mount(ItemsForm, {
      props: { fieldConfig: { item_name: { visible: false, required: true } } },
    });
    // item_name скрыт, quantity=1 (дефолт) -> строка валидна, эмит должен произойти
    w.vm.addItems();
    await w.vm.$nextTick();
    expect(w.emitted('item-added')).toBeTruthy();
    expect(w.emitted('item-added')[0][0].quantity).toBe(1);
  });
});

describe('CustomFieldsSection - is_required + submitted (#529)', () => {
  const fields = [
    { id: 1, label: 'Экспедитор', placeholder: '', is_required: true },
    { id: 2, label: 'Примечание', placeholder: '', is_required: false },
  ];

  it('без submitted: нет ошибочной подсветки даже при пустых required полях', () => {
    const w = mount(CustomFieldsSection, {
      props: { fields, modelValue: {} },
    });
    expect(w.findAll('.input--error')).toHaveLength(0);
  });

  it('submitted=true + пустое required поле -> подсветка input--error', () => {
    const w = mount(CustomFieldsSection, {
      props: { fields, modelValue: {}, submitted: true },
    });
    // поле 1 (is_required=true) должно подсветиться, поле 2 (is_required=false) - нет
    expect(w.findAll('.input--error')).toHaveLength(1);
  });

  it('submitted=true + заполненное required поле -> нет ошибки', () => {
    const w = mount(CustomFieldsSection, {
      props: { fields, modelValue: { 1: 'Иванов', 2: '' }, submitted: true },
    });
    expect(w.findAll('.input--error')).toHaveLength(0);
  });

  it('звёздочка отображается только для is_required=true полей', () => {
    const w = mount(CustomFieldsSection, {
      props: { fields, modelValue: {} },
    });
    expect(w.findAll('.required')).toHaveLength(1);
  });
});
