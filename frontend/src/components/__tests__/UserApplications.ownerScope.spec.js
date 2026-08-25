import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import UserApplications from '../UserApplications.vue';
import { getUserApplicationsPaginated } from '@/api/applications';
import { useAuthStore } from '@/stores/auth';

// Владелец списка ЛК (#2218): пропс userId приходит из /users/me позже первого кадра,
// и запрос без sender_user_id получал от бэка весь скоуп ЛК - свои ИЛИ заявки
// организации. Эта выдача успевала отрисоваться и уезжала, когда резолвился /users/me.
// Идентификатор владельца есть в маркере доступа сразу, поэтому неотфильтрованный
// запрос не должен уходить ни при каком порядке ответов.

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

// Маркер доступа проекта: полезная нагрузка вторым сегментом, claim user_id (#2218).
function tokenWith(payload) {
  return `h.${btoa(JSON.stringify(payload))}.s`;
}

function mountUA(props = {}) {
  return mount(UserApplications, {
    props: { userId: null, ...props },
    global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn(() => Promise.resolve()) } } },
  });
}

let wrapper;

describe('UserApplications — scope владельца до ответа /users/me (#2218)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getUserApplicationsPaginated.mockReset();
    getUserApplicationsPaginated.mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } });
  });

  afterEach(() => wrapper?.unmount());

  it('пропс userId ещё пуст - первый же запрос уходит с sender_user_id из маркера', async () => {
    useAuthStore().token = tokenWith({ user_id: 7, username: 'ivanov' });
    wrapper = mountUA({ userId: null });
    await flushPromises();

    expect(getUserApplicationsPaginated).toHaveBeenCalledTimes(1);
    expect(getUserApplicationsPaginated.mock.calls[0][0].sender_user_id).toBe(7);
  });

  it('владелец неизвестен - запрос не уходит вовсе, чужая выдача не рисуется', async () => {
    useAuthStore().token = tokenWith({ username: 'ivanov' });
    wrapper = mountUA({ userId: null });
    await flushPromises();

    expect(getUserApplicationsPaginated).not.toHaveBeenCalled();
    expect(wrapper.vm.applications).toEqual([]);
    expect(wrapper.vm.isLoading).toBe(true);
  });

  it('пришедший из /users/me тот же идентификатор не плодит второй запрос', async () => {
    useAuthStore().token = tokenWith({ user_id: 7 });
    wrapper = mountUA({ userId: null });
    await flushPromises();
    await wrapper.setProps({ userId: 7 });
    await flushPromises();

    expect(getUserApplicationsPaginated).toHaveBeenCalledTimes(1);
  });

  it('маркер важнее пропса: запрос уходит за того, от чьего имени работают', async () => {
    // Режим "войти как пользователь" (#1912) подменяет маркер сразу, а пропс из
    // /users/me догоняет позже - список должен принадлежать личности маркера.
    useAuthStore().token = tokenWith({ user_id: 9 });
    wrapper = mountUA({ userId: 1 });
    await flushPromises();

    expect(getUserApplicationsPaginated.mock.calls[0][0].sender_user_id).toBe(9);
  });

  it('вкладка "Заявки организации" запрашивается по organization_id, без sender_user_id', async () => {
    useAuthStore().token = tokenWith({ user_id: 7 });
    wrapper = mountUA({ userId: 7, userOrganizationId: 3 });
    await flushPromises();
    getUserApplicationsPaginated.mockClear();

    wrapper.vm.setFilter('organization');
    await flushPromises();

    const params = getUserApplicationsPaginated.mock.calls[0][0];
    expect(params.organization_id).toBe(3);
    expect(params.sender_user_id).toBeUndefined();
  });
});
