import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
const apiRequestRaw = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
  apiRequestRaw: (...args) => apiRequestRaw(...args),
}));

const buildPassageJournalBlob = vi.fn(async () => new Blob(['x']));
const saveJournalBlob = vi.fn();
vi.mock('@/utils/passageJournalExport', () => ({
  buildPassageJournalBlob: (...args) => buildPassageJournalBlob(...args),
  saveJournalBlob: (...args) => saveJournalBlob(...args),
}));

const notify = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}));


// collectPassageRows подменяем только там, где проверяется обрезка: честная сборка 20
// тысяч строк упирается в таймаут теста, а сам предел и признак обрезки уже стережёт
// passageJournal.spec.js на маленьком лимите.
const collectPassageRowsMock = vi.fn();
vi.mock('@/utils/passageJournal', async (importOriginal) => {
  const actual = await importOriginal();
  return { ...actual, collectPassageRows: (...args) => collectPassageRowsMock(...args) };
});

import CarsTableHistoryModal from '../CarsTableHistoryModal.vue';

/** Отметки журнала: ровно то, что отдаёт сервер страницей. */
function marks(from, count) {
  return Array.from({ length: count }, (_, i) => ({
    id: from + i,
    car_id: 1,
    user_id: 3,
    user_name: 'Иванов И.И.',
    action_type: i % 2 === 0 ? 'entry' : 'exit',
    created_at: `2026-09-1${(i % 9) + 1}T10:00:00Z`,
    car_number: 'A001AA777',
    car_brand: 'Volvo',
    table_name: 'КПП-1',
  }));
}

function pageResponse(items, total) {
  return { ok: true, json: async () => ({ success: true, data: items, meta: { total, page: 1, per_page: 50 } }) };
}

function paramsOf(call) {
  return new URLSearchParams(call[0].split('?')[1] || '');
}

function mountModal(props = {}) {
  return mount(CarsTableHistoryModal, {
    props: { cars: [{ id: 1, car_number: 'A001AA777', car_brand: 'Volvo' }], tableId: 7, ...props },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });
}

