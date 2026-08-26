import { describe, it, expect } from 'vitest';
import { buildScheduleReport } from '@/utils/scheduleCheck';

// #1183 polish: расписание сверяется по ПЕРЕСЕЧЕНИЮ окна пребывания [time_from,time_to]
// с рабочими интервалами дня, а не по точечным границам въезда/выезда.
// 2026-07-13 = понедельник (day_of_week 0), 14.07 = вторник (1), 15.07 = среда (2).

const slot = (over = {}) => ({
  day_of_week: 0,
  open_time: '10:00',
  close_time: '12:00',
  is_next_day: false,
  is_active: true,
  ...over,
});

const period = (over = {}) => ({
  date_from: '2026-07-13',
  date_to: '2026-07-13',
  time_from: '13:00',
  time_to: '14:00',
  ...over,
});

describe('buildScheduleReport - предусловия', () => {
  it('нет расписания -> null', () => {
    expect(buildScheduleReport([], period())).toBeNull();
    expect(buildScheduleReport(undefined, period())).toBeNull();
  });

  it('нет срока -> null', () => {
    expect(buildScheduleReport([slot()], null)).toBeNull();
  });

  it('дата есть, время пребывания не задано -> null (нет ложных срабатываний)', () => {
    expect(buildScheduleReport([slot()], period({ time_from: null, time_to: null }))).toBeNull();
    expect(buildScheduleReport([slot()], period({ time_from: '', time_to: '' }))).toBeNull();
  });

  it('to < from -> null', () => {
    expect(buildScheduleReport([slot()], period({ date_from: '2026-07-14', date_to: '2026-07-13' }))).toBeNull();
  });
});

describe('buildScheduleReport - пересечение окна пребывания с графиком', () => {
  it('пребывание ВНЕ графика (10-12 работа, 13-14 пребывание) -> закрыто', () => {
    const r = buildScheduleReport([slot()], period({ time_from: '13:00', time_to: '14:00' }));
    expect(r.anyClosed).toBe(true);
    expect(r.days).toHaveLength(1);
    expect(r.days[0].open).toBe(false);
    expect(r.days[0].hours).toEqual(['10:00—12:00']);
    expect(r.presence).toBe('13:00—14:00');
    expect(r.days[0].label).toContain('Пн');
  });

  it('пребывание пересекает график (10-12 работа, 11-13 пребывание) -> открыто', () => {
    const r = buildScheduleReport([slot()], period({ time_from: '11:00', time_to: '13:00' }));
    expect(r.anyClosed).toBe(false);
    expect(r.days[0].open).toBe(true);
  });

  it('пребывание внутри графика -> открыто', () => {
    const r = buildScheduleReport([slot()], period({ time_from: '10:30', time_to: '11:30' }));
    expect(r.anyClosed).toBe(false);
  });

  it('касание по границе (пребывание с 12:00, закрытие 12:00) -> не пересечение, закрыто', () => {
    const r = buildScheduleReport([slot()], period({ time_from: '12:00', time_to: '13:00' }));
    expect(r.anyClosed).toBe(true);
  });

  it('день без слотов -> "не работает", закрыто', () => {
    // срок на вторник (14.07), а слот только на понедельник
    const r = buildScheduleReport([slot()], period({ date_from: '2026-07-14', date_to: '2026-07-14', time_from: '10:30', time_to: '11:30' }));
    expect(r.anyClosed).toBe(true);
    expect(r.days[0].hours).toEqual(['не работает']);
    expect(r.days[0].open).toBe(false);
  });

  it('круглосуточно -> открыто в любое окно', () => {
    const rtc = slot({ open_time: '00:00', close_time: '23:59' });
    const r = buildScheduleReport([rtc], period({ time_from: '03:00', time_to: '05:00' }));
    expect(r.anyClosed).toBe(false);
    expect(r.days[0].hours).toEqual(['круглосуточно']);
  });

  it('is_next_day слот покрывает вечер и утро того же дня', () => {
    const nd = slot({ open_time: '22:00', close_time: '06:00', is_next_day: true });
    expect(buildScheduleReport([nd], period({ time_from: '23:00', time_to: '23:30' })).anyClosed).toBe(false); // в [22:00,24:00)
    expect(buildScheduleReport([nd], period({ time_from: '05:00', time_to: '05:30' })).anyClosed).toBe(false); // в [00:00,06:00]
    expect(buildScheduleReport([nd], period({ time_from: '12:00', time_to: '13:00' })).anyClosed).toBe(true); // днём закрыто
  });

  it('несколько слотов в день -> режим перечисляет их, пересечение с любым = открыто', () => {
    const slots = [slot({ open_time: '10:00', close_time: '12:00' }), slot({ open_time: '17:00', close_time: '18:00' })];
    const r = buildScheduleReport(slots, period({ time_from: '17:30', time_to: '17:45' }));
    expect(r.days[0].hours).toEqual(['10:00—12:00', '17:00—18:00']);
    expect(r.anyClosed).toBe(false);
  });

  it('неактивный слот не учитывается', () => {
    const r = buildScheduleReport([slot({ is_active: false })], period({ time_from: '10:30', time_to: '11:30' }));
    expect(r.days[0].hours).toEqual(['не работает']);
    expect(r.anyClosed).toBe(true);
  });
});

