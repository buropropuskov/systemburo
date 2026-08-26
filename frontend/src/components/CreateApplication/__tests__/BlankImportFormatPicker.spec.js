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
// проверяется, что формат строки определяется при разборе и что проверка идёт по
// ВЫБРАННОМУ формату, а не перебором по всем (доводка владельца).
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

// Номер не ложится НИ В ОДИН формат справочника: буквы там, где оба формата ждут
// цифры. На таком формат определить нечем - список остаётся без выбора.
const UNKNOWN_PLATE_ROW = {
  row_number: 9,
  vehicle: { car_number: 'ВВВВВВ', car_brand: 'Man', mark_id: null, unload_places: [], passage_tables: [] },
  errors: [{
    text: 'Номер Т/С "ВВВВВВ" не соответствует ни одному формату номеров',
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

// Меню дропдауна телепортится в body (иначе его обрезал бы прокручиваемый список
// карточек) - в тесте разворачиваем телепорт на месте, чтобы искать пункты в обёртке.
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
    global: { stubs: { teleport: true } },
  });
}

function formatPicker(wrapper, rowNumber) {
  return wrapper.find(`[data-testid="bim-format-${rowNumber}"]`);
}

function formatButtonText(wrapper, rowNumber) {
  return formatPicker(wrapper, rowNumber).find('.base-dropdown__text').text();
}

async function openFormatMenu(wrapper, rowNumber) {
  await formatPicker(wrapper, rowNumber).find('.base-dropdown__button').trigger('click');
  return formatPicker(wrapper, rowNumber);
}

async function chooseFormat(wrapper, rowNumber, name) {
  const picker = await openFormatMenu(wrapper, rowNumber);
  const item = picker.findAll('.base-dropdown__item').find((node) => node.text() === name);
  await item.trigger('click');
  await flushPromises();
}

function plateCells(wrapper, rowNumber) {
  return wrapper.findAll(`[data-testid="bim-problem-row-${rowNumber}"] .bim__plate-cell`);
}

// Смена формата очищает ячейки, как в форме ручного ввода, поэтому после переключения
// номер вводится заново - тесты ниже делают это явно.
async function typePlate(wrapper, rowNumber, parts) {
  const cells = plateCells(wrapper, rowNumber);
  for (let i = 0; i < parts.length; i += 1) {
    await cells[i].setValue(parts[i]);
  }
}

