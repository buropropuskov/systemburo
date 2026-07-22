import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
  pendingApprovalDays,
  pendingApprovalLabel,
  pendingApprovalShort,
  isAwaitingApproval,
  approverSilenceDays,
  approverSilenceLabel,
} from '../pendingApproval';

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

describe('isAwaitingApproval', () => {
  it.each(['Непрочитано', 'В обработке'])('на согласовании + активный статус %s -> true', (status) => {
    expect(isAwaitingApproval({ confirmation: 'Согласование', status })).toBe(true);
  });

  it('уже согласована -> false', () => {
    expect(isAwaitingApproval({ confirmation: 'Согласовано', status: 'В работе' })).toBe(false);
  });

  it.each(['Отозвана', 'Отказано', 'Завершено', 'В работе'])(
    'не-активный статус %s при неснятом confirmation -> false',
    (status) => {
      expect(isAwaitingApproval({ confirmation: 'Согласование', status })).toBe(false);
    },
  );

  it('пустой вход -> false', () => {
    expect(isAwaitingApproval(null)).toBe(false);
    expect(isAwaitingApproval({})).toBe(false);
  });
});

describe('approverSilenceDays', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
  });
  afterEach(() => vi.useRealTimers());

  it('назначен 4 дня назад -> 4', () => {
    expect(approverSilenceDays({ created_at: daysAgo(4) })).toBe(4);
  });

  it('назначен сегодня -> null (шум не рисуем)', () => {
    expect(approverSilenceDays({ created_at: daysAgo(0) })).toBeNull();
  });

  it('нет даты назначения / кривая дата -> null', () => {
    expect(approverSilenceDays({})).toBeNull();
    expect(approverSilenceDays({ created_at: 'не-дата' })).toBeNull();
    expect(approverSilenceDays(null)).toBeNull();
  });
});

describe('approverSilenceLabel', () => {
  it('без напоминаний - только дни', () => {
    expect(approverSilenceLabel(1, 0)).toBe('Не отвечает 1 день');
    expect(approverSilenceLabel(5, 0)).toBe('Не отвечает 5 дней');
  });

  it('с напоминаниями - склоняет и дни, и "раз"', () => {
    expect(approverSilenceLabel(3, 1)).toBe('Не отвечает 3 дня, напомнили 1 раз');
    expect(approverSilenceLabel(7, 2)).toBe('Не отвечает 7 дней, напомнили 2 раза');
    expect(approverSilenceLabel(21, 5)).toBe('Не отвечает 21 день, напомнили 5 раз');
    expect(approverSilenceLabel(4, 11)).toBe('Не отвечает 4 дня, напомнили 11 раз');
    expect(approverSilenceLabel(4, 22)).toBe('Не отвечает 4 дня, напомнили 22 раза');
  });
});
