import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// Рядовой отправитель в ЛК (mode=user) не должен дёргать админ-эндпоинты
// /users/all (requireUsers) и /application-approvers (requireAdmin) - иначе 403 и
// generic-тост "Недостаточно прав для этого действия" при открытии своей же заявки.
const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({ apiRequest: (...a) => apiRequest(...a) }));
vi.mock('@/api/applications', () => ({ markAsRead: vi.fn().mockResolvedValue(undefined) }));
vi.mock('@/stores/ui', () => ({ useUiStore: () => ({ confirm: vi.fn() }) }));

import ApplicationDetail from '../ApplicationDetail.vue';

function mountDetail(mode) {
  return shallowMount(ApplicationDetail, {
    props: {
      application: { id: 7, application_number: 'A-7', status: 'В работе', sender_user_id: 1 },
      currentUserId: 1,
      mode,
    },
  });
}

describe('ApplicationDetail - гейт админ-фетчей по режиму', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) });
  });

  it('mode=user: не дёргает /users/all и /application-approvers', async () => {
    mountDetail('user');
    await flushPromises();
    const paths = apiRequest.mock.calls.map(c => c[0]);
    expect(paths).not.toContain('/users/all');
    expect(paths).not.toContain('/application-approvers');
  });

  it('mode=center: дёргает /users/all и /application-approvers с silent403', async () => {
    mountDetail('center');
    await flushPromises();
    const users = apiRequest.mock.calls.find(c => c[0] === '/users/all');
    const approvers = apiRequest.mock.calls.find(c => c[0] === '/application-approvers');
    expect(users).toBeTruthy();
    expect(approvers).toBeTruthy();
    expect(users[1]).toMatchObject({ silent403: true });
    expect(approvers[1]).toMatchObject({ silent403: true });
  });
});
