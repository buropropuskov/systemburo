import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

vi.mock('@/api/citizenships', () => ({
  listCitizenships: vi.fn().mockResolvedValue([{ id: 1, name: 'Россия' }]),
}));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: vi.fn(() => ({ notify: vi.fn() })) }));
vi.mock('@/api/attachment-templates', () => ({ saveBlobAs: vi.fn() }));

const RU_FORMAT = {
  format: { id: 1, name: 'Россия', is_default: true },
  cells: [
    { cell_order: 1, cell_type: 'letters', min_length: 1, max_length: 1, alphabet_type: 'cyrillic' },
    { cell_order: 2, cell_type: 'numbers', min_length: 3, max_length: 3 },
    { cell_order: 3, cell_type: 'letters', min_length: 2, max_length: 2, alphabet_type: 'cyrillic' },
    { cell_order: 4, cell_type: 'numbers', min_length: 2, max_length: 3 },
  ],
};
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(async () => ({ ok: true, json: async () => [RU_FORMAT] })),
}));

import BlankImportResult from '../BlankImportResult.vue';

// Сводка показывает три числа рядом, и человек их складывает. Пока «с ошибками»
// приходило снимком разбора, а «готово к добавлению» считалось по живому списку,
// починка строки ломала арифметику: 15 прочитано при 14 готовых и 2 с ошибками
// (владелец увидел это на стенде). Оба счётчика обязаны идти от одного состояния.

const UNLOAD_PLACES = [{ id: 30, name: 'Склад 1', status: 'active' }];
const PASSAGE_TABLES = [
  { table: { id: 20, name: 'kpp-1', display_name: 'КПП 1', table_type: 'cars', status: 'active' } },
];

const badRow = (n, plate) => ({
  row_number: n,
  vehicle: { car_number: plate, car_brand: 'Kamaz' },
  errors: [{ text: `Номер Т/С "${plate}" не соответствует ни одному формату номеров`, code: 'plate_format', field: 'number', fixable: true }],
  warnings: [],
});
const goodRow = (n, plate) => ({
  row_number: n,
  vehicle: { car_number: plate, car_brand: 'Volvo' },
  errors: [],
  warnings: [],
});

const ROWS = [badRow(19, 'Писька'), badRow(20, 'ЫЫЫЫ'), goodRow(21, 'А 001 АА 777')];

function mountPanel(props = {}) {
  return mount(BlankImportResult, {
    props: {
      attachmentType: 'cars',
      hasResult: true,
      summary: { read: 3, accepted: 1, rejected: 2 },
      rows: ROWS,
      pendingCount: 1,
      allPassageTables: PASSAGE_TABLES,
      allUnloadingPlaces: UNLOAD_PLACES,
      fieldConfig: {},
      ...props,
    },
    global: { stubs: { teleport: true } },
  });
}

const errorCount = (w) => w.find('[data-testid="bim-error-count"]').text();
const problemTitleCount = (w) => w.find('.bim__problems-count').text();

async function fixRow(w, n) {
  await w.find(`[data-testid="bim-format-${n}"] .base-dropdown__button`).trigger('click');
  await w.findAll(`[data-testid="bim-format-${n}"] .base-dropdown__item`)[0].trigger('click');
  await flushPromises();
  const cells = w.findAll(`[data-testid="bim-problem-row-${n}"] .bim__plate-cell`);
  const parts = ['В', '777', 'ВВ', '177'];
  for (let i = 0; i < parts.length; i += 1) await cells[i].setValue(parts[i]);
  await flushPromises();
  await w.find(`[data-testid="bim-include-${n}"]`).trigger('click');
  await flushPromises();
}

describe('BlankImportResult - счётчики сводки', () => {
  it('после разбора счётчик ошибок равен числу карточек', async () => {
    const w = mountPanel();
    await flushPromises();

    expect(errorCount(w)).toBe('2');
    expect(problemTitleCount(w)).toBe('2');
  });

  it('исправленная строка уходит из счётчика ошибок, а не только из списка карточек', async () => {
    const w = mountPanel();
    await flushPromises();

    await fixRow(w, 19);

    // Родитель принял строку в список - счётчик готовых растёт его пропом.
    await w.setProps({ pendingCount: 2 });
    expect(errorCount(w)).toBe('1');
    expect(problemTitleCount(w)).toBe('1');
    // Арифметика сводки сходится: прочитано = готово + с ошибками.
    expect(Number(w.find('[data-testid="bim-pending-count"]').text()) + Number(errorCount(w)))
      .toBe(w.props('summary').read);
  });

  it('последняя исправленная строка убирает счётчик ошибок целиком', async () => {
    const w = mountPanel();
    await flushPromises();

    await fixRow(w, 19);
    await fixRow(w, 20);

    expect(w.find('[data-testid="bim-error-count"]').exists()).toBe(false);
    expect(w.find('.bim__problems').exists()).toBe(false);
  });

  it('счётчик ошибок не зависит от снимка summary.rejected', async () => {
    // Разбор сказал «ошибок 5», а карточек две - на экране правда та, что видна.
    const w = mountPanel({ summary: { read: 3, accepted: 1, rejected: 5 } });
    await flushPromises();

    expect(errorCount(w)).toBe('2');
  });
});
