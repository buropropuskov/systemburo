import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import EmployeeView from '../EmployeeView.vue';
import { usePermissionsStore } from '@/stores/permissions';

// Реестр сотрудников переведён на серверный поиск/пагинацию (#1158, срез 3): проверяем,
// что search_query/page/per_page реально уходят на бэк (getUniqueEmployeesPaginated),
// пагинация аккумулирует порции, а смена фильтра/поиска сбрасывает аккумулятор.
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
  apiRequestRaw: vi.fn().mockResolvedValue({
    ok: false,
    json: vi.fn().mockResolvedValue({ success: false, error: 'x' }),
  }),
}));
vi.mock('@/api/blacklist', () => ({ listPersonBlacklist: vi.fn().mockResolvedValue([]) }));

const getUniqueEmployeesPaginated = vi.fn();
vi.mock('@/api/employees', () => ({
  getUniqueEmployeesPaginated: (...args) => getUniqueEmployeesPaginated(...args),
}));

const stubs = {
  teleport: true,
  SearchComponent: true,
  RefreshButton: true,
  LoaderSpinner: true,
  StatusBadge: true,
  EmployeeEditModal: true,
  EmployeeDetailsModal: true,
  ApplicationDetail: true,
};

function mountView() {
  return mount(EmployeeView, {
    global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } } },
  });
}

function page(items, total, pageNum = 1, perPage = 30) {
  return Promise.resolve({ items, meta: { total, page: pageNum, per_page: perPage } });
}

let wrapper;

describe('EmployeeView - серверный поиск и пагинация (#1158, срез 3)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getUniqueEmployeesPaginated.mockReset();
    getUniqueEmployeesPaginated.mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } });
  });

  afterEach(() => {
    wrapper?.unmount();
    vi.useRealTimers();
  });

  it('первичная загрузка без поиска не передаёт search_query', async () => {
    wrapper = mountView();
    await flushPromises();

    expect(getUniqueEmployeesPaginated).toHaveBeenCalled();
    const params = getUniqueEmployeesPaginated.mock.calls.at(-1)[0];
    expect(params.filter_type).toBe('user');
    expect(params.page).toBe(1);
    expect(params.per_page).toBe(30);
    expect(params.search_query).toBeUndefined();
  });

  it('ввод в поиск уходит на сервер с дебаунсом (search_query в параметрах запроса)', async () => {
    vi.useFakeTimers();
    wrapper = mountView();
    await flushPromises();
    getUniqueEmployeesPaginated.mockClear();

    wrapper.vm.searchQuery = 'Иванов';
    await wrapper.vm.$nextTick();
    // До истечения дебаунса запрос ещё не ушёл.
    expect(getUniqueEmployeesPaginated).not.toHaveBeenCalled();

    vi.advanceTimersByTime(300);
    await flushPromises();

    expect(getUniqueEmployeesPaginated).toHaveBeenCalled();
    const params = getUniqueEmployeesPaginated.mock.calls.at(-1)[0];
    expect(params.search_query).toBe('Иванов');
  });

  it('пагинация аккумулирует порции: loadMore добавляет к списку, не заменяет его', async () => {
    getUniqueEmployeesPaginated.mockImplementation((params) => (
      params.page === 1 ? page([{ id: 1, last_name: 'A' }], 2) : page([{ id: 2, last_name: 'B' }], 2)
    ));
    wrapper = mountView();
    await flushPromises();
    expect(wrapper.vm.employeesData).toHaveLength(1);

    await wrapper.vm.loadMoreEmployeesList(wrapper.vm.buildEmployeesPage);
    await flushPromises();

    expect(wrapper.vm.employeesData).toHaveLength(2);
    expect(wrapper.vm.employeesData.map((e) => e.id)).toEqual([1, 2]);
  });

  it('смена filter_type сбрасывает на страницу 1 и очищает накопленные порции', async () => {
    getUniqueEmployeesPaginated.mockImplementation((params) => {
      if (params.filter_type === 'user' && params.page === 1) return page([{ id: 1, last_name: 'A' }], 2);
      if (params.filter_type === 'user' && params.page === 2) return page([{ id: 2, last_name: 'B' }], 2);
      if (params.filter_type === 'organization') return page([{ id: 9, last_name: 'ORG1' }], 1);
      return page([], 0);
    });
    wrapper = mountView();
    await flushPromises();
    await wrapper.vm.loadMoreEmployeesList(wrapper.vm.buildEmployeesPage);
    await flushPromises();
    expect(wrapper.vm.employeesData).toHaveLength(2);

    wrapper.vm.switchFilter('organization');
    await flushPromises();

    expect(wrapper.vm.employeesData).toHaveLength(1);
    expect(wrapper.vm.employeesData[0].id).toBe(9);
  });

  it('счётчик футера использует серверный meta.total, а не размер загруженной порции', async () => {
    getUniqueEmployeesPaginated.mockResolvedValue({
      items: [{ id: 1, last_name: 'A' }],
      meta: { total: 42, page: 1, per_page: 30 },
    });
    wrapper = mountView();
    await flushPromises();

    expect(wrapper.vm.employeesTotal).toBe(42);
    expect(wrapper.vm.footerText).toBe('Показано 1 из 42');
  });
});

