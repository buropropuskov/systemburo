import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import ApplicationsCenter from '../ApplicationsCenter.vue';
import { apiRequest } from '@/api/client';
import { getApplicationsPaginated, getApplicationById } from '@/api/applications';
import eventStream from '@/services/eventStream';
import { playPreset } from '@/utils/notificationSound';
import { useAuthStore } from '@/stores/auth';

// Бесшовная подгрузка Центра порциями (#1158, срез 1): fetchApplications шлёт
// page/per_page через getApplicationsPaginated, аккумулирует порции в this.applications,
// сбрасывается на страницу 1 при смене поиска/фильтра, и hasMoreApplications/total
// берутся из envelope.meta.

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/api/applications', () => ({ getApplicationsPaginated: vi.fn(), getApplicationById: vi.fn() }));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn(), SOUND_PRESETS: [] }));
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => () => {}),
    onStatus: vi.fn(() => () => {}),
  },
}));

const stubs = {
  teleport: true,
  RefreshButton: true,
  ApplicationDetail: true,
  DateFilter: true,
  FilterTabs: true,
  SkeletonTransition: { template: '<div><slot /></div>' },
  SkeletonTable: true,
  LoaderSpinner: true,
  DownloadBlanksModal: true,
  Badge: true,
  BaseDropdown: true,
};

function mountCenter() {
  return mount(ApplicationsCenter, {
    global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } } },
  });
}

function makeApp(id, over = {}) {
  return {
    id,
    application_number: `A-${id}`,
    sending_datetime: '2026-01-01T10:00:00Z',
    status: 'Согласование',
    confirmation: 'Согласование',
    organization_name: 'Орг',
    sender_name: 'Иванов',
    is_read: true,
    ...over,
  };
}

let wrapper;

