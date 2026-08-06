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
import UserNotifications from '../UserNotifications.vue';
import NotificationCategoryDot from '@/components/notifications/NotificationCategoryDot.vue';

function mountN() {
  setActivePinia(createPinia());
  return mount(UserNotifications, {
    props: { show: true },
    attachTo: document.body,
    global: { mocks: { $router: { push: vi.fn(() => Promise.resolve()) } } },
  });
}

describe('UserNotifications — список: дозагрузка/фильтр/прочитать все/повторы (#1748 S7)', () => {
  beforeEach(() => {
    apiRequest.mockClear();
    apiRequest.mockResolvedValue({ ok: true, json: () => Promise.resolve(0) });
    apiRequestRaw.mockReset();
  });

  it('счётчик непрочитанных берётся из meta.unread_count, а не из длины загруженного массива', async () => {
    // Массив несёт 2 элемента (оба непрочитаны), meta.unread_count = 50 - реальное
    // число по всему набору пользователя, не совпадающее с длиной загруженной страницы.
    apiRequestRaw.mockResolvedValue(page([
      { id: 1, is_read: false, title: 'a', message: 'm', created_at: '2026-08-06T10:00:00' },
      { id: 2, is_read: false, title: 'b', message: 'm', created_at: '2026-08-06T09:00:00' },
    ], 200, 50));

    const wrapper = mountN();
    await flushPromises();

    expect(wrapper.vm.unreadCount).toBe(50);
    expect(wrapper.find('.notifications__unread-count').text()).toContain('50');
    wrapper.unmount();
  });

  it('переключение фильтра шлёт новый запрос с параметром filter и offset=0 (не режет клиентски)', async () => {
    apiRequestRaw.mockResolvedValue(page([], 0, 0));
    const wrapper = mountN();
    await flushPromises();

    apiRequestRaw.mockClear();
    apiRequestRaw.mockResolvedValue(page([{ id: 9, is_read: false, title: 'x', message: 'y', created_at: '2026-08-06T10:00:00' }], 1, 1));
    wrapper.vm.setFilter('unread');
    await flushPromises();

    expect(apiRequestRaw).toHaveBeenCalledWith('/notifications?limit=20&offset=0&filter=unread');
    expect(wrapper.vm.notifications).toHaveLength(1);
    wrapper.unmount();
  });

  it('реальный клик по вкладке «Непрочитанные» тоже шлёт запрос с filter (не только прямой вызов setFilter)', async () => {
    apiRequestRaw.mockResolvedValue(page([], 0, 0));
    const wrapper = mountN();
    await flushPromises();

    apiRequestRaw.mockClear();
    apiRequestRaw.mockResolvedValue(page([{ id: 5, is_read: false, title: 'x', message: 'y', created_at: '2026-08-06T10:00:00' }], 1, 1));
    await wrapper.find('[data-testid="notif-filter-unread"]').trigger('click');
    await flushPromises();

    expect(apiRequestRaw).toHaveBeenCalledWith('/notifications?limit=20&offset=0&filter=unread');
    expect(wrapper.vm.notifications).toHaveLength(1);
    wrapper.unmount();
  });

  it('повторный вызов setFilter тем же значением не шлёт лишний запрос', async () => {
    apiRequestRaw.mockResolvedValue(page([], 0, 0));
    const wrapper = mountN();
    await flushPromises();
    apiRequestRaw.mockClear();
    wrapper.vm.setFilter('all');
    await flushPromises();
    expect(apiRequestRaw).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it('«Прочитать все» гасит счётчик и локально помечает загруженные элементы', async () => {
    apiRequestRaw.mockResolvedValue(page([
      { id: 1, is_read: false, title: 'a', message: 'm', created_at: '2026-08-06T10:00:00' },
      { id: 2, is_read: false, title: 'b', message: 'm', created_at: '2026-08-06T09:00:00' },
    ], 2, 2));
    const wrapper = mountN();
    await flushPromises();
    expect(wrapper.vm.unreadCount).toBe(2);

    apiRequest.mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(2) });
    await wrapper.vm.markAllRead();

    expect(apiRequest).toHaveBeenCalledWith('/notifications/read-all', { method: 'PUT' });
    expect(wrapper.vm.unreadCount).toBe(0);
    expect(wrapper.vm.notifications.every((n) => n.is_read)).toBe(true);
    wrapper.unmount();
  });

  it('«Прочитать все» не рисуется когда непрочитанных нет', async () => {
    apiRequestRaw.mockResolvedValue(page([{ id: 1, is_read: true, title: 'a', message: 'm', created_at: '2026-08-06T10:00:00' }], 1, 0));
    const wrapper = mountN();
    await flushPromises();
    expect(wrapper.find('.notifications__read-all-btn').exists()).toBe(false);
    wrapper.unmount();
  });

  it('дозагрузка ДОБАВЛЯЕТ вторую страницу, а не заменяет первую', async () => {
    apiRequestRaw.mockResolvedValueOnce(page([
      { id: 1, is_read: true, title: 'p1-a', message: 'm', created_at: '2026-08-06T10:00:00' },
      { id: 2, is_read: true, title: 'p1-b', message: 'm', created_at: '2026-08-06T10:00:00' },
    ], 4, 0));
    const wrapper = mountN();
    await flushPromises();
    expect(wrapper.vm.notifications.map((n) => n.id)).toEqual([1, 2]);
    expect(wrapper.vm.hasMoreNotifications).toBe(true);

    apiRequestRaw.mockResolvedValueOnce(page([
      { id: 3, is_read: true, title: 'p2-a', message: 'm', created_at: '2026-08-05T10:00:00' },
      { id: 4, is_read: true, title: 'p2-b', message: 'm', created_at: '2026-08-05T10:00:00' },
    ], 4, 0));
    await wrapper.vm.loadMoreNotificationsList(wrapper.vm.buildNotificationsPage);

    expect(apiRequestRaw).toHaveBeenLastCalledWith('/notifications?limit=20&offset=20&filter=all');
    expect(wrapper.vm.notifications.map((n) => n.id)).toEqual([1, 2, 3, 4]);
    expect(wrapper.vm.hasMoreNotifications).toBe(false);
    wrapper.unmount();
  });

  it('карточка с count>1 показывает счётчик повторов, count=1 - не показывает', async () => {
    apiRequestRaw.mockResolvedValue(page([
      { id: 1, is_read: true, title: 'a', message: 'm', created_at: '2026-08-06T10:00:00', count: 3, type: 'application_created' },
      { id: 2, is_read: true, title: 'b', message: 'm', created_at: '2026-08-06T09:00:00', count: 1, type: 'password_changed' },
    ], 2, 0));
    const wrapper = mountN();
    await flushPromises();

    const items = wrapper.findAll('.notification-item');
    expect(items[0].find('.notification-item__count').text()).toBe('3');
    expect(items[1].find('.notification-item__count').exists()).toBe(false);
    wrapper.unmount();
  });

  it('карточка несёт цветовую метку категории по типу уведомления', async () => {
    apiRequestRaw.mockResolvedValue(page([
      { id: 1, is_read: true, title: 'a', message: 'm', created_at: '2026-08-06T10:00:00', type: 'password_changed' },
    ], 1, 0));
    const wrapper = mountN();
    await flushPromises();

    const dot = wrapper.findComponent(NotificationCategoryDot);
    expect(dot.exists()).toBe(true);
    expect(dot.props('type')).toBe('password_changed');
    wrapper.unmount();
  });

  it('разделители дней расставлены группировкой по last_event_at/created_at', async () => {
    apiRequestRaw.mockResolvedValue(page([
      { id: 1, is_read: true, title: 'today', message: 'm', created_at: new Date().toISOString() },
      { id: 2, is_read: true, title: 'old', message: 'm', created_at: '2020-01-01T10:00:00' },
    ], 2, 0));
    const wrapper = mountN();
    await flushPromises();

    const headers = wrapper.findAll('.notification-day-header');
    expect(headers.length).toBe(2);
    expect(headers[0].text()).toBe('Сегодня');
    expect(headers[1].text()).toBe('01.01.2020');
    wrapper.unmount();
  });
});
