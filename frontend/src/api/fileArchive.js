import { apiRequest, apiRequestRaw } from './client';

/**
 * API клиент раздела «Файловый архив» бланков (#1615): показ действующих настроек,
 * ручное пересоздание файлов заявки, сводка «Обзора», оценка и скачивание ZIP за
 * период, донаполнение и список реестра для вкладки «Ошибки».
 *
 * Правки настроек здесь нет: раскладку каталогов и пороги места задаёт команда
 * server archive на сервере, роутов записи в API не существует.
 */

async function unwrap(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

/**
 * Настройки файлового архива: рубильник, шаблоны раскладки, квота и пороги.
 * @returns {Promise<object>} настройки без обёртки: apiRequest уже развернул конверт
 */
export async function getArchiveSettings() {
  const res = await apiRequest('/file-archive/settings');
  return unwrap(res, 'Не удалось загрузить настройки файлового архива');
}




/**
 * Сводка вкладки «Обзор»: занятое место реестра, файлы, состав диска и
 * разбивка по месяцам. Бэк держит ответ в кэше 5 минут - вкладка не обязана
 * дёргать агрегаты по реестру и обход каталогов на каждое открытие.
 * @returns {Promise<object>}
 */
export async function getArchiveStats() {
  const res = await apiRequest('/file-archive/stats');
  return unwrap(res, 'Не удалось загрузить сводку файлового архива');
}

/**
 * Пересоздаёт файлы заявки в архиве по текущим данным и настройкам - для
 * случаев, когда ждать фоновую очередь нечестно (администратор поправил
 * шаблон или починил бланк и должен увидеть результат сразу).
 * @param {number} applicationId
 */
export async function reexportApplication(applicationId) {
  const res = await apiRequest(`/file-archive/applications/${applicationId}/reexport`, {
    method: 'POST',
  });
  return unwrap(res, 'Не удалось пересоздать файлы заявки в архиве');
}

/**
 * Оценивает объём и число файлов ZIP-выгрузки за период до фактического
 * скачивания - конструктор показывает «~740 МБ», не запуская сам ZIP.
 * @param {{dateFrom: string, dateTo: string}} period  границы включительно, YYYY-MM-DD
 * @returns {Promise<{file_count: number, bytes: number, exceeds_limit: boolean}>}
 */
export async function estimateArchiveDownload({ dateFrom, dateTo } = {}) {
  const res = await apiRequest('/file-archive/estimate', {
    method: 'POST',
    body: JSON.stringify({ date_from: dateFrom, date_to: dateTo }),
  });
  return unwrap(res, 'Не удалось оценить объём выгрузки');
}

/**
 * Выдаёт одноразовый билет на потоковый ZIP за период (TTL 60 с) - дальше
 * startTicketDownload (utils/download.js) открывает публичный
 * GET /file-archive/download?ticket=... прямой навигацией.
 * @param {{dateFrom: string, dateTo: string}} period
 * @returns {Promise<{ticket: string}>}
 */
export async function issueArchiveDownloadTicket({ dateFrom, dateTo } = {}) {
  const res = await apiRequest('/file-archive/download-ticket', {
    method: 'POST',
    body: JSON.stringify({ date_from: dateFrom, date_to: dateTo }),
  });
  return unwrap(res, 'Не удалось получить билет на скачивание');
}

/**
 * Ставит в очередь пересборку бланков заявок периода, по желанию суженную типом
 * вложения (тот же запрос обслуживает и «пересоздать бланки этого типа» после
 * правки шаблона). Ответ асинхронный - разбор идёт фоновым воркером.
 * @param {{dateFrom: string, dateTo: string, uniqueAttachmentId?: number|null}} params
 * @returns {Promise<{queued: number}>}
 */
export async function runArchiveBackfill({ dateFrom, dateTo, uniqueAttachmentId } = {}) {
  const body = { date_from: dateFrom, date_to: dateTo };
  if (uniqueAttachmentId) body.unique_attachment_id = uniqueAttachmentId;
  const res = await apiRequest('/file-archive/backfill', {
    method: 'POST',
    body: JSON.stringify(body),
  });
  return unwrap(res, 'Не удалось поставить бэкфилл в очередь');
}

/**
 * Список строк реестра файлового архива постранично - вкладка «Ошибки» и
 * подсчёт очереди для прогресса бэкфилла. Пагинация лежит в envelope.meta
 * рядом с data, а apiRequest снимает только data и meta теряется - читаем
 * сырой ответ через apiRequestRaw (см. getApplicationsPaginated в api/applications.js).
 * @param {{status?: string, applicationId?: number, page?: number, perPage?: number}} params
 * @returns {Promise<{items: object[], meta: {total: number, page: number, per_page: number}}>}
 */
export async function listArchiveItems({
  status = '', applicationId, page = 1, perPage = 20,
} = {}) {
  const params = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  if (status) params.set('status', status);
  if (applicationId) params.set('application_id', String(applicationId));
  const res = await apiRequestRaw(`/file-archive/items?${params.toString()}`);
  const body = await res.json();
  if (!res.ok || !body || !body.success) {
    throw new Error(body?.error || 'Не удалось загрузить список файлового архива');
  }
  return {
    items: body.data || [],
    meta: body.meta || { total: 0, page: 1, per_page: perPage },
  };
}
