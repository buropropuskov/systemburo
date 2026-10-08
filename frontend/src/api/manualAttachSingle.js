import { apiRequest } from './client';

function endpoint(kind, entityID, tableID) {
  if (!['employee', 'car'].includes(kind) || !Number.isSafeInteger(entityID) || entityID <= 0 ||
      !Number.isSafeInteger(tableID) || tableID <= 0) throw new Error('Некорректная запись или таблица');
  return `/${kind === 'employee' ? 'employees' : 'cars'}/${entityID}`;
}

async function read(response, fallback) {
  const data = await response.json(); // client.js раскрывает envelope.data.
  if (!response.ok) {
    const error = new Error(response.status >= 500 ? fallback : (data?.message || fallback));
    error.status = response.status;
    throw error;
  }
  return data;
}

function normalizeKind(data, kind, entityID) {
  if (data?.entity_id !== entityID || data?.entity_kind !== (kind === 'employee' ? 'employees' : 'cars')) {
    const error = new Error('Ответ привязки не соответствует открытой записи');
    error.status = 409;
    throw error;
  }
  return { ...data, entity_kind: kind };
}

function validCarFlags(flags) {
  return !!flags && ['roof_access', 'free_parking', 'individual_roof_access', 'individual_free_parking']
    .every(key => typeof flags[key] === 'boolean') &&
    (!flags.individual_roof_access || flags.roof_access) && (!flags.individual_free_parking || flags.free_parking);
}

export async function getManualAttachContext(kind, entityID, tableID) {
  const base = endpoint(kind, entityID, tableID);
  const data = await read(await apiRequest(`${base}/manual-attach-context?table_id=${tableID}`), 'Не удалось проверить возможность привязки');
  if (data?.entity_id !== entityID || data?.entity_kind !== (kind === 'employee' ? 'employees' : 'cars') ||
      data?.is_manual !== true || data?.application_id != null || !Number.isSafeInteger(data?.attachment_id) || data.attachment_id <= 0 ||
      !['inherit', 'individual'].includes(data?.period_mode) || typeof data?.effective_period?.bounded !== 'boolean' ||
      typeof data?.requires_period_choice !== 'boolean' || typeof data?.can_assign_period !== 'boolean' ||
      data.requires_period_choice === data.effective_period.bounded || (kind === 'car' && !validCarFlags(data?.flags))) {
    throw new Error('Не удалось прочитать актуальное ручное основание');
  }
  return normalizeKind(data, kind, entityID);
}

export async function getManualAttachAttachments(kind, entityID, tableID, applicationID) {
  const base = endpoint(kind, entityID, tableID);
  if (!Number.isSafeInteger(applicationID) || applicationID <= 0) throw new Error('Некорректная заявка');
  const data = await read(await apiRequest(`${base}/manual-attach-attachments?application_id=${applicationID}&table_id=${tableID}`), 'Не удалось загрузить вложения для привязки');
  if (!Array.isArray(data) || data.some(a => !Number.isSafeInteger(a?.id) || a.id <= 0 ||
      a.application_id !== applicationID || a.status !== 1 || a.is_manual !== false ||
      !['people', 'cars', 'items'].includes(a.attachment_type))) {
    throw new Error('Не удалось прочитать вложения для привязки');
  }
  return data;
}

async function command(kind, entityID, tableID, request, preview) {
  const base = endpoint(kind, entityID, tableID);
  const data = await read(await apiRequest(`${base}/attach-to-application${preview ? '/preview' : ''}`, {
    method: 'POST', body: JSON.stringify({ ...request, table_id: tableID }),
  }), 'Не удалось привязать запись к заявке');
  if (kind === 'car' && [data?.current_flags, data?.new_flags].some(flags => !validCarFlags(flags))) {
    const error = new Error('Не удалось прочитать признаки автомобиля');
    error.status = 409;
    throw error;
  }
  return normalizeKind(data, kind, entityID);
}

export const previewManualAttachSingle = (kind, entityID, tableID, request) => command(kind, entityID, tableID, request, true);
export const executeManualAttachSingle = (kind, entityID, tableID, request) => command(kind, entityID, tableID, request, false);
