import { apiRequestRaw } from '@/api/client';
import { passageParams } from '@/utils/passageJournal';

export async function fetchSystemTableHistoryPage(tableId, filters = {}, options = {}) {
  const params = passageParams(filters, options);
  if (options.snapshotMaxID !== null && options.snapshotMaxID !== undefined) {
    params.set('snapshot_max_id', String(options.snapshotMaxID));
  }
  if (options.filterOptions) params.set('filter_options', '1');
  const response = await apiRequestRaw(`/system-tables/${tableId}/history?${params}`, {
    signal: options.signal,
    silent: true,
  });
  if (!response.ok) throw new Error('Не удалось загрузить историю таблицы');
  const body = await response.json();
  const meta = body?.meta;
  if (body?.success !== true || !Array.isArray(body.data)
    || !Number.isSafeInteger(meta?.total) || meta.total < 0
    || !Number.isSafeInteger(meta?.page) || meta.page < 1
    || !Number.isSafeInteger(meta?.per_page) || meta.per_page < 1 || meta.per_page > 200
    || !Number.isSafeInteger(meta?.snapshot_max_id) || meta.snapshot_max_id < 0
    || typeof meta?.name_mask_revision !== 'string' || !meta.name_mask_revision) {
    throw new Error('Некорректная страница истории таблицы');
  }
  return {
    items: body.data, total: meta.total, page: meta.page, perPage: meta.per_page,
    snapshotMaxID: meta.snapshot_max_id, nameMaskRevision: meta.name_mask_revision,
  };
}

// Explicit export and actor options traverse bounded responses. A fixed maximum
// audit ID excludes concurrent inserts; changes/deletions/privacy transitions
// abort the collection rather than producing a plausible incomplete workbook.
export async function collectSystemTableHistory(tableId, filters = {}, options = {}) {
  const frozenFilters = { ...filters };
  const rows = [];
  const ids = new Set();
  let snapshotMaxID = options.snapshotMaxID;
  let revision = options.expectedRevision;
  let total;
  for (let page = 1; ; page += 1) {
    if (options.signal?.aborted) throw new Error('Сбор истории отменён');
    const result = await fetchSystemTableHistoryPage(tableId, frozenFilters, {
      ...options, page, perPage: 200, snapshotMaxID,
    });
    if (page === 1) {
      total = result.total;
      snapshotMaxID ??= result.snapshotMaxID;
      revision ??= result.nameMaskRevision;
    }
    if (options.signal?.aborted || result.page !== page || result.total !== total
      || result.snapshotMaxID !== snapshotMaxID || result.nameMaskRevision !== revision
      || result.items.length > result.perPage) {
      throw new Error('История изменилась. Повтори выгрузку');
    }
    for (const row of result.items) {
      if (!Number.isSafeInteger(row.id) || row.id <= 0 || ids.has(row.id)) {
        throw new Error('Получена неполная история таблицы');
      }
      ids.add(row.id);
      rows.push(row);
    }
    if (rows.length === total) return rows;
    if (rows.length > total || result.items.length < result.perPage) {
      throw new Error('Получена неполная история таблицы');
    }
  }
}
