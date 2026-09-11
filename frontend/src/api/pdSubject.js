/** Сведения о субъекте персональных данных (#2356). */
import { apiRequest, apiRequestRaw } from '@/api/client';

/**
 * apiRequest отдаёт Response с подменённым json(): конверт {success,data} он
 * разворачивает уже внутри него. Поэтому ответ обязателен к чтению через
 * `await response.json()` - вернув сам Response, экран получил бы объект вместо
 * массива и молча показал «ничего не найдено».
 */
async function readJson(response) {
  if (!response.ok) {
    const body = await response.json().catch(() => null);
    throw new Error((body && body.message) || 'Запрос не выполнен');
  }
  return response.json();
}

/**
 * Поиск человека по имени или по номеру документа. По имени склейки нет - решает
 * оператор; по документу находится ровно один человек.
 */
export async function findSubjectCandidates({ fio, document }) {
  const q = document
    ? `document=${encodeURIComponent(document)}`
    : `fio=${encodeURIComponent(fio)}`;
  const response = await apiRequest(`/pd-subject/candidates?${q}`);
  return (await readJson(response)) || [];
}

/**
 * Состав сведений о человеке. Цель - запись реестра ИЛИ строка заявки: у человека,
 * попавшего в систему одной подачей, записи реестра нет вовсе.
 */
export async function fetchSubjectReport({ registryId, employeeId }) {
  const q = registryId ? `registry_id=${registryId}` : `employee_id=${employeeId}`;
  const response = await apiRequest(`/pd-subject/report?${q}`);
  return readJson(response);
}

/** Журнал выдач: весь или по одному человеку. */
export async function fetchSubjectDisclosures(target = {}) {
  const { registryId, employeeId } = target;
  let q = '';
  if (registryId) q = `?registry_id=${registryId}`;
  else if (employeeId) q = `?employee_id=${employeeId}`;
  const response = await apiRequest(`/pd-subject/disclosures${q}`);
  return (await readJson(response)) || [];
}

/**
 * Выгрузка справки файлом. Ответ - двоичный файл, а не конверт, поэтому сырой
 * запрос: имя файла берём из заголовка, его формирует сервер вместе с записью в
 * журнале выдач.
 */
export async function exportSubjectReport(payload) {
  const response = await apiRequestRaw('/pd-subject/export', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || 'Не удалось выгрузить справку');
  }
  // Имя файла кириллическое, поэтому сервер отдаёт его в filename*=UTF-8'' (заголовки
  // HTTP - ASCII). Читаем сначала его: простой filename="..." там запасной, ASCII-шный.
  const disposition = response.headers.get('Content-Disposition') || '';
  const utf8 = disposition.match(/filename\*=UTF-8''([^;]+)/i);
  const plain = disposition.match(/filename="([^"]+)"/);
  let filename = `Сведения_о_субъекте.${payload.format || 'xlsx'}`;
  if (utf8) filename = decodeURIComponent(utf8[1]);
  else if (plain) filename = plain[1];
  return { blob: await response.blob(), filename };
}
