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
vi.mock('@/api/onboarding', () => ({
  getOnboardingStatus: vi.fn().mockResolvedValue({ completed: {} }),
  markOnboardingComplete: vi.fn().mockResolvedValue({ message: 'ok' }),
  getSecurityFactRoute: vi.fn().mockResolvedValue(null),
}));
vi.mock('@/api/approvers', () => ({
  getMyApprovalRole: vi.fn().mockResolvedValue({ is_approver: false, is_reviewer: false }),
}));

import UserApplications from '../UserApplications.vue';
import { useOnboardingStore } from '@/stores/onboarding';

/**
 * Раскрытие `reveal.open: 'first-application'`: деталь заявки - модалка внутри
 * кабинета, а не роут, поэтому сам тур её открыть не может. Кабинет реагирует на
 * сигнал стора и закрывает за собой ровно то, что открыл.
 */

function mountUA() {
  return shallowMount(UserApplications, {
    props: { userId: 1 },
    global: { mocks: { $route: { query: {} }, $router: { replace: vi.fn(() => Promise.resolve()), push: vi.fn() } } },
  });
}

describe('UserApplications - раскрытие карточки заявки для онбординга', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('сигнал first-application открывает деталь первой заявки', async () => {
    const wrapper = mountUA();
    wrapper.vm.applications = [
      { id: 7, sender_user_id: 1, application_number: 'A-7' },
      { id: 8, sender_user_id: 1, application_number: 'A-8' },
    ];
    await flushPromises();

    useOnboardingStore().setRevealOpen('first-application');
    await flushPromises();

    expect(wrapper.vm.showDetailModal).toBe(true);
    expect(wrapper.vm.selectedApplication).toEqual(expect.objectContaining({ id: 7 }));
  });

  // Список приезжает своим запросом: при переходе к шагу карточки из списка шагов
  // сигнал встаёт раньше данных. Раньше попытка была одна - карточка не
  // открывалась, весь сегмент выбрасывался, и счётчик стоял на месте.
  it('заявки приехали после сигнала - карточка всё равно открывается', async () => {
    const wrapper = mountUA();
    await flushPromises();

    useOnboardingStore().setRevealOpen('first-application');
    await flushPromises();
    expect(wrapper.vm.showDetailModal).toBe(false);

    wrapper.vm.applications = [{ id: 9, sender_user_id: 1, application_number: 'A-9' }];
    await flushPromises();

    expect(wrapper.vm.showDetailModal).toBe(true);
    expect(wrapper.vm.selectedApplication).toEqual(expect.objectContaining({ id: 9 }));
  });

  it('гашение сигнала закрывает деталь, открытую туром', async () => {
    const wrapper = mountUA();
    wrapper.vm.applications = [{ id: 7, sender_user_id: 1, application_number: 'A-7' }];
    await flushPromises();

    const store = useOnboardingStore();
    store.setRevealOpen('first-application');
    await flushPromises();
    expect(wrapper.vm.showDetailModal).toBe(true);

    store.setRevealOpen(null);
    await flushPromises();
    expect(wrapper.vm.showDetailModal).toBe(false);
    expect(wrapper.vm.selectedApplication).toBe(null);
  });

  it('деталь, открытую пользователем, тур не закрывает', async () => {
    const wrapper = mountUA();
    wrapper.vm.applications = [{ id: 7, sender_user_id: 1, application_number: 'A-7' }];
    await flushPromises();

    await wrapper.vm.openApplication(wrapper.vm.applications[0]);
    await flushPromises();
    expect(wrapper.vm.showDetailModal).toBe(true);

    const store = useOnboardingStore();
    store.setRevealOpen('first-application');
    await flushPromises();
    store.setRevealOpen(null);
    await flushPromises();

    expect(wrapper.vm.showDetailModal).toBe(true);
  });

  it('пустой список - сигнал ничего не открывает и не падает', async () => {
    const wrapper = mountUA();
    wrapper.vm.applications = [];
    await flushPromises();

    useOnboardingStore().setRevealOpen('first-application');
    await flushPromises();

    expect(wrapper.vm.showDetailModal).toBe(false);
  });

  it('чужая цель раскрытия кабинет не трогает', async () => {
    const wrapper = mountUA();
    wrapper.vm.applications = [{ id: 7, sender_user_id: 1, application_number: 'A-7' }];
    await flushPromises();

    useOnboardingStore().setRevealOpen('admin-column');
    await flushPromises();

    expect(wrapper.vm.showDetailModal).toBe(false);
  });
});
