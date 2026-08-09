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

import BlankImportResult from '../BlankImportResult.vue';

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

// Срез U5 развёл два момента: принятые строки уходят наверх событием stage сразу после
// разбора и живут в списке предварительными, а «Добавить» (событие import) раскатывает
// по ним места и приносит вручную исправленные строки. Поэтому счётчик готовых сводка
// берёт от родителя (pendingCount) - здесь моделируем его тем, что родитель и положил бы
// в список: все строки без ошибок.
function mountPanel(props = {}) {
  const stagedRows = (props.rows || []).filter((r) => !(r.errors && r.errors.length));
  return mount(BlankImportResult, {
    props: {
      attachmentType: 'people',
      hasResult: true,
      summary: { read: 2, accepted: 1, rejected: 1 },
      rows: [],
      pendingCount: stagedRows.length,
      allPassageTables: PASSAGE_TABLES,
      allUnloadingPlaces: UNLOAD_PLACES,
      fieldConfig: {},
      ...props,
    },
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
      // Гражданство распозналось, отказ только по ФИО - сервер жалуется ровно на то,
      // что не заполнено, поэтому citizenship_id здесь валидный.
      last_name: '', first_name: '', middle_name: '',
      citizenship_id: 1, position: '',
      passport_series_number: '', patent_number: null, other_permission: null,
      target_tables: [],
    },
    // Причины приходят объектами: текст для человека плюс машинный признак, правится
    // ли причина прямо в таблице разбора (services.ImportRowError). Фронт текст не
    // разбирает, поэтому фикстура копирует форму ответа, а не выдумывает свою.
    errors: [
      { text: 'Поле «Фамилия» обязательно для заполнения', code: 'field_required', field: 'last_name', fixable: true },
      { text: 'Поле «Имя» обязательно для заполнения', code: 'field_required', field: 'first_name', fixable: true },
    ],
    warnings: [],
  },
];

// Гражданство не опознано справочником: правка ФИО такую строку не спасает, пока
// человек не выберет гражданство - иначе она отобьётся уже на подаче.
const PEOPLE_UNKNOWN_CITIZENSHIP_ROW = {
  row_number: 7,
  employee: {
    last_name: 'Сидоров', first_name: 'Сидор', middle_name: '',
    citizenship_id: 0, position: 'Разнорабочий',
    passport_series_number: '', patent_number: null, other_permission: null,
    target_tables: [],
  },
  errors: [{
    text: 'Гражданство "Узбекистн" не найдено в справочнике',
    code: 'citizenship_unknown',
    field: 'citizenship',
    fixable: true,
  }],
  warnings: [],
};

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
    // Текст дословно как его формирует бэк: метка берётся из реестра полей
    // (attachment_fields_registry.go, Label "Номер ТС"), ключ поля и признак
    // исправимости - оттуда же. Своя формулировка во фикстуре означала бы, что тест
    // сверяет фронт сам с собой.
    errors: [{
      text: 'Поле «Номер ТС» обязательно для заполнения',
      code: 'field_required',
      field: 'number',
      fixable: true,
    }],
    warnings: [],
  },
];

