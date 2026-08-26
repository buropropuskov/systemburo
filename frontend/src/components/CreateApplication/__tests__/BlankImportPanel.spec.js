import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const notifyMock = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: vi.fn(() => ({ notify: notifyMock })),
}));

vi.mock('@/api/citizenships', () => ({
  listCitizenships: vi.fn().mockResolvedValue([]),
}));

import BlankImportPanel from '../BlankImportPanel.vue';
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


// Срез U4: панель занимает место формы ручного ввода. Пока файла нет - крупная область
// загрузки и неброское скачивание пустого бланка; как только сервер разобрал файл, на
// месте области встаёт сводка. Сеть панель не трогает - только отдаёт файл наверх.

const RESULT = {
  rows: [{
    row_number: 2,
    employee: {
      last_name: 'Иванов', first_name: 'Иван', middle_name: '',
      citizenship_id: 1, position: '', passport_series_number: '',
      patent_number: null, other_permission: null, target_tables: [],
    },
    errors: [],
    warnings: [],
  }, {
    // Отклонённая строка приходит в rows вместе с принятыми - счётчик «с ошибками»
    // считает их карточки, а не число из summary (то оставалось снимком разбора и
    // спорило с соседним счётчиком, когда строку чинили).
    row_number: 3,
    employee: {
      last_name: '', first_name: 'Мария', middle_name: '',
      citizenship_id: 1, position: '', passport_series_number: '',
      patent_number: null, other_permission: null, target_tables: [],
    },
    errors: [{ text: 'Поле «Фамилия» обязательно для заполнения', code: 'field_required', field: 'last_name', fixable: true }],
    warnings: [],
  }],
  summary: { read: 7, accepted: 6, rejected: 1 },
};

function mountPanel(props = {}) {
  return mount(BlankImportPanel, {
    props: { attachmentType: 'people', ...props },
  });
}

function dropEvent(file) {
  return { dataTransfer: { files: file ? [file] : [] } };
}

// Согласие субъекта - обязательная отметка на всю пачку (поле pd_consent реестра полей
// вложения). Тесты ниже проверяют места и события, поэтому ставят её как данность;
// поведение самой отметки проверяет BlankImportConsent.spec.js.
async function markConsent(wrapper) {
  const box = wrapper.find('[data-testid="bim-pd-consent-checkbox"]');
  if (box.exists()) await box.setValue(true);
}

describe('BlankImportPanel (U4)', () => {
  beforeEach(() => {
    notifyMock.mockReset();
  });

  it('без разобранного файла показывает область загрузки и скачивание пустого бланка', () => {
    const wrapper = mountPanel();

    expect(wrapper.find('[data-testid="import-dropzone"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="download-blank-template-btn"]').exists()).toBe(true);
    expect(wrapper.findComponent(BlankImportResult).exists()).toBe(false);
  });

  it('выбор файла отдаётся наверх, повторный выбор того же файла не глохнет', async () => {
    const wrapper = mountPanel();
    const input = wrapper.find('[data-testid="import-file-input"]');
    const file = new File(['x'], 'blank.xlsx');

    // Значение инпута в jsdom не подменить - проверяем сам обработчик на форме события,
    // которую даёт браузер, включая сброс value (иначе второй выбор того же файла молчит).
    const target = { files: [file], value: 'C:\\fakepath\\blank.xlsx' };
    wrapper.vm.onFileChange({ target });

    expect(target.value).toBe('');
    expect(wrapper.emitted('file')[0]).toEqual([file]);
    expect(input.exists()).toBe(true);
  });

  it('перетаскивание .xlsx отдаёт файл наверх', async () => {
    const wrapper = mountPanel();
    const file = new File(['x'], 'Бланк заполненный.XLSX');

    await wrapper.find('[data-testid="import-dropzone"]').trigger('drop', dropEvent(file));

    expect(wrapper.emitted('file')[0]).toEqual([file]);
    expect(notifyMock).not.toHaveBeenCalled();
  });

  it('перетаскивание не .xlsx объясняет отказ, а не молчит', async () => {
    const wrapper = mountPanel();

    await wrapper.find('[data-testid="import-dropzone"]').trigger('drop', dropEvent(new File(['x'], 'список.csv')));

    expect(wrapper.emitted('file')).toBeUndefined();
    expect(notifyMock).toHaveBeenCalledWith(expect.objectContaining({
      bold: 'список.csv',
      type: 'error',
    }));
  });

  it('пока файл летит, повторное перетаскивание не уходит вторым запросом', async () => {
    const wrapper = mountPanel({ uploading: true });

    await wrapper.find('[data-testid="import-dropzone"]').trigger('drop', dropEvent(new File(['x'], 'b.xlsx')));

    expect(wrapper.emitted('file')).toBeUndefined();
  });

  // Средний счётчик с U5 приходит от родителя: принятые строки уже лежат в списке
  // предварительными, и удаление одной обязано отразиться здесь же.
  it('после разбора на месте области загрузки стоит сводка с ответом сервера', async () => {
    const wrapper = mountPanel({ result: RESULT, pendingCount: 6 });
    // Карточки строк строятся после справочника гражданств - до промисов их нет,
    // а с ними считается счётчик ошибок.
    await flushPromises();

    expect(wrapper.find('[data-testid="import-dropzone"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="download-blank-template-btn"]').exists()).toBe(false);

    const result = wrapper.findComponent(BlankImportResult);
    expect(result.exists()).toBe(true);
    expect(result.props('summary')).toEqual(RESULT.summary);
    expect(wrapper.findAll('.bim__counter-value').map((n) => n.text())).toEqual(['7', '6', '1']);
  });

  it('разбор и «Добавить в заявку» из сводки проходят наверх двумя разными событиями', async () => {
    const wrapper = mountPanel({
      result: RESULT,
      pendingCount: 1,
      fieldConfig: { target_tables: { visible: false } },
    });
    await flushPromises();

    expect(wrapper.emitted('stage')[0][0].rows[0]).toMatchObject({ lastName: 'Иванов', firstName: 'Иван' });

    await markConsent(wrapper);
    await wrapper.find('[data-testid="bim-submit"]').trigger('click');

    const payload = wrapper.emitted('import')[0][0];
    expect(payload.attachmentType).toBe('people');
    expect(payload.places).toEqual({ targetTables: [], passageTables: '', pdConsent: true });
  });

  it('скачивание пустого бланка и выход из режима просят родителя', async () => {
    const wrapper = mountPanel();

    await wrapper.find('[data-testid="download-blank-template-btn"]').trigger('click');
    await wrapper.find('[data-testid="blank-import-close"]').trigger('click');

    expect(wrapper.emitted('download-blank')).toHaveLength(1);
    expect(wrapper.emitted('close')).toHaveLength(1);
  });

  it('пока файл летит, повторное скачивание бланка заблокировано', () => {
    const wrapper = mountPanel({ downloading: true });

    expect(wrapper.find('[data-testid="download-blank-template-btn"]').attributes('disabled')).toBeDefined();
  });
});
