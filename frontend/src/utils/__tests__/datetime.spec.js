import { describe, it, expect, vi, afterEach } from 'vitest';
import { formatDateRu, formatTimeAgo } from '../datetime';

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
