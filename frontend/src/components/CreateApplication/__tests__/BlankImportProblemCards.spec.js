import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

vi.mock('@/api/citizenships', () => ({
  listCitizenships: vi.fn().mockResolvedValue([{ id: 1, name: 'Россия' }]),
}));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: vi.fn(() => ({ notify: vi.fn() })) }));
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


// Строки с ошибками показываются карточками, а не таблицей: причина отказа - готовая
// фраза, и в узкой колонке она рвалась на три строки, а поля правки жались вплотную.

const FIXABLE_ROW = {
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

const BLOCKED_ROW = {
  row_number: 7,
  employee: {
    last_name: 'Сидоров', first_name: 'Сидор', middle_name: '',
    citizenship_id: 1, position: 'Инженер',
    passport_series_number: '', patent_number: null, other_permission: null,
    target_tables: [],
  },
  errors: [{ text: 'Человек Сидоров Сидор в чёрном списке: решение суда', code: 'blacklisted', fixable: false }],
  warnings: [],
};

function mountPanel(rows) {
  return mount(BlankImportResult, {
    props: {
      attachmentType: 'people',
      hasResult: true,
      summary: { read: rows.length, accepted: 0, rejected: rows.length },
      rows,
      pendingCount: 0,
      allPassageTables: [],
      allUnloadingPlaces: [],
      fieldConfig: {},
    },
  });
}

describe('BlankImportResult - карточки строк с ошибками', () => {
  it('строка рисуется карточкой с подписанными полями, а не ячейками таблицы', async () => {
    const wrapper = mountPanel([FIXABLE_ROW]);
    await flushPromises();

    expect(wrapper.find('.bim__problems table').exists()).toBe(false);

    const card = wrapper.find('[data-testid="bim-problem-row-6"]');
    expect(card.classes()).toContain('bim__problem');
    expect(card.text()).toContain('Строка 6');

    const labels = card.findAll('.bim__field-label').map((n) => n.text());
    expect(labels).toEqual(['Фамилия', 'Имя', 'Отчество', 'Гражданство']);
  });

  it('каждая причина - своя строка блока, а не склейка через точку с запятой', async () => {
    const wrapper = mountPanel([{
      ...FIXABLE_ROW,
      errors: [
        { text: 'Поле «Фамилия» обязательно для заполнения', code: 'field_required', field: 'last_name', fixable: true },
        { text: 'Поле «Имя» обязательно для заполнения', code: 'field_required', field: 'first_name', fixable: true },
      ],
    }]);
    await flushPromises();

    const reasons = wrapper.find('[data-testid="bim-problem-row-6"]').findAll('.bim__reason');
    expect(reasons).toHaveLength(2);
    expect(reasons[0].text()).toBe('Поле «Фамилия» обязательно для заполнения');
    expect(wrapper.text()).not.toContain('заполнения; Поле');
  });

  it('карточка называет, можно ли починить строку здесь', async () => {
    const wrapper = mountPanel([FIXABLE_ROW, BLOCKED_ROW]);
    await flushPromises();

    expect(wrapper.find('[data-testid="bim-problem-row-6"]').text()).toContain('Можно исправить');

    const blocked = wrapper.find('[data-testid="bim-problem-row-7"]');
    expect(blocked.text()).toContain('Только вручную');
    expect(blocked.classes()).toContain('bim__problem--blocked');
    expect(blocked.find('.bim__reason--blocking').exists()).toBe(true);
  });

  it('исправимая, но недозаполненная строка объясняет, чего ждёт - место под подсказку зарезервировано и не переключается из/в разметку', async () => {
    const wrapper = mountPanel([FIXABLE_ROW]);
    await flushPromises();

    const note = () => wrapper.find('.bim__problem-note');
    expect(note().exists()).toBe(true);
    expect(note().classes()).not.toContain('bim__problem-note--hidden');
    expect(note().text()).toContain('Заполните фамилию и имя');

    await wrapper.find('[data-testid="bim-problem-row-6"]').findAll('input.bim__cell-input')[0]
      .setValue('Иванова');

    // Строка стала полной - подсказка гасится классом (видимость), а не исчезает из
    // DOM: иначе высота карточки скачет и у списка карточек то появляется, то
    // пропадает вертикальный скролл (жалоба владельца про дёргающийся скролл).
    expect(note().exists()).toBe(true);
    expect(note().classes()).toContain('bim__problem-note--hidden');
  });

  it('у неисправимой (заблокированной) строки подсказки нет вовсе - решение статичное, резервировать нечего', async () => {
    const wrapper = mountPanel([BLOCKED_ROW]);
    await flushPromises();

    expect(wrapper.find('[data-testid="bim-problem-row-7"] .bim__problem-note').exists()).toBe(false);
  });

  // Раскладку в jsdom не посчитать, поэтому раскладочные контракты карточки стережём
  // чтением самого SFC.
  it('на мобильной ширине поля идут одной колонкой с тач-таргетами 44px', () => {
    const sfc = readFileSync(resolve(__dirname, '../BlankImportResult.vue'), 'utf8');
    const mobile = sfc.slice(sfc.indexOf('@media (max-width: 768px)'));

    expect(mobile).toMatch(/\.bim__fields\s*{[^}]*grid-template-columns:\s*1fr/);
    expect(mobile).toMatch(/\.bim__cell-input\s*{[^}]*min-height:\s*44px/);
    expect(mobile).toMatch(/\.bim__row-add\s*{[^}]*min-height:\s*44px/);
    expect(mobile).toMatch(/\.bim__format :deep\(\.base-dropdown__button\)\s*{[^}]*min-height:\s*44px/);
  });

  it('подсказка резервирует высоту независимо от видимости, список ошибок не сужается скроллбаром', () => {
    const sfc = readFileSync(resolve(__dirname, '../BlankImportResult.vue'), 'utf8');

    expect(sfc).toMatch(/\.bim__problem-note\s*{[^}]*min-height:\s*calc\(/);
    expect(sfc).toMatch(/\.bim__problem-note--hidden\s*{[^}]*visibility:\s*hidden/);
    expect(sfc).toMatch(/\.bim__problems-list\s*{[^}]*scrollbar-gutter:\s*stable/);
  });

  it('сводка счётчиков не переносится на десктопе - одна строка, ровные трети', () => {
    const sfc = readFileSync(resolve(__dirname, '../BlankImportResult.vue'), 'utf8');
    const countersRule = sfc.slice(sfc.indexOf('.bim__counters {'), sfc.indexOf('.bim__counters {') + 400);
    const counterRule = sfc.slice(sfc.indexOf('.bim__counter {'), sfc.indexOf('.bim__counter {') + 300);

    expect(countersRule).toMatch(/flex-wrap:\s*nowrap/);
    expect(counterRule).toMatch(/flex:\s*1 1 0/);
  });
});
