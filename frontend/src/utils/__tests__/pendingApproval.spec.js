import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { pendingApprovalDays, pendingApprovalLabel, pendingApprovalShort } from '../pendingApproval';

// Опорный момент - 10.07.2026 12:00. Даты подачи задаём относительно него.
const NOW = new Date(2026, 6, 10, 12, 0, 0);
const daysAgo = (n) => new Date(NOW.getTime() - n * 86_400_000).toISOString();

describe('pendingApprovalDays', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
  });
  afterEach(() => vi.useRealTimers());

  it('заявка на согласовании 3 дня -> 3', () => {
    expect(pendingApprovalDays({ confirmation: 'Согласование', status: 'В обработке', sending_datetime: daysAgo(3) })).toBe(3);
  });

  it('свежая заявка того же дня -> null (не шумим бейджем)', () => {
    expect(pendingApprovalDays({ confirmation: 'Согласование', status: 'В обработке', sending_datetime: daysAgo(0) })).toBeNull();
  });

  it('уже согласована -> null', () => {
    expect(pendingApprovalDays({ confirmation: 'Согласовано', status: 'В работе', sending_datetime: daysAgo(5) })).toBeNull();
  });

  it('не согласована -> null', () => {
    expect(pendingApprovalDays({ confirmation: 'Не согласовано', status: 'В обработке', sending_datetime: daysAgo(5) })).toBeNull();
  });

  it.each(['Отозвана', 'Отказано', 'Завершено', 'В работе'])(
    'не-активный статус %s при неснятом confirmation -> null (бейдж не врёт)',
    (status) => {
      expect(pendingApprovalDays({ confirmation: 'Согласование', status, sending_datetime: daysAgo(9) })).toBeNull();
    },
  );

  it.each(['Непрочитано', 'В обработке'])('активный статус %s -> показываем', (status) => {
    expect(pendingApprovalDays({ confirmation: 'Согласование', status, sending_datetime: daysAgo(4) })).toBe(4);
  });

  it('нет даты подачи -> null', () => {
    expect(pendingApprovalDays({ confirmation: 'Согласование', status: 'В обработке' })).toBeNull();
  });

  it('пустой вход -> null', () => {
    expect(pendingApprovalDays(null)).toBeNull();
    expect(pendingApprovalDays({})).toBeNull();
  });
});

describe('pendingApprovalLabel', () => {
  it('склоняет дни', () => {
    expect(pendingApprovalLabel(1)).toBe('Ждёт согласования: 1 день');
    expect(pendingApprovalLabel(2)).toBe('Ждёт согласования: 2 дня');
    expect(pendingApprovalLabel(5)).toBe('Ждёт согласования: 5 дней');
    expect(pendingApprovalLabel(11)).toBe('Ждёт согласования: 11 дней');
    expect(pendingApprovalLabel(21)).toBe('Ждёт согласования: 21 день');
    expect(pendingApprovalLabel(22)).toBe('Ждёт согласования: 22 дня');
  });
});

describe('pendingApprovalShort', () => {
  it('компактная надпись без склонения', () => {
    expect(pendingApprovalShort(1)).toBe('1 дн.');
    expect(pendingApprovalShort(15)).toBe('15 дн.');
  });
});
