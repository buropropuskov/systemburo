import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })),
}));
vi.mock('@/api/applications', () => ({
  getUserApplicationsPaginated: vi.fn(() => Promise.resolve({ items: [], meta: { total: 0, page: 1, per_page: 30 } })),
  getApplicationById: vi.fn(() => Promise.resolve({ message: 'Не найдена' })),
  getUserStatusUpdatesCount: vi.fn(() => Promise.resolve({ status_updates: 0 })),
}));
import UserApplications from '../UserApplications.vue';
import { getApplicationById } from '@/api/applications';

function mountUA() {
  setActivePinia(createPinia());
  const replace = vi.fn(() => Promise.resolve());
  const wrapper = shallowMount(UserApplications, {
    props: { userId: 1 },
    global: { mocks: { $route: { query: {} }, $router: { replace, push: vi.fn() } } },
  });
  return { wrapper, replace };
}

describe('UserApplications — deep-link ?open (#973/#1158)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getApplicationById.mockClear();
  });

  it('не чистит query, пока заявка не появилась в списке (гонка userId)', () => {
    const { wrapper, replace } = mountUA();
    wrapper.vm.applications = [];
    wrapper.vm.$route.query = { open: '7' };
    wrapper.vm.openFromDeepLink();

    expect(replace).not.toHaveBeenCalled();
    expect(wrapper.vm.showDetailModal).toBe(false);
  });

  it('открывает заявку и чистит query, когда она есть в списке', async () => {
    const { wrapper, replace } = mountUA();
    wrapper.vm.applications = [{ id: 7, sender_user_id: 1, application_number: 'A-7' }];
    wrapper.vm.$route.query = { open: '7' };
    wrapper.vm.openFromDeepLink();
    await flushPromises();

    expect(wrapper.vm.selectedApplication).toEqual(expect.objectContaining({ id: 7 }));
    expect(wrapper.vm.showDetailModal).toBe(true);
    expect(replace).toHaveBeenCalled();
  });

  // #1158 срез 4: заявка вне загруженной порции (страница 2+, другая вкладка/дата) -
  // точечная догрузка по id, зеркало Центра (#1163).
  it('заявки нет в загруженной порции - догружает по id и открывает деталь', async () => {
    getApplicationById.mockResolvedValueOnce({ id: 42, sender_user_id: 1, application_number: 'A-42' });
    const { wrapper, replace } = mountUA();
    wrapper.vm.applications = [{ id: 1, sender_user_id: 1, application_number: 'A-1' }];
    wrapper.vm.$route.query = { open: '42' };
    wrapper.vm.openFromDeepLink();
    await flushPromises();

    expect(getApplicationById).toHaveBeenCalledWith(42);
    expect(wrapper.vm.selectedApplication).toEqual(expect.objectContaining({ id: 42 }));
    expect(wrapper.vm.showDetailModal).toBe(true);
    expect(replace).toHaveBeenCalled();
  });

  // #1158 срез 4: отказ доступа (403) - getApplicationById отдаёт {message} без id,
  // деталь не открывается, query остаётся до следующей попытки.
  it('deep-link на недоступную заявку не открывает деталь и не чистит query', async () => {
    getApplicationById.mockResolvedValueOnce({ message: 'Недостаточно прав' });
    const { wrapper, replace } = mountUA();
    wrapper.vm.applications = [];
    wrapper.vm.$route.query = { open: '77' };
    wrapper.vm.openFromDeepLink();
    await flushPromises();

    expect(getApplicationById).toHaveBeenCalledWith(77);
    expect(wrapper.vm.selectedApplication).toBeNull();
    expect(replace).not.toHaveBeenCalled();
  });
});
