import { apiRequest } from './client';

/**
 * Доназначение мест элементам заявки принимающим (#1393).
 * unwrap бросает на !res.ok с сообщением бэка (fallback - если тела нет),
 * чтобы 4xx не прошёл как молчаливый успех.
 */

async function unwrap(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

/**
 * Назначает посты проезда или прохода элементам заявки.
 * @param {number} applicationId
 * @param {{elementType: 'cars'|'people', elementIds: number[], tableIds: number[], mode: 'add'|'replace'}} params
 */
export async function assignElementTables(applicationId, { elementType, elementIds, tableIds, mode }) {
  const res = await apiRequest(`/applications/${applicationId}/elements/tables`, {
    method: 'PUT',
    body: JSON.stringify({
      element_type: elementType,
      element_ids: elementIds,
      table_ids: tableIds,
      mode,
    }),
  });
  return unwrap(res, 'Не удалось обновить посты');
}

/**
 * Назначает места разгрузки машинам заявки.
 * @param {number} applicationId
 * @param {{carIds: number[], placeIds: number[], mode: 'add'|'replace'}} params
 */
export async function assignCarUnloadPlaces(applicationId, { carIds, placeIds, mode }) {
  const res = await apiRequest(`/applications/${applicationId}/elements/unload-places`, {
    method: 'PUT',
    body: JSON.stringify({
      car_ids: carIds,
      place_ids: placeIds,
      mode,
    }),
  });
  return unwrap(res, 'Не удалось обновить места разгрузки');
}

/**
 * Убирает людей или машины из поданной заявки. Доступно принимающему.
 * Удаление мягкое: строка уходит в корзину, история остаётся.
 * @param {number} applicationId
 * @param {{elementType: 'cars'|'people', elementIds: number[], reason: string}} params
 * @returns {Promise<{data?: {removed: number}}>}
 */
export async function removeApplicationElements(applicationId, { elementType, elementIds, reason }) {
  const res = await apiRequest(`/applications/${applicationId}/elements`, {
    method: 'DELETE',
    body: JSON.stringify({
      element_type: elementType,
      element_ids: elementIds,
      reason,
    }),
  });
  return unwrap(res, 'Не удалось убрать элемент из заявки');
}
