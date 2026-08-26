import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

import { apiRequest } from '@/api/client';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}));
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import ApplicationHistory from '../ApplicationHistory.vue';

// Форма записи как её отдаёт GET /applications/:id/history: у системного завершения
// (крон, actor NULL) бэк отдаёт user_id 0 - reader COALESCE'ит NULL после LEFT JOIN.
const completedEntry = {
  id: 1,
  application_id: 11,
  action_type: 'completed',
  user_id: 0,
  user_name: '',
  old_value: 'В работе',
  new_value: 'Завершено',
  created_at: '2026-07-15T10:00:00Z',
};

describe('ApplicationHistory - лейбл завершения заявки по сроку (#1240)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockResolvedValue({ ok: true, json: () => Promise.resolve([completedEntry]) });
  });

  const mountHistory = () =>
    mount(ApplicationHistory, {
      props: { applicationId: 11 },
      global: { stubs: { LoaderSpinner: true, teleport: true } },
    });

  it('action_type "completed" -> русский лейбл и dot-success (не сырой "completed")', () => {
    const wrapper = mountHistory();
    expect(wrapper.vm.getActionText({ action_type: 'completed' })).toBe('Заявка завершена: срок действия истёк');
    expect(wrapper.vm.getActionClass('completed')).toBe('dot-success');
  });

  it('завершение пишет крон: актора нет -> строка истории подписана "Система"', async () => {
    const wrapper = mountHistory();
    await wrapper.find('.history-toggle').trigger('click');
    await flushPromises();

    const item = wrapper.find('.history-item');
    expect(item.exists()).toBe(true);
    expect(item.find('.system-name').text()).toBe('Система');
    expect(item.find('.action-text').text()).toBe('Заявка завершена: срок действия истёк');
  });
});
