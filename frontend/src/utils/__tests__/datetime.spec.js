import { describe, it, expect, vi, afterEach } from 'vitest';
import { formatDateRu, formatTimeAgo, formatReportCell, formatDuration, formatMonthRu, weekdayName, formatDateTime } from '../datetime';
import { syncServerTime } from '../serverTime';

describe('formatMonthRu', () => {
  it('YYYY-MM -> Месяц ГГГГ', () => {
    expect(formatMonthRu('2026-07')).toBe('Июль 2026');
    expect(formatMonthRu('2026-01')).toBe('Январь 2026');
    expect(formatMonthRu('2025-12')).toBe('Декабрь 2025');
  });

  it('не съезжает на соседний месяц (нет new Date/UTC-полуночи)', () => {
    expect(formatMonthRu('2026-03')).toBe('Март 2026');
  });

  it('невалидное/пустое возвращает как есть', () => {
    expect(formatMonthRu('')).toBe('');
    expect(formatMonthRu(null)).toBe('');
    expect(formatMonthRu('не период')).toBe('не период');
    expect(formatMonthRu('2026-13')).toBe('2026-13');
  });
});

describe('formatDateRu', () => {
  it('YYYY-MM-DD -> дд.мм.гггг', () => {
    expect(formatDateRu('2026-06-01')).toBe('01.06.2026');
    expect(formatDateRu('2026-12-31')).toBe('31.12.2026');
  });

  it('игнорирует хвост времени, берёт только дату', () => {
    expect(formatDateRu('2026-06-01 14:30')).toBe('01.06.2026');
    expect(formatDateRu('2026-06-01T14:30:00Z')).toBe('01.06.2026');
  });

  it('не-дату (название разреза) возвращает как есть', () => {
    expect(formatDateRu('Завершено')).toBe('Завершено');
    expect(formatDateRu('ООО Ромашка')).toBe('ООО Ромашка');
  });

  it('пустое -> пустая строка', () => {
    expect(formatDateRu('')).toBe('');
    expect(formatDateRu(null)).toBe('');
    expect(formatDateRu(undefined)).toBe('');
  });
});

describe('formatReportCell', () => {
  it('type=date: ISO-даты -> дд.мм.гггг (в т.ч. диапазон)', () => {
    expect(formatReportCell('2026-06-20', 'date')).toBe('20.06.2026');
    expect(formatReportCell('2026-06-20 - 2026-06-21', 'date')).toBe('20.06.2026 - 21.06.2026');
  });

  it('type=time: убирает секунды', () => {
    expect(formatReportCell('00:01:00 - 23:59:00', 'time')).toBe('00:01 - 23:59');
  });

  it('type=datetime: дата + время без секунд', () => {
    expect(formatReportCell('2026-06-20 14:30:45', 'datetime')).toBe('20.06.2026 14:30');
  });

  it('без типа НЕ форматирует — свободный текст не портится', () => {
    // Ключевая защита: в "Наименовании работ" может быть дата — её нельзя трогать.
    expect(formatReportCell('Ремонт 2026-06-15', undefined)).toBe('Ремонт 2026-06-15');
    expect(formatReportCell('Смена 00:01:00', '')).toBe('Смена 00:01:00');
    expect(formatReportCell('№ 20260619/001', 'text')).toBe('№ 20260619/001');
  });

  it('пустое -> пустая строка', () => {
    expect(formatReportCell('', 'date')).toBe('');
    expect(formatReportCell(null, 'date')).toBe('');
    expect(formatReportCell(undefined, 'date')).toBe('');
  });

  it('type=duration: секунды -> человекочитаемая длительность', () => {
    expect(formatReportCell(8100, 'duration')).toBe('2 ч 15 мин');
    expect(formatReportCell('8100', 'duration')).toBe('2 ч 15 мин');
  });

  it('секунды без типа колонки не трогаем — это обычное число', () => {
    expect(formatReportCell(8100, undefined)).toBe('8100');
    expect(formatReportCell(8100, 'text')).toBe('8100');
  });
});

