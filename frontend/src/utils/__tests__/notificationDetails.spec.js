import { describe, it, expect } from 'vitest';
import {
  parseNotificationData,
  notificationDetailFields,
  notificationCategory,
  notificationActionLabel,
} from '../notificationDetails';

describe('parseNotificationData', () => {
  it('разбирает data-строку JSON', () => {
    expect(parseNotificationData({ data: '{"application_id": 7}' })).toEqual({ application_id: 7 });
  });

  it('пропускает data-объект как есть', () => {
    expect(parseNotificationData({ data: { application_id: 9 } })).toEqual({ application_id: 9 });
  });

  it('битый JSON не роняет разбор - пустой объект', () => {
    expect(parseNotificationData({ data: 'битый json' })).toEqual({});
  });

  it('null/undefined data и уведомление целиком null - пустой объект', () => {
    expect(parseNotificationData({ data: null })).toEqual({});
    expect(parseNotificationData(null)).toEqual({});
    expect(parseNotificationData(undefined)).toEqual({});
  });
});

describe('notificationDetailFields', () => {
  it('собирает поля по маппингу в заданном порядке', () => {
    const fields = notificationDetailFields({
      data: JSON.stringify({
        application_number: 'A-100',
        forwarded_by: 'Иванов И.И.',
        status: 'Согласовано',
      }),
    });
    expect(fields).toEqual([
      { label: 'Заявка', value: 'A-100' },
      { label: 'Передал', value: 'Иванов И.И.' },
      { label: 'Решение', value: 'Согласовано' },
    ]);
  });

  it('технические идентификаторы (changed_by_user_id, question_id, application_id) не попадают в поля', () => {
    const fields = notificationDetailFields({
      data: JSON.stringify({
        changed_by_user_id: 5,
        question_id: 12,
        application_id: 42,
        application_number: 'A-1',
      }),
    });
    expect(fields).toEqual([{ label: 'Заявка', value: 'A-1' }]);
  });

  it('неизвестные ключи data не показываются', () => {
    const fields = notificationDetailFields({ data: JSON.stringify({ some_unmapped_key: 'x' }) });
    expect(fields).toEqual([]);
  });

  it('пустые/отсутствующие значения не попадают в результат', () => {
    const fields = notificationDetailFields({
      data: JSON.stringify({ application_number: '', forwarded_by: null, status: 'Отклонено' }),
    });
    expect(fields).toEqual([{ label: 'Решение', value: 'Отклонено' }]);
  });

  it('склонение waiting_days: 1 день, 2 дня, 5 дней', () => {
    expect(notificationDetailFields({ data: JSON.stringify({ waiting_days: 1 }) }))
      .toEqual([{ label: 'Ожидает решения', value: '1 день' }]);
    expect(notificationDetailFields({ data: JSON.stringify({ waiting_days: 2 }) }))
      .toEqual([{ label: 'Ожидает решения', value: '2 дня' }]);
    expect(notificationDetailFields({ data: JSON.stringify({ waiting_days: 5 }) }))
      .toEqual([{ label: 'Ожидает решения', value: '5 дней' }]);
  });

  it('changed_at форматируется в ДД.ММ.ГГГГ ЧЧ:ММ', () => {
    const fields = notificationDetailFields({ data: JSON.stringify({ changed_at: '2026-08-06T14:32:00' }) });
    expect(fields).toEqual([{ label: 'Когда', value: '06.08.2026 14:32' }]);
  });

  it('битый JSON в data не роняет сборку полей - пустой список', () => {
    expect(notificationDetailFields({ data: 'not json' })).toEqual([]);
  });
});

describe('notificationCategory', () => {
  it('passage у application_expiring/withdrawn/acceptor_assigned/passage_first - точное совпадение раньше префикса application_', () => {
    expect(notificationCategory('application_expiring')).toBe('passage');
    expect(notificationCategory('application_withdrawn')).toBe('passage');
    expect(notificationCategory('application_acceptor_assigned')).toBe('passage');
    expect(notificationCategory('application_passage_first')).toBe('passage');
  });

  it('application для настоящих событий заявки', () => {
    expect(notificationCategory('application_created')).toBe('application');
    expect(notificationCategory('application_question')).toBe('application');
  });

  it('security/content/system по точным кодам', () => {
    expect(notificationCategory('password_changed')).toBe('security');
    expect(notificationCategory('user_banned')).toBe('security');
    expect(notificationCategory('news_published')).toBe('content');
    expect(notificationCategory('feedback_answered')).toBe('content');
    expect(notificationCategory('maintenance_scheduled')).toBe('system');
    expect(notificationCategory('trash_restored')).toBe('system');
  });

  it('неизвестный код -> application', () => {
    expect(notificationCategory('something_new')).toBe('application');
    expect(notificationCategory(null)).toBe('application');
    expect(notificationCategory(undefined)).toBe('application');
  });
});

describe('notificationActionLabel', () => {
  it('«Открыть заявку» когда в data есть application_id', () => {
    expect(notificationActionLabel({ data: JSON.stringify({ application_id: 42 }) })).toBe('Открыть заявку');
  });

  it('null когда application_id отсутствует', () => {
    expect(notificationActionLabel({ data: JSON.stringify({ status: 'x' }) })).toBeNull();
    expect(notificationActionLabel({ data: null })).toBeNull();
  });
});
