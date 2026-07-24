import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

// Чип "Обновления" в ЛК (#1349 срез 4): серверный фильтр status_updated=true,
// счётчик из отдельного эндпоинта, оптимистичное гашение при открытии.
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })),
}));
vi.mock('@/api/applications', () => ({
  getUserApplicationsPaginated: vi.fn(() => Promise.resolve({ items: [], meta: { total: 0, page: 1, per_page: 30 } })),
  getApplicationById: vi.fn(() => Promise.resolve({ message: 'Не найдена' })),
  getUserStatusUpdatesCount: vi.fn(() => Promise.resolve({ status_updates: 0 })),
}));
import UserApplications from '../UserApplications.vue';
import { getUserApplicationsPaginated, getUserStatusUpdatesCount } from '@/api/applications';

function mountUA() {
  setActivePinia(createPinia());
  const wrapper = shallowMount(UserApplications, {
    props: { userId: 1 },
    global: { mocks: { $route: { query: {} }, $router: { replace: vi.fn(() => Promise.resolve()), push: vi.fn() } } },
  });
  return { wrapper };
}

describe('UserApplications — чип "Обновления" (#1349 срез 4)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getUserApplicationsPaginated.mockClear();
    getUserStatusUpdatesCount.mockClear();
    getUserStatusUpdatesCount.mockResolvedValue({ status_updates: 0 });
  });

  it('на mount тянет счётчик из эндпоинта ЛК и кладёт в statusUpdateCount', async () => {
    getUserStatusUpdatesCount.mockResolvedValueOnce({ status_updates: 5 });
    const { wrapper } = mountUA();
    await flushPromises();

    expect(getUserStatusUpdatesCount).toHaveBeenCalled();
    expect(wrapper.vm.statusUpdateCount).toBe(5);
  });

  it('чип рисует счётчик, когда обновления есть', async () => {
    const { wrapper } = mountUA();
    await flushPromises();
    wrapper.vm.statusUpdateCount = 3;
    await wrapper.vm.$nextTick();

    const chip = wrapper.find('[data-testid="lk-button-updates"]');
    expect(chip.exists()).toBe(true);
    expect(chip.text()).toContain('Обновления: 3');
  });

  it('toggleStatusUpdated переключает фильтр и уводит status_updated=true в запрос', async () => {
    const { wrapper } = mountUA();
    await flushPromises();

    expect(wrapper.vm.statusUpdatedOnly).toBe(false);
    wrapper.vm.toggleStatusUpdated();
    expect(wrapper.vm.statusUpdatedOnly).toBe(true);

    await wrapper.vm.buildUserApplicationsPage(1, 30);
    const params = getUserApplicationsPaginated.mock.calls.at(-1)[0];
    expect(params.status_updated).toBe('true');
    expect(params.sender_user_id).toBe(1);
  });

  it('без активного чипа фильтр status_updated не уходит в запрос', async () => {
    const { wrapper } = mountUA();
    await flushPromises();

    await wrapper.vm.buildUserApplicationsPage(1, 30);
    const params = getUserApplicationsPaginated.mock.calls.at(-1)[0];
    expect(params.status_updated).toBeUndefined();
  });

  it('openApplication оптимистично гасит флаг и уменьшает счётчик', async () => {
    const { wrapper } = mountUA();
    await flushPromises();
    wrapper.vm.statusUpdateCount = 3;
    const app = { id: 7, has_status_update: true };

    await wrapper.vm.openApplication(app);

    expect(app.has_status_update).toBe(false);
    expect(wrapper.vm.statusUpdateCount).toBe(2);
    expect(wrapper.vm.showDetailModal).toBe(true);
  });

  it('открытие заявки без флага не трогает счётчик', async () => {
    const { wrapper } = mountUA();
    await flushPromises();
    wrapper.vm.statusUpdateCount = 3;
    const app = { id: 8, has_status_update: false };

    await wrapper.vm.openApplication(app);

    expect(wrapper.vm.statusUpdateCount).toBe(3);
  });

  it('сбой загрузки счётчика сохраняет последнее значение, не обнуляет', async () => {
    const { wrapper } = mountUA();
    await flushPromises();
    wrapper.vm.statusUpdateCount = 4;

    getUserStatusUpdatesCount.mockRejectedValueOnce(new Error('boom'));
    await wrapper.vm.fetchStatusUpdateCount();

    expect(wrapper.vm.statusUpdateCount).toBe(4);
  });
});
