import { apiRequest } from './client';
import { parseContentDispositionFilename } from '@/utils/download';

/**
 * API клиент Excel-бланков и кастомных полей вложений (#183).
 */

// --- Templates ---

export async function getTemplate(uniqueAttachmentID) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/template`);
  return res.json();
}

export async function listTemplates(uniqueAttachmentID) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/templates`);
  return res.json();
}

export async function setActiveTemplate(uniqueAttachmentID, templateID) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/template/${templateID}/activate`, {
    method: 'PUT',
  });
  return res.json();
}

export async function deactivateAllTemplates(uniqueAttachmentID) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/template/deactivate`, {
    method: 'PUT',
  });
  return res.json();
}

export async function deleteTemplateByID(uniqueAttachmentID, templateID) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/template/${templateID}`, {
    method: 'DELETE',
  });
  return res.json();
}

export async function getTemplateFileByID(uniqueAttachmentID, templateID) {
  const { apiRequestRaw } = await import('./client');
  const res = await apiRequestRaw(`/attachments/${uniqueAttachmentID}/template/${templateID}/file`);
  if (!res.ok) throw new Error(`Failed to get template file: ${res.status}`);
  return res.arrayBuffer();
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

export async function updateMappings(uniqueAttachmentID, mappings, concatSeparator) {
  const body = { mappings };
  if (concatSeparator !== undefined) body.concat_separator = concatSeparator;
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/template/mappings`, {
    method: 'PUT',
    body: JSON.stringify(body),
  });
  return res.json();
}

/**
 * Изменить границы строк списка у активного шаблона без перезагрузки файла.
 * @param {number} uniqueAttachmentID
 * @param {{ listStartRow: number, listEndRow: number, maxListRows?: number }} params
 */
export async function updateTemplateParams(uniqueAttachmentID, {
  listStartRow, listEndRow, maxListRows = 0, itemsMaxListRows = 0,
}) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/template/params`, {
    method: 'PUT',
    body: JSON.stringify({
      list_start_row: listStartRow,
      list_end_row: listEndRow,
      max_list_rows: maxListRows,
      items_max_list_rows: itemsMaxListRows,
    }),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data?.message || 'Не удалось сохранить параметры списка');
  }
  return res.json();
}

/**
 * Шаблоны, с которых можно перенести привязки (все настроенные бланки системы).
 * @returns {Promise<Array<{ template_id: number, unique_attachment_id: number,
 *   attachment_name: string, attachment_type: string, original_file_name: string,
 *   mappings_count: number, is_active: boolean }>>}
 */
export async function listTemplateSources() {
  const res = await apiRequest('/attachments/template-sources');
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data?.message || 'Не удалось получить список шаблонов');
  }
  return res.json();
}

/**
 * Перенести привязки с другого шаблона в активный шаблон вложения.
 * @param {number} uniqueAttachmentID
 * @param {{ sourceTemplateID: number, replace?: boolean, copyParams?: boolean }} params
 * @returns {Promise<{ copied: number, skipped_foreign_list: number, skipped_custom: number,
 *   remapped_custom: number, skipped_duplicates: number, params_copied: boolean }>}
 */
export async function copyMappings(uniqueAttachmentID, { sourceTemplateID, replace = true, copyParams = false }) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/template/copy-mappings`, {
    method: 'POST',
    body: JSON.stringify({
      source_template_id: sourceTemplateID,
      replace,
      copy_params: copyParams,
    }),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data?.message || 'Не удалось перенести привязки');
  }
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

// --- Field Config (базовые поля: видимость/обязательность) ---

/**
 * Настройка полей вложения: базовые поля реестра типа (смерженные с оверрайдами)
 * и кастомные поля. Единый источник для админ-модалки и формы подачи (#529).
 * @returns {Promise<{ base: Array, custom: Array }>}
 */
export async function getFieldConfig(uniqueAttachmentID) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/field-config`);
  return res.json();
}

/**
 * Сохранить оверрайды видимости/обязательности базовых полей (bulk-upsert).
 * Залоченные поля (дата/время) бэк игнорирует.
 * @param {number} uniqueAttachmentID
 * @param {Array<{ key: string, visible: boolean, required: boolean }>} base
 */
export async function saveFieldConfig(uniqueAttachmentID, base) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/field-config`, {
    method: 'PUT',
    body: JSON.stringify({ base }),
  });
  return res.json();
}

