import { apiRequest } from './client';

/**
 * API клиент раздела «Файловый архив» бланков (#1615): настройки раскладки,
 * реестр плейсхолдеров для конструктора пути, живое превью и ручное
 * пересоздание файлов заявки. Скачивание и статистика (по билетам и
 * дисковому месту) приезжают следующими срезами эпика.
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
 * Частичное обновление настроек - только присланные поля меняются (бэк ждёт
 * указатели: отсутствующий ключ значит «не трогать»).
 * @param {{enabled?: boolean, dirTemplate?: string, fileTemplate?: string,
 *   quotaBytes?: number, minFreeBytes?: number, warnPercent?: number,
 *   recheckDays?: number, freezeAfterDays?: number, zipMaxBytes?: number}} changes
 */
export async function updateArchiveSettings({
  enabled, dirTemplate, fileTemplate, quotaBytes, minFreeBytes,
  warnPercent, recheckDays, freezeAfterDays, zipMaxBytes,
} = {}) {
  const body = {};
  if (enabled !== undefined) body.enabled = enabled;
  if (dirTemplate !== undefined) body.dir_template = dirTemplate;
  if (fileTemplate !== undefined) body.file_template = fileTemplate;
  if (quotaBytes !== undefined) body.quota_bytes = quotaBytes;
  if (minFreeBytes !== undefined) body.min_free_bytes = minFreeBytes;
  if (warnPercent !== undefined) body.warn_percent = warnPercent;
  if (recheckDays !== undefined) body.recheck_days = recheckDays;
  if (freezeAfterDays !== undefined) body.freeze_after_days = freezeAfterDays;
  if (zipMaxBytes !== undefined) body.zip_max_bytes = zipMaxBytes;

  const res = await apiRequest('/file-archive/settings', {
    method: 'PUT',
    body: JSON.stringify(body),
  });
  return unwrap(res, 'Не удалось сохранить настройки файлового архива');
}

/**
 * Реестр плейсхолдеров для палитры конструктора шаблона пути (ключ, подпись,
 * группа, пример, где допустим). Источник правды - internal/blankpath/tokens.go,
 * фронт не держит свою копию списка.
 */
export async function getArchiveTokens() {
  const res = await apiRequest('/file-archive/tokens');
  return unwrap(res, 'Не удалось загрузить список плейсхолдеров');
}

/**
 * Живое превью раскладки: как шаблоны разложатся в путь. Пустой шаблон в
 * запросе бэк подставляет из сохранённых настроек - конструктор правит одно
 * поле, а видеть должен путь целиком.
 * @param {{dirTemplate?: string, fileTemplate?: string, applicationId?: number}} params
 *   applicationId: 0 или не задано - последняя поданная заявка либо образец.
 */
export async function previewArchivePath({ dirTemplate = '', fileTemplate = '', applicationId = 0 } = {}) {
  const res = await apiRequest('/file-archive/preview', {
    method: 'POST',
    body: JSON.stringify({
      dir_template: dirTemplate,
      file_template: fileTemplate,
      application_id: applicationId,
    }),
  });
  return unwrap(res, 'Не удалось построить превью пути');
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
