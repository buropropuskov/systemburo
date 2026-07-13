import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import ApplicationsCenter from '../ApplicationsCenter.vue';
import { apiRequest } from '@/api/client';
import { getApplicationsPaginated } from '@/api/applications';
import { useAuthStore } from '@/stores/auth';

// Бесшовная подгрузка Центра порциями (#1158, срез 1): fetchApplications шлёт
// page/per_page через getApplicationsPaginated, аккумулирует порции в this.applications,
// сбрасывается на страницу 1 при смене поиска/фильтра, и hasMoreApplications/total
// берутся из envelope.meta.

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/api/applications', () => ({ getApplicationsPaginated: vi.fn() }));
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
  OrganizationFilter: true,
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
    global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn() } } },
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

    // До пагинации toggleConfirmation фильтровал уже загруженный applications клиентски,
    // без сети. После #1158 applications - лишь порция, поэтому toggle обязан
    // сбросить и перезапросить с бэка (см. applyFilters), иначе результат ограничится
    // тем, что уже подгружено, а не полным набором по новому фильтру.
    getApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(7)],
      meta: { total: 1, page: 1, per_page: 30 },
    });
    wrapper.vm.toggleConfirmation('Согласовано');
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
});
