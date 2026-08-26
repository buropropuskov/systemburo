/**
 * Прогоняет элементы через async-mapper с ограничением параллелизма: не более
 * `limit` промисов в полёте одновременно. Нужен, чтобы веер запросов (например
 * привязка новых ТС/сотрудников при подаче заявки) не выстреливал сотнями
 * одновременных POST и не упирался в rate limiter. Хвост, отбитый лимитом,
 * до-выполняется благодаря backoff-повтору в api/client.js.
 *
 * Семантика результата как у Promise.all: массив в порядке `items`, первый
 * reject отклоняет весь вызов (уже запущенные воркеры до-завершают текущий шаг).
 *
 * @template T, R
 * @param {T[]} items - входные элементы
 * @param {number} limit - максимум одновременных выполнений (>=1)
 * @param {(item: T, index: number) => Promise<R>} mapper - async-обработчик элемента
 * @returns {Promise<R[]>} результаты в порядке items
 */
export async function mapWithConcurrency(items, limit, mapper) {
  const list = Array.isArray(items) ? items : []
  const results = new Array(list.length)
  if (list.length === 0) return results

  const size = Math.min(Math.max(1, Math.floor(limit) || 1), list.length)
  let cursor = 0

  async function worker() {
    while (cursor < list.length) {
      const index = cursor
      cursor += 1
      results[index] = await mapper(list[index], index)
    }
  }

  const workers = []
  for (let w = 0; w < size; w += 1) {
    workers.push(worker())
  }
  await Promise.all(workers)
  return results
}
