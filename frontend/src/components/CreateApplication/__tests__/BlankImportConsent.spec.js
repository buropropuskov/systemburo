import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const listCitizenshipsMock = vi.fn();
vi.mock('@/api/citizenships', () => ({
  listCitizenships: (...args) => listCitizenshipsMock(...args),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: vi.fn(() => ({ notify: vi.fn() })),
}));

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

import BlankImportResult from '../BlankImportResult.vue';

// Согласия субъекта в бланке нет и не будет: колонки под него в файле не заводили. Поэтому
// отметка стоит в сводке разбора один раз на весь список - тем же порядком, что и места
// прохода, - и уезжает к строкам общим патчем. До этого работники, загруженные списком,
// попадали в заявку без отметки: серверный гейт настраивается тумблером и по умолчанию
// молчит, так что подача проходила, а следа согласия в базе не оставалось.

const PASSAGE_TABLES = [
  { table: { id: 10, name: 'people-1', display_name: 'Проход 1', table_type: 'people', status: 'active' } },
];

const PEOPLE_ROW = {
  row_number: 4,
  employee: {
    last_name: 'Иванов', first_name: 'Иван', middle_name: '',
    citizenship_id: 1, position: 'Монтажник', passport_series_number: '1234 567890',
  },
  errors: [],
  warnings: [],
};

function mountResult(props = {}) {
  return mount(BlankImportResult, {
    props: {
      attachmentType: 'people',
      hasResult: true,
      summary: { read: 1, accepted: 1, rejected: 0 },
      rows: [PEOPLE_ROW],
      pendingCount: 1,
      allPassageTables: PASSAGE_TABLES,
      allUnloadingPlaces: [],
      fieldConfig: {},
      ...props,
    },
    global: { stubs: { teleport: true } },
  });
}

const submitDisabled = (w) => w.find('[data-testid="bim-submit"]').attributes('disabled') !== undefined;

beforeEach(() => {
  listCitizenshipsMock.mockReset();
  listCitizenshipsMock.mockResolvedValue([{ id: 1, name: 'Россия' }]);
});

describe('BlankImportResult - согласие субъекта на весь список', () => {
  it('без отметки кнопка заблокирована, причина названа словами', async () => {
    const w = mountResult();
    await flushPromises();
    await w.findAll('.passage__item')[0].trigger('click');

    expect(submitDisabled(w)).toBe(true);
    expect(w.find('[data-testid="bim-submit-hint"]').attributes('data-hint'))
      .toContain('уведомлены об обработке персональных данных');
  });

  it('отметка включает кнопку и уходит к строкам общим патчем', async () => {
    const w = mountResult();
    await flushPromises();
    await w.findAll('.passage__item')[0].trigger('click');
    await w.find('[data-testid="bim-pd-consent-checkbox"]').setValue(true);

    expect(submitDisabled(w)).toBe(false);

    await w.find('[data-testid="bim-submit"]').trigger('click');
    const payload = w.emitted('import')[0][0];
    expect(payload.places.pdConsent).toBe(true);
    expect(payload.places.targetTables).toEqual([10]);
  });

  it('снятая обязательность не блокирует, но отметку по-прежнему можно поставить', async () => {
    const w = mountResult({ fieldConfig: { pd_consent: { visible: true, required: false } } });
    await flushPromises();
    await w.findAll('.passage__item')[0].trigger('click');

    expect(w.find('[data-testid="bim-pd-consent"]').exists()).toBe(true);
    expect(w.find('[data-testid="bim-pd-consent"] .required').exists()).toBe(false);
    expect(submitDisabled(w)).toBe(false);

    await w.find('[data-testid="bim-pd-consent-checkbox"]').setValue(true);
    await w.find('[data-testid="bim-submit"]').trigger('click');
    expect(w.emitted('import')[0][0].places.pdConsent).toBe(true);
  });

  it('поле выключено в шаблоне - отметки нет и патч несёт снятое согласие', async () => {
    const w = mountResult({ fieldConfig: { pd_consent: { visible: false, required: false } } });
    await flushPromises();
    await w.findAll('.passage__item')[0].trigger('click');

    expect(w.find('[data-testid="bim-pd-consent"]').exists()).toBe(false);
    expect(submitDisabled(w)).toBe(false);

    await w.find('[data-testid="bim-submit"]').trigger('click');
    expect(w.emitted('import')[0][0].places.pdConsent).toBe(false);
  });

  it('новый файл сбрасывает отметку: каждая пачка подтверждается заново', async () => {
    const w = mountResult();
    await flushPromises();
    await w.find('[data-testid="bim-pd-consent-checkbox"]').setValue(true);
    expect(w.vm.pdConsent).toBe(true);

    await w.setProps({ rows: [{ ...PEOPLE_ROW, row_number: 9 }] });
    await flushPromises();
    expect(w.vm.pdConsent).toBe(false);
  });

  it('у бланка машин подпись говорит про владельцев машин', async () => {
    const w = mountResult({
      attachmentType: 'cars',
      rows: [{ row_number: 2, vehicle: { car_number: 'А001АА777', car_brand: 'Volvo' }, errors: [], warnings: [] }],
      fieldConfig: { pd_consent: { visible: true, required: true } },
    });
    await flushPromises();

    expect(w.find('[data-testid="bim-pd-consent"]').text()).toContain('Владельцы машин из списка');
  });
});
