import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import DateRangeSection from '../DateRangeSection.vue';

// H-5 (#529): DateRangeSection уважает field-config выбранного шаблона.
// Даты/время реестром залочены (всегда visible+required), поэтому при пустом
// конфиге поведение прежнее. Проверяем сам механизм потребления конфига:
// хелперы fieldVisible/fieldRequired + биндинги v-if в шаблоне. Видимость блоков
// проверяем по уникальному тексту лейбла (устойчиво к переверстке разметки).

const hasDateBlock = (w) => w.text().includes('Дата действия');
const hasTimeBlock = (w) => w.text().includes('Время пребывания');

describe('DateRangeSection - потребление field-config (#529)', () => {
  it('без конфига: блоки даты и времени видимы, звёздочки обязательности на месте', () => {
    const w = mount(DateRangeSection);
    expect(hasDateBlock(w)).toBe(true);
    expect(hasTimeBlock(w)).toBe(true);
    expect(w.findAll('.input__label .required')).toHaveLength(2);
  });

  it('fieldVisible: нет строки -> true; visible:false -> false', () => {
    const w = mount(DateRangeSection, {
      props: { fieldConfig: { foo: { visible: false, required: true } } },
    });
    expect(w.vm.fieldVisible('missing')).toBe(true);
    expect(w.vm.fieldVisible('foo')).toBe(false);
  });

  it('fieldRequired: нет строки -> true; required:false -> false', () => {
    const w = mount(DateRangeSection, {
      props: { fieldConfig: { foo: { visible: true, required: false } } },
    });
    expect(w.vm.fieldRequired('missing')).toBe(true);
    expect(w.vm.fieldRequired('foo')).toBe(false);
  });

  it('конфиг скрывает блок даты при entry_date_from.visible=false', () => {
    const w = mount(DateRangeSection, {
      props: { fieldConfig: { entry_date_from: { visible: false, required: true } } },
    });
    expect(hasDateBlock(w)).toBe(false);
    expect(hasTimeBlock(w)).toBe(true);
  });

  it('конфиг скрывает блок времени при entry_time_from.visible=false', () => {
    const w = mount(DateRangeSection, {
      props: { fieldConfig: { entry_time_from: { visible: false, required: true } } },
    });
    expect(hasTimeBlock(w)).toBe(false);
    expect(hasDateBlock(w)).toBe(true);
  });

  it('конфиг убирает звёздочку даты при required=false', () => {
    const w = mount(DateRangeSection, {
      props: { fieldConfig: { entry_date_from: { visible: true, required: false } } },
    });
    // остаётся только звёздочка времени
    expect(w.findAll('.input__label .required')).toHaveLength(1);
  });
});
