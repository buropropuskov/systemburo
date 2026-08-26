import { describe, it, expect } from 'vitest';
import { applicationPeriodLabel, groupApplicationsByPeriod } from '../applicationPeriod';

// Опорный момент - среда 15.07.2026 12:00 (неделя считается от понедельника 13.07).
const NOW = new Date(2026, 6, 15, 12, 0, 0);

describe('applicationPeriodLabel', () => {
  it('сегодня и вчера', () => {
    expect(applicationPeriodLabel(new Date(2026, 6, 15, 8, 0), NOW)).toBe('Сегодня');
    expect(applicationPeriodLabel(new Date(2026, 6, 14, 23, 0), NOW)).toBe('Вчера');
  });

  it('неделя считается от понедельника', () => {
    expect(applicationPeriodLabel(new Date(2026, 6, 13, 9, 0), NOW)).toBe('На этой неделе');
    expect(applicationPeriodLabel(new Date(2026, 6, 12, 9, 0), NOW)).toBe('На прошлой неделе');
  });

  it('месяцы и всё остальное', () => {
    expect(applicationPeriodLabel(new Date(2026, 6, 2, 9, 0), NOW)).toBe('В этом месяце');
    expect(applicationPeriodLabel(new Date(2026, 5, 20, 9, 0), NOW)).toBe('В прошлом месяце');
    expect(applicationPeriodLabel(new Date(2026, 3, 1, 9, 0), NOW)).toBe('Ранее');
  });
});

describe('groupApplicationsByPeriod', () => {
  const apps = [
    { id: 1, sending_datetime: new Date(2026, 6, 15, 10, 0).toISOString() },
    { id: 2, sending_datetime: new Date(2026, 6, 15, 9, 0).toISOString() },
    { id: 3, sending_datetime: new Date(2026, 6, 14, 9, 0).toISOString() },
    { id: 4, sending_datetime: new Date(2026, 5, 20, 9, 0).toISOString() },
  ];

  it('режет список по смене периода, сохраняя порядок', () => {
    const groups = groupApplicationsByPeriod(apps, true, NOW);
    expect(groups.map((g) => g.label)).toEqual(['Сегодня', 'Вчера', 'В прошлом месяце']);
    expect(groups[0].apps.map((a) => a.id)).toEqual([1, 2]);
    expect(groups[2].apps.map((a) => a.id)).toEqual([4]);
  });

  it('при сортировке не по дате - одна группа без подписи (разделители не рисуем)', () => {
    const groups = groupApplicationsByPeriod(apps, false, NOW);
    expect(groups).toHaveLength(1);
    expect(groups[0].label).toBeNull();
    expect(groups[0].apps).toHaveLength(4);
  });

  it('пустой список - без групп', () => {
    expect(groupApplicationsByPeriod([], true, NOW)).toEqual([]);
  });
});
