import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const listCitizenshipsMock = vi.fn();
vi.mock('@/api/citizenships', () => ({
  listCitizenships: (...args) => listCitizenshipsMock(...args),
}));

const notifyMock = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: vi.fn(() => ({ notify: notifyMock })),
}));

const saveBlobAsMock = vi.fn();
vi.mock('@/api/attachment-templates', () => ({
  saveBlobAs: (...args) => saveBlobAsMock(...args),
}));

// Реальный ExcelJS.writeBuffer() под нагрузкой параллельного vitest-раннера медленный
// и флакует по таймауту (лишний вес библиотеки не по делу теста) - мокаем минимальным
// функциональным стабом, достаточным для цепочки addRow/eachCell/writeBuffer.
vi.mock('exceljs', () => {
  const cell = { fill: null, font: null };
  const row = { eachCell: (fn) => fn(cell) };
  const worksheet = { addRow: vi.fn(() => row), columns: [] };
  const workbook = {
    addWorksheet: vi.fn(() => worksheet),
    xlsx: { writeBuffer: vi.fn(async () => new Uint8Array([1, 2, 3])) },
  };
  // Мок конструктора: `new ExcelJS.Workbook()` требует function-форму, vi.fn() со
  // стрелочной реализацией не годится в роли `new`.
  function Workbook() { return workbook; }
  return { default: { Workbook } };
});

import BlankImportResultModal from '../BlankImportResultModal.vue';

// Реальная форма /system-tables: double-wrap { table: {...} } - как в
// TableBulkTargetModal.spec.js, плоская фикстура маскировала бы фильтр по table_type.
const PASSAGE_TABLES = [
  { table: { id: 10, name: 'people-1', display_name: 'Проход 1', table_type: 'people', status: 'active' } },
  { table: { id: 20, name: 'kpp-1', display_name: 'КПП 1', table_type: 'cars', status: 'active' } },
];
const UNLOAD_PLACES = [
  { id: 30, name: 'Склад 1', status: 'active' },
];
const CITIZENSHIPS = [
  { id: 1, name: 'Россия' },
  { id: 2, name: 'Узбекистан' },
];

function mountModal(props = {}) {
  return mount(BlankImportResultModal, {
    props: {
      show: true,
      attachmentType: 'people',
      summary: { read: 2, accepted: 1, rejected: 1 },
      rows: [],
      allPassageTables: PASSAGE_TABLES,
      allUnloadingPlaces: UNLOAD_PLACES,
      fieldConfig: {},
      ...props,
    },
    global: { stubs: { teleport: true } },
  });
}

const PEOPLE_ROWS = [
  {
    row_number: 5,
    employee: {
      last_name: 'Иванов', first_name: 'Иван', middle_name: '',
      citizenship_id: 1, position: 'Инженер',
      passport_series_number: '1234 567890', patent_number: null, other_permission: null,
      target_tables: [],
    },
    errors: [],
    warnings: [],
  },
  {
    row_number: 6,
    employee: {
      last_name: '', first_name: '', middle_name: '',
      citizenship_id: 0, position: '',
      passport_series_number: '', patent_number: null, other_permission: null,
      target_tables: [],
    },
    errors: ['Поле «Фамилия» обязательно для заполнения', 'Поле «Имя» обязательно для заполнения'],
    warnings: [],
  },
];

const CAR_ROWS = [
  {
    row_number: 2,
    vehicle: { car_number: 'А001АА777', car_brand: 'Volvo', mark_id: null, unload_places: [], passage_tables: [] },
    errors: [],
    warnings: [],
  },
  {
    row_number: 3,
    vehicle: { car_number: '', car_brand: 'Kamaz', mark_id: null, unload_places: [], passage_tables: [] },
    errors: ['Поле «Номер Т/С» обязательно для заполнения'],
    warnings: [],
  },
];

