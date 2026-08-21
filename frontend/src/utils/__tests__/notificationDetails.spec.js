import { describe, it, expect, vi, afterEach } from 'vitest';
import {
  parseNotificationData,
  notificationDetailFields,
  notificationCategory,
  notificationCategoryLabel,
  notificationActionLabel,
  notificationDayLabel,
  groupNotificationsByDay,
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
      { key: 'application_number', label: 'Заявка', value: 'A-100', action: 'application' },
      { key: 'forwarded_by', label: 'Передал', value: 'Иванов И.И.', action: null },
      { key: 'status', label: 'Решение', value: 'Согласовано', action: null },
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
    expect(fields).toEqual([{ key: 'application_number', label: 'Заявка', value: 'A-1', action: 'application' }]);
  });

  it('показывает, кто принял решение и с каким комментарием', () => {
    const fields = notificationDetailFields({
      data: JSON.stringify({
        application_id: 7,
        application_number: 'A-7',
        actor_name: 'Петров П.П.',
        decision_comment: 'Нет пропуска на въезд',
      }),
    });
    expect(fields).toEqual([
      { key: 'application_number', label: 'Заявка', value: 'A-7', action: 'application' },
      { key: 'actor_name', label: 'Решение принял', value: 'Петров П.П.', action: null },
      { key: 'decision_comment', label: 'Комментарий', value: 'Нет пропуска на въезд', action: null },
    ]);
  });

  it('неизвестные ключи data не показываются', () => {
    const fields = notificationDetailFields({ data: JSON.stringify({ some_unmapped_key: 'x' }) });
    expect(fields).toEqual([]);
  });

  it('пустые/отсутствующие значения не попадают в результат', () => {
    const fields = notificationDetailFields({
      data: JSON.stringify({ application_number: '', forwarded_by: null, status: 'Отклонено' }),
    });
    expect(fields).toEqual([{ key: 'status', label: 'Решение', value: 'Отклонено', action: null }]);
  });

  it('склонение waiting_days: 1 день, 2 дня, 5 дней', () => {
    expect(notificationDetailFields({ data: JSON.stringify({ waiting_days: 1 }) }))
      .toEqual([{ key: 'waiting_days', label: 'Ожидает решения', value: '1 день', action: null }]);
    expect(notificationDetailFields({ data: JSON.stringify({ waiting_days: 2 }) }))
      .toEqual([{ key: 'waiting_days', label: 'Ожидает решения', value: '2 дня', action: null }]);
    expect(notificationDetailFields({ data: JSON.stringify({ waiting_days: 5 }) }))
      .toEqual([{ key: 'waiting_days', label: 'Ожидает решения', value: '5 дней', action: null }]);
  });

  it('changed_at форматируется в ДД.ММ.ГГГГ ЧЧ:ММ', () => {
    const fields = notificationDetailFields({ data: JSON.stringify({ changed_at: '2026-08-06T14:32:00' }) });
    expect(fields).toEqual([{ key: 'changed_at', label: 'Когда', value: '06.08.2026 14:32', action: null }]);
  });

  it('битый JSON в data не роняет сборку полей - пустой список', () => {
    expect(notificationDetailFields({ data: 'not json' })).toEqual([]);
  });
});

describe('notificationCategory', () => {
  it('passage у application_expiring/withdrawn/passage_first - точное совпадение раньше префикса application_', () => {
    expect(notificationCategory('application_expiring')).toBe('passage');
    expect(notificationCategory('application_withdrawn')).toBe('passage');
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
    // Без записи в перечне тип свалился бы в 'application' и получил чужие иконку
    // и бейдж - молча, потому что запасной вариант всегда что-то возвращает.
    expect(notificationCategory('error_spike')).toBe('system');
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

describe('notificationCategoryLabel', () => {
  it('русская подпись по категории, неизвестная категория -> подпись application', () => {
    expect(notificationCategoryLabel('security')).toBe('Безопасность');
    expect(notificationCategoryLabel('passage')).toBe('Проезд');
    expect(notificationCategoryLabel('unknown')).toBe('Заявка');
  });
});

describe('notificationDayLabel и groupNotificationsByDay', () => {
  const NOW = new Date(2026, 7, 6, 15, 0, 0); // 06.08.2026 15:00 - опорный момент

  afterEach(() => vi.useRealTimers());

  it('Сегодня/Вчера/дата по last_event_at, если оно есть - created_at игнорируется', () => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);

    expect(notificationDayLabel({ last_event_at: '2026-08-06T09:00:00', created_at: '2026-08-01T09:00:00' })).toBe('Сегодня');
    expect(notificationDayLabel({ last_event_at: '2026-08-05T09:00:00' })).toBe('Вчера');
    expect(notificationDayLabel({ last_event_at: '2026-08-01T09:00:00' })).toBe('01.08.2026');
  });

  it('без last_event_at группировка идёт по created_at', () => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    expect(notificationDayLabel({ created_at: '2026-08-06T09:00:00' })).toBe('Сегодня');
  });

  it('без дат вовсе - пустая строка', () => {
    expect(notificationDayLabel({})).toBe('');
    expect(notificationDayLabel(null)).toBe('');
  });

  it('groupNotificationsByDay сегментирует последовательные записи по метке, не меняя порядок', () => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    const items = [
      { id: 1, last_event_at: '2026-08-06T14:00:00' },
      { id: 2, last_event_at: '2026-08-06T10:00:00' },
      { id: 3, last_event_at: '2026-08-05T10:00:00' },
      { id: 4, last_event_at: '2026-08-01T10:00:00' },
    ];
    expect(groupNotificationsByDay(items)).toEqual([
      { label: 'Сегодня', items: [items[0], items[1]] },
      { label: 'Вчера', items: [items[2]] },
      { label: '01.08.2026', items: [items[3]] },
    ]);
  });

  it('пустой список - пустой массив групп', () => {
    expect(groupNotificationsByDay([])).toEqual([]);
    expect(groupNotificationsByDay(undefined)).toEqual([]);
  });
});
