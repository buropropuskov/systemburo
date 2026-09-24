import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

import { apiRequest } from '@/api/client';
import { useDeletionsStore } from '@/stores/deletions';
import ApplicationDatesEditor from '../ApplicationDatesEditor.vue';

const ATTACHMENTS = [
  { id: 1, entry_date_from: '2099-10-01', entry_date_to: '2099-10-03', entry_time_from: '09:00:00', entry_time_to: '18:00:00' },
];

// Ответ в том виде, в каком его отдаёт обёртка client.js: json() уже развернул конверт.
const okResponse = (data) => ({ ok: true, json: () => Promise.resolve(data) });
const errResponse = (message) => ({ ok: false, json: () => Promise.resolve({ message }) });

function mountEditor(props = {}) {
  return mount(ApplicationDatesEditor, {
    props: {
      application: { id: 42, status: 'В обработке', confirmation: 'Согласование' },
      attachments: ATTACHMENTS,
      isApprover: true,
      ...props,
    },
    global: { stubs: { Teleport: true } },
  });
}

async function openWithReason(wrapper, reason = 'заявитель ошибся датой') {
  await wrapper.find('[data-testid="app-detail-change-dates"]').trigger('click');
  await wrapper.find('[data-testid="dates-editor-reason"]').setValue(reason);
}

describe('ApplicationDatesEditor', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
  });

  it('кнопку видит только принимающий, пока заявка не принята и не согласована', () => {
    expect(mountEditor().find('[data-testid="app-detail-change-dates"]').exists()).toBe(true);
    expect(mountEditor({ isApprover: false }).find('[data-testid="app-detail-change-dates"]').exists()).toBe(false);
    expect(mountEditor({ application: { id: 42, status: 'В работе' } })
      .find('[data-testid="app-detail-change-dates"]').exists()).toBe(false);
    expect(mountEditor({ application: { id: 42, status: 'В обработке', confirmation: 'Согласовано' } })
      .find('[data-testid="app-detail-change-dates"]').exists()).toBe(false);
    expect(mountEditor({ attachments: [] }).find('[data-testid="app-detail-change-dates"]').exists()).toBe(false);
  });

  it('окно открывается с текущим сроком, без причины сохранить нельзя', async () => {
    const wrapper = mountEditor();
    await wrapper.find('[data-testid="app-detail-change-dates"]').trigger('click');

    expect(wrapper.find('[data-testid="dates-editor-current"]').text()).toContain('01.10.2099 09:00 - 03.10.2099 18:00');
    expect(wrapper.find('[data-testid="dates-editor-save"]').attributes('disabled')).toBeDefined();
  });

  it('сохранение шлёт окно в формате сервера и причину, затем эмитит changed', async () => {
    apiRequest.mockResolvedValue(okResponse({
      old_period: '01.10.2099 09:00 - 03.10.2099 18:00',
      new_period: '02.10.2099 08:00 - 04.10.2099 20:00',
      approvals_reset: true,
    }));
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    const wrapper = mountEditor();
    await openWithReason(wrapper, '  заявитель ошибся датой  ');
    Object.assign(wrapper.vm.form, { startDate: '02.10.2099', endDate: '04.10.2099', startTime: '08:00', endTime: '20:00' });

    await wrapper.find('[data-testid="dates-editor-save"]').trigger('click');
    await flushPromises();

    expect(apiRequest).toHaveBeenCalledTimes(1);
    const [url, options] = apiRequest.mock.calls[0];
    expect(url).toBe('/applications/42/dates');
    expect(options.method).toBe('PUT');
    expect(JSON.parse(options.body)).toEqual({
      entry_date_from: '2099-10-02',
      entry_date_to: '2099-10-04',
      entry_time_from: '08:00:00',
      entry_time_to: '20:00:00',
      reason: 'заявитель ошибся датой',
    });
    expect(wrapper.emitted('changed')).toHaveLength(1);
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      type: 'success', bold: '02.10.2099 08:00 - 04.10.2099 20:00', suffix: 'Голоса согласующих сняты.',
    }));
    expect(wrapper.vm.show).toBe(false);
  });

  it('отказ сервера показывается его текстом, окно остаётся открытым', async () => {
    apiRequest.mockResolvedValue(errResponse('Заявка в статусе «В работе»: срок менять нельзя'));
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    const wrapper = mountEditor();
    await openWithReason(wrapper);
    wrapper.vm.form.endDate = '05.10.2099';

    await wrapper.find('[data-testid="dates-editor-save"]').trigger('click');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      type: 'error', bold: 'Заявка в статусе «В работе»: срок менять нельзя',
    }));
    expect(wrapper.emitted('changed')).toBeUndefined();
    expect(wrapper.vm.show).toBe(true);
  });

  it('неверное окно не уходит на сервер, ошибки появляются у полей', async () => {
    const wrapper = mountEditor();
    await openWithReason(wrapper);
    wrapper.vm.form.startDate = '05.10.2099';

    await wrapper.find('[data-testid="dates-editor-save"]').trigger('click');
    await flushPromises();

    expect(apiRequest).not.toHaveBeenCalled();
    expect(wrapper.vm.shownErrors.endDate).toMatch(/раньше/);
  });
});