describe('BlankImportResultModal (blank-import D1D2)', () => {
  beforeEach(() => {
    notifyMock.mockReset();
    listCitizenshipsMock.mockReset();
    listCitizenshipsMock.mockResolvedValue(CITIZENSHIPS);
  });

  it('207 разбирается: счётчики совпадают с summary, проблемная строка показана', async () => {
    const wrapper = mountModal({
      rows: PEOPLE_ROWS,
      summary: { read: 2, accepted: 1, rejected: 1 },
    });
    await flushPromises();

    expect(wrapper.findAll('.bim__counter-value').map((n) => n.text())).toEqual(['2', '1', '1']);
    expect(wrapper.find('[data-testid="bim-problem-row-6"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="bim-problem-row-5"]').exists()).toBe(false);
  });

  it('кнопка добавления заблокирована без выбора мест прохода и разблокируется после выбора', async () => {
    const wrapper = mountModal({ rows: [PEOPLE_ROWS[0]], summary: { read: 1, accepted: 1, rejected: 0 } });
    await flushPromises();

    const submit = wrapper.find('[data-testid="bim-submit"]');
    expect(submit.attributes('disabled')).toBeDefined();

    await wrapper.findAll('.passage__item')[0].trigger('click');
    expect(wrapper.find('[data-testid="bim-submit"]').attributes('disabled')).toBeUndefined();
  });

  it('принятые строки формируют payload с выбранными местами прохода, отклонённые без правки - нет', async () => {
    const wrapper = mountModal({ rows: PEOPLE_ROWS, summary: { read: 2, accepted: 1, rejected: 1 } });
    await flushPromises();
    await wrapper.findAll('.passage__item')[0].trigger('click');

    await wrapper.find('[data-testid="bim-submit"]').trigger('click');

    const emitted = wrapper.emitted('import');
    expect(emitted).toHaveLength(1);
    const payload = emitted[0][0];
    expect(payload.attachmentType).toBe('people');
    expect(payload.rows).toHaveLength(1);
    expect(payload.rows[0]).toMatchObject({
      lastName: 'Иванов',
      firstName: 'Иван',
      citizenshipId: 1,
      citizenshipName: 'Россия',
      passportSeriesNumber: '1234 567890',
      targetTables: [10],
      isExisting: false,
    });
  });

  it('правка проблемной строки на месте делает её добавляемой и включённой в payload', async () => {
    const wrapper = mountModal({ rows: PEOPLE_ROWS, summary: { read: 2, accepted: 1, rejected: 1 } });
    await flushPromises();
    await wrapper.findAll('.passage__item')[0].trigger('click');

    const row6 = wrapper.find('[data-testid="bim-problem-row-6"]');
    const inputs = row6.findAll('input.bim__cell-input');
    await inputs[0].setValue('Петров');
    await inputs[1].setValue('Пётр');

    const checkbox = wrapper.find('[data-testid="bim-include-6"]');
    expect(checkbox.attributes('disabled')).toBeUndefined();
    await checkbox.setValue(true);

    await wrapper.find('[data-testid="bim-submit"]').trigger('click');

    const payload = wrapper.emitted('import')[0][0];
    expect(payload.rows).toHaveLength(2);
    const fixed = payload.rows.find((r) => r.lastName === 'Петров');
    expect(fixed).toMatchObject({ firstName: 'Пётр', isExisting: false, targetTables: [10] });
  });

  it('незаполненную обязательную часть строки (пустое имя) нельзя отметить галочкой', async () => {
    const wrapper = mountModal({ rows: PEOPLE_ROWS, summary: { read: 2, accepted: 1, rejected: 1 } });
    await flushPromises();

    const checkbox = wrapper.find('[data-testid="bim-include-6"]');
    expect(checkbox.attributes('disabled')).toBeDefined();
  });

  it('машины: номер Т/С обязателен, места разгрузки и проезд применяются к payload', async () => {
    const wrapper = mountModal({
      attachmentType: 'cars',
      rows: [CAR_ROWS[0]],
      summary: { read: 1, accepted: 1, rejected: 0 },
    });
    await flushPromises();

    await wrapper.find('[data-testid="bim-unload-places"] .passage__item').trigger('click');
    await wrapper.find('[data-testid="bim-passage-tables"] .passage__item').trigger('click');
    await wrapper.find('[data-testid="bim-submit"]').trigger('click');

    const payload = wrapper.emitted('import')[0][0];
    expect(payload.attachmentType).toBe('cars');
    expect(payload.rows[0]).toMatchObject({
      plateNumber: 'А001АА777',
      mark: 'Volvo',
      unloadPlaces: [30],
      passage_tables: [20],
      isExisting: false,
    });
  });

  it('закрытие эмитит close', async () => {
    const wrapper = mountModal({ rows: [] });
    await flushPromises();
    await wrapper.find('[data-testid="modal-button-close"]').trigger('click');
    expect(wrapper.emitted('close')).toHaveLength(1);
  });

  it('открытие сбрасывает выбор мест и правки предыдущего показа', async () => {
    const wrapper = mountModal({ rows: PEOPLE_ROWS, summary: { read: 2, accepted: 1, rejected: 1 } });
    await flushPromises();
    await wrapper.findAll('.passage__item')[0].trigger('click');
    expect(wrapper.vm.selectedTargetTables).toEqual([10]);

    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    await flushPromises();

    expect(wrapper.vm.selectedTargetTables).toEqual([]);
  });
});

