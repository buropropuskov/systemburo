import { beforeEach, describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/utils/excelSheet', () => ({ downloadExcelSheet: vi.fn().mockResolvedValue(undefined) }));

import ApplicationHistory from '../ApplicationHistory.vue';
import { downloadExcelSheet } from '@/utils/excelSheet';

const recipient = { user_id: 2, display_name: '@test_recipient', purpose: 'approval', required_approval: true, access_granted: true };
const event = metadata => ({ id: 11, user_id: 1, user_name: 'Тестовый отправитель', action_type: 'forwarded', created_at: '2026-01-01T10:00:00Z', comment: 'Комментарий', metadata });
const structured = { recipient_details_available: true, recipient_details: [recipient], recipients: ['Unsafe legacy name'], attachment_scope: 'selected', attachment_details: [{ id: 3, name: 'Снимок приложения' }], whole: false, attachments: ['Wrong current name'] };
const createWrapper = () => mount(ApplicationHistory, {
  props: { applicationId: 7, applicationNumber: 'TEST', applicationOrganization: 'Test' },
  global: { stubs: { LoaderSpinner: true, teleport: true } },
});

describe('История пересылки #2664', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    downloadExcelSheet.mockClear();
  });

  it('показывает назначения и снимок материалов без legacy fallback', async () => {
    const wrapper = createWrapper();
    await wrapper.setData({ showModal: true, history: [event(structured)] });
    expect(wrapper.find('.action-text').text()).toBe('Переслал(-а) заявку');
    expect(wrapper.find('.forward-recipient').text()).toBe('@test_recipient — назначен обязательным согласующим');
    expect(wrapper.find('.forward-materials').text()).toContain('только выбранные приложения — Снимок приложения');
    expect(wrapper.text()).not.toContain('Unsafe legacy name');
    expect(wrapper.text()).not.toContain('Wrong current name');
  });

  it('legacy получатели недоступны и в интерфейсе, и в экспорте', async () => {
    const wrapper = createWrapper();
    await wrapper.setData({ showModal: true, history: [event({ recipients: ['Unsafe legacy name'], whole: true })] });
    expect(wrapper.find('.action-text').text()).toBe('Переслал(-а) заявку');
    expect(wrapper.find('.forward-legacy').text()).toBe('Сведения о получателях недоступны');
    expect(wrapper.text()).not.toContain('Unsafe legacy name');
    await wrapper.vm.exportToExcel();
    const [sheet] = downloadExcelSheet.mock.calls[0];
    expect(sheet.header.slice(-2)).toEqual(['Получатели и назначения', 'Материалы']);
    expect(sheet.rows[0].slice(-2)).toEqual(['Сведения о получателях недоступны', 'заявка со всеми приложениями']);
    expect(JSON.stringify(sheet)).not.toContain('Unsafe legacy name');
  });

  it('экспорт сохраняет безопасные назначения и исторические материалы', async () => {
    const wrapper = createWrapper();
    await wrapper.setData({ history: [event(structured)] });
    await wrapper.vm.exportToExcel();
    const [sheet] = downloadExcelSheet.mock.calls[0];
    expect(sheet.rows[0].slice(-2)).toEqual(['@test_recipient — назначен обязательным согласующим', 'только выбранные приложения — Снимок приложения']);
    expect(sheet.widths).toHaveLength(sheet.header.length);
    expect(sheet.wrapColumns).toEqual([8, 9]);
    expect(sheet.rows[0]).toHaveLength(sheet.header.length);
    expect(JSON.stringify(sheet)).not.toContain('Unsafe legacy name');
  });

  it('имя, материалы и комментарий остаются текстом, не HTML', async () => {
    const markup = '<img src=x onerror=alert(1)>';
    const wrapper = createWrapper();
    await wrapper.setData({ showModal: true, history: [{ ...event({ ...structured, recipient_details: [{ ...recipient, display_name: markup }], attachment_details: [{ id: 3, name: markup }] }), comment: markup }] });
    expect(wrapper.find('.forward-recipient').text()).toContain(markup);
    expect(wrapper.find('.forward-materials').text()).toContain(markup);
    expect(wrapper.find('.action-comment').text()).toBe(markup);
    expect(wrapper.find('img').exists()).toBe(false);
  });
});
