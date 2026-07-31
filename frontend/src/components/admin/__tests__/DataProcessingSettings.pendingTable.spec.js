import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// Список не подтвердивших стал таблицей с поиском и фильтром по организации.
// Сам список сервер отдаёт урезанным, поэтому искать надо по полному - иначе на
// человека, который в списке есть, интерфейс отвечает «никого не нашлось».

vi.mock('@/api/settings', () => ({ getSettings: vi.fn(), updateSetting: vi.fn() }));
vi.mock('@/api/dataProcessing', () => ({
  getDataProcessingMeta: vi.fn().mockResolvedValue(null),
  uploadDataProcessingDoc: vi.fn(),
  fetchDataProcessingBlob: vi.fn(),
  deleteDataProcessingDoc: vi.fn(),
  downloadDataProcessingDoc: vi.fn(),
}));
vi.mock('@/utils/documentTextExtract', () => ({
  extractDocumentHtml: vi.fn(),
  UnsupportedDocumentError: class extends Error {},
}));

const getPDConsentCollection = vi.fn();
vi.mock('@/api/pdConsent', () => ({
  getPDConsentSettings: vi.fn().mockResolvedValue({ text: '<p>Текст</p>', version: 4, required: true }),
  savePDConsentText: vi.fn(),
  setPDConsentRequired: vi.fn(),
  requirePDConsentAgain: vi.fn(),
  getPDConsentCollection: (...a) => getPDConsentCollection(...a),
}));

import DataProcessingSettings from '../DataProcessingSettings.vue';

const person = (id, full, login, org) => ({
  id, full_name: full, username: login, organization: org,
});

const shown = [
  person(1, 'Иванов Иван', 'ivanov', 'ООО Ромашка'),
  person(2, 'Петров Пётр', 'petrov', 'ООО Ландыш'),
];
const rest = [person(3, 'Сидоров Сидор', 'sidorov', 'ООО Ромашка')];

function collection(over = {}) {
  return {
    active: true,
    version: 4,
    total: 10,
    accepted: 7,
    pending: 3,
    pending_users: shown,
    truncated: true,
    ...over,
  };
}

async function mountSection() {
  const wrapper = shallowMount(DataProcessingSettings, {
    global: { stubs: { TextConstructor: true, RefreshButton: true, BaseDropdown: true } },
  });
  await flushPromises();
  return wrapper;
}

describe('Сбор согласий - таблица не подтвердивших', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getPDConsentCollection.mockReset();
    getPDConsentCollection.mockImplementation((opts) => Promise.resolve(
      opts?.full ? collection({ pending_users: [...shown, ...rest], truncated: false }) : collection(),
    ));
  });

  it('рисует строки таблицы, а не список', async () => {
    const wrapper = await mountSection();

    const rows = wrapper.findAll('.pdc-pending__row');
    expect(rows).toHaveLength(3);
    expect(rows[0].text()).toContain('Иванов Иван');
  });

  it('логин показывает с собачкой', async () => {
    const wrapper = await mountSection();

    // Первая ячейка с этим классом - заголовок колонки, логины идут в строках.
    expect(wrapper.find('.pdc-pending__row .pdc-pending__col--login').text()).toBe('@ivanov');
  });

  it('урезанный список сразу догружает полностью - иначе поиск врёт', async () => {
    const wrapper = await mountSection();

    expect(getPDConsentCollection).toHaveBeenCalledWith({ full: true });
    expect(wrapper.vm.pdcPendingSource).toHaveLength(3);
  });

  it('поиск находит человека, которого не было в показанной части', async () => {
    const wrapper = await mountSection();

    wrapper.vm.pdcPendingQuery = 'сидор';
    await flushPromises();

    expect(wrapper.vm.pdcPendingRows.map((p) => p.username)).toEqual(['sidorov']);
  });

  it('поиск идёт и по логину, и по организации', async () => {
    const wrapper = await mountSection();

    wrapper.vm.pdcPendingQuery = 'petrov';
    await flushPromises();
    expect(wrapper.vm.pdcPendingRows.map((p) => p.id)).toEqual([2]);

    wrapper.vm.pdcPendingQuery = 'Ландыш';
    await flushPromises();
    expect(wrapper.vm.pdcPendingRows.map((p) => p.id)).toEqual([2]);
  });

  it('фильтр по организации оставляет только её работников', async () => {
    const wrapper = await mountSection();

    wrapper.vm.pdcPendingOrg = 'ООО Ромашка';
    await flushPromises();

    expect(wrapper.vm.pdcPendingRows.map((p) => p.username)).toEqual(['ivanov', 'sidorov']);
  });

  it('организации для фильтра собирает из списка без повторов', async () => {
    const wrapper = await mountSection();

    expect(wrapper.vm.pdcPendingOrgOptions.map((o) => o.name))
      .toEqual(['Все организации', 'ООО Ландыш', 'ООО Ромашка']);
  });

  it('пустой результат поиска говорит об этом, а не показывает пустую таблицу', async () => {
    const wrapper = await mountSection();

    wrapper.vm.pdcPendingQuery = 'такого нет';
    await flushPromises();

    expect(wrapper.find('[data-testid="pdc-pending-empty"]').exists()).toBe(true);
  });

  it('когда список пришёл целиком, за полным повторно не ходим', async () => {
    getPDConsentCollection.mockResolvedValue(collection({ truncated: false, pending: 2 }));
    await mountSection();

    expect(getPDConsentCollection).toHaveBeenCalledTimes(1);
  });
});

describe('Сбор согласий - когда полный список не загрузился', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getPDConsentCollection.mockReset();
    getPDConsentCollection.mockImplementation((opts) => (opts?.full
      ? Promise.reject(new Error('сеть'))
      : Promise.resolve(collection())));
  });

  it('честно говорит, что поиск идёт по показанной части', async () => {
    const wrapper = await mountSection();

    expect(wrapper.vm.pdcPendingComplete).toBe(false);
    expect(wrapper.find('[data-testid="pdc-pending-partial"]').text())
      .toContain('Список загружен не полностью');
  });

  it('предупреждение видно и когда поиск ничего не нашёл', async () => {
    const wrapper = await mountSection();

    wrapper.vm.pdcPendingQuery = 'сидор';
    await flushPromises();

    // «Никого не нашлось» без оговорки означало бы, что человека в списке нет.
    expect(wrapper.find('[data-testid="pdc-pending-empty"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="pdc-pending-partial"]').exists()).toBe(true);
  });

  it('следующий поиск пробует догрузить список снова', async () => {
    const wrapper = await mountSection();
    expect(getPDConsentCollection).toHaveBeenCalledTimes(2);

    getPDConsentCollection.mockImplementation((opts) => Promise.resolve(
      opts?.full ? collection({ pending_users: [...shown, ...rest], truncated: false }) : collection(),
    ));
    wrapper.vm.pdcPendingQuery = 'сидор';
    await flushPromises();

    expect(wrapper.vm.pdcPendingComplete).toBe(true);
    expect(wrapper.vm.pdcPendingRows.map((p) => p.username)).toEqual(['sidorov']);
  });

  it('когда список полный, вместо предупреждения показывает число найденных', async () => {
    getPDConsentCollection.mockImplementation((opts) => Promise.resolve(
      opts?.full ? collection({ pending_users: [...shown, ...rest], truncated: false }) : collection(),
    ));
    const wrapper = await mountSection();

    wrapper.vm.pdcPendingQuery = 'ivanov';
    await flushPromises();

    expect(wrapper.find('[data-testid="pdc-pending-partial"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="pdc-pending-found"]').text()).toContain('Найдено 1 из 3');
  });
})
