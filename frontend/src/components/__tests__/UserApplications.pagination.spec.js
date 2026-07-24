import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import UserApplications from '../UserApplications.vue';
import { apiRequest } from '@/api/client';
import { getUserApplicationsPaginated, getApplicationById } from '@/api/applications';
import { useAuthStore } from '@/stores/auth';

// Бесшовная подгрузка ЛК порциями (#1158 срез 4): fetchUserApplications шлёт
// page/per_page через getUserApplicationsPaginated, аккумулирует порции в
// this.applications, сбрасывается на страницу 1 при смене поиска/вкладки/даты,
// и hasMoreApplications/total берутся из envelope.meta. Клиентский текстовый
// matchesSearch-слой убран (поиск теперь только серверный).

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/api/applications', () => ({
  getUserApplicationsPaginated: vi.fn(),
  getApplicationById: vi.fn(),
  getUserStatusUpdatesCount: vi.fn(() => Promise.resolve({ status_updates: 0 })),
}));

const stubs = {
  teleport: true,
  RefreshButton: true,
  SearchComponent: true,
  DateFilter: true,
  ApplicationDetail: true,
  DownloadBlanksModal: true,
  LoaderSpinner: true,
  Badge: true,
};

function mountUA(props = {}) {
  return mount(UserApplications, {
    props: { userId: 1, ...props },
    global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn(() => Promise.resolve()) } } },
  });
}

function makeApp(id, over = {}) {
  return {
    id,
    application_number: `A-${id}`,
    sending_datetime: '2026-01-01T10:00:00Z',
    status: 'Согласование',
    confirmation: 'Согласование',
    sender_user_id: 1,
    organization_name: 'Орг',
    sender_name: 'Иванов',
    is_read: true,
    ...over,
  };
}

let wrapper;

