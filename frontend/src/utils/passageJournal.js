/**
 * Журнал проходов: страницы, фильтры и выгрузка (#2469).
 *
 * До этого модалки журнала грузили всю историю входов и выходов одним запросом и
 * искали по загруженному массиву. Здесь собрана серверная сторона этой работы, чтобы
 * журнал машин и журнал людей не разъезжались в том, как считают страницу и период.
 */
import { apiRequest, apiRequestRaw } from '@/api/client';

/** Порция подгрузки в модалке. */
export const PASSAGE_PAGE_SIZE = 50;

/** Порция при сборе строк для выгрузки: файл собирается редко, крупные куски дешевле. */
export const PASSAGE_EXPORT_PAGE_SIZE = 200;

/**
 * Предел выгрузки. Больше 20 тысяч строк ExcelJS в браузере собирает минуты, поэтому
 * упираемся в предел и говорим об этом человеку: молча обрезанный файл хуже - по нему
 * считают итоги за период, не зная, что период в него не поместился.
 */
export const PASSAGE_EXPORT_LIMIT = 20000;

/**
 * Параметры запроса журнала. entityKey - имя параметра сущности: car_id у машин,
 * employee_id у людей.
 *
 * @param {{search?: string, userId?: number|null, entityId?: number|null, dateFrom?: string, dateTo?: string, order?: string}} filters
 * @param {{page?: number, perPage?: number, entityKey?: string}} [options]
 * @returns {URLSearchParams}
 */
export function passageParams(filters = {}, options = {}) {
  const { page = 1, perPage = PASSAGE_PAGE_SIZE, entityKey = 'car_id' } = options;
  const params = new URLSearchParams({
    page: String(page),
    per_page: String(perPage),
    order: filters.order === 'asc' ? 'asc' : 'desc',
  });
  const search = (filters.search || '').trim();
  if (search) params.set('search', search);
  if (filters.userId) params.set('user_id', String(filters.userId));
  if (filters.entityId) params.set(entityKey, String(filters.entityId));
  if (filters.dateFrom) params.set('date_from', filters.dateFrom);
  if (filters.dateTo) params.set('date_to', filters.dateTo);
  return params;
}

/**
 * Страница журнала вместе с общим числом строк по фильтру.
 *
 * Читается через apiRequestRaw: обычная обёртка отдаёт только data, а нам нужен meta -
 * без total нечего показать в «загружено 50 из 431» и непонятно, когда подгрузка кончилась.
 *
 * @param {string} path
 * @param {object} filters
 * @param {object} [options]
 * @returns {Promise<{items: object[], total: number, page: number, perPage: number}>}
 */
export async function fetchPassagePage(path, filters, options = {}) {
  const response = await apiRequestRaw(`${path}?${passageParams(filters, options)}`);
  if (!response.ok) {
    throw new Error(`Журнал проходов: ${response.status}`);
  }
  const body = await response.json();
  if (!body || body.success !== true) {
    throw new Error(body?.error || 'Журнал проходов: ответ без данных');
  }
  return {
    items: body.data || [],
    total: body.meta?.total || 0,
    page: body.meta?.page || 1,
    perPage: body.meta?.per_page || PASSAGE_PAGE_SIZE,
  };
}

/**
 * Значения выпадающих списков журнала в этой области: кто отмечал проходы и, у людей,
 * кого отмечали. У машин список сущностей не приходит - его модалка берёт от таблицы
 * проходной, в которой открыта.
 *
 * @param {string} path
 * @param {number|null} [tableId] сузить до таблицы проходной
 * @returns {Promise<{users: object[], employees: object[]}>}
 */
export async function fetchPassageFilterOptions(path, tableId = null) {
  const response = await apiRequest(`${path}${tableId ? `?table_id=${tableId}` : ''}`, { method: 'GET' });
  if (!response.ok) {
    throw new Error(`Фильтры журнала: ${response.status}`);
  }
  const data = await response.json();
  return { users: data?.users || [], employees: data?.employees || [] };
}

/**
 * Собирает строки журнала по текущему фильтру для выгрузки: страницами, а не одним
 * запросом на всю историю. Возвращает и общее число строк, чтобы файл мог честно
 * сказать, что в него попало не всё.
 *
 * @param {string} path
 * @param {object} filters
 * @param {{entityKey?: string, limit?: number, perPage?: number}} [options]
 * @returns {Promise<{rows: object[], total: number, truncated: boolean}>}
 */
export async function collectPassageRows(path, filters, options = {}) {
  const perPage = options.perPage || PASSAGE_EXPORT_PAGE_SIZE;
  const limit = options.limit || PASSAGE_EXPORT_LIMIT;
  const rows = [];
  let total = 0;
  for (let page = 1; rows.length < limit; page += 1) {
    const chunk = await fetchPassagePage(path, filters, { ...options, page, perPage });
    total = chunk.total;
    rows.push(...chunk.items);
    if (chunk.items.length < perPage || rows.length >= total) break;
  }
  return { rows: rows.slice(0, limit), total, truncated: total > rows.slice(0, limit).length };
}
