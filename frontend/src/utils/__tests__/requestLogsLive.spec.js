import { describe, it, expect } from 'vitest';
import { journalRefreshBlock, CHART_PERIODS, JOURNAL_REFRESH_MS } from '@/utils/requestLogsLive';

// Живая лента журнала (#2125): список перечитывает себя сам, но встаёт там, где
// обновление отобрало бы у человека то, что он уже держит на экране.

const watching = { tab: 'journal', hidden: false, hasSelection: false, page: 1 };

describe('journalRefreshBlock', () => {
  it('на первой странице журнала лента идёт', () => {
    expect(journalRefreshBlock(watching)).toBe('');
  });

  it('открытое окно запроса держит список на месте', () => {
    expect(journalRefreshBlock({ ...watching, hasSelection: true })).toBe('открыто окно запроса');
  });

  it('фоновая вкладка не опрашивает сервер', () => {
    expect(journalRefreshBlock({ ...watching, hidden: true })).toBe('вкладка в фоне');
  });

  it('на второй странице обновление подменило бы содержимое под курсором', () => {
    expect(journalRefreshBlock({ ...watching, page: 2 })).toBe('открыта не первая страница');
  });

  it('на вкладке аналитики журнал не перечитывается', () => {
    expect(journalRefreshBlock({ ...watching, tab: 'analytics' })).toBe('открыта аналитика');
  });
});

describe('периоды графика', () => {
  it('крупные периоды просят у сервера шаг от суток - их считает свёртка', () => {
    const month = CHART_PERIODS.find(p => p.key === 'last-month');
    const year = CHART_PERIODS.find(p => p.key === 'last-year');

    expect(month.interval).toBe(24 * 3600);
    expect(month.limit, 'месяц это тридцать суточных столбиков').toBe(30);
    expect(year.interval).toBe(7 * 24 * 3600);
    expect(year.limit, 'год это пятьдесят две недели').toBe(52);
  });

  it('период обновления ленты заметно реже опроса счётчиков', () => {
    expect(JOURNAL_REFRESH_MS).toBeGreaterThanOrEqual(5000);
  });
});
