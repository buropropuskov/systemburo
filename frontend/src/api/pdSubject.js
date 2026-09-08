/** Сведения о субъекте персональных данных (#2356). */
import { apiRequest, apiRequestRaw } from '@/api/client';

/** Записи с таким же именем. Склейки по имени нет - решает оператор. */
export async function findSubjectCandidates(fio) {
  const res = await apiRequest(`/pd-subject/candidates?fio=${encodeURIComponent(fio)}`);
  return res || [];
}

/** Состав сведений о человеке по записи реестра. */
export async function fetchSubjectReport(registryId) {
  return apiRequest(`/pd-subject/report?registry_id=${registryId}`);
}

/** Журнал выдач: весь или по одному человеку. */
export async function fetchSubjectDisclosures(registryId) {
  const q = registryId ? `?registry_id=${registryId}` : '';
  const res = await apiRequest(`/pd-subject/disclosures${q}`);
  return res || [];
}

/**
 * Выгрузка справки файлом. Ответ - двоичный файл, а не JSON, поэтому сырой запрос:
 * имя файла берём из заголовка, его формирует сервер вместе с записью в журнал выдач.
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