describe('BlankImportResult - выбор формата номера на каждую строку', () => {
  beforeEach(() => {
    listCitizenshipsMock.mockReset();
    listCitizenshipsMock.mockResolvedValue([{ id: 1, name: 'Россия' }]);
  });

  it('для строки машины показан выбор формата, для строки человека - нет', async () => {
    const carWrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();
    expect(formatPicker(carWrapper, 4).exists()).toBe(true);

    const peopleWrapper = mountPanel({ attachmentType: 'people', rows: [PEOPLE_ROW] });
    await flushPromises();
    expect(peopleWrapper.find('[data-testid="bim-format-6"]').exists()).toBe(false);
  });

  it('формат определяется при разборе и стоит выбранным в списке - подпись совпадает с ячейками', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    // "1234" ложится только в "Транзит" - он и выбран, хотя дефолтный в справочнике другой.
    expect(wrapper.vm.problemRows[0].fields.formatId).toBe(FORMAT_B.format.id);
    expect(formatButtonText(wrapper, 4)).toBe('Транзит');
    // Ячейка ровно одна - как у "Транзита", а не четыре, как у дефолтного РФ-формата.
    expect(plateCells(wrapper, 4)).toHaveLength(FORMAT_B.cells.length);
    expect(wrapper.find('[data-testid="bim-include-4"]').attributes('disabled')).toBeUndefined();
  });

  it('в списке нет пункта "Определить автоматически" - только реальные форматы справочника', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    // Выбор формата - дропдаун проекта, а не нативный select (замечание владельца).
    expect(formatPicker(wrapper, 4).classes()).toContain('base-dropdown');
    expect(wrapper.find('[data-testid="bim-problem-row-4"] select.bim__cell-input').exists()).toBe(false);

    const picker = await openFormatMenu(wrapper, 4);
    expect(picker.findAll('.base-dropdown__item').map((node) => node.text()))
      .toEqual(['Россия', 'Транзит']);
  });

  it('явный выбор НЕподходящего формата блокирует добавление', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    await chooseFormat(wrapper, 4, 'Россия');

    // "1234" под ячейки "России" (буква первой) не раскладывается - ячейки формата
    // пересобираются пустыми (см. selectFormat), человек печатает заново.
    expect(wrapper.find('[data-testid="bim-include-4"]').attributes('disabled')).toBeDefined();
    expect(plateCells(wrapper, 4).map((c) => c.element.value)).toEqual(['', '', '', '']);
    expect(wrapper.find('[data-testid="bim-problem-row-4"]').text()).toContain('Введите номер Т/С');
  });

  it('явный выбор подходящего формата включает добавление', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    await chooseFormat(wrapper, 4, 'Россия');
    await chooseFormat(wrapper, 4, 'Транзит');
    await typePlate(wrapper, 4, ['1234']);

    expect(wrapper.find('[data-testid="bim-include-4"]').attributes('disabled')).toBeUndefined();
  });

  it('номер, не подошедший ни одному формату, оставляет список пустым и ячейки не рисует', async () => {
    const wrapper = mountPanel({ rows: [UNKNOWN_PLATE_ROW] });
    await flushPromises();

    expect(wrapper.vm.problemRows[0].fields.formatId).toBeNull();
    expect(formatButtonText(wrapper, 9)).toBe('Выберите формат');
    expect(plateCells(wrapper, 9)).toHaveLength(0);
    const row = wrapper.find('[data-testid="bim-problem-row-9"]');
    expect(row.find('.bim__plate-empty').text()).toBe('Выберите формат номера');
    expect(row.text()).toContain('Выберите формат номера, чтобы отметить строку.');
    expect(wrapper.find('[data-testid="bim-include-9"]').attributes('disabled')).toBeDefined();
  });

  it('выбор формата на неопознанной строке рисует его ячейки и открывает добавление', async () => {
    const wrapper = mountPanel({ rows: [UNKNOWN_PLATE_ROW] });
    await flushPromises();

    await chooseFormat(wrapper, 9, 'Россия');
    expect(plateCells(wrapper, 9)).toHaveLength(FORMAT_A.cells.length);

    await typePlate(wrapper, 9, ['А', '123', 'ВС', '777']);
    expect(wrapper.find('[data-testid="bim-include-9"]').attributes('disabled')).toBeUndefined();
  });

  it('"По факту" остаётся допустимым значением и при выбранном формате, и без него', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW, UNKNOWN_PLATE_ROW], summary: { read: 2, accepted: 0, rejected: 2 } });
    await flushPromises();

    await chooseFormat(wrapper, 4, 'Россия');
    // Компактный тумблер "по факту" (перенесён из VehicleForm) - вместо ячеек, не
    // отдельное текстовое значение, которое раньше нужно было впечатать вручную.
    await wrapper.find('[data-testid="bim-problem-row-4"] input[type="checkbox"]').setValue(true);
    expect(wrapper.find('[data-testid="bim-include-4"]').attributes('disabled')).toBeUndefined();

    // Строка без опознанного формата: "по факту" не подчиняется формату и принимается
    // без выбора в списке.
    await wrapper.find('[data-testid="bim-problem-row-9"] input[type="checkbox"]').setValue(true);
    expect(wrapper.find('[data-testid="bim-include-9"]').attributes('disabled')).toBeUndefined();
  });

  it('тумблер "по факту" работает в обе стороны: назад возвращаются пустые ячейки формата', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    const toggle = wrapper.find('[data-testid="bim-problem-row-4"] input[type="checkbox"]');
    await toggle.setValue(true);
    expect(plateCells(wrapper, 4)).toHaveLength(0);

    await toggle.setValue(false);
    const cells = plateCells(wrapper, 4);
    expect(cells).toHaveLength(FORMAT_B.cells.length);
    expect(cells.map((c) => c.element.value)).toEqual(['']);
  });

  it('выбранный формат уезжает вместе со строкой в formatId - то же поле, что использует VehicleForm', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    await chooseFormat(wrapper, 4, 'Транзит');
    await typePlate(wrapper, 4, ['1234']);
    await wrapper.find('[data-testid="bim-include-4"]').trigger('click');
    await flushPromises();

    const staged = wrapper.emitted('stage');
    const last = staged[staged.length - 1][0];
    expect(last.rows[0]).toMatchObject({ plateNumber: '1234', formatId: FORMAT_B.format.id });
  });

  // Ветка, которую ревью отметило непокрытой: исходный номер из файла раскладывается
  // под недефолтный формат, и при первом показе строки ячейки уже заполнены им - человеку
  // не приходится перепечатывать корректный номер. При РУЧНОЙ смене формата ячейки,
  // наоборот, очищаются, как в форме подачи.
  it('исходный номер подставляется в ячейки при первом показе, но не при смене формата', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    expect(plateCells(wrapper, 4).map((c) => c.element.value).join('')).toBe('1234');

    await chooseFormat(wrapper, 4, 'Россия');
    expect(plateCells(wrapper, 4).every((c) => c.element.value === '')).toBe(true);
  });

  it('строка без правки формата уходит в список с тем форматом, который определился при разборе', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    await wrapper.find('[data-testid="bim-include-4"]').trigger('click');
    await flushPromises();

    const staged = wrapper.emitted('stage');
    const last = staged[staged.length - 1][0];
    expect(last.rows[0].formatId).toBe(FORMAT_B.format.id);
  });

  it('меню закрывается по клику вне карточки и по Escape', async () => {
    const wrapper = mountPanel({ rows: [TRANSIT_ONLY_ROW] });
    await flushPromises();

    const picker = await openFormatMenu(wrapper, 4);
    expect(picker.find('.base-dropdown__menu').exists()).toBe(true);

    document.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await flushPromises();
    expect(picker.find('.base-dropdown__menu').exists()).toBe(false);

    await openFormatMenu(wrapper, 4);
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await flushPromises();
    expect(picker.find('.base-dropdown__menu').exists()).toBe(false);
  });
});
