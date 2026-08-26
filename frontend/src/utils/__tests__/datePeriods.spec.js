import { describe, it, expect } from 'vitest';
import { QUICK_PERIODS, isSingleDayPeriod, periodBounds } from '../datePeriods';

// Отсчёт от среды 12.08.2026, чтобы границы недели считались от середины.
const WED = new Date(2026, 7, 12, 15, 30);
const ymd = (d) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;

describe('быстрые периоды календаря', () => {
  it('границы дня охватывают его целиком', () => {
    const [from, to] = periodBounds('today', WED);
    expect(ymd(from)).toBe('2026-08-12');
    expect(ymd(to)).toBe('2026-08-12');
    expect(from.getHours()).toBe(0);
    expect(to.getHours()).toBe(23);
    expect(to.getMilliseconds()).toBe(999);
  });

  it.each([
    ['yesterday', '2026-08-11', '2026-08-11'],
    ['dayBeforeYesterday', '2026-08-10', '2026-08-10'],
    ['tomorrow', '2026-08-13', '2026-08-13'],
    ['dayAfterTomorrow', '2026-08-14', '2026-08-14'],
    ['thisWeek', '2026-08-10', '2026-08-16'],
    ['lastWeek', '2026-08-03', '2026-08-09'],
    ['nextWeek', '2026-08-17', '2026-08-23'],
    ['thisMonth', '2026-08-01', '2026-08-31'],
    ['lastMonth', '2026-07-01', '2026-07-31'],
    ['nextMonth', '2026-09-01', '2026-09-30'],
    ['thisYear', '2026-01-01', '2026-12-31'],
    ['lastYear', '2025-01-01', '2025-12-31'],
  ])('%s: %s - %s', (key, from, to) => {
    const [start, end] = periodBounds(key, WED);
    expect(ymd(start)).toBe(from);
    expect(ymd(end)).toBe(to);
  });

  it('неделя воскресенья считается от того же понедельника', () => {
    const sunday = new Date(2026, 7, 16, 9);
    const [start, end] = periodBounds('thisWeek', sunday);
    expect(ymd(start)).toBe('2026-08-10');
    expect(ymd(end)).toBe('2026-08-16');
  });

  it('неизвестный ключ границ не даёт', () => {
    expect(periodBounds('lastCentury', WED)).toBeNull();
  });

  it('однодневные периоды помечены в реестре', () => {
    const single = QUICK_PERIODS.filter((p) => p.single).map((p) => p.key);
    expect(single).toEqual(['today', 'yesterday', 'tomorrow', 'dayBeforeYesterday', 'dayAfterTomorrow']);
    expect(isSingleDayPeriod('today')).toBe(true);
    expect(isSingleDayPeriod('thisMonth')).toBe(false);
  });
});
