import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import VehicleNumberFormatInput from '../VehicleNumberFormatInput.vue';

const FORMATS = [
  {
    format: { id: 1, name: 'РФ', is_default: true },
    cells: [{ cell_type: 'letters', max_length: 1 }, { cell_type: 'numbers', max_length: 3 }],
  },
  {
    format: { id: 2, name: 'Один блок', is_default: false },
    cells: [{ cell_type: 'letters', max_length: 6 }],
  },
];

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ json: () => Promise.resolve(FORMATS) })),
}));

function lastModel(wrapper) {
  const ev = wrapper.emitted('update:modelValue');
  return ev ? ev[ev.length - 1][0] : undefined;
}

describe('VehicleNumberFormatInput', () => {
  beforeEach(() => vi.clearAllMocks());

  it('по умолчанию выбирает дефолтный формат с пустыми ячейками', async () => {
    const w = mount(VehicleNumberFormatInput, { props: { modelValue: '' } });
    await flushPromises();
    expect(w.vm.selectedFormatId).toBe(1);
    expect(w.vm.numberParts.length).toBe(2);
    expect(w.findAll('.number__input').length).toBe(2);
  });

  it('префилл раскладывает номер по ячейкам формата с совпадающим числом частей', async () => {
    const w = mount(VehicleNumberFormatInput, { props: { modelValue: 'А 123' } });
    await flushPromises();
    expect(w.vm.selectedFormatId).toBe(1);
    expect(w.vm.numberParts).toEqual(['А', '123']);
    expect(lastModel(w)).toBe('А 123');
  });

  it('префилл с одной частью выбирает одноблочный формат', async () => {
    const w = mount(VehicleNumberFormatInput, { props: { modelValue: 'АВС123' } });
    await flushPromises();
    expect(w.vm.selectedFormatId).toBe(2);
    expect(w.vm.numberParts).toEqual(['АВС123']);
  });

  it('смена формата сбрасывает ячейки', async () => {
    const w = mount(VehicleNumberFormatInput, { props: { modelValue: 'А 123' } });
    await flushPromises();
    w.vm.selectFormat(FORMATS[1]);
    await flushPromises();
    expect(w.vm.selectedFormatId).toBe(2);
    expect(w.vm.numberParts.length).toBe(1);
  });
});