// --- Custom Fields ---

export async function listCustomFields(uniqueAttachmentID) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/custom-fields`);
  return res.json();
}

export async function createCustomField(uniqueAttachmentID, {
  label, placeholder = '', sortOrder = 0, isRequired = false,
}) {
  const res = await apiRequest(`/attachments/${uniqueAttachmentID}/custom-fields`, {
    method: 'POST',
    body: JSON.stringify({
      label, placeholder, sort_order: sortOrder, is_required: isRequired,
    }),
  });
  return res.json();
}

export async function updateCustomField(fieldID, {
  label, placeholder = '', sortOrder = 0, isRequired = false,
}) {
  const res = await apiRequest(`/attachments/custom-fields/${fieldID}`, {
    method: 'PUT',
    body: JSON.stringify({
      label, placeholder, sort_order: sortOrder, is_required: isRequired,
    }),
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
 * @param {number} applicationID
 * @param {number} attachmentID
 * @param {{source?: 'archive'|'live', withDocuments?: boolean}} [options] source:
 *   'archive' - отдать сохранённый на диске файл файлового архива вместо генерации
 *   заново (#1615, C6); нет сохранённого файла - 404, а не тихий откат на live.
 *   withDocuments - подставить документы участников (паспорт, патент, иное
 *   разрешение). Решение всё равно перепроверяет сервер по правам detail.documents
 *   и detail.documents.export: без них бланк приходит с прочерками, а сохранённый
 *   файл не отдаётся вовсе.
 */
export async function downloadBlank(applicationID, attachmentID, { source, withDocuments } = {}) {
  const { apiRequestRaw } = await import('./client');
  let url = `/applications/${applicationID}/blank?attachment_id=${attachmentID}`;
  if (source === 'archive') url += '&source=archive';
  if (withDocuments) url += '&documents=1';
  const res = await apiRequestRaw(url);
  if (!res.ok) {
    throw new Error(`Failed to download blank: ${res.status}`);
  }
  const blob = await res.blob();
  const cd = res.headers.get('Content-Disposition') || '';
  const filename = parseContentDispositionFilename(cd, `blank_${applicationID}_${attachmentID}.xlsx`);
  return { blob, filename };
}

/**
 * Скачать все сохранённые бланки заявки единым ZIP с сервера (#1615, C6):
 * архив собирается на бэке из файлов реестра файлового архива (status=ok),
 * в отличие от клиентского JSZip в DownloadBlanksModal, который стягивает
 * бланки по одному через downloadBlank.
 * @param {number} applicationID
 * @returns {Promise<{blob: Blob, filename: string}>}
 */
export async function downloadApplicationArchive(applicationID) {
  const { apiRequestRaw } = await import('./client');
  const res = await apiRequestRaw(`/applications/${applicationID}/archive`);
  if (!res.ok) {
    throw new Error(`Failed to download application archive: ${res.status}`);
  }
  const blob = await res.blob();
  const cd = res.headers.get('Content-Disposition') || '';
  const filename = parseContentDispositionFilename(cd, `application_${applicationID}.zip`);
  return { blob, filename };
}

/**
 * Загрузить заполненный бланк вложения как ArrayBuffer для предпросмотра в XlsxViewer (#706 S4).
 * Тот же эндпоинт, что downloadBlank, но без сохранения в файл - буфер парсит exceljs во вьювере.
 * @param {number} applicationID
 * @param {number} attachmentID
 * @param {{withDocuments?: boolean}} [options] withDocuments - показать документы
 *   участников; проверяется сервером по правам, как и при скачивании.
 * @returns {Promise<ArrayBuffer>}
 */
export async function previewBlank(applicationID, attachmentID, { withDocuments } = {}) {
  const { apiRequestRaw } = await import('./client');
  const docs = withDocuments ? '&documents=1' : '';
  const res = await apiRequestRaw(`/applications/${applicationID}/blank?attachment_id=${attachmentID}${docs}`);
  if (!res.ok) {
    throw new Error(`Failed to preview blank: ${res.status}`);
  }
  return res.arrayBuffer();
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
