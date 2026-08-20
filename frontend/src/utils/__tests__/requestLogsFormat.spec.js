import { describe, it, expect } from 'vitest';
import { describeLoadError, analyticsKpis, headerKpis } from '@/utils/requestLogsFormat';

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
      'Запросов всего', 'Запросов сегодня', 'Отклик за час, медиана и p95',
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
