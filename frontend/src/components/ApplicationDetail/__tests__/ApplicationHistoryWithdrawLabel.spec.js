import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) }),
}));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import ApplicationHistory from '../ApplicationHistory.vue';

describe('ApplicationHistory - лейбл отзыва заявки (#951)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('action_type "withdraw" -> русский лейбл и dot-reject (не сырой "withdraw")', () => {
    const wrapper = mount(ApplicationHistory, {
      props: { applicationId: 7 },
      global: { stubs: { LoaderSpinner: true, teleport: true } },
    });
    expect(wrapper.vm.getActionText({ action_type: 'withdraw' })).toBe('Отозвал(-а) заявку');
    expect(wrapper.vm.getActionClass('withdraw')).toBe('dot-reject');
  });
});
