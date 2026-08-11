import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const listCitizenshipsMock = vi.fn();
vi.mock('@/api/citizenships', () => ({
  listCitizenships: (...args) => listCitizenshipsMock(...args),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: vi.fn(() => ({ notify: vi.fn() })),
}));

vi.mock('@/api/attachment-templates', () => ({ saveBlobAs: vi.fn() }));

import BlankImportResult from '../BlankImportResult.vue';

// Компонент сам подтягивает справочник форматов номеров: по нему сводка отличает
// исправленный номер от такого же негодного. Без мока реальный клиент дёргает Pinia
// вне приложения и справочник остаётся пустым.
const RU_PLATE_FORMAT = {
  format: { id: 1, name: 'Россия', is_default: true },
  cells: [
    { cell_order: 1, cell_type: 'letters', min_length: 1, max_length: 1, alphabet_type: 'cyrillic' },
    { cell_order: 2, cell_type: 'numbers', min_length: 3, max_length: 3 },
    { cell_order: 3, cell_type: 'letters', min_length: 2, max_length: 2, alphabet_type: 'cyrillic' },
    { cell_order: 4, cell_type: 'numbers', min_length: 2, max_length: 3 },
  ],
};
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(async () => ({ ok: true, json: async () => [RU_PLATE_FORMAT] })),
}));


const UNLOAD_PLACES = [{ id: 30, name: 'Склад 1', status: 'active' }];
const PASSAGE_TABLES = [
  { table: { id: 20, name: 'kpp-1', display_name: 'КПП 1', table_type: 'cars', status: 'active' } },
];
const CITIZENSHIPS = [{ id: 1, name: 'Россия' }, { id: 2, name: 'Узбекистан' }];

function mountPanel(props = {}) {
  return mount(BlankImportResult, {
    props: {
      attachmentType: 'cars',
      hasResult: true,
      summary: { read: 1, accepted: 0, rejected: 1 },
      rows: [],
      pendingCount: 0,
      allPassageTables: PASSAGE_TABLES,
      allUnloadingPlaces: UNLOAD_PLACES,
      fieldConfig: {},
      ...props,
    },
    // Меню выбора формата телепортится в body - разворачиваем его на месте, чтобы
    // искать пункты в обёртке теста.
    global: { stubs: { teleport: true } },
  });
}

// Номер, не подошедший ни одному формату при разборе, оставляет список форматов
// пустым: ячейки появляются только после того, как человек назвал формат.
async function chooseFormat(wrapper, rowNumber, name) {
  const picker = wrapper.find(`[data-testid="bim-format-${rowNumber}"]`);
  await picker.find('.base-dropdown__button').trigger('click');
  const item = picker.findAll('.base-dropdown__item').find((node) => node.text() === name);
  await item.trigger('click');
  await flushPromises();
}

// Признак «правится прямо здесь» приходит с сервера полем fixable у каждой причины
// (ImportRowError, internal/services/attachment_import_validate.go). До этого сводка
// угадывала его, разбирая текст причины по префиксу «Поле «<подпись>»», и любая
// формулировка вне этого шаблона - в частности несовпадение формата номера - блокировала
// галочку навсегда, хотя номер редактируется прямо в таблице разбора.
const PLATE_FORMAT_ROW = {
  row_number: 4,
  vehicle: { car_number: 'Писька', car_brand: 'Toyota', mark_id: null, unload_places: [], passage_tables: [] },
  errors: [{
    text: 'Номер Т/С "Писька" не соответствует ни одному формату номеров',
    code: 'plate_format_unknown',
    field: 'number',
    fixable: true,
  }],
  warnings: [],
};

const BLACKLIST_ROW = {
  row_number: 5,
  vehicle: { car_number: 'А001АА777', car_brand: 'Volvo', mark_id: null, unload_places: [], passage_tables: [] },
  errors: [{
    text: 'Машина А001АА777 Volvo в чёрном списке: решение суда',
    code: 'blacklisted',
    field: '',
    fixable: false,
  }],
  warnings: [],
};

