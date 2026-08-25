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
import { useUiStore } from '@/stores/ui';
import UserNotifications from '../UserNotifications.vue';

const NOTIFICATIONS = [
  { id: 1, is_read: false, title: 'a', message: 'm', created_at: '2026-08-13T10:00:00' },
  { id: 2, is_read: true, title: 'b', message: 'm', created_at: '2026-08-13T09:00:00' },
];

function mountN() {
  setActivePinia(createPinia());
  return mount(UserNotifications, {
    props: { show: true },
    attachTo: document.body,
    global: { mocks: { $router: { push: vi.fn(() => Promise.resolve()) } } },
  });
}

const deleteCalls = () => apiRequest.mock.calls.filter(([url, opts]) => url === '/notifications' && opts?.method === 'DELETE');

describe('UserNotifications — очистка спрашивает подтверждение (#2058)', () => {
  beforeEach(() => {
    apiRequest.mockClear();
    apiRequest.mockResolvedValue({ ok: true, json: () => Promise.resolve(0) });
    apiRequestRaw.mockReset();
    apiRequestRaw.mockResolvedValue(page(NOTIFICATIONS, 2, 1));
  });

  it('клик по «Очистить» не удаляет ничего, пока человек не ответил на вопрос', async () => {
    const wrapper = mountN();
    await flushPromises();

    await wrapper.find('.notifications__clear-btn').trigger('click');
    await flushPromises();

    const ui = useUiStore();
    expect(ui.confirmState).toBeTruthy();
    expect(ui.confirmState.message).toBe('Все уведомления будут удалены.');
    expect(deleteCalls()).toHaveLength(0);
    expect(wrapper.vm.notifications).toHaveLength(2);
    wrapper.unmount();
  });

  it('подтверждение удаляет уведомления и обнуляет счётчик', async () => {
    const wrapper = mountN();
    await flushPromises();

    await wrapper.find('.notifications__clear-btn').trigger('click');
    await flushPromises();
    useUiStore().resolveConfirm(true);
    await flushPromises();

    expect(deleteCalls()).toHaveLength(1);
    expect(wrapper.vm.notifications).toHaveLength(0);
    expect(wrapper.vm.unreadCount).toBe(0);
    wrapper.unmount();
  });

  it('отказ оставляет список нетронутым', async () => {
    const wrapper = mountN();
    await flushPromises();

    await wrapper.find('.notifications__clear-btn').trigger('click');
    await flushPromises();
    useUiStore().resolveConfirm(false);
    await flushPromises();

    expect(deleteCalls()).toHaveLength(0);
    expect(wrapper.vm.notifications).toHaveLength(2);
    expect(wrapper.vm.unreadCount).toBe(1);
    wrapper.unmount();
  });

  // Панель слушает Escape в фазе перехвата, то есть раньше глобального диалога.
  // Без гейта одно нажатие закрывало бы панель, оставив вопрос висеть над пустым местом.
  it('Escape при открытом вопросе не закрывает панель', async () => {
    const wrapper = mountN();
    await flushPromises();

    await wrapper.find('.notifications__clear-btn').trigger('click');
    await flushPromises();

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await flushPromises();
    expect(wrapper.emitted('close')).toBeUndefined();

    useUiStore().resolveConfirm(false);
    await flushPromises();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await flushPromises();
    expect(wrapper.emitted('close')).toHaveLength(1);
    wrapper.unmount();
  });
});
