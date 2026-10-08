import { apiRequest } from './client';

function base(kind, tableID, source = 'table') {
  if (!['employee', 'car'].includes(kind) || !['table', 'admin_summary'].includes(source) ||
      (source === 'table' ? !Number.isSafeInteger(tableID) || tableID <= 0 : tableID !== null)) {
    throw new Error('Некорректная таблица проходов');
  }
  return kind === 'employee' ? 'employees' : 'cars';
}
async function read(response, fallback) {
  const data = await response.json();
  if (!response.ok) {
    const error = new Error(response.status >= 500 ? fallback : data?.message || fallback);
    error.status = response.status;
    throw error;
  }
  return data;
}
const validInstant = value => typeof value === 'string' && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(value) && Number.isFinite(Date.parse(value));
function normalizeRow(row, kind) {
  const state = row?.passage_state;
  if (!Number.isSafeInteger(row?.entity_id) || row.entity_id <= 0 ||
      row.entity_kind !== (kind === 'employee' ? 'employees' : 'cars') || !validInstant(row.server_now) ||
      !state || !Number.isSafeInteger(state.last_event_id) || state.last_event_id < 0 ||
      ['has_event', 'open', 'in_exit_grace', 'needs_attention', 'entry_time_known', 'can_correct', 'can_revert_correction']
        .some(key => typeof state[key] !== 'boolean') ||
      (state.entry_time_known && !validInstant(state.entry_at)) ||
      ['entry_at', 'exit_recorded_at', 'grace_until'].some(key => state[key] != null && !validInstant(state[key])) ||
      (state.can_revert_correction && !validInstant(state.exit_recorded_at)) ||
      typeof row.admission?.can_enter !== 'boolean' || typeof row.admission?.can_exit !== 'boolean' ||
      typeof row.effective_period?.bounded !== 'boolean') throw new Error('Не удалось прочитать состояние прохода');
  return { ...row, entity_kind: kind };
}
export async function listOpenPassages(kind, tableID, { attentionOnly = true, view = 'open', search = '', organizationID = null, page = 1, perPage = 25, source = 'table' } = {}) {
  const plural = base(kind, tableID, source);
  if (!['open', 'corrections'].includes(view) || typeof attentionOnly !== 'boolean' ||
      !Number.isSafeInteger(page) || page < 1 || !Number.isSafeInteger(perPage) || perPage < 1 || perPage > 100 ||
      (organizationID !== null && (!Number.isSafeInteger(organizationID) || organizationID <= 0))) throw new Error('Некорректный фильтр проходов');
  const query = new URLSearchParams({ view, page: String(page), per_page: String(perPage) });
  if (view === 'open') query.set('attention_only', String(attentionOnly));
  if (search.trim()) query.set('search', search.trim());
  if (organizationID !== null) query.set('organization_id', String(organizationID));
  const data = await read(await apiRequest(`/${plural}/${source === 'admin_summary' ? 'open-admin-summary' : `open-for-table/${tableID}`}?${query}`), 'Не удалось загрузить незакрытые отметки');
  if (!Array.isArray(data?.items) || !validInstant(data.server_now) ||
      ['all_open', 'attention', 'unknown_time'].some(key => !Number.isSafeInteger(data.counts?.[key]) || data.counts[key] < 0) ||
      !Number.isSafeInteger(data.total) || data.total < 0 || data.page !== page || data.per_page !== perPage) {
    throw new Error('Не удалось прочитать список незакрытых отметок');
  }
  return { ...data, items: data.items.map(row => normalizeRow(row, kind)) };
}
async function command(kind, entityID, tableID, request, revert) {
  const source = request.source ?? 'table';
  const plural = base(kind, tableID, source);
  if (!Number.isSafeInteger(entityID) || entityID <= 0 || !Number.isSafeInteger(request.expected_last_event_id) || request.expected_last_event_id < 0 ||
      !request.reason?.trim() || request.reason.trim().length > 1000 ||
      (!revert && request.actual_exit_at != null && !validInstant(request.actual_exit_at))) throw new Error('Проверь причину и данные исправления');
  const body = { source, ...(source === 'table' ? { table_id: tableID } : {}), expected_last_event_id: request.expected_last_event_id, reason: request.reason.trim(),
    ...(!revert ? { actual_exit_at: request.actual_exit_at ?? null } : {}) };
  const data = await read(await apiRequest(`/${plural}/${entityID}/passage-close${revert ? '/revert' : ''}`, {
    method: 'POST', body: JSON.stringify(body),
  }), 'Не удалось изменить учёт прохода');
  const row = normalizeRow(data, kind);
  if (row.entity_id !== entityID) throw new Error('Ответ не соответствует открытой записи');
  return row;
}
export const closeOpenPassage = (kind, entityID, tableID, request) => command(kind, entityID, tableID, request, false);
export const revertPassageCorrection = (kind, entityID, tableID, request) => command(kind, entityID, tableID, request, true);
