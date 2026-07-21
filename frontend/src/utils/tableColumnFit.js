/**
 * Подбор столбцов таблицы проходной под доступную ширину (#1307).
 *
 * Таблицы делят ширину между столбцами и на 12 столбцах режут значения
 * многоточием. Вместо этого лишние столбцы скрываются: сначала наименее
 * важные, а при равной важности - те, что правее (левые столбцы считаются
 * приоритетнее). Скрытые поля остаются доступны в панели «Подробнее».
 */

/** Минимальная читаемая ширина столбца по имени поля. */
export const COLUMN_MIN_WIDTHS = {
  car_number: 120,
  car_brand: 100,
  organization: 150,
  company: 150,
  application_id: 135,
  unload_place: 135,
  valid_until: 110,
  time_range: 105,
  pass_time: 105,
  status: 150,
  last_name: 140,
  first_name: 120,
  middle_name: 120,
  position: 130,
  citizenship_name: 130,
};

/** Ширина столбца, для которого минимум не задан явно. */
export const DEFAULT_COLUMN_MIN_WIDTH = 120;

/** Служебные столбцы (въезд/выезд, удаление, «Подробнее») не скрываются. */
export const SERVICE_COLUMNS_WIDTH = {
  passage: 172,
  actions: 44,
  expand: 44,
};

/**
 * @param {string} fieldName
 * @returns {number} минимальная ширина столбца
 */
export function columnMinWidth(fieldName) {
  return COLUMN_MIN_WIDTHS[fieldName] ?? DEFAULT_COLUMN_MIN_WIDTH;
}

/**
 * Выбирает поля, которые не помещаются в доступную ширину.
 *
 * Порядок скрытия: сначала больший priority (менее важные), при равенстве -
 * больший order, то есть правые столбцы уходят раньше левых. Поле без
 * приоритета считается самым важным и скрывается последним.
 *
 * @param {object} params
 * @param {string[]} params.fields поля, видимые по настройкам таблицы
 * @param {number} params.available доступная ширина области столбцов
 * @param {Record<string, number>} [params.priorities] приоритет по полю (больше = менее важное)
 * @param {Record<string, number>} [params.orders] порядок столбца слева направо
 * @param {number} [params.reserved] ширина служебных столбцов
 * @param {number} [params.keepAtLeast] сколько полей оставить в любом случае
 * @returns {string[]} имена скрываемых полей
 */
export function pickOverflowFields({
  fields,
  available,
  priorities = {},
  orders = {},
  reserved = 0,
  keepAtLeast = 2,
}) {
  if (!Array.isArray(fields) || fields.length === 0) return [];
  // Ширина ещё не измерена (компонент не смонтирован) - ничего не скрываем,
  // иначе на первом кадре столбцы моргнут.
  if (!available || available <= 0) return [];

  let total = reserved + fields.reduce((sum, name) => sum + columnMinWidth(name), 0);
  if (total <= available) return [];

  const byLeastImportant = [...fields].sort((a, b) => {
    const pa = priorities[a] ?? 0;
    const pb = priorities[b] ?? 0;
    if (pa !== pb) return pb - pa;
    const oa = orders[a] ?? fields.indexOf(a);
    const ob = orders[b] ?? fields.indexOf(b);
    return ob - oa;
  });

  const hidden = [];
  for (const name of byLeastImportant) {
    if (total <= available) break;
    if (fields.length - hidden.length <= keepAtLeast) break;
    hidden.push(name);
    total -= columnMinWidth(name);
  }
  return hidden;
}
