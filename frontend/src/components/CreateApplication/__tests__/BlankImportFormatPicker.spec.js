import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const listCitizenshipsMock = vi.fn();
vi.mock('@/api/citizenships', () => ({
  listCitizenships: (...args) => listCitizenshipsMock(...args),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: vi.fn(() => ({ notify: vi.fn() })),
}));

// Два активных формата в справочнике: РФ-вид (буквы+цифры) и учебный "Транзит" -
// чисто цифровой. Номер, подходящий одному, заведомо не подходит другому - этим и
// проверяется, что явный выбор формата СУЖАЕТ проверку до него, а не остаётся
// перебором по всем (доводка владельца: выбор формата на каждую строку).
const FORMAT_A = {
  format: { id: 1, name: 'Россия', is_default: true },
  cells: [
    { cell_order: 1, cell_type: 'letters', min_length: 1, max_length: 1, alphabet_type: 'cyrillic' },
    { cell_order: 2, cell_type: 'numbers', min_length: 3, max_length: 3 },
    { cell_order: 3, cell_type: 'letters', min_length: 2, max_length: 2, alphabet_type: 'cyrillic' },
    { cell_order: 4, cell_type: 'numbers', min_length: 2, max_length: 3 },
  ],
};
const FORMAT_B = {
  format: { id: 2, name: 'Транзит', is_default: false },
  cells: [
    { cell_order: 1, cell_type: 'numbers', min_length: 4, max_length: 4 },
  ],
};
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(async () => ({ ok: true, json: async () => [FORMAT_A, FORMAT_B] })),
}));

import BlankImportResult from '../BlankImportResult.vue';

const PASSAGE_TABLES = [];
const UNLOAD_PLACES = [];

// Номер подходит ТОЛЬКО "Транзиту" (чисто цифровой, 4 знака) - под РФ-формат (нужна
// буква первой ячейкой) не ложится ни при каком раскладе.
const TRANSIT_ONLY_ROW = {
  row_number: 4,
  vehicle: { car_number: '1234', car_brand: 'Kamaz', mark_id: null, unload_places: [], passage_tables: [] },
  errors: [{
    text: 'Номер Т/С "1234" не соответствует ни одному формату номеров',
    code: 'plate_format_unknown',
    field: 'number',
    fixable: true,
  }],
  warnings: [],
};

const PEOPLE_ROW = {
  row_number: 6,
  employee: {
    last_name: '', first_name: 'Мария', middle_name: '',
    citizenship_id: 1, position: 'Бухгалтер',
    passport_series_number: '', patent_number: null, other_permission: null,
    target_tables: [],
  },
  errors: [{ text: 'Поле «Фамилия» обязательно для заполнения', code: 'field_required', field: 'last_name', fixable: true }],
  warnings: [],
};

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
  });
}

describe('BlankImportResult - выбор формата номера на каждую строку', () => {
  beforeEach(() => {
    listCitizenshipsMock.mockReset();
    listCitizenshipsMock.mockResolvedValue([{ id: 1, name: 'Россия' }]);
  });

  it('для строки машины показан выбор формата, для строки человека - нет', async () => {
    const carWrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();
    expect(carWrapper.find('[data-testid="bim-format-4"]').exists()).toBe(true);

    const peopleWrapper = mountPanel({ attachmentType: 'people', rows: [PEOPLE_ROW] });
    await flushPromises();
    expect(peopleWrapper.find('[data-testid="bim-format-6"]').exists()).toBe(false);
  });

  it('по умолчанию выбрано "Определить автоматически", и подбор по всем форматам работает как раньше', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    expect(wrapper.vm.problemRows[0].fields.formatId).toBeNull();
    // Автоподбор перебирает оба формата - цифровой номер ложится в "Транзит".
    expect(wrapper.find('[data-testid="bim-include-4"]').attributes('disabled')).toBeUndefined();
  });

  it('явный выбор НЕподходящего формата блокирует добавление, хотя автоподбор номер бы принял', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    await wrapper.find('[data-testid="bim-format-4"]').setValue(String(FORMAT_A.format.id));

    const include = wrapper.find('[data-testid="bim-include-4"]');
    expect(include.attributes('disabled')).toBeDefined();
    expect(wrapper.find('[data-testid="bim-problem-row-4"]').text()).toContain('не подходит формату "Россия"');
  });

  it('явный выбор подходящего формата включает добавление', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    await wrapper.find('[data-testid="bim-format-4"]').setValue(String(FORMAT_B.format.id));

    expect(wrapper.find('[data-testid="bim-include-4"]').attributes('disabled')).toBeUndefined();
  });

  it('возврат к "Определить автоматически" снова принимает номер по общему перебору', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    const select = wrapper.find('[data-testid="bim-format-4"]');
    await select.setValue(String(FORMAT_A.format.id));
    expect(wrapper.find('[data-testid="bim-include-4"]').attributes('disabled')).toBeDefined();

    await select.setValue('');
    expect(wrapper.find('[data-testid="bim-include-4"]').attributes('disabled')).toBeUndefined();
  });

  it('"По факту" остаётся допустимым значением при любом выбранном формате', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    await wrapper.find('[data-testid="bim-format-4"]').setValue(String(FORMAT_A.format.id));
    await wrapper.find('[data-testid="bim-problem-row-4"] input.bim__cell-input').setValue('По факту');

    expect(wrapper.find('[data-testid="bim-include-4"]').attributes('disabled')).toBeUndefined();
  });

  it('выбранный формат уезжает вместе со строкой в formatId - то же поле, что использует VehicleForm', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    await wrapper.find('[data-testid="bim-format-4"]').setValue(String(FORMAT_B.format.id));
    await wrapper.find('[data-testid="bim-include-4"]').trigger('click');
    await flushPromises();

    const staged = wrapper.emitted('stage');
    const last = staged[staged.length - 1][0];
    expect(last.rows[0]).toMatchObject({ plateNumber: '1234', formatId: FORMAT_B.format.id });
  });

  it('строка без правки формата (автоподбор) уходит в список с formatId null', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    await wrapper.find('[data-testid="bim-include-4"]').trigger('click');
    await flushPromises();

    const staged = wrapper.emitted('stage');
    const last = staged[staged.length - 1][0];
    expect(last.rows[0].formatId).toBeNull();
  });
});