/**
 * Переход из сквозного поиска: раньше он приводил в раздел, и найденного сотрудника
 * приходилось искать в списке заново. Теперь `?q` сужает список, а `?open` раскрывает
 * карточку той самой записи.
 */
describe('EmployeeView - открытие карточки по ссылке из сквозного поиска', () => {
  const EMPLOYEE = { id: 42, last_name: 'Иванов', first_name: 'Иван', middle_name: 'Иванович', position: 'Водитель' };

  function mountWithRoute(query, replace = vi.fn().mockResolvedValue(undefined)) {
    return mount(EmployeeView, {
      global: { stubs, mocks: { $route: { query }, $router: { push: vi.fn(), replace } } },
    });
  }

  beforeEach(() => {
    setActivePinia(createPinia());
    getUniqueEmployeesPaginated.mockReset();
    getUniqueEmployeesPaginated.mockResolvedValue({ items: [EMPLOYEE], meta: { total: 1, page: 1, per_page: 30 } });
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
  });

  it('карточка найденного сотрудника открывается сразу', async () => {
    wrapper = mountWithRoute({ q: 'иванов', open: '42' });
    await flushPromises();

    expect(wrapper.vm.showDetailsModal).toBe(true);
    expect(wrapper.vm.detailsEmployee.id).toBe(42);
  });

  it('строка поиска из адреса уходит на сервер - без неё запись не попала бы в список', async () => {
    wrapper = mountWithRoute({ q: 'иванов', open: '42' });
    await flushPromises();

    expect(getUniqueEmployeesPaginated).toHaveBeenCalledWith(expect.objectContaining({ search_query: 'иванов' }));
  });

  it('open вычищается из адреса: обновление страницы не открывает карточку заново', async () => {
    const replace = vi.fn().mockResolvedValue(undefined);
    wrapper = mountWithRoute({ q: 'иванов', open: '42' }, replace);
    await flushPromises();

    expect(replace).toHaveBeenCalledWith({ query: { q: 'иванов' } });
  });

  it('без open список открывается обычным образом', async () => {
    wrapper = mountWithRoute({ q: 'иванов' });
    await flushPromises();

    expect(wrapper.vm.showDetailsModal).toBe(false);
  });

  it('записи нет среди загруженных - карточка не открывается и ошибок нет', async () => {
    wrapper = mountWithRoute({ q: 'иванов', open: '999' });
    await flushPromises();

    expect(wrapper.vm.showDetailsModal).toBe(false);
  });
});

/**
 * Переход из сквозного поиска приводил в пустой реестр: поиск ищет по всей
 * доступной области, а страница открывалась на «Мои сотрудники», куда чужая
 * найденная запись не попадает. Поймано живой проверкой на стенде - в юнит-тестах
 * список отдавал мок, и подмены области видно не было.
 */
describe('EmployeeView - область реестра при переходе из поиска', () => {
  function seedPerms(allow) {
    const perms = usePermissionsStore();
    perms.mode = 'normal';
    perms.effective = Object.fromEntries(allow.map((k) => [k, { value: 'allow', source: 'role' }]));
  }

  function mountWithRoute(query) {
    return mount(EmployeeView, {
      global: { stubs, mocks: { $route: { query }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } } },
    });
  }

  beforeEach(() => {
    setActivePinia(createPinia());
    getUniqueEmployeesPaginated.mockReset();
    getUniqueEmployeesPaginated.mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } });
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
  });

  it('с open в адресе список запрашивается по всей системе', async () => {
    seedPerms(['section.registry.all_system']);
    wrapper = mountWithRoute({ q: 'иванов', open: '17' });
    await flushPromises();

    expect(getUniqueEmployeesPaginated).toHaveBeenCalledWith(expect.objectContaining({ filter_type: 'all_system' }));
  });

  it('без права на всю систему берётся организация', async () => {
    seedPerms(['section.registry.organization']);
    wrapper = mountWithRoute({ q: 'иванов', open: '17' });
    await flushPromises();

    expect(getUniqueEmployeesPaginated).toHaveBeenCalledWith(expect.objectContaining({ filter_type: 'organization' }));
  });

  it('обычный заход по-прежнему открывает «Мои сотрудники»', async () => {
    seedPerms(['section.registry.all_system']);
    wrapper = mountWithRoute({});
    await flushPromises();

    expect(getUniqueEmployeesPaginated).toHaveBeenCalledWith(expect.objectContaining({ filter_type: 'user' }));
  });
});
