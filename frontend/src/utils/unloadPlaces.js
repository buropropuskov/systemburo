/**
 * Подпись мест разгрузки в строке таблицы «Проезд».
 *
 * Одна и та же сборка жила копией в таблице машин и в таблице «по факту»: список
 * идентификаторов резолвится по справочнику, пустые имена отбрасываются, а из
 * нескольких мест показывается первое с пометкой «и др.» - в колонку больше не
 * влезает.
 *
 * @param {{unload_place_ids?: number[], unload_place?: string}} item строка таблицы
 * @param {Array<{id: number, name: string}>} places справочник мест разгрузки
 * @returns {string}
 */
export function formatUnloadPlaces(item, places) {
  const ids = item?.unload_place_ids;
  if (!ids?.length) return item?.unload_place || '-';

  const names = ids
    .map((id) => places?.find((p) => p.id === id)?.name)
    .filter(Boolean);
  if (!names.length) return '-';
  return names.length === 1 ? names[0] : `${names[0]} и др.`;
}
