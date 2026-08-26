import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) }),
}));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import ApplicationHistory from '../ApplicationHistory.vue';

describe('ApplicationHistory - лейбл снятия дополнения (#1685)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('action_type "supplement_cancelled" -> русский лейбл и dot-warning (не сырой ключ)', () => {
    const wrapper = mount(ApplicationHistory, {
      props: { applicationId: 11 },
      global: { stubs: { LoaderSpinner: true, teleport: true } },
    });
    expect(wrapper.vm.getActionText({ action_type: 'supplement_cancelled' }))
      .toBe('Дополнение снято: заявка закрыта');
    expect(wrapper.vm.getActionClass('supplement_cancelled')).toBe('dot-warning');
  });
});