describe('BlankImportResultModal - обязательность мест по fieldConfig (ревью, блокер 1)', () => {
  beforeEach(() => {
    notifyMock.mockReset();
  });

  it('required:false у visible-поля не блокирует кнопку и не рисует звёздочку - зеркало EmployeeForm/VehicleForm', async () => {
    const wrapper = mountModal({
      rows: [PEOPLE_ROWS[0]],
      summary: { read: 1, accepted: 1, rejected: 0 },
      fieldConfig: { target_tables: { visible: true, required: false } },
    });
    await flushPromises();

    // Видимое необязательное поле - грид всё ещё рисуется (можно выбрать по желанию),
    // но submit не должен требовать выбора: EmployeeForm.vue:491-492/573-574.
    expect(wrapper.find('[data-testid="bim-target-tables"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="bim-target-tables"] .required').exists()).toBe(false);
    expect(wrapper.find('[data-testid="bim-submit"]').attributes('disabled')).toBeUndefined();
  });

  it('required:false для машин (места разгрузки и проезд) не блокирует submit', async () => {
    const wrapper = mountModal({
      attachmentType: 'cars',
      rows: [CAR_ROWS[0]],
      summary: { read: 1, accepted: 1, rejected: 0 },
      fieldConfig: {
        unloading_places: { visible: true, required: false },
        passage_tables: { visible: true, required: false },
      },
    });
    await flushPromises();

    expect(wrapper.find('[data-testid="bim-unload-places"] .required').exists()).toBe(false);
    expect(wrapper.find('[data-testid="bim-passage-tables"] .required').exists()).toBe(false);
    expect(wrapper.find('[data-testid="bim-submit"]').attributes('disabled')).toBeUndefined();
  });
});

describe('BlankImportResultModal - блокирующие причины не обходятся правкой (ревью, блокер 2)', () => {
  const BLACKLIST_ROW = {
    row_number: 7,
    employee: {
      last_name: 'Сидоров', first_name: 'Пётр', middle_name: '',
      citizenship_id: 1, position: 'Инженер',
      passport_series_number: '', patent_number: null, other_permission: null,
      target_tables: [],
    },
    // ФИО у блокирующей строки заведомо непустое - иначе совпадение по ключу
    // ЧС/дубля не случилось бы (fmtErrEmployeeBlacklisted/"Дублирует строку").
    errors: ['Человек Сидоров Пётр в чёрном списке: судимость'],
    warnings: [],
  };
  const DUPLICATE_ROW = {
    row_number: 8,
    employee: {
      last_name: 'Кузнецов', first_name: 'Олег', middle_name: '',
      citizenship_id: 1, position: '',
      passport_series_number: '', patent_number: null, other_permission: null,
      target_tables: [],
    },
    errors: ['Дублирует строку 4: то же ФИО'],
    warnings: [],
  };
  const PATENT_ROW = {
    row_number: 9,
    employee: {
      last_name: 'Рахимов', first_name: 'Азиз', middle_name: '',
      citizenship_id: 2, position: 'Разнорабочий',
      passport_series_number: '', patent_number: null, other_permission: null,
      target_tables: [],
    },
    // Патент/паспорт полей в этой таблице нет и не должно быть (152-ФЗ) - косметическая
    // правка ФИО не должна включать такую строку.
    errors: ['Для гражданства "Узбекистан" нужен номер патента или иное разрешение на работы'],
    warnings: [],
  };

  it('строку с блокировкой по чёрному списку нельзя отметить даже после правки ФИО', async () => {
    const wrapper = mountModal({ rows: [BLACKLIST_ROW], summary: { read: 1, accepted: 0, rejected: 1 } });
    await flushPromises();

    const row = wrapper.find('[data-testid="bim-problem-row-7"]');
    const inputs = row.findAll('input.bim__cell-input');
    await inputs[0].setValue('Другая Фамилия');
    await inputs[1].setValue('Другое Имя');

    expect(wrapper.find('[data-testid="bim-include-7"]').attributes('disabled')).toBeDefined();
  });

  it('строку-дубль внутри файла нельзя отметить даже после правки ФИО', async () => {
    const wrapper = mountModal({ rows: [DUPLICATE_ROW], summary: { read: 1, accepted: 0, rejected: 1 } });
    await flushPromises();

    const row = wrapper.find('[data-testid="bim-problem-row-8"]');
    const inputs = row.findAll('input.bim__cell-input');
    await inputs[0].setValue('Другая Фамилия');
    await inputs[1].setValue('Другое Имя');

    expect(wrapper.find('[data-testid="bim-include-8"]').attributes('disabled')).toBeDefined();
  });

  it('строку с ошибкой про патент нельзя отметить косметической правкой ФИО - полей патента здесь нет', async () => {
    const wrapper = mountModal({ rows: [PATENT_ROW], summary: { read: 1, accepted: 0, rejected: 1 } });
    await flushPromises();

    const row = wrapper.find('[data-testid="bim-problem-row-9"]');
    const inputs = row.findAll('input.bim__cell-input');
    await inputs[0].setValue('Рахимов-испр');
    await inputs[1].setValue('Азиз-испр');

    expect(wrapper.find('[data-testid="bim-include-9"]').attributes('disabled')).toBeDefined();
  });

  it('реально исправимая строка (пустое имя) по-прежнему становится добавляемой', async () => {
    const wrapper = mountModal({ rows: PEOPLE_ROWS, summary: { read: 2, accepted: 1, rejected: 1 } });
    await flushPromises();

    const row = wrapper.find('[data-testid="bim-problem-row-6"]');
    const inputs = row.findAll('input.bim__cell-input');
    await inputs[0].setValue('Петров');
    await inputs[1].setValue('Пётр');

    expect(wrapper.find('[data-testid="bim-include-6"]').attributes('disabled')).toBeUndefined();
  });
});

