import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// Данные окна пересылки (кандидаты в получатели и состав принимающих) нужны только в
// "Центре": в ЛК окна нет, а /application-approvers закрыт правом администратора -
// рядовой отправитель получил бы 403 и generic-тост при открытии своей же заявки.
// Источник получателей зависит от права (#1948): без page.admin.users идут неадминские
// кандидаты - на /users/all такой участник получал 403 и пустой список. Права в этой
// спеке никому не выданы, поэтому в "Центре" ждём именно кандидатов; выбор источника по
// праву разобран в ApplicationDetailForwardAccess.spec.js.
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

  it('mode=user: не дёргает списки окна пересылки', async () => {
    mountDetail('user');
    await flushPromises();
    const paths = apiRequest.mock.calls.map(c => c[0]);
    expect(paths).not.toContain('/users/recipient-candidates');
    expect(paths).not.toContain('/users/all');
    expect(paths).not.toContain('/application-approvers');
  });

  it('mode=center: дёргает кандидатов в получатели и /application-approvers с silent403', async () => {
    mountDetail('center');
    await flushPromises();
    const users = apiRequest.mock.calls.find(c => c[0] === '/users/recipient-candidates');
    const approvers = apiRequest.mock.calls.find(c => c[0] === '/application-approvers');
    expect(users).toBeTruthy();
    expect(approvers).toBeTruthy();
    expect(users[1]).toMatchObject({ silent403: true });
    expect(approvers[1]).toMatchObject({ silent403: true });
  });
});
