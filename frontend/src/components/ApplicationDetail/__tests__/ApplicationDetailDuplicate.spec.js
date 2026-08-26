import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({ apiRequest: (...a) => apiRequest(...a) }));
vi.mock('@/api/applications', () => ({ markAsRead: vi.fn().mockResolvedValue(undefined) }));
vi.mock('@/stores/ui', () => ({ useUiStore: () => ({ confirm: vi.fn() }) }));

import ApplicationDetail from '../ApplicationDetail.vue';

function mountDetail(application = {}) {
  return shallowMount(ApplicationDetail, {
    props: {
      application: { id: 7, application_number: 'A-7', status: 'В работе', sender_user_id: 1, message: 'msg', ...application },
      currentUserId: 1,
      mode: 'user',
    },
  });
}

// Дубль пишется во временный ключ pendingDuplicateState (не затирая черновик формы, #952).
function draftFrom(setItem) {
  const call = [...setItem.mock.calls].reverse().find(c => c[0] === 'pendingDuplicateState');
  return call ? JSON.parse(call[1]) : null;
}

describe('ApplicationDetail - дублирование с пресетом срока (#952)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) });
    vi.useFakeTimers();
    // 01.07.2026 (локальное время) - завтра 02.07, следующий месяц 01.08-31.08.
    vi.setSystemTime(new Date(2026, 6, 1, 12, 0, 0));
  });
  afterEach(() => { vi.useRealTimers(); });

  it('buildPresetDate: tomorrow -> один день завтра', () => {
    expect(mountDetail().vm.buildPresetDate('tomorrow')).toEqual({
      isOneDay: true, singleDate: '02.07.2026', startDate: '', endDate: '',
    });
  });

  it('buildPresetDate: nextMonth -> весь следующий календарный месяц', () => {
    expect(mountDetail().vm.buildPresetDate('nextMonth')).toEqual({
      isOneDay: false, singleDate: '', startDate: '01.08.2026', endDate: '31.08.2026',
    });
  });

  it('buildPresetDate: other -> null (без даты)', () => {
    expect(mountDetail().vm.buildPresetDate('other')).toBeNull();
  });

  it('duplicateApplication("tomorrow"): дата завтра + время скопировано из исходного вложения', async () => {
    const wrapper = mountDetail();
    await flushPromises();
    await wrapper.setData({
      attachments: [
        { id: 11, attachment_type: 'cars', unique_attachment_id: 1, entry_time_from: '12:00:00', entry_time_to: '19:00:00', attachment_name: 'a' },
      ],
    });
    const setItem = vi.spyOn(Storage.prototype, 'setItem');
    await wrapper.vm.duplicateApplication('tomorrow');

    const draft = draftFrom(setItem);
    const dates = Object.values(draft.attachmentDatesByAttachment);
    expect(dates).toHaveLength(1);
    expect(dates[0]).toMatchObject({
      isOneDay: true, singleDate: '02.07.2026', startTime: '12:00', endTime: '19:00',
    });
    expect(wrapper.emitted('duplicate')).toBeTruthy();
  });

  it('duplicateApplication("nextMonth"): период весь следующий месяц + скопированное время', async () => {
    const wrapper = mountDetail();
    await flushPromises();
    await wrapper.setData({
      attachments: [
        { id: 11, attachment_type: 'cars', unique_attachment_id: 1, entry_time_from: '08:30:00', entry_time_to: '17:45:00' },
      ],
    });
    const setItem = vi.spyOn(Storage.prototype, 'setItem');
    await wrapper.vm.duplicateApplication('nextMonth');

    const dates = Object.values(draftFrom(setItem).attachmentDatesByAttachment);
    expect(dates[0]).toMatchObject({
      isOneDay: false, startDate: '01.08.2026', endDate: '31.08.2026', startTime: '08:30', endTime: '17:45',
    });
  });

  it('duplicateApplication("other"): дублирует без даты (attachmentDatesByAttachment пуст)', async () => {
    const wrapper = mountDetail();
    await flushPromises();
    await wrapper.setData({
      attachments: [{ id: 11, attachment_type: 'cars', unique_attachment_id: 1, entry_time_from: '12:00:00', entry_time_to: '19:00:00' }],
    });
    const setItem = vi.spyOn(Storage.prototype, 'setItem');
    await wrapper.vm.duplicateApplication('other');

    expect(draftFrom(setItem).attachmentDatesByAttachment).toEqual({});
  });
});
