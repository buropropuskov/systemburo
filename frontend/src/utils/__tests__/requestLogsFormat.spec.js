import { describe, it, expect } from 'vitest';
import { describeLoadError, analyticsKpis } from '@/utils/requestLogsFormat';

// Тексты раздела мониторинга (#2125): отказ чтения объясняется словами, а
// показатели шапки собираются одним перечнем вместо четырёх копий разметки.

describe('describeLoadError', () => {
  it('различает отказ по правам и отказ сервера', () => {
    expect(describeLoadError({ status: 403 }, 'загрузить журнал')).toContain('нет прав');
    expect(describeLoadError({ status: 500 }, 'загрузить журнал')).toContain('500');
  });

  it('истёкшую сессию называет сессией, а не общей ошибкой', () => {
    expect(describeLoadError({ status: 401 }, 'загрузить журнал')).toContain('сессия истекла');
  });

  it('обрыв связи не притворяется ответом сервера', () => {
    expect(describeLoadError(new Error('network'), 'выгрузить журнал')).toContain('нет связи');
  });

  it('называет действие, которое не получилось', () => {
    expect(describeLoadError({ status: 403 }, 'выгрузить журнал')).toContain('выгрузить журнал');
  });
});

describe('analyticsKpis', () => {
  it('собирает четыре показателя периода', () => {
    const kpis = analyticsKpis({ requests: 1200, error_rate: 2.5, avg_duration_ms: 9.34, errors: 30 });

    expect(kpis).toHaveLength(4);
    expect(kpis[1].value).toBe('2.50%');
    expect(kpis[1].bad, 'доля ошибок выше процента подсвечивается').toBe(true);
    expect(kpis[3].value).toContain('30');
  });

  it('переживает пустые итоги: раздел открывают и до первого запроса', () => {
    const kpis = analyticsKpis(undefined);

    expect(kpis).toHaveLength(4);
    expect(kpis[1].value).toBe('0.00%');
    expect(kpis[1].bad).toBe(false);
  });
});
