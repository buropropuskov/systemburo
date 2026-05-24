import { apiRequest } from './client';

/**
 * API клиент Excel-бланков и кастомных полей вложений (#183).
 */

// --- Templates ---

export async function getTemplate(uniqueAttachmentID) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/template`);
  return res.json();
}

export async function uploadTemplate(uniqueAttachmentID, file, { listStartRow, listEndRow, maxListRows = 0 }) {
  const form = new FormData();
  form.append('file', file);
  form.append('list_start_row', String(listStartRow));
  form.append('list_end_row', String(listEndRow));
  if (maxListRows) form.append('max_list_rows', String(maxListRows));

  // FormData -> client.js не ставит Content-Type (см. isFormData в doFetch).
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/template`, {
    method: 'POST',
    body: form,
  });
  return res.json();
}

export async function updateMappings(uniqueAttachmentID, mappings) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/template/mappings`, {
    method: 'PUT',
    body: JSON.stringify({ mappings }),
  });
  return res.json();
}

export async function deleteTemplate(uniqueAttachmentID) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/template`, {
    method: 'DELETE',
  });
  return res.json();
}

export async function getTemplateFields(uniqueAttachmentID) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/template-fields`);
  return res.json();
}

// --- Custom Fields ---

export async function listCustomFields(uniqueAttachmentID) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/custom-fields`);
  return res.json();
}

export async function createCustomField(uniqueAttachmentID, { label, placeholder = '', sortOrder = 0 }) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/custom-fields`, {
    method: 'POST',
    body: JSON.stringify({ label, placeholder, sort_order: sortOrder }),
  });
  return res.json();
}

export async function updateCustomField(fieldID, { label, placeholder = '', sortOrder = 0 }) {
  const res = await apiRequest(`/attachments/custom-fields/${fieldID}`, {
    method: 'PUT',
    body: JSON.stringify({ label, placeholder, sort_order: sortOrder }),
  });
  return res.json();
}

export async function deleteCustomField(fieldID) {
  const res = await apiRequest(`/attachments/custom-fields/${fieldID}`, {
    method: 'DELETE',
  });
  return res.json();
}

// --- Template File (preview) ---

export async function getTemplateFile(uniqueAttachmentID) {
  const { apiRequestRaw } = await import('./client');
  const res = await apiRequestRaw(`/attachments/${uniqueAttachmentID}/template/file`);
  if (!res.ok) {
    throw new Error(`Failed to get template file: ${res.status}`);
  }
  return res.arrayBuffer();
}

// --- Download ---

/**
 * Скачать заполненный бланк для одного вложения заявки.
 * Возвращает Blob, который нужно сохранить через createObjectURL + link.click().
 */
export async function downloadBlank(applicationID, attachmentID) {
  const { apiRequestRaw } = await import('./client');
  const res = await apiRequestRaw(`/applications/${applicationID}/blank?attachment_id=${attachmentID}`);
  if (!res.ok) {
    throw new Error(`Failed to download blank: ${res.status}`);
  }
  const blob = await res.blob();
  // Извлекаем имя файла из Content-Disposition.
  const cd = res.headers.get('Content-Disposition') || '';
  const match = cd.match(/filename="?([^"]+)"?/);
  const filename = match ? match[1] : `blank_${applicationID}_${attachmentID}.xlsx`;
  return { blob, filename };
}

/**
 * Триггерит сохранение Blob под filename на стороне браузера.
 */
export function saveBlobAs(blob, filename) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 100);
}
