import { describe, it, expect, vi, beforeEach } from 'vitest';

// #980 срез 5: обёртки версий таблицы. Проверяем реальную механику формы ответа,
// которую компонентные тесты (мокающие сам модуль) не покрывают: total из meta
// пагинации через apiRequestRaw (wrapJsonUnwrap отдал бы только data), разворот
// success-envelope и проброс ошибки при не-2xx (иначе 404 молча стал бы "пустой
// версией" - см. ревью среза).

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  apiRequestRaw: vi.fn(),
}));

import { listTableSnapshots, getTableSnapshot } from '@/api/system-tables';
import { apiRequestRaw } from '@/api/client';

function res(ok, body) {
  return { ok, status: ok ? 200 : 404, json: () => Promise.resolve(body) };
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('listTableSnapshots', () => {
  it('парсит items из data и total из meta пагинации', async () => {
    apiRequestRaw.mockResolvedValue(res(true, {
      success: true,
      data: [{ id: 1 }, { id: 2 }],
      meta: { total: 57, page: 1, per_page: 20 },
    }));

    const { items, total } = await listTableSnapshots(5, { page: 1, perPage: 20 });

    expect(items).toEqual([{ id: 1 }, { id: 2 }]);
    expect(total).toBe(57);
    // total из meta, а не из длины страницы (иначе футер врал бы при пагинации).
    expect(total).not.toBe(items.length);
  });

  it('передаёт page/per_page и фильтр периода в query', async () => {
    apiRequestRaw.mockResolvedValue(res(true, { success: true, data: [], meta: { total: 0 } }));

    await listTableSnapshots(9, { page: 3, perPage: 50, from: '2026-06-01', to: '2026-06-30' });

    const url = apiRequestRaw.mock.calls[0][0];
    expect(url).toContain('/system-tables/9/snapshots?');
    expect(url).toContain('page=3');
    expect(url).toContain('per_page=50');
    expect(url).toContain('from=2026-06-01');
    expect(url).toContain('to=2026-06-30');
  });

  it('без meta берёт total из длины data (фолбэк)', async () => {
    apiRequestRaw.mockResolvedValue(res(true, { success: true, data: [{ id: 1 }] }));
    const { total } = await listTableSnapshots(5);
    expect(total).toBe(1);
  });

  it('бросает при не-2xx', async () => {
    apiRequestRaw.mockResolvedValue(res(false, {}));
    await expect(listTableSnapshots(5)).rejects.toThrow();
  });
});

describe('getTableSnapshot', () => {
  it('разворачивает data из success-envelope', async () => {
    const snap = { id: 7, payload: { table_type: 'cars', rows: [] }, counts: {} };
    apiRequestRaw.mockResolvedValue(res(true, { success: true, data: snap }));

    const out = await getTableSnapshot(5, 7);
    expect(out).toEqual(snap);
    expect(apiRequestRaw).toHaveBeenCalledWith('/system-tables/5/snapshots/7');
  });

  it('бросает при 404 (снимок вычищен ретеншном), а не отдаёт "пустую версию"', async () => {
    apiRequestRaw.mockResolvedValue(res(false, { success: false, error: 'Snapshot not found' }));
    await expect(getTableSnapshot(5, 999)).rejects.toThrow();
  });
});
