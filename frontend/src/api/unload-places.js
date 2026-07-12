import { apiRequest } from './client';

/** Групповые операции над местами разгрузки (id-keyed). Возвращают BulkOpResult. */
export async function bulkArchiveUnloadPlaces(ids) {
  const res = await apiRequest('/unload-places/bulk/archive', { method: 'POST', body: JSON.stringify({ ids }) });
  return res.json();
}

export async function bulkRestoreUnloadPlaces(ids) {
  const res = await apiRequest('/unload-places/bulk/restore', { method: 'POST', body: JSON.stringify({ ids }) });
  return res.json();
}