describe('formatDuration', () => {
  const MIN = 60;
  const HOUR = 60 * MIN;
  const DAY = 24 * HOUR;

  it('ноль осмыслен: пустое окно движка -> «0 с», а не «нет данных»', () => {
    expect(formatDuration(0)).toBe('0 с');
  });

  it('меньше минуты -> секунды: «0 мин» на шестисекундном этапе читалось как поломка', () => {
    expect(formatDuration(1)).toBe('1 с');
    expect(formatDuration(6)).toBe('6 с');
    expect(formatDuration(59)).toBe('59 с');
  });

  it('минуты: секунды остатка показываем, ровная минута — без «0 с»', () => {
    expect(formatDuration(MIN)).toBe('1 мин');
    expect(formatDuration(45 * MIN)).toBe('45 мин');
    expect(formatDuration(59 * MIN + 59)).toBe('59 мин 59 с');
    expect(formatDuration(10 * MIN + 20)).toBe('10 мин 20 с');
  });

  it('часы: остаток минут показываем, ровный час — без «0 мин»', () => {
    expect(formatDuration(2 * HOUR + 15 * MIN)).toBe('2 ч 15 мин');
    expect(formatDuration(HOUR)).toBe('1 ч');
    expect(formatDuration(23 * HOUR + 59 * MIN)).toBe('23 ч 59 мин');
  });

  it('сутки: старшие две единицы, ровные сутки — без «0 ч»', () => {
    expect(formatDuration(DAY)).toBe('1 сут');
    expect(formatDuration(3 * DAY + 4 * HOUR)).toBe('3 сут 4 ч');
    // Минуты при сутках не показываем — точность здесь шум.
    expect(formatDuration(2 * DAY + 5 * HOUR + 30 * MIN)).toBe('2 сут 5 ч');
  });

  it('единицы округляются вниз — остаток не переполняет старшую', () => {
    // 1 ч 59 мин 59 с: округление минут вверх дало бы невозможное «1 ч 60 мин».
    expect(formatDuration(HOUR + 59 * MIN + 59)).toBe('1 ч 59 мин');
    expect(formatDuration(DAY - 1)).toBe('23 ч 59 мин');
  });

  it('отрицательное (грязные данные) показываем со знаком, а не прячем', () => {
    expect(formatDuration(-5 * MIN)).toBe('-5 мин');
    expect(formatDuration(-2 * HOUR)).toBe('-2 ч');
  });

  it('доля минуты в минус -> секунды со знаком', () => {
    expect(formatDuration(-30)).toBe('-30 с');
  });

  it('пустое -> пустая строка, нечисловое -> как есть', () => {
    expect(formatDuration(null)).toBe('');
    expect(formatDuration(undefined)).toBe('');
    expect(formatDuration('')).toBe('');
    expect(formatDuration('нет')).toBe('нет');
  });
});