describe('BlankImportResult (blank-import D1D2)', () => {
  beforeEach(() => {
    notifyMock.mockReset();
    listCitizenshipsMock.mockReset();
    listCitizenshipsMock.mockResolvedValue(CITIZENSHIPS);
  });

  it('207 разбирается: прочитано и с ошибками из summary, готово - из списка, проблемная строка показана', async () => {
    const wrapper = mountPanel({
      rows: PEOPLE_ROWS,
      summary: { read: 2, accepted: 1, rejected: 1 },
    });
    await flushPromises();

    expect(wrapper.findAll('.bim__counter-value').map((n) => n.text())).toEqual(['2', '1', '1']);
    expect(wrapper.find('[data-testid="bim-problem-row-6"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="bim-problem-row-5"]').exists()).toBe(false);
  });

  // Номер с латиницей и короткая числовая часть чинятся сервером автоматически.
  // Строка при этом принимается, поэтому единственный способ узнать о правке -
  // увидеть предупреждение в сводке.
  it('предупреждения принятых строк показаны отдельным блоком', async () => {
    const fixed = {
      row_number: 8,
      employee: {
        last_name: 'Иванов', first_name: 'Иван', middle_name: '',
        citizenship_id: 1, position: 'Инженер',
        passport_series_number: '', patent_number: null, other_permission: null,
        target_tables: [],
      },
      errors: [],
      warnings: ['Поле «Фамилия»: похожие латинские буквы заменены на русские, "Ивaнов" -> "Иванов"'],
    };
    const wrapper = mountPanel({ rows: [fixed], summary: { read: 1, accepted: 1, rejected: 0 } });
    await flushPromises();

    const block = wrapper.find('.bim__warnings');
    expect(block.exists()).toBe(true);
    expect(block.text()).toContain('Стр. 8');
    expect(block.text()).toContain('заменены на русские');
    expect(wrapper.find('.bim__problems').exists()).toBe(false);
  });

  it('кнопка добавления заблокирована без выбора мест прохода и разблокируется после выбора', async () => {
    const wrapper = mountPanel({ rows: [PEOPLE_ROWS[0]], summary: { read: 1, accepted: 1, rejected: 0 } });
    await flushPromises();

    const submit = wrapper.find('[data-testid="bim-submit"]');
    expect(submit.attributes('disabled')).toBeDefined();

    await wrapper.findAll('.passage__item')[0].trigger('click');
    expect(wrapper.find('[data-testid="bim-submit"]').attributes('disabled')).toBeUndefined();
  });

  it('принятые строки уходят наверх сразу, отклонённые без правки - нет, места приезжают на «Добавить»', async () => {
    const wrapper = mountPanel({ rows: PEOPLE_ROWS, summary: { read: 2, accepted: 1, rejected: 1 } });
    await flushPromises();

    const staged = wrapper.emitted('stage');
    expect(staged).toHaveLength(1);
    expect(staged[0][0].attachmentType).toBe('people');
    expect(staged[0][0].rows).toHaveLength(1);
    expect(staged[0][0].rows[0]).toMatchObject({
      lastName: 'Иванов',
      firstName: 'Иван',
      citizenshipId: 1,
      citizenshipName: 'Россия',
      passportSeriesNumber: '1234 567890',
      isExisting: false,
    });

    await wrapper.findAll('.passage__item')[0].trigger('click');
    await wrapper.find('[data-testid="bim-submit"]').trigger('click');

    const emitted = wrapper.emitted('import');
    expect(emitted).toHaveLength(1);
    expect(emitted[0][0].attachmentType).toBe('people');
    expect(emitted[0][0].places).toMatchObject({ targetTables: [10] });
    // Принятая строка уже в списке - второй раз её не шлём.
    expect(emitted[0][0].rows).toEqual([]);
  });

  it('правка проблемной строки на месте делает её добавляемой и включённой в payload', async () => {
    const wrapper = mountPanel({ rows: PEOPLE_ROWS, summary: { read: 2, accepted: 1, rejected: 1 } });
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
    // Принятая строка уже в списке предварительной, исправленная приходит этим событием.
    expect(payload.rows).toHaveLength(1);
    expect(payload.rows[0]).toMatchObject({
      lastName: 'Петров', firstName: 'Пётр', isExisting: false, targetTables: [10],
    });
  });

  it('незаполненную обязательную часть строки (пустое имя) нельзя отметить галочкой', async () => {
    const wrapper = mountPanel({ rows: PEOPLE_ROWS, summary: { read: 2, accepted: 1, rejected: 1 } });
    await flushPromises();

    const checkbox = wrapper.find('[data-testid="bim-include-6"]');
    expect(checkbox.attributes('disabled')).toBeDefined();
  });

  it('строку с неопознанным гражданством нельзя включить, пока гражданство не выбрано', async () => {
    const wrapper = mountPanel({
      rows: [PEOPLE_ROWS[0], PEOPLE_UNKNOWN_CITIZENSHIP_ROW],
      summary: { read: 2, accepted: 1, rejected: 1 },
    });
    await flushPromises();
    await wrapper.findAll('.passage__item')[0].trigger('click');

    const checkbox = wrapper.find('[data-testid="bim-include-7"]');
    expect(checkbox.attributes('disabled')).toBeDefined();

    const row = wrapper.find('[data-testid="bim-problem-row-7"]');
    await row.find('select.bim__cell-input').setValue(String(CITIZENSHIPS[0].id));

    expect(wrapper.find('[data-testid="bim-include-7"]').attributes('disabled')).toBeUndefined();
  });

  it('машина с пустым номером чинится правкой на месте и попадает в payload', async () => {
    const wrapper = mountPanel({
      attachmentType: 'cars',
      rows: CAR_ROWS,
      summary: { read: 2, accepted: 1, rejected: 1 },
    });
    await flushPromises();
    await wrapper.find('[data-testid="bim-unload-places"] .passage__item').trigger('click');
    await wrapper.find('[data-testid="bim-passage-tables"] .passage__item').trigger('click');

    const problemRow = wrapper.find('[data-testid="bim-problem-row-3"]');
    await problemRow.findAll('input.bim__cell-input')[0].setValue('В777ВВ177');

    const checkbox = wrapper.find('[data-testid="bim-include-3"]');
    expect(checkbox.attributes('disabled')).toBeUndefined();
    await checkbox.setValue(true);

    await wrapper.find('[data-testid="bim-submit"]').trigger('click');

    const payload = wrapper.emitted('import')[0][0];
    expect(payload.rows).toHaveLength(1);
    expect(payload.rows[0].plateNumber).toBe('В777ВВ177');
  });

  it('машины: принятая строка уходит наверх сразу, места разгрузки и проезд - на «Добавить»', async () => {
    const wrapper = mountPanel({
      attachmentType: 'cars',
      rows: [CAR_ROWS[0]],
      summary: { read: 1, accepted: 1, rejected: 0 },
    });
    await flushPromises();

    expect(wrapper.emitted('stage')[0][0].rows[0]).toMatchObject({
      plateNumber: 'А001АА777',
      mark: 'Volvo',
      isExisting: false,
    });

    await wrapper.find('[data-testid="bim-unload-places"] .passage__item').trigger('click');
    await wrapper.find('[data-testid="bim-passage-tables"] .passage__item').trigger('click');
    await wrapper.find('[data-testid="bim-submit"]').trigger('click');

    const payload = wrapper.emitted('import')[0][0];
    expect(payload.attachmentType).toBe('cars');
    expect(payload.places).toEqual({
      unloadPlaces: [30],
      unloadingPlace: 'Склад 1',
      passage_tables: [20],
    });
  });

  it('«Загрузить другой файл» эмитит reset - панель вернётся к области загрузки', async () => {
    const wrapper = mountPanel({ rows: [] });
    await flushPromises();
    await wrapper.find('[data-testid="bim-reset"]').trigger('click');
    expect(wrapper.emitted('reset')).toHaveLength(1);
  });

  // Панель живёт в форме подачи, а не открывается заново: сброс привязан к приходу
  // новых строк, иначе выбор мест и правки прошлого файла перетекут в следующий.
  it('новая загрузка сбрасывает выбор мест и правки предыдущего файла', async () => {
    const wrapper = mountPanel({ rows: PEOPLE_ROWS, summary: { read: 2, accepted: 1, rejected: 1 } });
    await flushPromises();
    await wrapper.findAll('.passage__item')[0].trigger('click');
    expect(wrapper.vm.selectedTargetTables).toEqual([10]);

    await wrapper.setProps({ rows: [PEOPLE_ROWS[0]], summary: { read: 1, accepted: 1, rejected: 0 } });
    await flushPromises();

    expect(wrapper.vm.selectedTargetTables).toEqual([]);
    expect(wrapper.vm.problemRows).toEqual([]);
  });
});

