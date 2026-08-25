import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })),
  // Список порциями (#1748 S7): apiRequestRaw возвращает envelope с data+meta,
  // apiRequest не годится - не несёт meta.unread_count/total.
  apiRequestRaw: vi.fn(() => Promise.resolve({
    ok: true,
    json: () => Promise.resolve({ success: true, data: [], meta: { total: 0, unread_count: 0 } }),
  })),
}));
// mounted() теперь поднимает real-time подписку (#840); без мока реальный
// eventStream ушёл бы в fetchTicket -> reconnect с фоновым таймером на весь прогон.
vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));
import { apiRequest } from '@/api/client';
import { usePermissionsStore } from '@/stores/permissions';
import UserNotifications from '../UserNotifications.vue';

// mounted() уже запускает свой fetchNotifications() (замокан на []) - он резолвится
// микротаском ПОЗЖЕ synchronous-возврата mount(). Если тестовые уведомления
// присвоить сразу, эта отложенная загрузка потом (на первом же flushPromises)
// молча затирает их пустым массивом. Ждём её здесь ДО присвоения тестовых данных.
async function mountN(notifications, { hasCenter = true } = {}) {
  setActivePinia(createPinia());
  const perms = usePermissionsStore();
  perms.mode = hasCenter ? 'super' : 'normal'; // super -> hasPermission true; normal+пусто -> false
  perms.effective = {};
  const push = vi.fn(() => Promise.resolve());
  const wrapper = mount(UserNotifications, {
    props: { show: false },
    global: { mocks: { $router: { push } } },
  });
  await flushPromises();
  wrapper.vm.notifications = notifications;
  return { wrapper, push };
}

