import { apiRequest } from './client';

/**
 * Корзина системных таблиц (#186).
 * Бэк сам определяет тип элементов (cars/people) по table_type системной таблицы.
 */

export async function listTrash(systemTableID, { search = '', organizationId = 0, dateFrom = '', dateTo = '' } = {}) {
  const params = new URLSearchParams();
  if (search) params.set('search', search);
  if (organizationId) params.set('organization_id', String(organizationId));
  if (dateFrom) params.set('date_from', dateFrom);
  if (dateTo) params.set('date_to', dateTo);
  const qs = params.toString();
  const res = await apiRequest(`/system-tables/${systemTableID}/trash${qs ? '?' + qs : ''}`);
  return res.json();
}

export async function restoreItems(systemTableID, ids) {
  const res = await apiRequest(`/system-tables/${systemTableID}/trash/restore`, {
    method: 'POST',
    body: JSON.stringify({ ids }),
  });
  return res.json();
}

export async function purgeItem(systemTableID, itemID) {
  const res = await apiRequest(`/system-tables/${systemTableID}/trash/${itemID}`, {
    method: 'DELETE',
  });
  return res.json();
}

export async function clearTrash(systemTableID) {
  const res = await apiRequest(`/system-tables/${systemTableID}/trash`, {
    method: 'DELETE',
  });
  return res.json();
}
