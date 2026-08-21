import { describe, it, expect } from 'vitest';
import {
  journalStateFromQuery,
  journalQueryFromState,
  mergeJournalQuery,
  statusFilterParams,
  isJournalPresetOn,
  toggleJournalPreset,
  dateToYmd,
  ymdToDate,
  journalFilterDropdowns
} from '@/utils/requestLogsQuery';
import { formatMs, formatDuration } from '@/utils/requestLogsFormat';

// Отбор журнала обращений (#2125): адресная строка и состояние экрана обязаны
// переводиться друг в друга без потерь, а чужие значения в адресе не должны
// доезжать до сервера.

describe('journalStateFromQuery', () => {
  it('читает отбор из адреса и приводит метод к верхнему регистру', () => {
    const state = journalStateFromQuery({
      search: 'applications', method: 'post', status: '500',
      user: '7', from: '2026-08-01', to: '2026-08-19',
      sort: 'duration', order: 'asc', page: '3', per_page: '50'
    });

    expect(state).toEqual({
      search: 'applications', method: 'POST', status: '500', user: '7',
      from: '2026-08-01', to: '2026-08-19', since: '', minDuration: '',
      sort: 'duration', order: 'asc', page: 3, perPage: 50
    });
  });

  it('чужое поле сортировки и чужой размер страницы заменяет своими', () => {
    const state = journalStateFromQuery({ sort: 'response_body', order: 'asc', per_page: '999', page: '0' });

    expect(state.sort).toBe('created_at');
    expect(state.order, 'направление без известного поля тоже сбрасывается').toBe('desc');
    expect(state.perPage).toBe(20);
    expect(state.page).toBe(1);
  });

  it('отбор быстрых кнопок читается из адреса', () => {
    const state = journalStateFromQuery({
      status: 'errors', min_duration: '1000', since: '2026-08-19T10:00:00.000Z'
    });

    expect(state.status).toBe('errors');
    expect(state.minDuration).toBe('1000');
    expect(state.since).toBe('2026-08-19T10:00:00.000Z');
  });

  it('мусор в отборе по коду ответа и в порогах отбрасывается', () => {
    const state = journalStateFromQuery({ status: 'drop table', min_duration: 'сто', since: 'вчера' });

    expect(state.status, 'на сервер уходит число, чужая строка вернулась бы отказом').toBe('');
    expect(state.minDuration).toBe('');
    expect(state.since).toBe('');
  });

  it('пустой адрес даёт отбор по умолчанию', () => {
    expect(journalStateFromQuery()).toMatchObject({ search: '', sort: 'created_at', order: 'desc', page: 1, perPage: 20 });
  });
});

describe('journalQueryFromState', () => {
  it('не пишет значения по умолчанию', () => {
    const state = journalStateFromQuery();
    expect(journalQueryFromState(state)).toEqual({});
  });

  it('пишет только то, что отличается от умолчания', () => {
    const state = { ...journalStateFromQuery(), method: 'DELETE', page: 2, sort: 'status', order: 'asc' };
    expect(journalQueryFromState(state)).toEqual({ method: 'DELETE', page: '2', sort: 'status', order: 'asc' });
  });
});

describe('statusFilterParams', () => {
  it('класс статусов разворачивается в границы диапазона', () => {
    expect(statusFilterParams('4xx')).toEqual({ status_min: 400, status_max: 499 });
    expect(statusFilterParams('5xx')).toEqual({ status_min: 500, status_max: 599 });
  });

  it('«все ошибки» это нижняя граница без верхней', () => {
    expect(statusFilterParams('errors')).toEqual({ status_min: 400 });
  });

  it('точный код уходит числом, пустой фильтр не добавляет параметров', () => {
    expect(statusFilterParams('404')).toEqual({ status: 404 });
    expect(statusFilterParams('')).toEqual({});
  });
});

