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

import {
  listTableSnapshots,
  getTableSnapshot,
  createTableSnapshot,
  exportTableSnapshot,
  cleanupTableSnapshots,
} from '@/api/system-tables';
import { apiRequestRaw } from '@/api/client';

function res(ok, body) {
  return { ok, status: ok ? 200 : 404, json: () => Promise.resolve(body) };
}

function blobRes(ok, { disposition = '', blob = new Blob(['x']) } = {}) {
  return {
    ok,
    status: ok ? 200 : 404,
    blob: () => Promise.resolve(blob),
    headers: { get: (h) => (h === 'Content-Disposition' ? disposition : null) },
  };
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

describe('createTableSnapshot (#980 срез 6)', () => {
  it('POST-ит ручной снимок и разворачивает { id, message }', async () => {
    apiRequestRaw.mockResolvedValue(res(true, { success: true, data: { id: 42, message: 'ok' } }));

    const out = await createTableSnapshot(5);

    expect(out).toEqual({ id: 42, message: 'ok' });
    const [url, opts] = apiRequestRaw.mock.calls[0];
    expect(url).toBe('/system-tables/5/snapshots');
    expect(opts).toMatchObject({ method: 'POST', silent403: true });
  });

  it('бросает при не-2xx (провал не проглатывается в {message})', async () => {
    apiRequestRaw.mockResolvedValue(res(false, { success: false, error: 'boom' }));
    await expect(createTableSnapshot(5)).rejects.toThrow();
  });
});

describe('exportTableSnapshot (#980 срез 6)', () => {
  it('передаёт format и достаёт кириллическое имя из filename* (RFC 5987)', async () => {
    const disposition = "attachment; filename=\"snapshot.xlsx\"; filename*=UTF-8''%D0%9A%D0%9F%D0%9F-1.xlsx";
    apiRequestRaw.mockResolvedValue(blobRes(true, { disposition }));

    const { blob, filename } = await exportTableSnapshot(5, 7, 'xlsx');

    expect(blob).toBeInstanceOf(Blob);
    expect(filename).toBe('КПП-1.xlsx');
    const url = apiRequestRaw.mock.calls[0][0];
    expect(url).toBe('/system-tables/5/snapshots/7/export?format=xlsx');
  });

  it('поддерживает current и pdf, имя-фолбэк без Content-Disposition', async () => {
    apiRequestRaw.mockResolvedValue(blobRes(true, { disposition: '' }));

    const { filename } = await exportTableSnapshot(5, 'current', 'pdf');

    expect(filename).toBe('snapshot.pdf');
    expect(apiRequestRaw.mock.calls[0][0]).toBe('/system-tables/5/snapshots/current/export?format=pdf');
  });

  it('бросает при не-2xx', async () => {
    apiRequestRaw.mockResolvedValue(blobRes(false));
    await expect(exportTableSnapshot(5, 7, 'xlsx')).rejects.toThrow();
  });
});

describe('cleanupTableSnapshots (#980 срез 6)', () => {
  it('DELETE с older_than и разворачивает { deleted }', async () => {
    apiRequestRaw.mockResolvedValue(res(true, { success: true, data: { deleted: 3, message: 'ok' } }));

    const out = await cleanupTableSnapshots(5, 24);

    expect(out).toEqual({ deleted: 3, message: 'ok' });
    const [url, opts] = apiRequestRaw.mock.calls[0];
    expect(url).toBe('/system-tables/5/snapshots?older_than=24');
    expect(opts).toMatchObject({ method: 'DELETE', silent403: true });
  });

  it('бросает при 403 (не admin)', async () => {
    apiRequestRaw.mockResolvedValue({ ok: false, status: 403, json: () => Promise.resolve({}) });
    await expect(cleanupTableSnapshots(5, 24)).rejects.toThrow();
  });
});