describe('buildScheduleReport - многодневный период', () => {
  it('период пн-вт: закрытый вторник ловится, метки по дням недели', () => {
    // слот только пн 10-12; пребывание 10:30-11:30 (в графике по пн, но вт не работает)
    const r = buildScheduleReport([slot()], {
      date_from: '2026-07-13', date_to: '2026-07-14', time_from: '10:30', time_to: '11:30',
    });
    expect(r.days).toHaveLength(2);
    const mon = r.days.find((d) => d.weekday === 0);
    const tue = r.days.find((d) => d.weekday === 1);
    expect(mon.open).toBe(true);
    expect(tue.open).toBe(false);
    expect(tue.hours).toEqual(['не работает']);
    expect(r.anyClosed).toBe(true);
    // многодневный -> метка = день недели без даты
    expect(mon.label).toBe('Пн');
  });

  it('дни недели не дублируются на длинном периоде', () => {
    const r = buildScheduleReport([slot()], {
      date_from: '2026-07-13', date_to: '2026-07-27', time_from: '10:30', time_to: '11:30',
    });
    expect(r.days).toHaveLength(7); // максимум 7 уникальных дней недели
  });

  it('дни идут в хронологии периода, а не по номеру дня недели', () => {
    // Сб(18.07)->Вт(21.07): порядок должен быть Сб,Вс,Пн,Вт
    const r = buildScheduleReport([slot()], {
      date_from: '2026-07-18', date_to: '2026-07-21', time_from: '10:30', time_to: '11:30',
    });
    expect(r.days.map((d) => d.weekday)).toEqual([5, 6, 0, 1]);
  });
});

describe('buildScheduleReport - ночное окно пребывания (time_from > time_to)', () => {
  it('окно через полночь раскрывается и пересекается с круглосуточным слотом', () => {
    // круглосуточный слот пн - однодневный срок пн, пребывание 20:00-06:00 пересекается
    const rtc = slot({ open_time: '00:00', close_time: '23:59' });
    const r = buildScheduleReport([rtc], {
      date_from: '2026-07-13', date_to: '2026-07-13', time_from: '20:00', time_to: '06:00',
    });
    expect(r).not.toBeNull();
    expect(r.presence).toBe('20:00—06:00');
    expect(r.days[0].open).toBe(true);
  });

  it('ночное окно вне дневного графика -> закрыто (раньше молча пропускалось)', () => {
    // место работает пн 10:00-12:00, пребывание 20:00-06:00 не пересекает
    const r = buildScheduleReport([slot()], {
      date_from: '2026-07-13', date_to: '2026-07-14', time_from: '20:00', time_to: '06:00',
    });
    expect(r).not.toBeNull();
    expect(r.anyClosed).toBe(true);
  });

  it('вырожденное окно from==to -> null', () => {
    expect(buildScheduleReport([slot()], period({ time_from: '10:00', time_to: '10:00' }))).toBeNull();
  });
});
