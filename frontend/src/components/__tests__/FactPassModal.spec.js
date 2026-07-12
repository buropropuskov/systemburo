import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const listMarks = vi.fn();
vi.mock('@/api/marks', () => ({
  listMarks: (...args) => listMarks(...args),
}));

import FactPassModal from '../FactPassModal.vue';

const FORMATS = [
  {
    format: { id: 1, name: 'Стандартный', is_default: true },
    cells: [
      { cell_type: 'letters', alphabet_type: 'cyrillic', min_length: 1, max_length: 1 },
      { cell_type: 'numbers', min_length: 3, max_length: 3 },
      { cell_type: 'letters', alphabet_type: 'cyrillic', min_length: 2, max_length: 2 },
    ],
  },
];

function mountModal(props = {}) {
  return mount(FactPassModal, {
    props: { show: true, formats: FORMATS, ...props },
    global: { stubs: { teleport: true, transition: false } },
  });
}

async function fillNumber(wrapper) {
  const cells = wrapper.findAll('[data-testid="fact-pass-number-cell"]');
  await cells[0].setValue('А');
  await cells[1].setValue('123');
  await cells[2].setValue('ВС');
}

describe('FactPassModal (#1132)', () => {
  beforeEach(() => {
    listMarks.mockReset();
    listMarks.mockResolvedValue([]);
  });

  it('кнопка "Пропустить" заблокирована, пока номер не заполнен полностью', async () => {
    const wrapper = mountModal();
    await flushPromises();

    const confirm = wrapper.get('[data-testid="fact-pass-confirm"]');
    expect(confirm.attributes('disabled')).toBeDefined();

    // Частично заполненный номер тоже не разблокирует.
    const cells = wrapper.findAll('[data-testid="fact-pass-number-cell"]');
    await cells[0].setValue('А');
    expect(wrapper.get('[data-testid="fact-pass-confirm"]').attributes('disabled')).toBeDefined();
  });

  it('emit confirm с собранным номером, форматом и маркой=null при пустой марке', async () => {
    const wrapper = mountModal();
    await flushPromises();
    await fillNumber(wrapper);

    const confirm = wrapper.get('[data-testid="fact-pass-confirm"]');
    expect(confirm.attributes('disabled')).toBeUndefined();

    await confirm.trigger('click');

    const emitted = wrapper.emitted('confirm');
    expect(emitted).toHaveLength(1);
    expect(emitted[0][0]).toEqual({
      number: 'А 123 ВС',
      format_id: 1,
      format_name: 'Стандартный',
      mark_id: null,
      mark_name: null,
    });
  });

  it('при loading=true кнопка "Пропустить" заблокирована даже с валидным номером', async () => {
    const wrapper = mountModal({ loading: true });
    await flushPromises();
    await fillNumber(wrapper);

    expect(wrapper.get('[data-testid="fact-pass-confirm"]').attributes('disabled')).toBeDefined();
  });

  it('кнопка "Отмена" эмитит close без подтверждения', async () => {
    const wrapper = mountModal();
    await flushPromises();

    const cancel = wrapper.findAll('button').find((b) => b.text() === 'Отмена');
    await cancel.trigger('click');

    expect(wrapper.emitted('close')).toHaveLength(1);
    expect(wrapper.emitted('confirm')).toBeUndefined();
  });
});
