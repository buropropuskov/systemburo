import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({ apiRequest: (...a) => apiRequest(...a) }));
vi.mock('@/api/applications', () => ({ markAsRead: vi.fn().mockResolvedValue(undefined) }));

// Глобальный confirm-диалог: подтверждение отзыва берём из ui-стора (#951).
const confirmMock = vi.fn();
vi.mock('@/stores/ui', () => ({ useUiStore: () => ({ confirm: confirmMock }) }));

import { useDeletionsStore } from '@/stores/deletions';
import ApplicationDetail from '../ApplicationDetail.vue';

function mountDetail(application = {}) {
  return shallowMount(ApplicationDetail, {
    props: {
      application: { id: 7, application_number: 'A-7', status: 'Непрочитано', sender_user_id: 1, ...application },
      currentUserId: 1,
      mode: 'user',
    },
  });
}

describe('ApplicationDetail - отзыв своей заявки (#951)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) });
    confirmMock.mockReset();
  });

  it('canWithdraw: отправитель + не терминальный статус -> true', () => {
    expect(mountDetail().vm.canWithdraw).toBe(true);
  });

  it('canWithdraw: не отправитель -> false', () => {
    expect(mountDetail({ sender_user_id: 2 }).vm.canWithdraw).toBe(false);
  });

  it('canWithdraw: терминальные статусы -> false', () => {
    for (const status of ['Завершено', 'Не согласовано', 'Отказано', 'Отозвана']) {
      expect(mountDetail({ status }).vm.canWithdraw, status).toBe(false);
    }
  });

  it('withdrawApplication: подтверждение -> POST /withdraw + emit("withdraw")', async () => {
    confirmMock.mockResolvedValue(true);
    const wrapper = mountDetail();
    await wrapper.vm.withdrawApplication();
    expect(apiRequest).toHaveBeenCalledWith('/applications/7/withdraw', { method: 'POST' });
    expect(wrapper.emitted('withdraw')).toBeTruthy();
  });

  it('withdrawApplication: отказ в подтверждении -> запроса нет и нет emit', async () => {
    confirmMock.mockResolvedValue(false);
    const wrapper = mountDetail();
    apiRequest.mockClear();
    await wrapper.vm.withdrawApplication();
    expect(apiRequest).not.toHaveBeenCalledWith('/applications/7/withdraw', { method: 'POST' });
    expect(wrapper.emitted('withdraw')).toBeFalsy();
  });

  it('withdrawApplication: ошибка от бэка читается из data.message (не data.error)', async () => {
    confirmMock.mockResolvedValue(true);
    // apiRequest при !success возвращает { message } (wrapJsonUnwrap), не { error }.
    apiRequest.mockResolvedValue({ ok: false, json: () => Promise.resolve({ message: 'Заявку в этом статусе отозвать нельзя' }) });
    const wrapper = mountDetail();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    await wrapper.vm.withdrawApplication();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'Заявку в этом статусе отозвать нельзя', type: 'error' }));
    expect(wrapper.emitted('withdraw')).toBeFalsy();
  });
});