describe('UserNotifications — подробности и навигация по заявке (#973, #1748 S6)', () => {
  beforeEach(() => {
    apiRequest.mockClear();
    apiRequest.mockResolvedValue({ ok: true, json: () => Promise.resolve([]) });
  });

  it('клик по карточке помечает прочитанным и открывает модалку подробностей, не роутит сразу', async () => {
    const { wrapper, push } = await mountN([
      { id: 1, is_read: false, type: 'application_question', data: JSON.stringify({ application_id: 42 }), title: 'Вопрос', message: 'm' },
    ], { hasCenter: true });
    await wrapper.vm.markAsRead(wrapper.vm.notifications[0]);
    await flushPromises();

    expect(push).not.toHaveBeenCalled();
    expect(wrapper.emitted('close')).toBeFalsy();
    expect(wrapper.vm.showDetailModal).toBe(true);
    // toEqual, не toBe: notifications - реактивный массив useInfiniteList, Vue разворачивает
    // reactive-прокси в raw на записи и заново оборачивает на чтении - тот же объект
    // содержательно, но не гарантированно та же ссылка (#1748 S7).
    expect(wrapper.vm.detailNotification).toEqual(wrapper.vm.notifications[0]);
    expect(wrapper.vm.notifications[0].is_read).toBe(true);
  });

  it('уведомление без application_id тоже открывает модалку (подробности доступны любому типу)', async () => {
    const { wrapper } = await mountN([
      { id: 2, is_read: false, type: 'password_changed', data: null, title: 'x', message: 'y' },
    ]);
    await wrapper.vm.markAsRead(wrapper.vm.notifications[0]);
    await flushPromises();

    expect(wrapper.vm.showDetailModal).toBe(true);
  });

  it('кнопка действия модалки: с доступом к Центру роутит в Центр и закрывает панель', async () => {
    const { wrapper, push } = await mountN([
      { id: 1, is_read: false, type: 'application_question', data: JSON.stringify({ application_id: 42 }), title: 'Вопрос', message: 'm' },
    ], { hasCenter: true });
    await wrapper.vm.markAsRead(wrapper.vm.notifications[0]);
    wrapper.vm.handleDetailAction();
    await flushPromises();

    expect(push).toHaveBeenCalledWith({ path: '/center', query: { open: 42 } });
    expect(wrapper.emitted('close')).toBeTruthy();
    expect(wrapper.vm.showDetailModal).toBe(false);
  });

  it('кнопка действия модалки: без доступа к Центру роутит в личный кабинет', async () => {
    const { wrapper, push } = await mountN([
      { id: 3, is_read: false, type: 'application_question', data: JSON.stringify({ application_id: 55 }), title: 'Вопрос', message: 'm' },
    ], { hasCenter: false });
    await wrapper.vm.markAsRead(wrapper.vm.notifications[0]);
    wrapper.vm.handleDetailAction();
    await flushPromises();

    expect(push).toHaveBeenCalledWith({ path: '/personal-cabinet', query: { open: 55 } });
  });

  it('кнопка действия модалки без application_id ничего не делает', async () => {
    const { wrapper, push } = await mountN([
      { id: 2, is_read: false, type: 'password_changed', data: null, title: 'x', message: 'y' },
    ]);
    await wrapper.vm.markAsRead(wrapper.vm.notifications[0]);
    wrapper.vm.handleDetailAction();
    await flushPromises();

    expect(push).not.toHaveBeenCalled();
    expect(wrapper.vm.showDetailModal).toBe(true);
  });

  it('«Вернуть в непрочитанные» шлёт is_read=false и закрывает модалку', async () => {
    const { wrapper } = await mountN([
      { id: 4, is_read: false, type: 'password_changed', data: null, title: 'x', message: 'y' },
    ]);
    await wrapper.vm.markAsRead(wrapper.vm.notifications[0]);
    apiRequest.mockClear();
    apiRequest.mockResolvedValueOnce({ ok: true, json: () => Promise.resolve({}) });
    await wrapper.vm.handleDetailUnread();

    expect(apiRequest).toHaveBeenCalledWith('/notifications/4/read', {
      method: 'PUT',
      body: JSON.stringify({ is_read: false }),
    });
    expect(wrapper.vm.notifications[0].is_read).toBe(false);
    expect(wrapper.vm.showDetailModal).toBe(false);
  });

  it('«Удалить» удаляет уведомление из списка и закрывает модалку', async () => {
    const { wrapper } = await mountN([
      { id: 5, is_read: true, type: 'password_changed', data: null, title: 'x', message: 'y' },
    ]);
    await wrapper.vm.markAsRead(wrapper.vm.notifications[0]);
    apiRequest.mockClear();
    apiRequest.mockResolvedValueOnce({ ok: true, json: () => Promise.resolve({}) });
    await wrapper.vm.handleDetailDelete();

    expect(apiRequest).toHaveBeenCalledWith('/notifications/5', { method: 'DELETE' });
    expect(wrapper.vm.notifications).toEqual([]);
    expect(wrapper.vm.showDetailModal).toBe(false);
  });

  it('реальный DOM-клик по карточке открывает модалку и не роутит сам по себе', async () => {
    setActivePinia(createPinia());
    const push = vi.fn(() => Promise.resolve());
    const wrapper = mount(UserNotifications, {
      props: { show: true },
      global: { mocks: { $router: { push } } },
    });
    await flushPromises();
    wrapper.vm.notifications = [
      { id: 6, is_read: false, type: 'application_question', data: JSON.stringify({ application_id: 9 }), title: 'Вопрос', message: 'm' },
    ];
    await wrapper.vm.$nextTick();

    await wrapper.find('.notification-item').trigger('click');
    await flushPromises();

    expect(push).not.toHaveBeenCalled();
    expect(wrapper.vm.showDetailModal).toBe(true);
    expect(wrapper.findComponent({ name: 'NotificationDetailModal' }).props('show')).toBe(true);
  });

  it('Escape при открытой модалке не закрывает панель уведомлений', async () => {
    const wrapper = mount(UserNotifications, {
      props: { show: true },
      global: { stubs: { teleport: true }, mocks: { $router: { push: vi.fn() } } },
    });
    await flushPromises();
    wrapper.vm.showDetailModal = true;

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await flushPromises();

    expect(wrapper.emitted('close')).toBeFalsy();

    wrapper.vm.showDetailModal = false;
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await flushPromises();
    expect(wrapper.emitted('close')).toBeTruthy();
    wrapper.unmount();
  });
});
