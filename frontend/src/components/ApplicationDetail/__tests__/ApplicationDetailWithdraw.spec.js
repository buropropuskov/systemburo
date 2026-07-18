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

describe('ApplicationDetail - отображение отозванной заявки (#951)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) });
    confirmMock.mockReset();
  });

  it('блок "Статус заявки" остаётся после отзыва, статус сырой "Отозвана" (без "инициатором")', () => {
    const wrapper = mountDetail({
      status: 'Отозвана',
      responsible_user_id: 3,
      responsible_name: 'Иванов И.',
      confirmation_datetime: '2026-07-01T10:00:00Z',
    });
    const text = wrapper.text();
    expect(text).toContain('Статус заявки');
    expect(text).toContain('Отозвана');
    // "Отозвана инициатором" - только в шапке (ActionBar), не в блоке "Статус заявки".
    expect(text).not.toContain('инициатором');
    expect(text).toContain('Принял(-а):');
    expect(text).toContain('Иванов И.');
  });

  it('отозванная без принятия: блок статуса виден, но без "Принял(-а)"', () => {
    const wrapper = mountDetail({ status: 'Отозвана', responsible_user_id: null });
    const text = wrapper.text();
    expect(text).toContain('Отозвана');
    expect(text).not.toContain('Принял(-а):');
  });

  it('canLeaveComment/canForwardApplication: отозванную нельзя комментировать/переслать', async () => {
    // Ставим сценарий, где иначе действия были бы доступны (ответственный, ещё не голосовал).
    const wrapper = mountDetail({ status: 'Отозвана' });
    await wrapper.setData({ responsibleUsers: [{ id: 1, approval_status: 'pending' }] });
    expect(wrapper.vm.isResponsibleUser).toBe(true);
    expect(wrapper.vm.canLeaveComment).toBe(false);
    expect(wrapper.vm.canForwardApplication).toBe(false);
  });

  it('canLeaveComment: на ВСЕХ терминальных статусах false у ответственного без голоса (#1097 - баг «Завершено»)', async () => {
    // Раньше проверялась только «Отозвана», поэтому на «Завершено»/«Отказано»/
    // «Не согласовано» ответственный-без-голоса проходил ветку return !hasUserVoted = true
    // и видел поле комментария на закрытой заявке.
    for (const status of ['Завершено', 'Не согласовано', 'Отказано', 'Отозвана']) {
      const wrapper = mountDetail({ status });
      await wrapper.setData({ responsibleUsers: [{ id: 1, approval_status: 'pending' }] });
      expect(wrapper.vm.isResponsibleUser, status).toBe(true);
      expect(wrapper.vm.canLeaveComment, status).toBe(false);
    }
  });

  it('контроль: у не-отозванной те же действия доступны', async () => {
    const wrapper = mountDetail({ status: 'Согласование' });
    await wrapper.setData({ responsibleUsers: [{ id: 1, approval_status: 'pending' }] });
    expect(wrapper.vm.canLeaveComment).toBe(true);
    expect(wrapper.vm.canForwardApplication).toBe(true);
  });
});