describe('ApplicationsCenter — бесшовная подгрузка порциями (#1158)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockResolvedValue({ ok: false, text: async () => '', json: async () => [] });
    getApplicationsPaginated.mockReset();
    getApplicationById.mockReset();
    playPreset.mockClear();
    eventStream.subscribe.mockClear();
    useAuthStore().token = 'test-token';
  });

  afterEach(() => wrapper?.unmount());

  it('первая загрузка шлёт page=1 и per_page (30)', async () => {
    getApplicationsPaginated.mockResolvedValue({
      items: [makeApp(1)],
      meta: { total: 1, page: 1, per_page: 30 },
    });
    wrapper = mountCenter();
    await flushPromises();

    expect(getApplicationsPaginated).toHaveBeenCalled();
    const params = getApplicationsPaginated.mock.calls[0][0];
    expect(params.page).toBe(1);
    expect(params.per_page).toBe(30);
  });

  // Регресс: "Непрочитано" уходил как status='Непрочитано' -> бэк искал колонку a.status,
  // отдавал пусто ("нет заявок"). Теперь это отдельный флаг unread; статусы/подтверждения -
  // весь выбор через запятую (бэк матчит IN), а не только [0].
  async function paramsFor(setup) {
    getApplicationsPaginated.mockResolvedValue({ items: [], meta: { total: 0 } });
    wrapper = mountCenter();
    await flushPromises();
    setup(wrapper.vm);
    getApplicationsPaginated.mockClear();
    await wrapper.vm.buildApplicationsPage(1, 30);
    return getApplicationsPaginated.mock.calls[0][0];
  }

  it('фильтр Непрочитано -> unread=true, БЕЗ status=Непрочитано', async () => {
    const p = await paramsFor((vm) => { vm.selectedApplicationStatuses = ['Непрочитано']; });
    expect(p.unread).toBe('true');
    expect(p.status).toBeUndefined();
  });

  it('Непрочитано + статусы -> unread=true И status с остальными через запятую', async () => {
    const p = await paramsFor((vm) => { vm.selectedApplicationStatuses = ['Непрочитано', 'В работе', 'Завершено']; });
    expect(p.unread).toBe('true');
    expect(p.status).toBe('В работе,Завершено');
  });

  it('только статусы (без Непрочитано) -> status comma-joined, unread не шлётся', async () => {
    const p = await paramsFor((vm) => { vm.selectedApplicationStatuses = ['В работе', 'Завершено']; });
    expect(p.status).toBe('В работе,Завершено');
    expect(p.unread).toBeUndefined();
  });

  it('мультивыбор подтверждений -> confirmation comma-joined (не только первый)', async () => {
    const p = await paramsFor((vm) => { vm.selectedConfirmations = ['Согласовано', 'Не согласовано']; });
    expect(p.confirmation).toBe('Согласовано,Не согласовано');
  });

  it('мультивыбор справочников -> *_ids comma-joined (#1398)', async () => {
    const p = await paramsFor((vm) => {
      vm.selectedOrganizationIds = [1, 2];
      vm.selectedCompanyIds = [5];
      vm.selectedUnloadPlaceIds = [3, 4];
      vm.selectedPassageTableIds = [9];
    });
    expect(p.organization_ids).toBe('1,2');
    expect(p.company_ids).toBe('5');
    expect(p.unload_place_ids).toBe('3,4');
    expect(p.passage_table_ids).toBe('9');
  });

  it('пустые справочные фильтры не шлются в запрос (#1398)', async () => {
    const p = await paramsFor(() => {});
    expect(p.organization_ids).toBeUndefined();
    expect(p.company_ids).toBeUndefined();
    expect(p.unload_place_ids).toBeUndefined();
    expect(p.passage_table_ids).toBeUndefined();
  });

  it('вторая порция дописывается в конец, не затирая первую', async () => {
    getApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(1), makeApp(2)],
      meta: { total: 5, page: 1, per_page: 30 },
    });
    wrapper = mountCenter();
    await flushPromises();

    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([1, 2]);
    expect(wrapper.vm.total).toBe(5);
    expect(wrapper.vm.hasMoreApplications).toBe(true);

    getApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(3), makeApp(4)],
      meta: { total: 5, page: 2, per_page: 30 },
    });
    await wrapper.vm.loadMoreApplicationsList(wrapper.vm.buildApplicationsPage);
    await flushPromises();

    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([1, 2, 3, 4]);
    // Вторая порция дошла с page=2 - подгрузка реально продвинула страницу.
    const secondCallParams = getApplicationsPaginated.mock.calls[1][0];
    expect(secondCallParams.page).toBe(2);
    expect(wrapper.vm.hasMoreApplications).toBe(true);
  });

  it('смена поиска сбрасывает на страницу 1 и затирает накопленное', async () => {
    getApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(1), makeApp(2)],
      meta: { total: 5, page: 1, per_page: 30 },
    });
    wrapper = mountCenter();
    await flushPromises();

    getApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(3), makeApp(4)],
      meta: { total: 5, page: 2, per_page: 30 },
    });
    await wrapper.vm.loadMoreApplicationsList(wrapper.vm.buildApplicationsPage);
    await flushPromises();
    expect(wrapper.vm.applications).toHaveLength(4);

    // Новый поиск возвращает СВОЙ, другой набор - проверяем, что старые (1-4) не остаются.
    getApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(9)],
      meta: { total: 1, page: 1, per_page: 30 },
    });
    wrapper.vm.searchQuery = 'редкий запрос';
    wrapper.vm.onSearchInput();
    await vi.waitFor(() => {
      expect(getApplicationsPaginated).toHaveBeenCalledTimes(3);
    });
    await flushPromises();

    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([9]);
    expect(wrapper.vm.total).toBe(1);
    expect(wrapper.vm.hasMoreApplications).toBe(false);
    const searchCallParams = getApplicationsPaginated.mock.calls[2][0];
    expect(searchCallParams.page).toBe(1);
    expect(searchCallParams.search_query).toBe('редкий запрос');
  });

  it('смена серверного фильтра (подтверждение) сбрасывает накопленное и шлёт page=1', async () => {
    getApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(1), makeApp(2)],
      meta: { total: 5, page: 1, per_page: 30 },
    });
    wrapper = mountCenter();
    await flushPromises();

    getApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(3), makeApp(4)],
      meta: { total: 5, page: 2, per_page: 30 },
    });
    await wrapper.vm.loadMoreApplicationsList(wrapper.vm.buildApplicationsPage);
    await flushPromises();
    expect(wrapper.vm.applications).toHaveLength(4);

    // До пагинации смена подтверждения фильтровала уже загруженный applications
    // клиентски, без сети. После #1158 applications - лишь порция, поэтому смена
    // фильтра обязана сбросить и перезапросить с бэка (см. applyFilters), иначе
    // результат ограничится подгруженным, а не полным набором по новому фильтру.
    getApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(7)],
      meta: { total: 1, page: 1, per_page: 30 },
    });
    wrapper.vm.setMultiFilter('selectedConfirmations', ['Согласовано']);
    await flushPromises();

    expect(getApplicationsPaginated).toHaveBeenCalledTimes(3);
    const thirdCallParams = getApplicationsPaginated.mock.calls[2][0];
    expect(thirdCallParams.page).toBe(1);
    expect(thirdCallParams.confirmation).toBe('Согласовано');
    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([7]);
  });

  it('hasMoreApplications=false прячет sentinel, футер показывает "Показано X из Y"', async () => {
    getApplicationsPaginated.mockResolvedValue({
      items: [makeApp(1), makeApp(2)],
      meta: { total: 2, page: 1, per_page: 30 },
    });
    wrapper = mountCenter();
    wrapper.vm.loading = false;
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.find('[data-testid="center-scroll-sentinel"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="center-table-footer"]').text()).toContain('Показано 2 из 2');
  });

  it('sentinel рендерится, пока hasMoreApplications=true', async () => {
    getApplicationsPaginated.mockResolvedValue({
      items: [makeApp(1)],
      meta: { total: 5, page: 1, per_page: 30 },
    });
    wrapper = mountCenter();
    wrapper.vm.loading = false;
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.find('[data-testid="center-scroll-sentinel"]').exists()).toBe(true);
  });

  it('loadMoreApplicationsList не пускает подгрузку, пока список ещё грузится', async () => {
    let resolveFirst;
    getApplicationsPaginated.mockImplementationOnce(
      () => new Promise((r) => { resolveFirst = r; }),
    );
    wrapper = mountCenter();
    await wrapper.vm.$nextTick();

    // Первый (mounted) запрос ещё висит - подгрузка второй порции не должна стартовать.
    const before = getApplicationsPaginated.mock.calls.length;
    wrapper.vm.loadMoreApplicationsList(wrapper.vm.buildApplicationsPage);
    expect(getApplicationsPaginated.mock.calls.length).toBe(before);

    resolveFirst({ items: [makeApp(1)], meta: { total: 1, page: 1, per_page: 30 } });
    await flushPromises();
  });

  // FIX 1 (#1158): тихий real-time рефреш НЕ должен схлопывать накопленный скролл.
  it('SSE-событие при накопленных порциях НЕ сбрасывает длину списка', async () => {
    getApplicationsPaginated
      .mockResolvedValueOnce({ items: [makeApp(1)], meta: { total: 3, page: 1, per_page: 30 } })
      .mockResolvedValueOnce({ items: [makeApp(2)], meta: { total: 3, page: 2, per_page: 30 } })
      .mockResolvedValueOnce({ items: [makeApp(3)], meta: { total: 3, page: 3, per_page: 30 } });
    wrapper = mountCenter();
    await flushPromises();
    await wrapper.vm.loadMoreApplicationsList(wrapper.vm.buildApplicationsPage);
    await flushPromises();
    await wrapper.vm.loadMoreApplicationsList(wrapper.vm.buildApplicationsPage);
    await flushPromises();

    expect(wrapper.vm.applications).toHaveLength(3);
    expect(wrapper.vm.applicationsPage).toBe(3);
    const paginatedCallsBefore = getApplicationsPaginated.mock.calls.length;

    // Real-time событие: инкрементальный синк идёт через apiRequest (легаси список),
    // НЕ через reset-fetch getApplicationsPaginated - иначе скролл прыгает под юзером.
    wrapper.vm.isInitialLoad = false;
    apiRequest.mockResolvedValue({
      ok: true,
      json: async () => [makeApp(1), makeApp(2), makeApp(3)],
    });
    const sseCall = eventStream.subscribe.mock.calls.find((c) => c[0] === 'applications-center');
    await sseCall[1]();
    await flushPromises();

    // Накопленные 3 порции сохранены, reset не произошёл.
    expect(wrapper.vm.applications).toHaveLength(3);
    expect(getApplicationsPaginated.mock.calls.length).toBe(paginatedCallsBefore);
  });

  // FIX 3 (#1158): футер показывает реально видимые строки, без вранья "из {total}".
  it('футер при активном теге = числу видимых строк, без "из {total}"', async () => {
    getApplicationsPaginated.mockResolvedValue({
      items: [makeApp(1, { has_roof_access: true }), makeApp(2)],
      meta: { total: 2, page: 1, per_page: 30 },
    });
    wrapper = mountCenter();
    wrapper.vm.loading = false;
    await flushPromises();
    await wrapper.vm.$nextTick();

    // Без тега (клиент не урезает) - "Показано 2 из 2".
    expect(wrapper.find('[data-testid="center-table-footer"]').text()).toContain('Показано 2 из 2');

    wrapper.vm.setMultiFilter('selectedTags', ['roof']);
    await flushPromises();
    await wrapper.vm.$nextTick();

    const footer = wrapper.find('[data-testid="center-table-footer"]').text();
    expect(footer).toContain('Показано 1');
    expect(footer).not.toContain('из 2');
  });

  // FIX 4 (#1158): одиночная дата уходит как date_from=date_to (бэк не знает поля date).
  it('одиночная дата уходит как date_from=date_to (локальный YMD, без поля date)', async () => {
    getApplicationsPaginated.mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } });
    wrapper = mountCenter();
    await flushPromises();
    getApplicationsPaginated.mockClear();

    wrapper.vm.selectedDate = new Date(2026, 6, 10); // 10 июля 2026, локальная полночь
    wrapper.vm.applyDateFilters();
    await flushPromises();

    const params = getApplicationsPaginated.mock.calls.at(-1)[0];
    expect(params.date_from).toBe('2026-07-10');
    expect(params.date_to).toBe('2026-07-10');
    expect(params.date).toBeUndefined();
  });

  // FIX 5 (#1158): deep-link ?open= на заявку вне первой порции догружает её по id.
  it('?open= на заявку вне первой порции догружает её по id и открывает деталь', async () => {
    getApplicationsPaginated.mockResolvedValue({ items: [makeApp(1)], meta: { total: 5, page: 1, per_page: 30 } });
    getApplicationById.mockResolvedValue(makeApp(42, { is_read: true }));
    const replace = vi.fn(() => Promise.resolve());
    wrapper = mount(ApplicationsCenter, {
      global: {
        stubs,
        mocks: { $route: { query: { open: '42' }, path: '/center' }, $router: { push: vi.fn(), replace } },
      },
    });
    await flushPromises();

    // Заявки 42 нет в загруженной порции ([id:1]) - догрузили точечно по id.
    expect(getApplicationById).toHaveBeenCalledWith(42);
    expect(wrapper.vm.selectedApplication).toBeTruthy();
    expect(wrapper.vm.selectedApplication.id).toBe(42);
    // ?open вычищен после успешного открытия (обновление страницы не переоткроет).
    expect(replace).toHaveBeenCalled();
  });

  // Регресс (#1158): теги/сортировка фильтруют по ВСЕМУ набору (как на dev), а не по порции.
  it('активный тег включает full-load: догружаются все порции для клиентской фильтрации', async () => {
    getApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(1)], meta: { total: 2, page: 1, per_page: 30 },
    });
    wrapper = mountCenter();
    await flushPromises();
    expect(getApplicationsPaginated).toHaveBeenCalledTimes(1); // без тега full-load не включён

    getApplicationsPaginated
      .mockResolvedValueOnce({ items: [makeApp(1, { has_roof_access: true })], meta: { total: 2, page: 1, per_page: 30 } })
      .mockResolvedValueOnce({ items: [makeApp(2)], meta: { total: 2, page: 2, per_page: 30 } });
    wrapper.vm.setMultiFilter('selectedTags', ['roof']);
    await vi.waitFor(() => expect(getApplicationsPaginated).toHaveBeenCalledTimes(3));
    await flushPromises();

    // Загружены обе порции (весь набор) - затем клиентский фильтр по тегу.
    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([1, 2]);
    expect(getApplicationsPaginated.mock.calls.at(-1)[0].page).toBe(2);
    expect(wrapper.vm.sortedApplications.map((a) => a.id)).toEqual([1]);
  });

  // YELLOW (#1158): инкрементальный поллинг детектит появление membership-снимком
  // (не id-порогом, ломавшим архив) и синкает total, иначе hasMore/футер врут.
  it('инкрементальный опрос prepend\'ит новую заявку и синкает total, без дублей', async () => {
    getApplicationsPaginated.mockResolvedValue({
      items: [makeApp(1), makeApp(2)], meta: { total: 2, page: 1, per_page: 30 },
    });
    wrapper = mountCenter();
    await flushPromises();
    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([1, 2]);
    expect(wrapper.vm.total).toBe(2);

    // Первый опрос лишь инициализирует снимок серверных id - ничего не prepend'ит.
    apiRequest.mockResolvedValue({ ok: true, json: async () => [makeApp(1), makeApp(2)] });
    await wrapper.vm._pollApplicationsIncremental();
    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([1, 2]);
    expect(wrapper.vm.total).toBe(2);

    // Второй опрос: появилась заявка 3 - prepend сверху, total синкнут (2 -> 3), без дублей.
    apiRequest.mockResolvedValue({ ok: true, json: async () => [makeApp(3), makeApp(1), makeApp(2)] });
    await wrapper.vm._pollApplicationsIncremental();
    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([3, 1, 2]);
    expect(wrapper.vm.total).toBe(3);
  });

  // БЛОКЕР (#1158): снимок _pollKnownIds инвалидируется при смене «вселенной» (archiveMode),
  // иначе недогруженные страницы нового набора считаются "новыми" -> bulk-prepend + ложный звук.
  it('смена archiveMode инвалидирует снимок: нет bulk-prepend архива, total стабилен, звук молчит', async () => {
    getApplicationsPaginated.mockResolvedValue({ items: [makeApp(101)], meta: { total: 1, page: 1, per_page: 30 } });
    const replace = vi.fn(() => Promise.resolve());
    wrapper = mount(ApplicationsCenter, {
      global: { stubs, mocks: { $route: { query: {}, path: '/center' }, $router: { push: vi.fn(), replace } } },
    });
    await flushPromises();
    wrapper.vm.soundStore.setEnabled(true);
    wrapper.vm.pollPrimed = true;

    // Опрос активных строит снимок _pollKnownIds по активным id.
    apiRequest.mockResolvedValue({ ok: true, json: async () => [makeApp(101)] });
    await wrapper.vm._pollApplicationsIncremental();
    expect(wrapper.vm._pollKnownIds).toBeTruthy();

    // Переключение на Архив - другое пространство id.
    getApplicationsPaginated.mockResolvedValue({ items: [makeApp(5)], meta: { total: 20, page: 1, per_page: 30 } });
    wrapper.vm.archiveMode = 'archive';
    await flushPromises();
    expect(wrapper.vm._pollKnownIds).toBeNull(); // снимок инвалидирован

    // Скролл за page1 в архиве.
    getApplicationsPaginated.mockResolvedValueOnce({ items: [makeApp(4)], meta: { total: 20, page: 2, per_page: 30 } });
    await wrapper.vm.loadMoreApplicationsList(wrapper.vm.buildApplicationsPage);
    await flushPromises();
    wrapper.vm.pollPrimed = true; // после refetch pollPrimed=true, звук был бы возможен
    const lenBefore = wrapper.vm.applications.length;
    const totalBefore = wrapper.vm.total;

    // Полный архивный снимок опросом: недогруженные архивные id (3,2,1) НЕ считаются
    // "новыми" - снимок пуст после смены вселенной, поэтому НЕ bulk-prepend.
    playPreset.mockClear();
    apiRequest.mockResolvedValue({
      ok: true,
      json: async () => [makeApp(5), makeApp(4), makeApp(3), makeApp(2), makeApp(1)],
    });
    await wrapper.vm._pollApplicationsIncremental();
    await flushPromises();

    expect(wrapper.vm.applications).toHaveLength(lenBefore);
    expect(wrapper.vm.total).toBe(totalBefore);
    expect(playPreset).not.toHaveBeenCalled();
  });

  // YELLOW (#1158): deep-link на заявку без доступа (403) - getApplicationById отдаёт
  // {message} без id, openFromDeepLink не открывает и не чистит query, без исключения.
  it('deep-link ?open= с отказом доступа не открывает деталь и не чистит query', async () => {
    getApplicationsPaginated.mockResolvedValue({ items: [makeApp(1)], meta: { total: 5, page: 1, per_page: 30 } });
    getApplicationById.mockResolvedValue({ message: 'Недостаточно прав' }); // envelope !success -> без id
    const replace = vi.fn(() => Promise.resolve());
    wrapper = mount(ApplicationsCenter, {
      global: { stubs, mocks: { $route: { query: { open: '77' }, path: '/center' }, $router: { push: vi.fn(), replace } } },
    });
    await flushPromises();

    expect(getApplicationById).toHaveBeenCalledWith(77);
    expect(wrapper.vm.selectedApplication).toBeNull(); // деталь не открылась
    expect(replace).not.toHaveBeenCalled(); // ?open оставлен до след. попытки
  });

  it('выбор сортировки по колонке догружает весь набор (клиентская сортировка по dev-семантике)', async () => {
    getApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(1)], meta: { total: 2, page: 1, per_page: 30 },
    });
    wrapper = mountCenter();
    await flushPromises();
    expect(getApplicationsPaginated).toHaveBeenCalledTimes(1);
    expect(wrapper.vm.hasMoreApplications).toBe(true);

    getApplicationsPaginated
      .mockResolvedValueOnce({ items: [makeApp(1)], meta: { total: 2, page: 1, per_page: 30 } })
      .mockResolvedValueOnce({ items: [makeApp(2)], meta: { total: 2, page: 2, per_page: 30 } });
    wrapper.vm.sortBy('number');
    await vi.waitFor(() => expect(getApplicationsPaginated).toHaveBeenCalledTimes(3));
    await flushPromises();

    expect(wrapper.vm.isFullLoad).toBe(true);
    expect([...wrapper.vm.applications.map((a) => a.id)].sort()).toEqual([1, 2]);
  });
});