describe('быстрый отбор', () => {
  const base = journalStateFromQuery();

  it('«только ошибки» включается и снимается тем же нажатием', () => {
    const on = toggleJournalPreset(base, 'errors');
    expect(on.status).toBe('errors');
    expect(isJournalPresetOn(on, 'errors')).toBe(true);

    const off = toggleJournalPreset(on, 'errors');
    expect(off.status).toBe('');
    expect(isJournalPresetOn(off, 'errors')).toBe(false);
  });

  it('«медленнее 1 с» ставит порог в миллисекундах', () => {
    const on = toggleJournalPreset(base, 'slow');
    expect(on.minDuration).toBe('1000');
    expect(isJournalPresetOn(on, 'slow')).toBe(true);
  });

  it('«последний час» отсчитывается от нажатия и вытесняет выбранные дни', () => {
    const now = new Date('2026-08-19T15:30:00.000Z');
    const on = toggleJournalPreset({ ...base, from: '2026-08-01', to: '2026-08-19' }, 'hour', now);

    expect(on.since).toBe('2026-08-19T14:30:00.000Z');
    expect(on.from, 'момент и день борются за одну границу периода').toBe('');
    expect(on.to).toBe('');
    expect(isJournalPresetOn(on, 'hour')).toBe(true);
  });

  it('быстрый отбор возвращает на первую страницу', () => {
    expect(toggleJournalPreset({ ...base, page: 7 }, 'slow').page).toBe(1);
  });

  it('отбор быстрых кнопок доезжает до адреса', () => {
    const on = toggleJournalPreset(base, 'slow');
    expect(journalQueryFromState(on)).toEqual({ min_duration: '1000' });
  });
});

describe('mergeJournalQuery', () => {
  it('чужие параметры адреса остаются на месте', () => {
    const next = mergeJournalQuery({ open: '42', method: 'GET' }, { ...journalStateFromQuery(), method: 'POST' });
    expect(next).toEqual({ open: '42', method: 'POST' });
  });

  it('снятый фильтр уходит из адреса', () => {
    const next = mergeJournalQuery({ method: 'GET' }, journalStateFromQuery());
    expect(next).toEqual({});
  });

  it('когда менять нечего, возвращает null и адрес не переписывается', () => {
    expect(mergeJournalQuery({ method: 'POST' }, { ...journalStateFromQuery(), method: 'POST' })).toBeNull();
  });
});

describe('длительность записи', () => {
  it('ответ быстрее миллисекунды не выглядит нулевым', () => {
    expect(formatDuration({ duration_us: 120 })).toBe('0.1мс');
    expect(formatMs(0.04)).toBe('0мс');
  });

  it('записи до перехода на микросекунды читаются из миллисекунд', () => {
    expect(formatDuration({ duration_us: null, duration_ms: 240 })).toBe('240мс');
  });
});

describe('дни календаря и отбор', () => {
  it('день собирается по локальным частям, а не по UTC', () => {
    // Полночь 1 августа в МСК - это 31 июля по UTC: toISOString отдал бы чужой день.
    expect(dateToYmd(new Date(2026, 7, 1))).toBe('2026-08-01');
    expect(dateToYmd(new Date(2026, 7, 20, 23, 59, 59))).toBe('2026-08-20');
    expect(dateToYmd(null)).toBe('');
    expect(dateToYmd(new Date('нет такой даты'))).toBe('');
  });

  it('день из адреса разбирается в ту же дату, а мусор - в пустоту', () => {
    const date = ymdToDate('2026-08-01');
    expect([date.getFullYear(), date.getMonth(), date.getDate()]).toEqual([2026, 7, 1]);
    expect(ymdToDate('')).toBeNull();
    expect(ymdToDate('01.08.2026')).toBeNull();
  });

  it('списки отбора несут выбранное значение, а пользователь - строковый идентификатор', () => {
    const state = { ...journalStateFromQuery({ method: 'post', user: '42' }) };
    const [method, status, user] = journalFilterDropdowns(state, [{ id: 42, username: 'ivanov' }], (u) => `@${u}`);

    expect(method.value).toBe('POST');
    expect(status.options[0]).toEqual({ value: '', label: 'Все статусы' });
    expect(user.value).toBe('42');
    // Список сверяет выбранное строго: числовой id никогда не совпал бы со строкой из адреса.
    expect(user.options[1]).toEqual({ value: '42', label: '@ivanov' });
  });

  it('архивная учётная запись в списке помечена, порядок берётся с сервера', () => {
    const state = { ...journalStateFromQuery({}) };
    const users = [
      { id: 1, username: 'active', is_active: true },
      { id: 2, username: 'fired', is_active: false }
    ];
    const [, , user] = journalFilterDropdowns(state, users, (u) => `@${u}`);

    expect(user.options.map(o => o.label)).toEqual([
      'Все пользователи', '@active', '@fired (архив)'
    ]);
  });

  it('ответ без признака активности не превращает весь список в архив', () => {
    const state = { ...journalStateFromQuery({}) };
    const [, , user] = journalFilterDropdowns(state, [{ id: 7, username: 'ivanov' }], (u) => `@${u}`);

    expect(user.options[1].label).toBe('@ivanov');
  });
});
