import { describe, it, expect } from 'vitest';
import { describeLoadError, analyticsKpis, dailyChartPoints, headerKpis, formatStamp
} from '@/utils/requestLogsFormat';

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

describe('headerKpis', () => {
  const stats = {
    total: 353093, today: 4120, median_duration: 8.42, p95_duration: 140.6,
    error_rate: 7.3, requests_per_minute: 61.25,
  };

  it('собирает показатели шапки и подсвечивает высокую долю ошибок', () => {
    const kpis = headerKpis(stats, {});

    expect(kpis.map(k => k.label)).toEqual([
      'Запросов в журнале', 'Запросов сегодня', 'Отклик за час, медиана и p95',
      'Доля ошибок за час', 'Запросов в минуту',
    ]);
    expect(kpis[2].value, 'медиана без единиц, перцентиль с ними').toBe('8.4 / 141мс');
    expect(kpis[3].value).toBe('7.3%');
    expect(kpis[3].bad).toBe(true);
    expect(kpis[4].value).toBe('61.3');
  });

  it('доля ошибок ниже порога шапки не красится', () => {
    expect(headerKpis({ ...stats, error_rate: 4.9 }, {}).at(3).bad).toBe(false);
  });

  it('счётчик ленты появляется только с ответом сервера', () => {
    // До первого ответа карточка «0/с» читалась бы как затишье, хотя счётчиков
    // ещё просто нет.
    expect(headerKpis(stats, {})).toHaveLength(5);

    const live = headerKpis(stats, { last_second_count: 3, last_minute_count: 118 }).at(-1);
    expect(live.label).toBe('Сейчас');
    expect(live.value).toBe('3/с');
    expect(live.sub).toBe('118/мин');
    expect(live.live).toBe(true);
  });

  it('переживает пустые показатели: раздел открывают и до первого ответа', () => {
    const kpis = headerKpis(undefined, undefined);

    expect(kpis).toHaveLength(5);
    expect(kpis[0].value).toBe('0');
    expect(kpis[3].value).toBe('0.0%');
  });
});

describe('dailyChartPoints', () => {
  it('добавляет пропущенные сутки, чтобы шаг оси равнялся суткам', () => {
    const points = dailyChartPoints([
      { day: '2026-08-10', requests: 100, errors: 2 },
      { day: '2026-08-13', requests: 50, errors: 0 }
    ]);

    expect(points.map(p => p.day))
      .toEqual(['2026-08-10', '2026-08-11', '2026-08-12', '2026-08-13']);
  });

  it('день без записей помечает отсутствием данных, а не нулём', () => {
    const points = dailyChartPoints([
      { day: '2026-08-10', requests: 100, errors: 2 },
      { day: '2026-08-12', requests: 50, errors: 0 }
    ]);

    expect(points[1]).toEqual({ day: '2026-08-11', requests: null, errors: null });
    expect(points[2]).toEqual({ day: '2026-08-12', requests: 50, errors: 0 });
  });

  it('держит шаг в сутки на переходе через смену месяца и года', () => {
    const points = dailyChartPoints([
      { day: '2025-12-30', requests: 1, errors: 0 },
      { day: '2026-01-02', requests: 4, errors: 1 }
    ]);

    expect(points.map(p => p.day))
      .toEqual(['2025-12-30', '2025-12-31', '2026-01-01', '2026-01-02']);
  });

  it('восстанавливает порядок, если сутки пришли вперемешку', () => {
    const points = dailyChartPoints([
      { day: '2026-08-12', requests: 50, errors: 0 },
      { day: '2026-08-11', requests: 70, errors: 3 }
    ]);

    expect(points.map(p => p.day)).toEqual(['2026-08-11', '2026-08-12']);
  });

  it('пустой и негодный ряд отдаёт пустым, а не ломает разметку', () => {
    expect(dailyChartPoints([])).toEqual([]);
    expect(dailyChartPoints(null)).toEqual([]);
    expect(dailyChartPoints([{ day: '', requests: 5, errors: 0 }])).toEqual([]);
  });

  it('отметка строки журнала несёт день, а не только время', () => {
    // День обязателен: подробные записи живут 30 суток и отбираются по датам,
    // а по одному времени строку к дню не привязать. Время московское (#2298):
    // выгрузка того же журнала печатается по Москве, экран обязан совпасть с ней.
    expect(formatStamp('2026-08-21T09:32:05Z')).toBe('21.08 12:32:05');
    // Момент после 21:00 UTC относится уже к следующим московским суткам.
    expect(formatStamp('2026-08-21T21:30:00Z')).toBe('22.08 00:30:00');
    expect(formatStamp('')).toBe('');
    expect(formatStamp('не дата')).toBe('');
  });
});
