import { describe, it, expect } from 'vitest';
import {
  isSlotOpenAt,
  schedulePlaceStatusAt,
  collectScheduleWarnings,
} from '@/utils/scheduleCheck';

// #1183 S5: авто-проверка расписания (time_slots) против срока заявки.
// 2026-07-13 = понедельник = проектный day_of_week 0 (см. VehicleFormWarnings.spec).
const MON_09_30 = new Date(2026, 6, 13, 9, 30);
const MON_20_00 = new Date(2026, 6, 13, 20, 0);
const TUE_09_30 = new Date(2026, 6, 14, 9, 30);

const slot = (over = {}) => ({
  day_of_week: 0,
  open_time: '09:00',
  close_time: '18:00',
  is_next_day: false,
  is_active: true,
  ...over,
});

describe('isSlotOpenAt', () => {
  it('открыт внутри интервала того же дня недели', () => {
    expect(isSlotOpenAt(slot(), MON_09_30)).toBe(true);
  });

  it('закрыт вне интервала', () => {
    expect(isSlotOpenAt(slot(), MON_20_00)).toBe(false);
  });

  it('закрыт в другой день недели', () => {
    expect(isSlotOpenAt(slot(), TUE_09_30)).toBe(false);
  });

  it('неактивный слот всегда закрыт', () => {
    expect(isSlotOpenAt(slot({ is_active: false }), MON_09_30)).toBe(false);
  });

  it('круглосуточный слот открыт в любой момент своего дня', () => {
    const rtc = slot({ open_time: '00:00', close_time: '23:59' });
    expect(isSlotOpenAt(rtc, MON_09_30)).toBe(true);
    expect(isSlotOpenAt(rtc, MON_20_00)).toBe(true);
  });

  it('is_next_day: открыт после open ИЛИ до close того же дня', () => {
    const nd = slot({ open_time: '22:00', close_time: '06:00', is_next_day: true });
    expect(isSlotOpenAt(nd, MON_20_00)).toBe(false); // 20:00 < 22:00 и > 06:00
    expect(isSlotOpenAt(nd, new Date(2026, 6, 13, 23, 0))).toBe(true); // >= 22:00
    expect(isSlotOpenAt(nd, new Date(2026, 6, 13, 5, 0))).toBe(true); // <= 06:00
  });

  it('игнорирует секунды в open/close_time', () => {
    expect(isSlotOpenAt(slot({ open_time: '09:00:00', close_time: '18:00:00' }), MON_09_30)).toBe(true);
  });
});

describe('schedulePlaceStatusAt', () => {
  it('нет слотов -> no-schedule', () => {
    expect(schedulePlaceStatusAt([], MON_09_30)).toBe('no-schedule');
    expect(schedulePlaceStatusAt(undefined, MON_09_30)).toBe('no-schedule');
  });

  it('есть открытый слот -> open', () => {
    expect(schedulePlaceStatusAt([slot()], MON_09_30)).toBe('open');
  });

  it('слоты есть, но ни один не открыт в момент -> closed', () => {
    expect(schedulePlaceStatusAt([slot()], MON_20_00)).toBe('closed');
    expect(schedulePlaceStatusAt([slot()], TUE_09_30)).toBe('closed');
  });
});

describe('collectScheduleWarnings', () => {
  const openSlots = [slot()]; // Пн 09:00-18:00

  it('нет срока -> нет предупреждений', () => {
    expect(collectScheduleWarnings(openSlots, null)).toEqual([]);
  });

  it('нет расписания -> нет предупреждений даже вне рабочего времени', () => {
    const period = { date_from: '2026-07-13', date_to: '2026-07-13', time_from: '20:00', time_to: '21:00' };
    expect(collectScheduleWarnings([], period)).toEqual([]);
  });

  it('срок в рабочем окне -> нет предупреждений', () => {
    const period = { date_from: '2026-07-13', date_to: '2026-07-13', time_from: '09:30', time_to: '17:00' };
    expect(collectScheduleWarnings(openSlots, period)).toEqual([]);
  });

  it('закрыто на момент въезда -> одно предупреждение о въезде', () => {
    const period = { date_from: '2026-07-13', date_to: '2026-07-13', time_from: '08:00', time_to: '17:00' };
    const msgs = collectScheduleWarnings(openSlots, period);
    expect(msgs).toHaveLength(1);
    expect(msgs[0]).toContain('въезда');
    expect(msgs[0]).toContain('13.07.2026 08:00');
  });

  it('закрыто на момент выезда -> одно предупреждение о выезде', () => {
    const period = { date_from: '2026-07-13', date_to: '2026-07-13', time_from: '10:00', time_to: '20:00' };
    const msgs = collectScheduleWarnings(openSlots, period);
    expect(msgs).toHaveLength(1);
    expect(msgs[0]).toContain('выезда');
    expect(msgs[0]).toContain('13.07.2026 20:00');
  });

  it('закрыто на обеих границах -> два предупреждения', () => {
    const period = { date_from: '2026-07-13', date_to: '2026-07-14', time_from: '08:00', time_to: '09:30' };
    const msgs = collectScheduleWarnings(openSlots, period);
    expect(msgs).toHaveLength(2); // Пн 08:00 закрыто + Вт вообще нет слотов
  });

  it('совпадающий момент въезда/выезда не дублирует предупреждение', () => {
    const period = { date_from: '2026-07-13', date_to: '2026-07-13', time_from: '20:00', time_to: '20:00' };
    const msgs = collectScheduleWarnings(openSlots, period);
    expect(msgs).toHaveLength(1);
    expect(msgs[0]).toContain('въезда');
  });

  it('без даты границы -> эта граница пропускается', () => {
    const period = { date_from: '', date_to: '2026-07-13', time_from: '', time_to: '20:00' };
    const msgs = collectScheduleWarnings(openSlots, period);
    expect(msgs).toHaveLength(1);
    expect(msgs[0]).toContain('выезда');
  });

  it('дата есть, время НЕ введено -> граница не проверяется (нет ложного 00:00)', () => {
    // DateRangeSection: дата и время - независимые поля; пока время пусто, срок
    // не готов и не должен давать "закрыто в 00:00".
    const bothNoTime = { date_from: '2026-07-13', date_to: '2026-07-13', time_from: null, time_to: null };
    expect(collectScheduleWarnings(openSlots, bothNoTime)).toEqual([]);

    const startNoTime = { date_from: '2026-07-13', date_to: '2026-07-13', time_from: '', time_to: '20:00' };
    const msgs = collectScheduleWarnings(openSlots, startNoTime);
    expect(msgs).toHaveLength(1); // только выезд 20:00 (закрыт), въезд без времени - пропуск
    expect(msgs[0]).toContain('выезда');
  });
});
