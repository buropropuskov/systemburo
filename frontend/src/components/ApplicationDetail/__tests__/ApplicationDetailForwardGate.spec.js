import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// Гейт кнопки "Переслать": режим "Центр" + право action.forward.application + доступ
// к заявке. Здесь проверяются mode и отсутствие доступа; полный разбор ролей, у кого
// доступ есть, - в ApplicationDetailForwardAccess.spec.js (#1948).
// Право ниже выдаётся во всех тестах, чтобы изолировать именно role/mode-gate (#680).

import { usePermissionsStore } from '@/stores/permissions';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({
  markAsRead: vi.fn().mockResolvedValue(undefined),
}));

import ApplicationDetail from '../ApplicationDetail.vue';

const FORWARD = '[data-testid="app-detail-button-forward"]';

/** Выдаёт право пересылки в свежем сторе - тесты проверяют именно role/mode-gate. */
function grantForward() {
  const perms = usePermissionsStore();
  perms.effective = { 'action.forward.application': { value: 'allow', source: 'role' } };
}

async function mountDetail(props = {}, data = {}) {
  grantForward();
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

  // Раньше здесь ждали обратного (#680: гейт = только ответственный), но состав
  // application_approvers - это ПРИНИМАЮЩИЕ, и заявку они видят все: с #1948 гейт
  // пересылки сравнялся с гейтом доступа, и кнопка им положена.
  it('принимающий (не ответственный) кнопку видит', async () => {
    const wrapper = await mountDetail({}, { responsibleUsers: [{ id: 2 }], approvers: [{ user_id: 1 }] });
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.isApprover).toBe(true);
    expect(wrapper.find(FORWARD).exists()).toBe(true);
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
