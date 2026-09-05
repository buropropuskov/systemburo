import { describe, it, expect, vi, beforeEach } from 'vitest';

const { getReportDataPeriod } = vi.hoisted(() => ({ getReportDataPeriod: vi.fn() }));
vi.mock('@/api/statistics', () => ({ getReportDataPeriod }));

import { computePeriodRange, dateToIso, isoToDate, fetchReportPeriodBounds } from '../reportPeriod';

describe('computePeriodRange', () => {
  const wednesday = new Date(2026, 8, 2); // среда, 02.09.2026

  it('неделя считается от понедельника, месяц и год — от первого числа', () => {
    expect(computePeriodRange('week', wednesday)).toEqual({ from: '2026-08-31', to: '2026-09-02' });
    expect(computePeriodRange('month', wednesday)).toEqual({ from: '2026-09-01', to: '2026-09-02' });
    expect(computePeriodRange('year', wednesday)).toEqual({ from: '2026-01-01', to: '2026-09-02' });
  });

  it('«весь период» границ не задаёт — их подставляет запрос к бэкенду', () => {
    expect(computePeriodRange('all', wednesday)).toEqual({ from: '', to: '' });
  });

  it('дата собирается по календарным частям, без сдвига на границе суток', () => {
    expect(dateToIso(new Date(2026, 0, 1, 23, 59))).toBe('2026-01-01');
    expect(isoToDate('2026-03-08')).toEqual(new Date(2026, 2, 8));
    expect(isoToDate('')).toBeNull();
    expect(isoToDate('2026-03')).toBeNull();
  });
});

describe('fetchReportPeriodBounds (#2341)', () => {
  // Тело в скобках обязательно: стрелка без них вернула бы сам мок, а Vitest считает
  // возвращённую из beforeEach функцию teardown-обработчиком и вызвал бы её после теста
  // - с реализацией «бросить ошибку» это роняло прогон уже после ассертов.
  beforeEach(() => { getReportDataPeriod.mockReset(); });

  it('полные границы возвращаются мастеру', async () => {
    getReportDataPeriod.mockResolvedValue({ from: '2026-04-09', to: '2026-09-05' });
    await expect(fetchReportPeriodBounds({ mode: 'aggregate', metric: 'applications_count' }))
      .resolves.toEqual({ from: '2026-04-09', to: '2026-09-05' });
  });

  it('пустые границы (данных нет или у отчёта нет оси времени) -> null', async () => {
    getReportDataPeriod.mockResolvedValue({ from: '', to: '' });
    await expect(fetchReportPeriodBounds({ mode: 'list', entity: 'cars' })).resolves.toBeNull();
  });

  it('сбой запроса не роняет выбор периода', async () => {
    getReportDataPeriod.mockImplementation(() => { throw new Error('сеть'); });
    expect(await fetchReportPeriodBounds({ mode: 'aggregate', metric: 'applications_count' })).toBeNull();
  });
});
