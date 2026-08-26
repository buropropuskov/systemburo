import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const listCitizenshipsMock = vi.fn();
vi.mock('@/api/citizenships', () => ({
  listCitizenships: (...args) => listCitizenshipsMock(...args),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: vi.fn(() => ({ notify: vi.fn() })),
}));

import BlankImportResult from '../BlankImportResult.vue';
import BlankImportPanel from '../BlankImportPanel.vue';

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


// Срез U5: сводка перестала быть точкой, где строки создаются. Разбор она отдаёт наверх
// сразу (событие stage), а «Добавить» только раскатывает выбранные места и снимает
// предварительность - поэтому счётчик «готово к добавлению» считает живые строки списка
// (родитель отдаёт их числом), а не то, что насчитал сервер при разборе.

const PASSAGE_TABLES = [
  { table: { id: 10, name: 'people-1', display_name: 'Проход 1', table_type: 'people', status: 'active' } },
  { table: { id: 20, name: 'kpp-1', display_name: 'КПП 1', table_type: 'cars', status: 'active' } },
];
const UNLOAD_PLACES = [{ id: 30, name: 'Склад 1', status: 'active' }];

const CAR_ROW = {
  row_number: 3,
  vehicle: { car_number: 'А001АА777', car_brand: 'Volvo' },
  errors: [],
  warnings: [],
};

// Проезд прячем настройкой шаблона, чтобы обязательным осталось одно поле - тест про
// гейт «Добавить», а не про перебор всех мест сразу.
const CARS_FIELD_CONFIG = {
  passage_tables: { visible: false, required: false },
};

function mountResult(props = {}) {
  return mount(BlankImportResult, {
    props: {
      attachmentType: 'cars',
      hasResult: true,
      summary: { read: 1, accepted: 1, rejected: 0 },
      rows: [CAR_ROW],
      pendingCount: 0,
      allPassageTables: PASSAGE_TABLES,
      allUnloadingPlaces: UNLOAD_PLACES,
      fieldConfig: CARS_FIELD_CONFIG,
      ...props,
    },
  });
}

beforeEach(() => {
  listCitizenshipsMock.mockReset();
  listCitizenshipsMock.mockResolvedValue([{ id: 1, name: 'Россия' }]);
});

// Согласие субъекта - обязательная отметка на всю пачку (поле pd_consent реестра полей
// вложения). Тесты ниже проверяют места и события, поэтому ставят её как данность;
// поведение самой отметки проверяет BlankImportConsent.spec.js.
async function markConsent(wrapper) {
  const box = wrapper.find('[data-testid="bim-pd-consent-checkbox"]');
  if (box.exists()) await box.setValue(true);
}

describe('BlankImportResult - предварительные строки (U5)', () => {
  it('разобранные строки уходят наверх сразу, не дожидаясь «Добавить»', async () => {
    const w = mountResult();
    await flushPromises();

    const staged = w.emitted('stage');
    expect(staged).toHaveLength(1);
    expect(staged[0][0].attachmentType).toBe('cars');
    expect(staged[0][0].rows).toHaveLength(1);
    expect(staged[0][0].rows[0]).toMatchObject({ plateNumber: 'А001АА777', mark: 'Volvo' });
    // Места ещё не выбраны - они приезжают к строкам на «Добавить».
    expect(staged[0][0].rows[0].unloadPlaces).toEqual([]);
  });

  it('счётчик и подпись кнопки считают живые предварительные строки, а не разбор', async () => {
    const w = mountResult({ summary: { read: 9, accepted: 9, rejected: 0 }, pendingCount: 4 });
    await flushPromises();

    expect(w.get('[data-testid="bim-pending-count"]').text()).toBe('4');
    expect(w.get('[data-testid="bim-submit"]').text()).toContain('(4)');
  });

  it('«Добавить» недоступна без обязательных мест и включается после выбора', async () => {
    const w = mountResult({ pendingCount: 1 });
    await flushPromises();

    expect(w.get('[data-testid="bim-submit"]').attributes('disabled')).toBeDefined();

    await markConsent(w);
    await w.setData({ selectedUnloadPlaces: [30] });
    expect(w.get('[data-testid="bim-submit"]').attributes('disabled')).toBeUndefined();

    await w.get('[data-testid="bim-submit"]').trigger('click');

    const committed = w.emitted('import');
    expect(committed).toHaveLength(1);
    expect(committed[0][0].places).toEqual({
      unloadPlaces: [30],
      unloadingPlace: 'Склад 1',
      passage_tables: [],
      // Отметка согласия едет тем же патчем: у строк из файла своей галочки нет.
      pdConsent: true,
    });
    // Принятые строки уже в списке предварительными - второй раз их не шлём.
    expect(committed[0][0].rows).toEqual([]);
  });

  it('без предварительных строк и без исправленных «Добавить» недоступна', async () => {
    const w = mountResult({ rows: [], summary: { read: 0, accepted: 0, rejected: 0 }, pendingCount: 0 });
    await flushPromises();

    await w.setData({ selectedUnloadPlaces: [30] });
    expect(w.get('[data-testid="bim-submit"]').attributes('disabled')).toBeDefined();
  });
});

describe('BlankImportPanel - возврат к сводке по предварительным строкам (U5)', () => {
  function mountPanel(props = {}) {
    return mount(BlankImportPanel, {
      props: {
        attachmentType: 'cars',
        result: null,
        pendingCount: 0,
        allPassageTables: PASSAGE_TABLES,
        allUnloadingPlaces: UNLOAD_PLACES,
        fieldConfig: CARS_FIELD_CONFIG,
        ...props,
      },
    });
  }

  it('без разбора и без предварительных строк показывает область загрузки', async () => {
    const w = mountPanel();
    await flushPromises();

    expect(w.find('[data-testid="import-dropzone"]').exists()).toBe(true);
    expect(w.find('[data-testid="blank-import-result"]').exists()).toBe(false);
  });

  // Перезагрузка страницы разбор не переживает, а предварительные строки переживают:
  // без этой ветки их нечем было бы перевести в обычные.
  it('предварительные строки без свежего разбора всё равно открывают сводку', async () => {
    const w = mountPanel({ pendingCount: 2 });
    await flushPromises();

    expect(w.find('[data-testid="blank-import-result"]').exists()).toBe(true);
    expect(w.get('[data-testid="bim-pending-count"]').text()).toBe('2');
    expect(w.find('[data-testid="import-dropzone"]').exists()).toBe(false);
  });

  it('«Загрузить другой файл» открывает загрузку, а «К сводке» возвращает обратно', async () => {
    const w = mountPanel({ pendingCount: 2 });
    await flushPromises();

    await w.get('[data-testid="bim-reset"]').trigger('click');
    expect(w.emitted('reset')).toHaveLength(1);
    expect(w.find('[data-testid="import-dropzone"]').exists()).toBe(true);

    await w.get('[data-testid="import-back-to-summary"]').trigger('click');
    expect(w.find('[data-testid="blank-import-result"]').exists()).toBe(true);
  });
});
