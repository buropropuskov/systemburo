import { describe, it, expect, vi, afterEach } from 'vitest';
import { formatDateRu, formatTimeAgo, formatReportCell, formatDuration } from '../datetime';

describe('formatDateRu', () => {
  it('YYYY-MM-DD -> дд.мм.гггг', () => {
    expect(formatDateRu('2026-06-01')).toBe('01.06.2026');
    expect(formatDateRu('2026-12-31')).toBe('31.12.2026');
  });

  it('игнорирует хвост времени, берёт только дату', () => {
    expect(formatDateRu('2026-06-01 14:30')).toBe('01.06.2026');
    expect(formatDateRu('2026-06-01T14:30:00Z')).toBe('01.06.2026');
  });

  it('не-дату (название разреза) возвращает как есть', () => {
    expect(formatDateRu('Завершено')).toBe('Завершено');
    expect(formatDateRu('ООО Ромашка')).toBe('ООО Ромашка');
  });

  it('пустое -> пустая строка', () => {
    expect(formatDateRu('')).toBe('');
    expect(formatDateRu(null)).toBe('');
    expect(formatDateRu(undefined)).toBe('');
  });
});

describe('formatReportCell', () => {
  it('type=date: ISO-даты -> дд.мм.гггг (в т.ч. диапазон)', () => {
    expect(formatReportCell('2026-06-20', 'date')).toBe('20.06.2026');
    expect(formatReportCell('2026-06-20 - 2026-06-21', 'date')).toBe('20.06.2026 - 21.06.2026');
  });

  it('type=time: убирает секунды', () => {
    expect(formatReportCell('00:01:00 - 23:59:00', 'time')).toBe('00:01 - 23:59');
  });

  it('type=datetime: дата + время без секунд', () => {
    expect(formatReportCell('2026-06-20 14:30:45', 'datetime')).toBe('20.06.2026 14:30');
  });

  it('без типа НЕ форматирует — свободный текст не портится', () => {
    // Ключевая защита: в "Наименовании работ" может быть дата — её нельзя трогать.
    expect(formatReportCell('Ремонт 2026-06-15', undefined)).toBe('Ремонт 2026-06-15');
    expect(formatReportCell('Смена 00:01:00', '')).toBe('Смена 00:01:00');
    expect(formatReportCell('№ 20260619/001', 'text')).toBe('№ 20260619/001');
  });

  it('пустое -> пустая строка', () => {
    expect(formatReportCell('', 'date')).toBe('');
    expect(formatReportCell(null, 'date')).toBe('');
    expect(formatReportCell(undefined, 'date')).toBe('');
  });

  it('type=duration: секунды -> человекочитаемая длительность', () => {
    expect(formatReportCell(8100, 'duration')).toBe('2 ч 15 мин');
    expect(formatReportCell('8100', 'duration')).toBe('2 ч 15 мин');
  });

  it('секунды без типа колонки не трогаем — это обычное число', () => {
    expect(formatReportCell(8100, undefined)).toBe('8100');
    expect(formatReportCell(8100, 'text')).toBe('8100');
  });
});

describe('formatDuration', () => {
  const MIN = 60;
  const HOUR = 60 * MIN;
  const DAY = 24 * HOUR;

  it('ноль осмыслен: пустое окно движка -> «0 мин», а не «нет данных»', () => {
    expect(formatDuration(0)).toBe('0 мин');
  });

  it('меньше минуты -> «<1 мин»', () => {
    expect(formatDuration(1)).toBe('<1 мин');
    expect(formatDuration(59)).toBe('<1 мин');
  });

  it('минуты', () => {
    expect(formatDuration(MIN)).toBe('1 мин');
    expect(formatDuration(45 * MIN)).toBe('45 мин');
    expect(formatDuration(59 * MIN + 59)).toBe('59 мин');
  });

  it('часы: остаток минут показываем, ровный час — без «0 мин»', () => {
    expect(formatDuration(2 * HOUR + 15 * MIN)).toBe('2 ч 15 мин');
    expect(formatDuration(HOUR)).toBe('1 ч');
    expect(formatDuration(23 * HOUR + 59 * MIN)).toBe('23 ч 59 мин');
  });

  it('сутки: старшие две единицы, ровные сутки — без «0 ч»', () => {
    expect(formatDuration(DAY)).toBe('1 сут');
    expect(formatDuration(3 * DAY + 4 * HOUR)).toBe('3 сут 4 ч');
    // Минуты при сутках не показываем — точность здесь шум.
    expect(formatDuration(2 * DAY + 5 * HOUR + 30 * MIN)).toBe('2 сут 5 ч');
  });

  it('единицы округляются вниз — остаток не переполняет старшую', () => {
    // 1 ч 59 мин 59 с: округление минут вверх дало бы невозможное «1 ч 60 мин».
    expect(formatDuration(HOUR + 59 * MIN + 59)).toBe('1 ч 59 мин');
    expect(formatDuration(DAY - 1)).toBe('23 ч 59 мин');
  });

  it('отрицательное (грязные данные) показываем со знаком, а не прячем', () => {
    expect(formatDuration(-5 * MIN)).toBe('-5 мин');
    expect(formatDuration(-2 * HOUR)).toBe('-2 ч');
  });

  it('доля минуты в минус -> «<1 мин» без знака: «-<1 мин» не читается', () => {
    expect(formatDuration(-30)).toBe('<1 мин');
  });

  it('пустое -> пустая строка, нечисловое -> как есть', () => {
    expect(formatDuration(null)).toBe('');
    expect(formatDuration(undefined)).toBe('');
    expect(formatDuration('')).toBe('');
    expect(formatDuration('нет')).toBe('нет');
  });
});

describe('formatTimeAgo', () => {
  const NOW = new Date('2026-06-20T12:00:00Z');
  const ago = (ms) => new Date(NOW.getTime() - ms).toISOString();
  const MIN = 60_000;
  const HOUR = 60 * MIN;
  const DAY = 24 * HOUR;

  afterEach(() => vi.useRealTimers());

  it('меньше минуты (и будущее) -> "только что"', () => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    expect(formatTimeAgo(ago(10_000))).toBe('только что');
    expect(formatTimeAgo(ago(-5_000))).toBe('только что');
  });

  it('минуты и часы', () => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    expect(formatTimeAgo(ago(3 * MIN))).toBe('3 мин назад');
    expect(formatTimeAgo(ago(59 * MIN))).toBe('59 мин назад');
    expect(formatTimeAgo(ago(2 * HOUR))).toBe('2 ч назад');
  });

  it('сутки и больше', () => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    expect(formatTimeAgo(ago(DAY))).toBe('вчера');
    expect(formatTimeAgo(ago(3 * DAY))).toBe('3 дн назад');
  });

  it('пустое/невалидное -> пустая строка', () => {
    expect(formatTimeAgo('')).toBe('');
    expect(formatTimeAgo(null)).toBe('');
    expect(formatTimeAgo('не дата')).toBe('');
  });
});