describe('BlankImportResult - обязательность мест по fieldConfig (ревью, блокер 1)', () => {
  beforeEach(() => {
    notifyMock.mockReset();
  });

  it('required:false у visible-поля не блокирует кнопку и не рисует звёздочку - зеркало EmployeeForm/VehicleForm', async () => {
    const wrapper = mountPanel({
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
    const wrapper = mountPanel({
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

describe('BlankImportResult - блокирующие причины не обходятся правкой (ревью, блокер 2)', () => {
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
    errors: [{ text: 'Человек Сидоров Пётр в чёрном списке: судимость', code: 'blacklisted', fixable: false }],
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
    errors: [{ text: 'Дублирует строку 4: то же ФИО', code: 'duplicate_in_file', fixable: false }],
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
    errors: [{
      text: 'Для гражданства "Узбекистан" нужен номер патента или иное разрешение на работы',
      code: 'patent_required',
      field: 'patent',
      fixable: false,
    }],
    warnings: [],
  };

  it('строку с блокировкой по чёрному списку нельзя отметить даже после правки ФИО', async () => {
    const wrapper = mountPanel({ rows: [BLACKLIST_ROW], summary: { read: 1, accepted: 0, rejected: 1 } });
    await flushPromises();

    const row = wrapper.find('[data-testid="bim-problem-row-7"]');
    const inputs = row.findAll('input.bim__cell-input');
    await inputs[0].setValue('Другая Фамилия');
    await inputs[1].setValue('Другое Имя');

    expect(wrapper.find('[data-testid="bim-include-7"]').attributes('disabled')).toBeDefined();
  });

  it('строку-дубль внутри файла нельзя отметить даже после правки ФИО', async () => {
    const wrapper = mountPanel({ rows: [DUPLICATE_ROW], summary: { read: 1, accepted: 0, rejected: 1 } });
    await flushPromises();

    const row = wrapper.find('[data-testid="bim-problem-row-8"]');
    const inputs = row.findAll('input.bim__cell-input');
    await inputs[0].setValue('Другая Фамилия');
    await inputs[1].setValue('Другое Имя');

    expect(wrapper.find('[data-testid="bim-include-8"]').attributes('disabled')).toBeDefined();
  });

  it('строку с ошибкой про патент нельзя отметить косметической правкой ФИО - полей патента здесь нет', async () => {
    const wrapper = mountPanel({ rows: [PATENT_ROW], summary: { read: 1, accepted: 0, rejected: 1 } });
    await flushPromises();

    const row = wrapper.find('[data-testid="bim-problem-row-9"]');
    const inputs = row.findAll('input.bim__cell-input');
    await inputs[0].setValue('Рахимов-испр');
    await inputs[1].setValue('Азиз-испр');

    expect(wrapper.find('[data-testid="bim-include-9"]').attributes('disabled')).toBeDefined();
  });

  it('реально исправимая строка (пустое имя) по-прежнему становится добавляемой', async () => {
    const wrapper = mountPanel({ rows: PEOPLE_ROWS, summary: { read: 2, accepted: 1, rejected: 1 } });
    await flushPromises();

    const row = wrapper.find('[data-testid="bim-problem-row-6"]');
    const inputs = row.findAll('input.bim__cell-input');
    await inputs[0].setValue('Петров');
    await inputs[1].setValue('Пётр');

    expect(wrapper.find('[data-testid="bim-include-6"]').attributes('disabled')).toBeUndefined();
  });
});

describe('BlankImportResult - ПДн не выводятся (ревью, замечание 4)', () => {
  const PASSPORT_VALUE = '4009 112233';
  const ROW_WITH_PASSPORT = {
    row_number: 11,
    employee: {
      last_name: '', first_name: 'Мария', middle_name: '',
      citizenship_id: 1, position: 'Бухгалтер',
      passport_series_number: PASSPORT_VALUE, patent_number: null, other_permission: null,
      target_tables: [],
    },
    errors: [{
      text: 'Поле «Фамилия» обязательно для заполнения',
      code: 'field_required',
      field: 'last_name',
      fixable: true,
    }],
    warnings: [],
  };

  it('паспорт не встречается в тексте таблицы проблемных строк', async () => {
    const wrapper = mountPanel({ rows: [ROW_WITH_PASSPORT], summary: { read: 1, accepted: 0, rejected: 1 } });
    await flushPromises();

    expect(wrapper.text()).not.toContain(PASSPORT_VALUE);
  });

  it('паспорт не попадает в буфер выгрузки списка ошибок', async () => {
    const wrapper = mountPanel({ rows: [ROW_WITH_PASSPORT], summary: { read: 1, accepted: 0, rejected: 1 } });
    await flushPromises();

    await wrapper.find('[data-testid="bim-download-errors"]').trigger('click');
    await flushPromises();

    const ExcelJS = (await import('exceljs')).default;
    const worksheet = new ExcelJS.Workbook().addWorksheet();
    const addedRows = worksheet.addRow.mock.calls.map((call) => JSON.stringify(call[0]));
    expect(addedRows.some((r) => r.includes(PASSPORT_VALUE))).toBe(false);
  });
});

describe('BlankImportResult - выгрузка списка ошибок', () => {
  beforeEach(() => {
    saveBlobAsMock.mockReset();
  });

  it('клик по "Скачать список ошибок" собирает Excel-книгу и отдаёт её через saveBlobAs', async () => {
    const wrapper = mountPanel({ rows: PEOPLE_ROWS, summary: { read: 2, accepted: 1, rejected: 1 } });
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
