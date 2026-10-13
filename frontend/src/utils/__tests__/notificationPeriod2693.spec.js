import { describe, expect, it } from 'vitest';
import { notificationMessageText } from '../notificationDetails';
const period = '01.02.2030 00:00 - 15.02.2030 23:59';
const legacy = (subject, before, after, reason = 'По обращению') => ({ type: 'application_dates_changed', message: 'Изменён срок ' + subject + ': было ' + before + ': ' + period + ', стало ' + after + ': ' + period + '. Причина: ' + reason });
describe('period notification presentation #2693', () => {
  it.each(['человека', 'машины'])('translates known system fields for %s without changing the stored object', subject => {
    const input = legacy(subject, 'inherit', 'individual', 'inherit: оставить комментарий\nindividual: тоже');
    const original = input.message;
    expect(notificationMessageText(input)).toBe(original.replace('было inherit:', 'было срок вложения:').replace('стало individual:', 'стало индивидуальный срок:'));
    expect(input.message).toBe(original);
  });
  it('supports reverting to inheritance and unlimited periods', () => {
    const input = legacy('машины', 'individual', 'inherit');
    input.message = input.message.replaceAll(period, 'без ограничения срока');
    expect(notificationMessageText(input)).toContain('стало срок вложения: без ограничения срока');
  });
  it('does not modify another type, an arbitrary message or an unknown mode', () => {
    for (const input of [{ ...legacy('человека', 'inherit', 'individual'), type: 'news' }, { type: 'application_dates_changed', message: 'Комментарий individual: текст' }, legacy('машины', 'other', 'individual')]) expect(notificationMessageText(input)).toBe(input.message);
  });
  it('passes through new readable messages and missing values', () => {
    expect(notificationMessageText({ type: 'application_dates_changed', message: 'Изменён срок машины: было срок вложения: ' + period })).toContain('срок вложения');
    expect(notificationMessageText(null)).toBe('');
  });
});
