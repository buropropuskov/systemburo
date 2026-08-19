import { describe, it, expect } from 'vitest';
import {
  journalStateFromQuery,
  journalQueryFromState,
  mergeJournalQuery
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
      from: '2026-08-01', to: '2026-08-19', sort: 'duration', order: 'asc',
      page: 3, perPage: 50
    });
  });

  it('чужое поле сортировки и чужой размер страницы заменяет своими', () => {
    const state = journalStateFromQuery({ sort: 'response_body', order: 'asc', per_page: '999', page: '0' });

    expect(state.sort).toBe('created_at');
    expect(state.order, 'направление без известного поля тоже сбрасывается').toBe('desc');
    expect(state.perPage).toBe(20);
    expect(state.page).toBe(1);
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
