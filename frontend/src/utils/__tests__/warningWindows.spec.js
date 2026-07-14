import { describe, it, expect } from 'vitest';

import { isWindowActiveAt, collectActiveWarnings } from '../warningWindows';

// Опорные моменты (месяц 0-индексный: 6 = июль).
const MON_1030 = new Date(2026, 6, 13, 10, 30); // Пн, projectDay 0
const TUE_1030 = new Date(2026, 6, 14, 10, 30); // Вт, projectDay 1
const SUN_1000 = new Date(2026, 6, 19, 10, 0);  // Вс, projectDay 6

const win = (over = {}) => ({
  day_of_week: null,
  time_from: null,
  time_to: null,
  is_next_day: false,
  message: 'текст',
  is_active: true,
  ...over,
});

describe('isWindowActiveAt', () => {
  it('окно на конкретный день активно только в этот день', () => {
    const w = win({ day_of_week: 0, time_from: '09:00', time_to: '18:00' });
    expect(isWindowActiveAt(w, MON_1030)).toBe(true);
    expect(isWindowActiveAt(w, TUE_1030)).toBe(false);
  });

  it('day_of_week=null = каждый день', () => {
    const w = win({ day_of_week: null, time_from: '09:00', time_to: '18:00' });
    expect(isWindowActiveAt(w, MON_1030)).toBe(true);
    expect(isWindowActiveAt(w, TUE_1030)).toBe(true);
  });

  it('null-границы времени = весь день (день должен совпасть)', () => {
    const w = win({ day_of_week: 0, time_from: null, time_to: null });
    expect(isWindowActiveAt(w, new Date(2026, 6, 13, 23, 0))).toBe(true);
    expect(isWindowActiveAt(w, TUE_1030)).toBe(false);
  });

  it('интервал не покрывает момент -> неактивно', () => {
    const w = win({ day_of_week: null, time_from: '09:00', time_to: '18:00' });
    expect(isWindowActiveAt(w, new Date(2026, 6, 13, 20, 0))).toBe(false);
  });

  it('ночное окно is_next_day покрывает вечер и утро следующих суток', () => {
    const w = win({ day_of_week: null, time_from: '22:00', time_to: '06:00', is_next_day: true });
    expect(isWindowActiveAt(w, new Date(2026, 6, 13, 23, 0))).toBe(true);
    expect(isWindowActiveAt(w, new Date(2026, 6, 13, 3, 0))).toBe(true);
    expect(isWindowActiveAt(w, new Date(2026, 6, 13, 12, 0))).toBe(false);
  });

  it('next-day окно на КОНКРЕТНый день активно только в этот день (как слоты бэка)', () => {
    // Пт 22:00-06:00 (projectDay 4). Оценивается только в пятницу, на субботу не переносится.
    const w = win({ day_of_week: 4, time_from: '22:00', time_to: '06:00', is_next_day: true });
    expect(isWindowActiveAt(w, new Date(2026, 6, 17, 23, 0))).toBe(true); // Пт вечер
    expect(isWindowActiveAt(w, new Date(2026, 6, 17, 3, 0))).toBe(true); // Пт раннее утро (тот же день)
    expect(isWindowActiveAt(w, new Date(2026, 6, 18, 3, 0))).toBe(false); // Сб утро - не переносится
  });

  it('круглосуточное 00:00-23:59 активно в любой момент дня', () => {
    const w = win({ day_of_week: null, time_from: '00:00', time_to: '23:59' });
    expect(isWindowActiveAt(w, MON_1030)).toBe(true);
    expect(isWindowActiveAt(w, new Date(2026, 6, 13, 0, 5))).toBe(true);
  });

  it('is_active=false никогда не активно', () => {
    const w = win({ day_of_week: null, time_from: '09:00', time_to: '18:00', is_active: false });
    expect(isWindowActiveAt(w, MON_1030)).toBe(false);
  });

  it('воскресенье (getDay=0) маппится в projectDay 6', () => {
    const w = win({ day_of_week: 6, time_from: '09:00', time_to: '18:00' });
    expect(isWindowActiveAt(w, SUN_1000)).toBe(true);
    expect(isWindowActiveAt(w, MON_1030)).toBe(false);
  });

  it('время в формате ЧЧ:ММ:СС парсится корректно', () => {
    const w = win({ day_of_week: null, time_from: '09:00:00', time_to: '18:00:00' });
    expect(isWindowActiveAt(w, MON_1030)).toBe(true);
  });
});

describe('collectActiveWarnings', () => {
  it('свободный warning триммится и всегда отдаётся', () => {
    expect(collectActiveWarnings({ warning: '  Малогабарит  ' }, MON_1030).free).toBe('Малогабарит');
  });

  it('пустой/отсутствующий warning -> free=null', () => {
    expect(collectActiveWarnings({ warning: '   ' }, MON_1030).free).toBeNull();
    expect(collectActiveWarnings({}, MON_1030).free).toBeNull();
    expect(collectActiveWarnings({ warning: null }, MON_1030).free).toBeNull();
  });

  it('окна фильтруются по активности на момент, сообщения триммятся', () => {
    const entity = {
      warning: null,
      warning_windows: [
        win({ day_of_week: 0, time_from: '09:00', time_to: '18:00', message: ' сейчас ' }),
        win({ day_of_week: 1, time_from: '09:00', time_to: '18:00', message: 'завтра' }),
        win({ day_of_week: null, time_from: '00:00', time_to: '08:00', message: 'ночью' }),
      ],
    };
    expect(collectActiveWarnings(entity, MON_1030).windows).toEqual(['сейчас']);
  });

  it('окно без message не попадает в результат', () => {
    const entity = { warning_windows: [win({ day_of_week: null, message: '  ' })] };
    expect(collectActiveWarnings(entity, MON_1030).windows).toEqual([]);
  });

  it('отсутствие warning_windows -> пустой список', () => {
    expect(collectActiveWarnings({ warning: 'x' }, MON_1030).windows).toEqual([]);
  });
});