describe('CarsTableHistoryModal - страницы и серверные фильтры (#2469)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockImplementation(() => Promise.resolve({ ok: true, json: async () => ({ users: [{ id: 3, name: 'Иванов И.И.' }] }) }));
    apiRequestRaw.mockReset();
    apiRequestRaw.mockImplementation(() => Promise.resolve(pageResponse(marks(1, 2), 5)));
    collectPassageRowsMock.mockReset();
    collectPassageRowsMock.mockImplementation(async (path, filters, options) => {
      const { fetchPassagePage } = await import('@/utils/passageJournal');
      const page = await fetchPassagePage(path, filters, { ...options, page: 1, perPage: 200 });
      return { rows: page.items, total: page.total, truncated: page.total > page.items.length };
    });
    buildPassageJournalBlob.mockClear();
    saveJournalBlob.mockClear();
    notify.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('открывается первой страницей и говорит, сколько нашлось всего', async () => {
    const wrapper = mountModal();
    await flushPromises();

    const params = paramsOf(apiRequestRaw.mock.calls[0]);
    expect(params.get('page')).toBe('1');
    expect(params.get('per_page')).toBe('50');
    expect(wrapper.findAll('.history-item')).toHaveLength(2);
    expect(wrapper.text()).toContain('Показано 2 из 5');
  });

  // Подгрузка дописывает страницу, а не подменяет список: иначе кнопка «Показать ещё»
  // выглядела бы как перелистывание и теряла уже прочитанное.
  it('подгрузка дописывает следующую страницу', async () => {
    const wrapper = mountModal();
    await flushPromises();

    apiRequestRaw.mockImplementation(() => Promise.resolve(pageResponse(marks(3, 2), 5)));
    await wrapper.find('.load-more-btn').trigger('click');
    await flushPromises();

    expect(paramsOf(apiRequestRaw.mock.calls[1]).get('page')).toBe('2');
    expect(wrapper.findAll('.history-item')).toHaveLength(4);
    expect(wrapper.text()).toContain('Показано 4 из 5');
  });

  it('выбор пользователя уходит в запрос и начинает список заново', async () => {
    const wrapper = mountModal();
    await flushPromises();
    await wrapper.find('.load-more-btn').trigger('click');
    await flushPromises();

    await wrapper.find('.user-filter .custom-select').trigger('click');
    const userOptions = wrapper.findAll('.user-filter .select-option');
    expect(userOptions).toHaveLength(2, 'вариант «Все пользователи» и один отметивший');
    await userOptions[1].trigger('click');
    await flushPromises();

    const last = paramsOf(apiRequestRaw.mock.calls.at(-1));
    expect(last.get('user_id')).toBe('3');
    expect(last.get('page')).toBe('1');
    expect(wrapper.findAll('.history-item')).toHaveLength(2);
  });

  it('поиск ждёт паузы в наборе и уходит одним запросом', async () => {
    vi.useFakeTimers();
    const wrapper = mountModal();
    await flushPromises();
    const before = apiRequestRaw.mock.calls.length;

    const input = wrapper.find('.search-input');
    await input.setValue('A00');
    await input.trigger('input');
    await input.setValue('A001AA');
    await input.trigger('input');
    expect(apiRequestRaw.mock.calls.length).toBe(before);

    vi.advanceTimersByTime(300);
    await flushPromises();

    expect(apiRequestRaw.mock.calls.length).toBe(before + 1);
    expect(paramsOf(apiRequestRaw.mock.calls.at(-1)).get('search')).toBe('A001AA');
  });

  // Ответ отставшего запроса не должен затирать свежий: человек успевает дописать
  // символ в поиске, и без проверки номера список показывал бы прошлый результат.
  it('ответ устаревшего запроса отбрасывается', async () => {
    const wrapper = mountModal();
    await flushPromises();

    let releaseStale;
    apiRequestRaw.mockImplementationOnce(() => new Promise((resolve) => {
      releaseStale = () => resolve(pageResponse(marks(100, 1), 1));
    }));
    wrapper.vm.applyFilters({ immediate: true });

    apiRequestRaw.mockImplementationOnce(() => Promise.resolve(pageResponse(marks(200, 3), 3)));
    wrapper.vm.applyFilters({ immediate: true });
    await flushPromises();

    releaseStale();
    await flushPromises();

    expect(wrapper.findAll('.history-item')).toHaveLength(3);
    expect(wrapper.text()).toContain('Показано 3 из 3');
  });

  it('выгрузка собирает журнал по фильтру, а не показанную страницу', async () => {
    const wrapper = mountModal();
    await flushPromises();

    apiRequestRaw.mockImplementation(() => Promise.resolve(pageResponse(marks(1, 5), 5)));
    await wrapper.find('.export-btn').trigger('click');
    await flushPromises();

    const exportCall = apiRequestRaw.mock.calls.at(-1);
    expect(paramsOf(exportCall).get('per_page')).toBe('200');
    expect(buildPassageJournalBlob).toHaveBeenCalledTimes(1);
    const spec = buildPassageJournalBlob.mock.calls[0][0];
    expect(spec.rows).toHaveLength(5);
    expect(spec.note).toBe('');
    expect(saveJournalBlob).toHaveBeenCalledTimes(1);
  });

  // Упёрлись в предел выгрузки - об этом должно быть сказано и в файле, и человеку:
  // молча обрезанный файл читается как полный журнал за период.
  it('обрезанная выгрузка помечается в файле и тостом', async () => {
    const wrapper = mountModal();
    await flushPromises();

    collectPassageRowsMock.mockResolvedValue({ rows: marks(1, 3), total: 100000, truncated: true });
    await wrapper.find('.export-btn').trigger('click');
    await flushPromises();

    const spec = buildPassageJournalBlob.mock.calls[0][0];
    expect(spec.rows).toHaveLength(3);
    expect(spec.note).toContain('3 из 100000');
    expect(spec.note).toContain('предел выгрузки');
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'warning' }));
  });
});
