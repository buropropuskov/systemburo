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

/** Записи с таким же именем. Склейки по имени нет - решает оператор. */
export async function findSubjectCandidates(fio) {
  const response = await apiRequest(`/pd-subject/candidates?fio=${encodeURIComponent(fio)}`);
  return (await readJson(response)) || [];
}

/** Состав сведений о человеке по записи реестра. */
export async function fetchSubjectReport(registryId) {
  const response = await apiRequest(`/pd-subject/report?registry_id=${registryId}`);
  return readJson(response);
}

/** Журнал выдач: весь или по одному человеку. */
export async function fetchSubjectDisclosures(registryId) {
  const q = registryId ? `?registry_id=${registryId}` : '';
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
  const disposition = response.headers.get('Content-Disposition') || '';
  const match = disposition.match(/filename="([^"]+)"/);
  return {
    blob: await response.blob(),
    filename: match ? match[1] : `Сведения_о_субъекте.${payload.format || 'xlsx'}`,
  };
}
