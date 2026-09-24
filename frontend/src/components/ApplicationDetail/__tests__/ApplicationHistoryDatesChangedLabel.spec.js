import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) }),
}));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import ApplicationHistory from '../ApplicationHistory.vue';

describe('ApplicationHistory - правка срока принимающим (#2575)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('action_type "dates_changed" -> русский лейбл, а не сырой ключ', () => {
    const wrapper = mount(ApplicationHistory, {
      props: { applicationId: 7 },
      global: { stubs: { LoaderSpinner: true, teleport: true } },
    });
    expect(wrapper.vm.getActionText({ action_type: 'dates_changed' })).toBe('Изменил(-а) срок заявки');
    expect(wrapper.vm.getActionClass('dates_changed')).toBe('dot-warning');
  });
});