describe('formatTimeAgo', () => {
  const NOW = new Date('2026-06-20T12:00:00Z');
  const ago = (ms) => new Date(NOW.getTime() - ms).toISOString();
  const MIN = 60_000;
  const HOUR = 60 * MIN;
  const DAY = 24 * HOUR;

  afterEach(() => vi.useRealTimers());

  it('меньше минуты (и будущее) -> "только что"', () => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    expect(formatTimeAgo(ago(10_000))).toBe('только что');
    expect(formatTimeAgo(ago(-5_000))).toBe('только что');
  });

  it('минуты и часы', () => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    expect(formatTimeAgo(ago(3 * MIN))).toBe('3 мин назад');
    expect(formatTimeAgo(ago(59 * MIN))).toBe('59 мин назад');
    expect(formatTimeAgo(ago(2 * HOUR))).toBe('2 ч назад');
  });

  it('сутки и больше', () => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    expect(formatTimeAgo(ago(DAY))).toBe('вчера');
    expect(formatTimeAgo(ago(3 * DAY))).toBe('3 дн назад');
  });

  it('пустое/невалидное -> пустая строка', () => {
    expect(formatTimeAgo('')).toBe('');
    expect(formatTimeAgo(null)).toBe('');
    expect(formatTimeAgo('не дата')).toBe('');
  });

  it('возраст считается от серверных часов, а не от сбитых часов машины', () => {
    // Часы машины убежали на два часа вперёд. По ним свежая отметка выглядела бы
    // двухчасовой давности - и «кто сейчас онлайн» показывало бы ушедших.
    vi.useFakeTimers();
    vi.setSystemTime(new Date(NOW.getTime() + 2 * HOUR));
    syncServerTime({ headers: { get: (n) => (n === 'date' ? NOW.toUTCString() : null) } });

    expect(formatTimeAgo(ago(3 * MIN))).toBe('3 мин назад');

    // Смещение живёт в модуле - возвращаем его к нулю, иначе поедут соседние тесты.
    syncServerTime({ headers: { get: () => new Date().toUTCString() } });
  });
});

/**
 * Общий показ момента времени: московский и по серверным часам (#2298).
 *
 * Через formatDateTime проходит время во всём интерфейсе - история входов, срок
 * пропуска, отметки поста. Раньше он звал getHours() и показывал зону машины:
 * работник поста в другом поясе видел не то время, по которому система приняла
 * решение, а сбитые часы на рабочем месте делали то же самое незаметно.
 *
 * Зона проверяется явно: на компьютере в Москве потеря `timeZone` ничего не
 * ломает, поэтому берём моменты, у которых московская дата отличается от UTC.
 */
describe('formatDateTime - московское время', () => {
  it('показывает московский час, а не UTC', () => {
    expect(formatDateTime('2026-09-04T12:00:00Z')).toBe('04.09.2026 15:00');
  });

  it('после 21:00 UTC дата уже московская, следующая', () => {
    // Ночная отметка: в UTC ещё 4-е, в Москве уже 5-е. Ошибка здесь уводит
    // отметку в соседние отчётные сутки.
    expect(formatDateTime('2026-09-04T21:30:00Z')).toBe('05.09.2026 00:30');
  });

  it('момент со своим смещением приводится к Москве, а не показывается как есть', () => {
    // Бэкенд отдаёт UTC, но границы отчётных суток приходят со смещением +03:00.
    expect(formatDateTime('2026-09-04T23:30:00+03:00')).toBe('04.09.2026 23:30');
    expect(formatDateTime('2026-09-04T10:00:00-05:00')).toBe('04.09.2026 18:00');
  });

  it('пустое и невалидное ведут себя как раньше', () => {
    expect(formatDateTime('')).toBe('');
    expect(formatDateTime(null)).toBe('');
    expect(formatDateTime('не дата')).toBe('не дата');
  });
});

describe('weekdayName', () => {
  it('называет день недели по дате', () => {
    // 24.08.2026 - понедельник.
    expect(weekdayName('2026-08-24T10:00:00Z')).toBe('Понедельник');
    expect(weekdayName('2026-08-23T10:00:00Z')).toBe('Воскресенье');
  });

  it('день берётся по московской дате момента', () => {
    // Заявка отправлена в понедельник 00:30 МСК: в UTC это ещё воскресенье.
    expect(weekdayName('2026-08-23T21:30:00Z')).toBe('Понедельник');
  });

  it('пустое и невалидное значение не превращает в "Воскресенье"', () => {
    // Индексация от нуля: невалидная дата дала бы NaN и undefined, а пустая строка
    // до фикса вернула бы первый день списка.
    expect(weekdayName('')).toBe('');
    expect(weekdayName(null)).toBe('');
    expect(weekdayName('не дата')).toBe('');
  });
});
