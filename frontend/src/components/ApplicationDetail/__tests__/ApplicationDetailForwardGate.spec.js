import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// #680 срез role-gate: кнопку "Переслать" видит только ответственный по заявке
// (Принимающий). Согласующий (глобальная роль, видит все заявки) и обычный читатель
// её не видят - BE-проверка forward пускает только sender||responsible.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({
  markAsRead: vi.fn().mockResolvedValue(undefined),
}));

import ApplicationDetail from '../ApplicationDetail.vue';

const FORWARD = '[data-testid="app-detail-button-forward"]';

async function mountDetail(props = {}, data = {}) {
  const wrapper = shallowMount(ApplicationDetail, {
    props: {
      application: { id: 7, application_number: 'A-7', status: 'Непрочитано' },
      currentUserId: 1,
      mode: 'center',
      ...props,
    },
  });
  await wrapper.setData({ responsibleUsers: [], approvers: [], ...data });
  return wrapper;
}

describe('ApplicationDetail - гейт кнопки "Переслать" (#680 role-gate)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('ответственный по заявке видит кнопку', async () => {
    const wrapper = await mountDetail({}, { responsibleUsers: [{ id: 1, approval_status: 'pending' }] });
    await wrapper.vm.$nextTick();
    expect(wrapper.find(FORWARD).exists()).toBe(true);
  });

  it('согласующий (не ответственный) кнопку не видит - forward вернул бы 403', async () => {
    const wrapper = await mountDetail({}, { responsibleUsers: [{ id: 2 }], approvers: [{ user_id: 1 }] });
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.isApprover).toBe(true);
    expect(wrapper.find(FORWARD).exists()).toBe(false);
  });

  it('посторонний пользователь кнопку не видит', async () => {
    const wrapper = await mountDetail({}, { responsibleUsers: [{ id: 2 }] });
    await wrapper.vm.$nextTick();
    expect(wrapper.find(FORWARD).exists()).toBe(false);
  });

  it('вне режима "Центр" кнопки нет даже у ответственного', async () => {
    const wrapper = await mountDetail({ mode: 'my' }, { responsibleUsers: [{ id: 1 }] });
    await wrapper.vm.$nextTick();
    expect(wrapper.find(FORWARD).exists()).toBe(false);
  });
});
