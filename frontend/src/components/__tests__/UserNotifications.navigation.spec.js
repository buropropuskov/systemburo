import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })),
}));
import { apiRequest } from '@/api/client';
import UserNotifications from '../UserNotifications.vue';

function mountN(notifications) {
  const push = vi.fn(() => Promise.resolve());
  const wrapper = mount(UserNotifications, {
    props: { show: false },
    global: { mocks: { $router: { push } } },
  });
  wrapper.vm.notifications = notifications;
  return { wrapper, push };
}

describe('UserNotifications — навигация по заявке (#973)', () => {
  beforeEach(() => {
    apiRequest.mockClear();
    apiRequest.mockResolvedValue({ ok: true, json: () => Promise.resolve([]) });
  });

  it('клик по уведомлению о заявке открывает её в Центре и закрывает дропдаун', async () => {
    const { wrapper, push } = mountN([
      { id: 1, is_read: false, type: 'application_question', data: JSON.stringify({ application_id: 42 }), title: 'Вопрос', message: 'm' },
    ]);
    await wrapper.vm.markAsRead(wrapper.vm.notifications[0]);
    await flushPromises();

    expect(push).toHaveBeenCalledWith({ path: '/center', query: { open: 42 } });
    expect(wrapper.emitted('close')).toBeTruthy();
  });

  it('уведомление без application_id не роутит', async () => {
    const { wrapper, push } = mountN([
      { id: 2, is_read: false, type: 'password_changed', data: null, title: 'x', message: 'y' },
    ]);
    await wrapper.vm.markAsRead(wrapper.vm.notifications[0]);
    await flushPromises();

    expect(push).not.toHaveBeenCalled();
  });

  it('notificationAppId парсит data (строку/объект/битый JSON)', () => {
    const { wrapper } = mountN([]);
    expect(wrapper.vm.notificationAppId({ data: '{"application_id": 7}' })).toBe(7);
    expect(wrapper.vm.notificationAppId({ data: { application_id: 9 } })).toBe(9);
    expect(wrapper.vm.notificationAppId({ data: 'битый json' })).toBe(null);
    expect(wrapper.vm.notificationAppId({ data: null })).toBe(null);
  });
});