describe('BlankImportResultModal - ПДн не выводятся (ревью, замечание 4)', () => {
  const PASSPORT_VALUE = '4009 112233';
  const ROW_WITH_PASSPORT = {
    row_number: 11,
    employee: {
      last_name: '', first_name: 'Мария', middle_name: '',
      citizenship_id: 1, position: 'Бухгалтер',
      passport_series_number: PASSPORT_VALUE, patent_number: null, other_permission: null,
      target_tables: [],
    },
    errors: ['Поле «Фамилия» обязательно для заполнения'],
    warnings: [],
  };

  it('паспорт не встречается в тексте таблицы проблемных строк', async () => {
    const wrapper = mountModal({ rows: [ROW_WITH_PASSPORT], summary: { read: 1, accepted: 0, rejected: 1 } });
    await flushPromises();

    expect(wrapper.text()).not.toContain(PASSPORT_VALUE);
  });

  it('паспорт не попадает в буфер выгрузки списка ошибок', async () => {
    const wrapper = mountModal({ rows: [ROW_WITH_PASSPORT], summary: { read: 1, accepted: 0, rejected: 1 } });
    await flushPromises();

    await wrapper.find('[data-testid="bim-download-errors"]').trigger('click');
    await flushPromises();

    const ExcelJS = (await import('exceljs')).default;
    const worksheet = new ExcelJS.Workbook().addWorksheet();
    const addedRows = worksheet.addRow.mock.calls.map((call) => JSON.stringify(call[0]));
    expect(addedRows.some((r) => r.includes(PASSPORT_VALUE))).toBe(false);
  });
});

describe('BlankImportResultModal - выгрузка списка ошибок', () => {
  beforeEach(() => {
    saveBlobAsMock.mockReset();
  });

  it('клик по "Скачать список ошибок" собирает Excel-книгу и отдаёт её через saveBlobAs', async () => {
    const wrapper = mountModal({ rows: PEOPLE_ROWS, summary: { read: 2, accepted: 1, rejected: 1 } });
    await flushPromises();

    await wrapper.find('[data-testid="bim-download-errors"]').trigger('click');
    await flushPromises();

    expect(saveBlobAsMock).toHaveBeenCalledTimes(1);
    const [blob, filename] = saveBlobAsMock.mock.calls[0];
    expect(blob).toBeInstanceOf(Blob);
    expect(filename).toMatch(/\.xlsx$/);
    expect(notifyMock).not.toHaveBeenCalled();
  });
});
