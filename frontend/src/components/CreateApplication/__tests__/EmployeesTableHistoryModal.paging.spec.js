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

import EmployeesTableHistoryModal from '../EmployeesTableHistoryModal.vue';

function events(from, count) {
  return Array.from({ length: count }, (_, i) => ({
    id: from + i,
    employee_id: 11,
    user_id: 3,
    user_name: 'Иванов И.И.',
    action_type: i % 2 === 0 ? 'entry' : 'exit',
    created_at: `2026-09-1${(i % 9) + 1}T10:00:00Z`,
    employee_last_name: 'Петров',
    employee_first_name: 'Пётр',
    table_name: 'КПП-1',
  }));
}

function pageResponse(items, total) {
  return { ok: true, json: async () => ({ success: true, data: items, meta: { total, page: 1, per_page: 50 } }) };
}

function paramsOf(call) {
  return new URLSearchParams(call[0].split('?')[1] || '');
}

function mountModal() {
  return mount(EmployeesTableHistoryModal, {
    props: { tableId: 4, currentUserName: 'Сидоров С.С.' },
    global: { stubs: { teleport: true, transition: false, 'transition-group': false } },
  });
}

describe('EmployeesTableHistoryModal - страницы и серверные фильтры (#2469)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockImplementation(() => Promise.resolve({
      ok: true,
      json: async () => ({
        users: [{ id: 3, name: 'Иванов И.И.' }],
        employees: [{ id: 11, last_name: 'Петров', first_name: 'Пётр', middle_name: null }],
      }),
    }));
    apiRequestRaw.mockReset();
    apiRequestRaw.mockImplementation(() => Promise.resolve(pageResponse(events(1, 2), 5)));
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

  it('открывается первой страницей своей таблицы и говорит, сколько нашлось', async () => {
    const wrapper = mountModal();
    await flushPromises();

    const call = apiRequestRaw.mock.calls[0];
    expect(call[0].startsWith('/employees/history/table/4?')).toBe(true);
    expect(paramsOf(call).get('per_page')).toBe('50');
    expect(wrapper.findAll('.history-item')).toHaveLength(2);
    expect(wrapper.text()).toContain('Показано 2 из 5');
  });

  it('списки фильтров приходят отдельным запросом по своей таблице', async () => {
    const wrapper = mountModal();
    await flushPromises();

    expect(apiRequest).toHaveBeenCalledWith('/employees/history/filter-options?table_id=4', { method: 'GET' });

    await wrapper.find('.employee-filter .custom-select').trigger('click');
    const options = wrapper.findAll('.employee-filter .select-option');
    expect(options).toHaveLength(2, 'вариант «Все сотрудники» и один человек из журнала');
    expect(options[1].text()).toContain('Петров');
  });

  // Ключ сущности у людей свой: employee_id, а не car_id. Ошибка здесь тихо снимает
  // фильтр - сервер просто не увидит неизвестный параметр.
  it('выбор сотрудника уходит в запрос как employee_id', async () => {
    const wrapper = mountModal();
    await flushPromises();

    await wrapper.find('.employee-filter .custom-select').trigger('click');
    await wrapper.findAll('.employee-filter .select-option')[1].trigger('click');
    await flushPromises();

    const last = paramsOf(apiRequestRaw.mock.calls.at(-1));
    expect(last.get('employee_id')).toBe('11');
    expect(last.get('car_id')).toBeNull();
    expect(last.get('page')).toBe('1');
  });

  it('подгрузка дописывает следующую страницу', async () => {
    const wrapper = mountModal();
    await flushPromises();

    apiRequestRaw.mockImplementation(() => Promise.resolve(pageResponse(events(3, 2), 5)));
    await wrapper.find('.load-more-btn').trigger('click');
    await flushPromises();

    expect(paramsOf(apiRequestRaw.mock.calls.at(-1)).get('page')).toBe('2');
    expect(wrapper.findAll('.history-item')).toHaveLength(4);
  });

  it('поиск ждёт паузы в наборе и уходит одним запросом', async () => {
    vi.useFakeTimers();
    const wrapper = mountModal();
    await flushPromises();
    const before = apiRequestRaw.mock.calls.length;

    const input = wrapper.find('.search-input');
    await input.setValue('Петр');
    await input.trigger('input');
    await input.setValue('Петров');
    await input.trigger('input');
    expect(apiRequestRaw.mock.calls.length).toBe(before);

    vi.advanceTimersByTime(300);
    await flushPromises();

    expect(apiRequestRaw.mock.calls.length).toBe(before + 1);
    expect(paramsOf(apiRequestRaw.mock.calls.at(-1)).get('search')).toBe('Петров');
  });

  it('выгрузка собирает журнал по фильтру и помечает обрезку', async () => {
    const wrapper = mountModal();
    await flushPromises();

    apiRequestRaw.mockImplementation(() => Promise.resolve(pageResponse(events(1, 3), 100000)));
    await wrapper.find('.export-btn').trigger('click');
    await flushPromises();

    expect(paramsOf(apiRequestRaw.mock.calls.at(-1)).get('per_page')).toBe('200');
    const spec = buildPassageJournalBlob.mock.calls[0][0];
    expect(spec.rows).toHaveLength(3);
    expect(spec.note).toContain('3 из 100000');
    expect(spec.note).toContain('предел выгрузки');
    expect(saveJournalBlob).toHaveBeenCalledTimes(1);
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'warning' }));
  });

  // Сбой обрабатывается на месте: тост человеку, промис не остаётся отклонённым, а
  // счётчик страниц после неудавшейся подгрузки возвращается назад.
  it('сбой загрузки показывает тост и не сдвигает страницу', async () => {
    const wrapper = mountModal();
    await flushPromises();

    apiRequestRaw.mockRejectedValueOnce(new Error('сеть отвалилась'));
    await wrapper.find('.load-more-btn').trigger('click');
    await flushPromises();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'историю таблицы', type: 'error' }));

    apiRequestRaw.mockImplementation(() => Promise.resolve(pageResponse(events(3, 2), 5)));
    await wrapper.find('.load-more-btn').trigger('click');
    await flushPromises();

    expect(paramsOf(apiRequestRaw.mock.calls.at(-1)).get('page')).toBe('2');
  });
});
