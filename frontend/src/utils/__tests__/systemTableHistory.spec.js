import { beforeEach, describe, expect, it, vi } from 'vitest';

const apiRequestRaw = vi.hoisted(() => vi.fn());
vi.mock('@/api/client', () => ({ apiRequestRaw, apiRequest: vi.fn() }));
import { collectSystemTableHistory, fetchSystemTableHistoryPage } from '../systemTableHistory';

function response(items, total, page = 1, meta = {}) {
  return { ok: true, json: async () => ({ success: true, data: items, meta: {
    total, page, per_page: 200, snapshot_max_id: 501, name_mask_revision: 'policy1', ...meta,
  } }) };
}
function rows(start, count) {
  return Array.from({ length: count }, (_, i) => ({ id: start + i }));
}

describe('system table history bounded page collection', () => {
  beforeEach(() => apiRequestRaw.mockReset());

  it('collects every filtered page with a fixed audit boundary and frozen filters', async () => {
    const filters = { search: 'Начальное значение', userId: 4, dateFrom: '2030-01-02', order: 'asc' };
    apiRequestRaw.mockImplementationOnce(async () => {
      filters.search = 'Изменено после начала';
      return response(rows(1, 200), 201);
    }).mockResolvedValueOnce(response(rows(201, 1), 201, 2));
    const result = await collectSystemTableHistory(7, filters);
    expect(result).toHaveLength(201);
    const queries = apiRequestRaw.mock.calls.map(([path]) => new URLSearchParams(path.split('?')[1]));
    expect(queries[0].get('search')).toBe('Начальное значение');
    expect(queries[1].get('search')).toBe('Начальное значение');
    expect(queries[1].get('snapshot_max_id')).toBe('501');
    expect(queries[1].get('page')).toBe('2');
    expect(queries[1].get('per_page')).toBe('200');
    expect(queries[1].get('user_id')).toBe('4');
  });

  it.each([
    ['duplicate IDs', response([{ id: 200 }], 201, 2)],
    ['missing page', response([], 201, 2)],
    ['changed total', response([{ id: 201 }], 202, 2)],
    ['changed boundary', response([{ id: 201 }], 201, 2, { snapshot_max_id: 502 })],
    ['changed consent policy', response([{ id: 201 }], 201, 2, { name_mask_revision: 'policy2' })],
    ['HTTP failure', { ok: false, status: 503 }],
  ])('rejects %s instead of returning a partial workbook dataset', async (_, nextPage) => {
    apiRequestRaw.mockResolvedValueOnce(response(rows(1, 200), 201)).mockResolvedValueOnce(nextPage);
    await expect(collectSystemTableHistory(7)).rejects.toThrow();
  });

  it('pages complete distinct actor options with the same snapshot and policy', async () => {
    apiRequestRaw.mockResolvedValueOnce(response(rows(1, 200), 201)).mockResolvedValueOnce(response([{ id: 201 }], 201, 2));
    const result = await collectSystemTableHistory(7, {}, { filterOptions: true, snapshotMaxID: 501, expectedRevision: 'policy1' });
    expect(result).toHaveLength(201);
    for (const [path] of apiRequestRaw.mock.calls) {
      expect(path).toContain('filter_options=1');
      expect(path).toContain('snapshot_max_id=501');
    }
  });

  it('rejects a response without a snapshot and rejects canceled collections', async () => {
    apiRequestRaw.mockResolvedValueOnce(response([], 0, 1, { snapshot_max_id: undefined }));
    await expect(fetchSystemTableHistoryPage(7)).rejects.toThrow();
    const controller = new AbortController();
    controller.abort();
    await expect(collectSystemTableHistory(7, {}, { signal: controller.signal })).rejects.toThrow();
    expect(apiRequestRaw).toHaveBeenCalledTimes(1);
  });
});
