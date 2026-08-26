/**
 * Контракт колонок агрегатного отчёта (движок отчётов, #1240) в одном месте:
 * таблица на экране и выгрузка Excel/PDF читают значения одинаково и не разъедутся.
 *
 * Транспорт значения задаёт сама колонка:
 * - `float=true` -> значение в `float_values`/`float_totals` (доли, средние);
 * - иначе -> в `values`/`totals` (счётчики, длительности в секундах, pivot cross-tab).
 * Формат задаёт `type` (`'duration'` — секунды).
 */

/**
 * Длительность (секунды) — форматируется как «2 ч 15 мин», а не как число.
 * @param {{type?: string}|null|undefined} col
 * @returns {boolean}
 */
export function isDurationColumn(col) {
  return col?.type === 'duration';
}

/**
 * Производная метрика (длительность, доля, среднее) — в отличие от счётчика её
 * пустому бину движок НЕ дорисовывает 0: «нет данных» неотличимо от реального
 * значения (у длительности — от «прошло мгновенно», у доли — от «отказов не было»).
 * Отсюда правило чтения: нет ключа -> нет данных, а не ноль (metricOmitsFakeZero).
 * @param {{type?: string, float?: boolean}|null|undefined} col
 * @returns {boolean}
 */
export function isDerivedColumn(col) {
  return isDurationColumn(col) || col?.float === true;
}

/**
 * Значение колонки из бакета строки/итогов.
 * @param {object|null|undefined} bucket values/totals
 * @param {object|null|undefined} floatBucket float_values/float_totals
 * @param {{key: string, type?: string, float?: boolean}|null|undefined} col
 * @returns {number|null} null — «нет данных» (только у производных колонок)
 */
export function metricValue(bucket, floatBucket, col) {
  if (!col) return 0;
  const raw = col.float ? floatBucket?.[col.key] : bucket?.[col.key];
  if (raw != null) return Number(raw);
  return isDerivedColumn(col) ? null : 0;
}
