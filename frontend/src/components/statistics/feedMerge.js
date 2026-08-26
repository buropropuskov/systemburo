/**
 * Стабильный ключ записи живой ленты. У бэка нет id для проходов/проездов,
 * поэтому ключ собираем из времени, субъекта и направления.
 * @param {{ created_at: string, subject: string, action_type: string }} row
 * @returns {string}
 */
export function feedRowKey(row) {
  return `${row.created_at}|${row.subject}|${row.action_type}`;
}

/**
 * Слить входящие записи ленты с текущими: новые (по ключу) добавляются сверху,
 * существующие остаются на месте. Стабильный порядок -> Vue не перерисовывает
 * старые строки на каждом тике, анимация появления играет только на новых.
 * @param {Array} current  текущие записи (новые сверху)
 * @param {Array} incoming входящие, отсортированы по времени по убыванию
 * @param {number} [max=50] ограничение размера ленты
 * @returns {Array} тот же массив current, если новых нет (ссылочная стабильность)
 */
export function mergeFeed(current, incoming, max = 50) {
  const existing = new Set(current.map(feedRowKey));
  const fresh = incoming.filter((row) => !existing.has(feedRowKey(row)));
  if (fresh.length === 0) return current;
  return [...fresh, ...current].slice(0, max);
}
