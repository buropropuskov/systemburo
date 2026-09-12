import { describe, it, expect, vi, beforeEach } from 'vitest';

const apiRequest = vi.fn();
const apiRequestRaw = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
  apiRequestRaw: (...args) => apiRequestRaw(...args),
}));

import {
  PASSAGE_PAGE_SIZE,
  collectPassageRows,
  fetchPassageFilterOptions,
  fetchPassagePage,
  passageParams,
} from '../passageJournal';

function page(items, total) {
  return { ok: true, json: async () => ({ success: true, data: items, meta: { total, page: 1, per_page: items.length } }) };
}

beforeEach(() => {
  apiRequest.mockReset();
  apiRequestRaw.mockReset();
});

describe('passageParams', () => {
  it('всегда задаёт страницу, размер и порядок', () => {
    const params = passageParams({});
    expect(params.get('page')).toBe('1');
    expect(params.get('per_page')).toBe(String(PASSAGE_PAGE_SIZE));
    expect(params.get('order')).toBe('desc');
  });

  it('пустые фильтры в запрос не уходят', () => {
    const params = passageParams({ search: '   ', userId: null, entityId: 0, dateFrom: '', dateTo: '' });
    expect(params.has('search')).toBe(false);
    expect(params.has('user_id')).toBe(false);
    expect(params.has('car_id')).toBe(false);
    expect(params.has('date_from')).toBe(false);
  });

  // Имя параметра сущности разное: у машин car_id, у людей employee_id. Ошибка здесь
  // тихо снимает фильтр - сервер просто не увидит неизвестный параметр.
  it('ключ сущности подставляется по сущности журнала', () => {
    expect(passageParams({ entityId: 7 }, { entityKey: 'car_id' }).get('car_id')).toBe('7');
    expect(passageParams({ entityId: 7 }, { entityKey: 'employee_id' }).get('employee_id')).toBe('7');
  });

  it('поиск уходит без окружающих пробелов, порядок читается только из asc', () => {
    const params = passageParams({ search: '  A001AA  ', order: 'asc' });
    expect(params.get('search')).toBe('A001AA');
    expect(params.get('order')).toBe('asc');
    expect(passageParams({ order: 'сначала новые' }).get('order')).toBe('desc');
  });
});

describe('fetchPassagePage', () => {
  it('отдаёт строки вместе с общим числом из meta', async () => {
    apiRequestRaw.mockResolvedValue(page([{ id: 1 }], 431));
    const result = await fetchPassagePage('/cars/history/all', {});
    expect(result.items).toHaveLength(1);
    expect(result.total).toBe(431);
  });

  // Ошибку нельзя превращать в пустой журнал: пустой список читается как «проходов не
  // было», и человек решит, что отметки пропали.
  it('сбой запроса пробрасывается, а не отдаёт пустой список', async () => {
    apiRequestRaw.mockResolvedValue({ ok: false, status: 500, json: async () => ({}) });
    await expect(fetchPassagePage('/cars/history/all', {})).rejects.toThrow();

    apiRequestRaw.mockResolvedValue({ ok: true, json: async () => ({ success: false, error: 'нет доступа' }) });
    await expect(fetchPassagePage('/cars/history/all', {})).rejects.toThrow('нет доступа');
  });
});

describe('collectPassageRows', () => {
  it('собирает все страницы по фильтру', async () => {
    apiRequestRaw
      .mockResolvedValueOnce(page([{ id: 1 }, { id: 2 }], 3))
      .mockResolvedValueOnce(page([{ id: 3 }], 3));
    const { rows, total, truncated } = await collectPassageRows('/cars/history/all', {}, { perPage: 2 });
    expect(rows.map(r => r.id)).toEqual([1, 2, 3]);
    expect(total).toBe(3);
    expect(truncated).toBe(false);
    expect(apiRequestRaw).toHaveBeenCalledTimes(2);
  });

  // Предел выгрузки обязан быть виден вызывающему: молча обрезанный файл выглядит как
  // полный журнал за период.
  it('упёршись в предел, сообщает об обрезке и больше не запрашивает', async () => {
    apiRequestRaw.mockResolvedValue(page([{ id: 1 }, { id: 2 }], 100));
    const { rows, total, truncated } = await collectPassageRows('/cars/history/all', {}, { perPage: 2, limit: 2 });
    expect(rows).toHaveLength(2);
    expect(total).toBe(100);
    expect(truncated).toBe(true);
    expect(apiRequestRaw).toHaveBeenCalledTimes(1);
  });
});

describe('fetchPassageFilterOptions', () => {
  it('сужает список таблицей проходной', async () => {
    apiRequest.mockResolvedValue({ ok: true, json: async () => ({ users: [{ id: 3, name: 'Иванов' }] }) });
    const options = await fetchPassageFilterOptions('/cars/history/filter-options', 42);
    expect(apiRequest).toHaveBeenCalledWith('/cars/history/filter-options?table_id=42', { method: 'GET' });
    expect(options.users).toHaveLength(1);
    expect(options.employees).toEqual([]);
  });

  // Журнал людей отдаёт вторым списком тех, кого отмечали: у машин этот список берётся
  // от таблицы проходной, а сотрудников таблица целиком не знает.
  it('пробрасывает список людей, когда сервер его отдал', async () => {
    apiRequest.mockResolvedValue({
      ok: true,
      json: async () => ({ users: [], employees: [{ id: 11, last_name: 'Петров' }] }),
    });
    const options = await fetchPassageFilterOptions('/employees/history/filter-options', 4);
    expect(options.employees).toHaveLength(1);
  });

  it('без таблицы берёт весь журнал', async () => {
    apiRequest.mockResolvedValue({ ok: true, json: async () => ({ users: [] }) });
    await fetchPassageFilterOptions('/cars/history/filter-options', null);
    expect(apiRequest).toHaveBeenCalledWith('/cars/history/filter-options', { method: 'GET' });
  });
});
