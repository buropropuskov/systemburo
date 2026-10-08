import { describe, it, expect } from 'vitest';
import { forwardRecipients, forwardActionText, forwardRecipientsText, forwardMaterialsText, FORWARD_RECIPIENTS_UNAVAILABLE } from '../applicationForwardHistory';

const approval = { user_id: 1, display_name: 'Тестовый согласующий', purpose: 'approval', required_approval: true, access_granted: true };
const viewer = { user_id: 2, display_name: '@test_viewer', purpose: 'view', required_approval: false, access_granted: true };
const event = recipients => ({ recipient_details_available: true, recipient_details: recipients });

describe('Безопасное представление пересылки #2664', () => {
  it('сохраняет единый заголовок, различая назначения получателей', () => {
    expect(forwardActionText(event([approval]))).toBe('Переслал(-а) заявку');
    expect(forwardActionText(event([viewer]))).toBe('Переслал(-а) заявку');
    expect(forwardActionText(event([approval, viewer]))).toBe('Переслал(-а) заявку');
    expect(forwardRecipientsText(event([approval, { ...viewer, access_granted: false }]))).toBe('Тестовый согласующий — назначен обязательным согласующим\n@test_viewer — повторно направлено для просмотра');
    expect(forwardRecipientsText(event([{ ...approval, required_approval: false }]))).toContain('— назначен согласующим');
  });

  it.each([
    { recipients: ['Raw legacy name'] },
    { ...event([approval]), recipient_details_available: false },
    event([{ ...approval, purpose: 'unknown' }]),
    event([{ ...approval, access_granted: undefined }]),
    event([{ ...approval, display_name: '' }]),
    event([approval, approval]),
    event([{ ...viewer, required_approval: true }]),
    event([{ ...approval, access_granted: false }]),
  ])('не выводит raw recipients для legacy/некорректных DTO %#', data => {
    data.recipients = ['Raw legacy name'];
    expect(forwardRecipients(data)).toEqual([]);
    expect(forwardRecipientsText(data)).toBe(FORWARD_RECIPIENTS_UNAVAILABLE);
    expect(forwardActionText(data)).toBe('Переслал(-а) заявку');
  });

  it('показывает все материалы или снимки выбранных, не подменяя пустой selected всей заявкой', () => {
    expect(forwardMaterialsText({ attachment_scope: 'all', attachments: ['Old'] })).toBe('заявка со всеми приложениями');
    expect(forwardMaterialsText({ attachment_scope: 'selected', whole: true, attachment_details: [{ id: 9, name: 'Снимок' }], attachments: ['Other'] })).toBe('только выбранные приложения — Снимок');
    expect(forwardMaterialsText({ whole: false, attachments: ['Снимок'] })).toBe('только выбранные приложения — Снимок');
    expect(forwardMaterialsText({ attachment_scope: 'selected', attachments: [] })).toBe('только выбранные приложения — сведения недоступны');
    expect(forwardMaterialsText({})).toBe('Сведения о материалах недоступны');
  });
});