const EMPTY_NAME_ROW = {
  row_number: 6,
  employee: {
    last_name: '', first_name: '', middle_name: '',
    citizenship_id: 1, position: 'Инженер',
    passport_series_number: '', patent_number: null, other_permission: null,
    target_tables: [],
  },
  errors: [
    { text: 'Поле «Фамилия» обязательно для заполнения', code: 'field_required', field: 'last_name', fixable: true },
    { text: 'Поле «Имя» обязательно для заполнения', code: 'field_required', field: 'first_name', fixable: true },
  ],
  warnings: [],
};

const POSITION_ROW = {
  row_number: 7,
  employee: {
    last_name: 'Иванов', first_name: 'Иван', middle_name: '',
    citizenship_id: 1, position: '',
    passport_series_number: '', patent_number: null, other_permission: null,
    target_tables: [],
  },
  // Должности в таблице разбора нет и не будет - причина приходит неисправимой.
  errors: [{ text: 'Поле «Должность» обязательно для заполнения', code: 'field_required', field: 'position', fixable: false }],
  warnings: [],
};

describe('BlankImportResult - исправимость причины приходит с сервера', () => {
  beforeEach(() => {
    listCitizenshipsMock.mockReset();
    listCitizenshipsMock.mockResolvedValue(CITIZENSHIPS);
  });

  it('строка с несовпадением формата номера включается после правки номера', async () => {
    const wrapper = mountPanel({ rows: [PLATE_FORMAT_ROW] });
    await flushPromises();

    const row = wrapper.find('[data-testid="bim-problem-row-4"]');
    expect(row.exists()).toBe(true);
    await chooseFormat(wrapper, 4, 'Россия');

    const cells = wrapper.find('[data-testid="bim-problem-row-4"]').findAll('input.bim__plate-cell');
    await cells[0].setValue('В');
    await cells[1].setValue('777');
    await cells[2].setValue('ВВ');
    await cells[3].setValue('177');

    expect(wrapper.find('[data-testid="bim-include-4"]').attributes('disabled')).toBeUndefined();
  });

  it('строка из чёрного списка не включается никакой правкой номера', async () => {
    const wrapper = mountPanel({ rows: [BLACKLIST_ROW] });
    await flushPromises();

    const row = wrapper.find('[data-testid="bim-problem-row-5"]');
    const cells = row.findAll('input.bim__plate-cell');
    await cells[0].setValue('В');
    await cells[1].setValue('777');
    await cells[2].setValue('ВВ');
    await cells[3].setValue('177');

    expect(wrapper.find('[data-testid="bim-include-5"]').attributes('disabled')).toBeDefined();
  });

  it('пустые ФИО остаются исправимыми, а отсутствие должности - нет', async () => {
    const wrapper = mountPanel({
      attachmentType: 'people',
      rows: [EMPTY_NAME_ROW, POSITION_ROW],
      summary: { read: 2, accepted: 0, rejected: 2 },
    });
    await flushPromises();

    const fixable = wrapper.find('[data-testid="bim-problem-row-6"]').findAll('input.bim__cell-input');
    await fixable[0].setValue('Петров');
    await fixable[1].setValue('Пётр');
    expect(wrapper.find('[data-testid="bim-include-6"]').attributes('disabled')).toBeUndefined();

    expect(wrapper.find('[data-testid="bim-include-7"]').attributes('disabled')).toBeDefined();
  });

  it('текст причины показан человеку как есть, без разбора его фронтом', async () => {
    const wrapper = mountPanel({ rows: [PLATE_FORMAT_ROW] });
    await flushPromises();

    expect(wrapper.find('[data-testid="bim-problem-row-4"]').text())
      .toContain('не соответствует ни одному формату номеров');
  });
});
