import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) }),
}));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import ApplicationHistory from '../ApplicationHistory.vue';

describe('ApplicationHistory - лейблы голосования по дополнению (#1685)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  const mountHistory = () => mount(ApplicationHistory, {
    props: { applicationId: 12 },
    global: { stubs: { LoaderSpinner: true, teleport: true } },
  });

  // Голоса по дополнению приходят отдельными действиями от голосов по самой заявке -
  // иначе "Согласовал(-а) заявку" в ленте читалось бы как повторное согласование заявки.
  it.each([
    ['supplement_approve', 'Согласовал(-а) дополнение', 'dot-approve'],
    ['supplement_reject', 'Не согласовал(-а) дополнение', 'dot-reject'],
    ['supplement_revoke_approval', 'Отозвал(-а) согласование дополнения', 'dot-revoke'],
    ['supplement_confirmation_change', 'Статус согласования дополнения изменился', 'dot-system'],
  ])('action_type "%s" -> русский лейбл, не сырой ключ', (actionType, text, dotClass) => {
    const wrapper = mountHistory();
    expect(wrapper.vm.getActionText({ action_type: actionType })).toBe(text);
    expect(wrapper.vm.getActionClass(actionType)).toBe(dotClass);
  });
});
