import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

vi.mock('@/api/citizenships', () => ({
  listCitizenships: vi.fn().mockResolvedValue([{ id: 1, name: 'Россия' }]),
}));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: vi.fn(() => ({ notify: vi.fn() })) }));
vi.mock('@/api/attachment-templates', () => ({ saveBlobAs: vi.fn() }));
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(async () => ({ ok: true, json: async () => [] })),
}));

import BlankImportResult from '../BlankImportResult.vue';

// Кнопка «Добавить в заявку» блокируется невыбранными местами, а НЕ строками с
// ошибками - те живут своими карточками и общей кнопке не мешают. Владелец прочитал
// серую кнопку рядом со счётчиком «с ошибками» как «пока не исправишь - не пущу»,
// поэтому причина теперь написана словами.

const PASSAGE_TABLES = [
  { table: { id: 10, name: 'people-1', display_name: 'Проход 1', table_type: 'people', status: 'active' } },
  { table: { id: 20, name: 'kpp-1', display_name: 'КПП 1', table_type: 'cars', status: 'active' } },
];
const UNLOAD_PLACES = [{ id: 30, name: 'Склад 1', status: 'active' }];

const BAD_ROW = {
  row_number: 4,
  vehicle: { car_number: 'Писька', car_brand: 'Kamaz' },
  errors: [{ text: 'Номер Т/С "Писька" не соответствует ни одному формату номеров', code: 'plate_format', field: 'number', fixable: true }],
  warnings: [],
};
const GOOD_ROW = {
  row_number: 5,
  vehicle: { car_number: 'А 123 ВС 777', car_brand: 'Volvo' },
  errors: [],
  warnings: [],
};

function mountPanel(props = {}) {
  return mount(BlankImportResult, {
    props: {
      attachmentType: 'cars',
      hasResult: true,
      summary: { read: 2, accepted: 1, rejected: 1 },
      rows: [BAD_ROW, GOOD_ROW],
      // Родитель уже положил принятую строку в список предварительной.
      pendingCount: 1,
      allPassageTables: PASSAGE_TABLES,
      allUnloadingPlaces: UNLOAD_PLACES,
      fieldConfig: {},
      ...props,
    },
    global: { stubs: { teleport: true } },
  });
}

const hint = (wrapper) => wrapper.find('[data-testid="bim-submit-hint"]').attributes('data-hint');
const submitDisabled = (wrapper) => wrapper.find('[data-testid="bim-submit"]').attributes('disabled') !== undefined;

// Место выбирается кликом по плитке грида - тем же путём, что у человека: правка
// wrapper.vm.selected* прошла бы мимо v-model и ничего бы не доказала.
async function pickPlace(wrapper, testid) {
  await wrapper.find(`[data-testid="${testid}"] .passage__item`).trigger('click');
  await flushPromises();
}

// Согласие субъекта - обязательная отметка на всю пачку (поле pd_consent реестра полей
// вложения). Тесты ниже проверяют места и события, поэтому ставят её как данность;
// поведение самой отметки проверяет BlankImportConsent.spec.js.
async function markConsent(wrapper) {
  const box = wrapper.find('[data-testid="bim-pd-consent-checkbox"]');
  if (box.exists()) await box.setValue(true);
}

describe('BlankImportResult - почему «Добавить в заявку» заблокирована', () => {
  it('подсказка висит на обёртке, а не на самой кнопке - disabled событий мыши не получает', async () => {
    const wrapper = mountPanel();
    await flushPromises();

    const anchor = wrapper.find('[data-testid="bim-submit-hint"]');
    expect(anchor.classes()).toContain('hint-anchor');
    expect(anchor.find('[data-testid="bim-submit"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="bim-submit"]').attributes('data-hint')).toBeUndefined();
  });

  it('места не выбраны - подсказка называет оба недостающих поля, а не ошибки строк', async () => {
    const wrapper = mountPanel();
    await flushPromises();

    expect(submitDisabled(wrapper)).toBe(true);
    expect(hint(wrapper)).toBe('Выберите места разгрузки и проезд, чтобы добавить строки в заявку.');
    // Строка с ошибкой на экране есть, но в тексте про неё ни слова - она не блокирует.
    expect(wrapper.find('[data-testid="bim-problem-row-4"]').exists()).toBe(true);
  });

  it('выбрано одно поле - в подсказке остаётся только второе', async () => {
    const wrapper = mountPanel();
    await flushPromises();

    await pickPlace(wrapper, 'bim-unload-places');

    expect(hint(wrapper)).toBe('Выберите проезд, чтобы добавить строки в заявку.');
  });

  it('места выбраны - кнопка работает, подсказка гаснет', async () => {
    const wrapper = mountPanel();
    await flushPromises();

    await markConsent(wrapper);
    await pickPlace(wrapper, 'bim-unload-places');
    await pickPlace(wrapper, 'bim-passage-tables');

    expect(submitDisabled(wrapper)).toBe(false);
    // Пустой data-hint гасит пузырёк целиком (hints.css) - на рабочей кнопке молчит.
    expect(hint(wrapper)).toBe('');
  });

  it('готовых строк нет - подсказка про строки, а не про места', async () => {
    const wrapper = mountPanel({ pendingCount: 0, rows: [BAD_ROW], summary: { read: 1, accepted: 0, rejected: 1 } });
    await flushPromises();

    expect(hint(wrapper)).toBe('Готовых строк пока нет: поправьте строки с ошибками или загрузите другой файл.');
  });

  it('в справочнике нет мест вовсе - подсказка не зовёт выбирать несуществующее', async () => {
    const wrapper = mountPanel({ allUnloadingPlaces: [], allPassageTables: [] });
    await flushPromises();

    expect(hint(wrapper)).toBe('Не из чего выбрать: места разгрузки и проезд. Обратитесь в бюро пропусков.');
  });

  it('для людей подсказка называет места прохода', async () => {
    const wrapper = mountPanel({
      attachmentType: 'people',
      rows: [{
        row_number: 5,
        employee: {
          last_name: 'Иванов', first_name: 'Иван', middle_name: '', citizenship_id: 1, position: 'Инженер',
        },
        errors: [],
        warnings: [],
      }],
      summary: { read: 1, accepted: 1, rejected: 0 },
    });
    await flushPromises();

    expect(hint(wrapper)).toBe('Выберите места прохода, чтобы добавить строки в заявку.');
  });
});