describe('UserApplications — бесшовная подгрузка порциями (#1158 срез 4)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockResolvedValue({ ok: true, json: async () => ({}) });
    getUserApplicationsPaginated.mockReset();
    getApplicationById.mockReset();
    useAuthStore().token = 'test-token';
  });

  afterEach(() => wrapper?.unmount());

  it('первая загрузка шлёт page=1, per_page=30 и sender_user_id (вкладка "Мои" по умолчанию)', async () => {
    getUserApplicationsPaginated.mockResolvedValue({
      items: [makeApp(1)],
      meta: { total: 1, page: 1, per_page: 30 },
    });
    wrapper = mountUA();
    await flushPromises();

    expect(getUserApplicationsPaginated).toHaveBeenCalled();
    const params = getUserApplicationsPaginated.mock.calls[0][0];
    expect(params.page).toBe(1);
    expect(params.per_page).toBe(30);
    expect(params.sender_user_id).toBe(1);
    expect(params.organization_id).toBeUndefined();
  });

  it('вторая порция дописывается в конец, не затирая первую', async () => {
    getUserApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(1), makeApp(2)],
      meta: { total: 5, page: 1, per_page: 30 },
    });
    wrapper = mountUA();
    await flushPromises();

    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([1, 2]);
    expect(wrapper.vm.total).toBe(5);
    expect(wrapper.vm.hasMoreApplications).toBe(true);

    getUserApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(3), makeApp(4)],
      meta: { total: 5, page: 2, per_page: 30 },
    });
    await wrapper.vm.loadMoreApplicationsList(wrapper.vm.buildUserApplicationsPage);
    await flushPromises();

    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([1, 2, 3, 4]);
    const secondCallParams = getUserApplicationsPaginated.mock.calls[1][0];
    expect(secondCallParams.page).toBe(2);
    expect(wrapper.vm.hasMoreApplications).toBe(true);
  });

  it('смена поиска (дебаунс 300мс) сбрасывает на страницу 1 и затирает накопленное', async () => {
    getUserApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(1), makeApp(2)],
      meta: { total: 2, page: 1, per_page: 30 },
    });
    wrapper = mountUA();
    await flushPromises();
    expect(wrapper.vm.applications).toHaveLength(2);

    getUserApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(9)],
      meta: { total: 1, page: 1, per_page: 30 },
    });
    wrapper.vm.searchQuery = 'редкий запрос';
    await vi.waitFor(() => {
      expect(getUserApplicationsPaginated).toHaveBeenCalledTimes(2);
    });
    await flushPromises();

    expect(wrapper.vm.applications.map((a) => a.id)).toEqual([9]);
    expect(wrapper.vm.total).toBe(1);
    const searchCallParams = getUserApplicationsPaginated.mock.calls[1][0];
    expect(searchCallParams.page).toBe(1);
    expect(searchCallParams.search_query).toBe('редкий запрос');
  });

  it('вкладка "Заявки организации" шлёт organization_id вместо sender_user_id', async () => {
    getUserApplicationsPaginated.mockResolvedValue({
      items: [makeApp(1)], meta: { total: 1, page: 1, per_page: 30 },
    });
    wrapper = mountUA({ userOrganizationId: 42 });
    await flushPromises();

    getUserApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(5, { organization_id: 42 })], meta: { total: 3, page: 1, per_page: 30 },
    });
    wrapper.vm.setFilter('organization');
    await flushPromises();

    const params = getUserApplicationsPaginated.mock.calls.at(-1)[0];
    expect(params.organization_id).toBe(42);
    expect(params.sender_user_id).toBeUndefined();
    expect(params.page).toBe(1);
  });

  it('hasMoreApplications=false прячет sentinel, футер показывает "Показано X из Y"', async () => {
    getUserApplicationsPaginated.mockResolvedValue({
      items: [makeApp(1), makeApp(2)],
      meta: { total: 2, page: 1, per_page: 30 },
    });
    wrapper = mountUA();
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.find('[data-testid="user-applications-sentinel"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="user-applications-footer"]').text()).toContain('Показано 2 из 2');
  });

  it('sentinel рендерится, пока hasMoreApplications=true', async () => {
    getUserApplicationsPaginated.mockResolvedValue({
      items: [makeApp(1)],
      meta: { total: 5, page: 1, per_page: 30 },
    });
    wrapper = mountUA();
    await flushPromises();
    await wrapper.vm.$nextTick();

    expect(wrapper.find('[data-testid="user-applications-sentinel"]').exists()).toBe(true);
  });

  it('выбор сортировки по колонке догружает весь набор (клиентская сортировка)', async () => {
    getUserApplicationsPaginated.mockResolvedValueOnce({
      items: [makeApp(1)], meta: { total: 2, page: 1, per_page: 30 },
    });
    wrapper = mountUA();
    await flushPromises();
    expect(getUserApplicationsPaginated).toHaveBeenCalledTimes(1);
    expect(wrapper.vm.hasMoreApplications).toBe(true);

    getUserApplicationsPaginated
      .mockResolvedValueOnce({ items: [makeApp(1)], meta: { total: 2, page: 1, per_page: 30 } })
      .mockResolvedValueOnce({ items: [makeApp(2)], meta: { total: 2, page: 2, per_page: 30 } });
    wrapper.vm.sortBy('application_number');
    await vi.waitFor(() => expect(getUserApplicationsPaginated).toHaveBeenCalledTimes(3));
    await flushPromises();

    expect(wrapper.vm.isFullLoad).toBe(true);
    expect([...wrapper.vm.applications.map((a) => a.id)].sort()).toEqual([1, 2]);
  });

  it('клиентский слой больше не режет по тексту: applications = серверный ответ 1:1 (без учёта date-мирроринга)', async () => {
    // Раньше filteredApplications ре-фильтровал уже загруженный массив через matchesSearch -
    // если бы это осталось, заявка, найденная сервером по fuzzy-совпадению, но не совпадающая
    // по точной строке, пропала бы из отображения. Проверяем, что этого больше нет.
    getUserApplicationsPaginated.mockResolvedValue({
      items: [makeApp(1, { message: 'Совсем другой текст, не похожий на запрос' })],
      meta: { total: 1, page: 1, per_page: 30 },
    });
    wrapper = mountUA();
    wrapper.vm.searchQuery = 'запрос-найденный-только-fuzzy-поиском-на-бэке';
    await flushPromises();

    expect(wrapper.vm.filteredApplications.map((a) => a.id)).toEqual([1]);
  });
});
