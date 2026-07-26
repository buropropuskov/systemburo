import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';

import XlsxViewer from '../XlsxViewer.vue';

// Без файла компонент рисует заглушку, но разбор привязок считается так же.
function mountViewer(props = {}) {
  return mount(XlsxViewer, { props: { fileBuffer: null, ...props } });
}

const MAPPINGS = [
  { cell_ref: 'A43', field_path: 'application.sender.phone', fieldLabel: 'Телефон отправителя' },
  { cell_ref: 'A43', field_path: 'application.sender.short_name', fieldLabel: 'Фамилия И.О. отправителя' },
  { cell_ref: 'B10', field_path: 'car.car_number', fieldLabel: 'Номер ТС' },
];

describe('XlsxViewer: привязки ячейки', () => {
  it('держит все поля ячейки в порядке склейки, а не последнее', () => {
    const wrapper = mountViewer({ mappings: MAPPINGS });
    expect(wrapper.vm.mappedCells.get('A43')).toEqual([
      'Телефон отправителя', 'Фамилия И.О. отправителя',
    ]);
    expect(wrapper.vm.mappedCells.get('B10')).toEqual(['Номер ТС']);
  });

  it('в тултипе совмещённой ячейки показывает число полей и склейку разделителем', () => {
    const wrapper = mountViewer({ mappings: MAPPINGS, concatSeparator: ' / ' });
    const hint = wrapper.vm.cellTooltip('A43');
    expect(hint).toContain('полей 2');
    expect(hint).toContain('Телефон отправителя / Фамилия И.О. отправителя');
  });

  it('у одиночной ячейки тултип это её адрес', () => {
    const wrapper = mountViewer({ mappings: MAPPINGS });
    expect(wrapper.vm.cellTooltip('B10')).toBe('B10');
  });

  it('пустой разделитель склеивает без пробела', () => {
    const wrapper = mountViewer({ mappings: MAPPINGS, concatSeparator: '' });
    expect(wrapper.vm.cellTooltip('A43')).toContain('Телефон отправителяФамилия И.О. отправителя');
  });
});
