import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) }),
}));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import ApplicationHistory from '../ApplicationHistory.vue';

describe('ApplicationHistory - лейбл начала обсуждения по заявке (#973)', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('question_created -> русский лейбл с темой (не сырой "question_created")', () => {
    const wrapper = mount(ApplicationHistory, {
      props: { applicationId: 7 },
      global: { stubs: { LoaderSpinner: true, teleport: true } },
    });
    expect(wrapper.vm.getActionText({ action_type: 'question_created', metadata: { subject: 'Прицеп у фуры' } }))
      .toBe('Начал(-а) обсуждение: Прицеп у фуры');
    expect(wrapper.vm.getActionText({ action_type: 'question_created' })).toBe('Начал(-а) обсуждение');
    expect(wrapper.vm.getActionClass('question_created')).toBe('dot-info');
  });
});
