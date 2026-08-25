import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

function page(items, total, unreadCount) {
  return {
    ok: true,
    json: () => Promise.resolve({ success: true, data: items, meta: { total, unread_count: unreadCount } }),
  };
}

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve(0) })),
  apiRequestRaw: vi.fn(),
}));
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

import { apiRequest, apiRequestRaw } from '@/api/client';
import UserNotificationsInline from '../UserNotificationsInline.vue';

function mountN() {
  setActivePinia(createPinia());
  return mount(UserNotificationsInline, { attachTo: document.body });
}

describe('UserNotificationsInline — список: дозагрузка/фильтр/прочитать все/повторы (#1748 S7)', () => {
  beforeEach(() => {
    apiRequest.mockClear();
    apiRequest.mockResolvedValue({ ok: true, json: () => Promise.resolve(0) });
    apiRequestRaw.mockReset();
  });

  it('счётчик непрочитанных берётся из meta.unread_count, а не из длины загруженного массива', async () => {
    apiRequestRaw.mockResolvedValue(page([
      { id: 1, is_read: false, title: 'a', message: 'm', created_at: '2026-08-06T10:00:00' },
    ], 200, 50));
    const wrapper = mountN();
    await flushPromises();

    expect(wrapper.vm.unreadCount).toBe(50);
    expect(wrapper.find('.notification-badge').text()).toBe('50');
    wrapper.unmount();
  });

  it('фильтр Непрочитанные шлёт filter=unread серверу, не режет клиентски', async () => {
    apiRequestRaw.mockResolvedValue(page([], 0, 0));
    const wrapper = mountN();
    await flushPromises();

    apiRequestRaw.mockClear();
    apiRequestRaw.mockResolvedValue(page([{ id: 9, is_read: false, title: 'x', message: 'y', created_at: '2026-08-06T10:00:00' }], 1, 1));
    await wrapper.find('[data-testid="notif-filter-unread"]').trigger('click');
    await flushPromises();

    expect(apiRequestRaw).toHaveBeenCalledWith('/notifications?limit=20&offset=0&filter=unread');
    expect(wrapper.vm.notifications).toHaveLength(1);
    wrapper.unmount();
  });

  it('дозагрузка ДОБАВЛЯЕТ вторую страницу, а не заменяет первую', async () => {
    apiRequestRaw.mockResolvedValueOnce(page([
      { id: 1, is_read: true, title: 'p1-a', message: 'm', created_at: '2026-08-06T10:00:00' },
    ], 2, 0));
    const wrapper = mountN();
    await flushPromises();
    expect(wrapper.vm.notifications.map((n) => n.id)).toEqual([1]);

    apiRequestRaw.mockResolvedValueOnce(page([
      { id: 2, is_read: true, title: 'p2-a', message: 'm', created_at: '2026-08-05T10:00:00' },
    ], 2, 0));
    await wrapper.vm.loadMoreNotificationsList(wrapper.vm.buildNotificationsPage);

    expect(apiRequestRaw).toHaveBeenLastCalledWith('/notifications?limit=20&offset=20&filter=all');
    expect(wrapper.vm.notifications.map((n) => n.id)).toEqual([1, 2]);
    wrapper.unmount();
  });

  it('карточка с count>1 показывает счётчик повторов', async () => {
    apiRequestRaw.mockResolvedValue(page([
      { id: 1, is_read: true, title: 'a', message: 'm', created_at: '2026-08-06T10:00:00', count: 4 },
    ], 1, 0));
    const wrapper = mountN();
    await flushPromises();

    expect(wrapper.find('.notification-count').text()).toBe('4');
    wrapper.unmount();
  });
});
