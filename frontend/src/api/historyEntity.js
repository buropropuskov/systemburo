import { apiRequest } from './client';

const validID = id => Number.isSafeInteger(id) && id > 0;
const unavailable = () => new Error('Запись удалена или недоступна для просмотра');

async function readRows(path) {
  const response = await apiRequest(path, { method: 'GET' });
  if (!response.ok) throw unavailable();
  const rows = await response.json();
  if (!Array.isArray(rows)) throw new Error('Не удалось прочитать карточку');
  return rows;
}

// History permission alone does not grant access to the current entity.
// Reuse current table/application readers, which enforce their own visibility.
export async function loadHistoryEntity(kind, history, tableID = null) {
  if (!['car', 'employee'].includes(kind)) throw unavailable();
  const id = history?.[kind === 'car' ? 'car_id' : 'employee_id'];
  if (!validID(id) || history.entity_deleted) throw unavailable();
  const plural = kind === 'car' ? 'cars' : 'employees';
  const table = validID(tableID) ? tableID : history.table_id;
  let row = null;
  let attachment = null;
  if (validID(table)) {
    const rows = await readRows(`/${plural}/active-for-table/${table}`);
    row = rows.find(item => item.id === id);
  }
  if (!row && validID(history.application_id)) {
    const attachments = await readRows(`/applications/${history.application_id}/attachments`);
    for (const source of attachments.filter(item => item.attachment_type === (kind === 'car' ? 'cars' : 'people'))) {
      if (!validID(source.id)) continue;
      const rows = await readRows(`/attachments/${source.id}/${plural}`);
      row = rows.find(item => item.id === id);
      if (row) { attachment = source; break; }
    }
  }
  if (!row) throw unavailable();
  const period = Object.fromEntries(['entry_date_from', 'entry_date_to', 'entry_time_from', 'entry_time_to']
    .map(key => [key, row[key] === undefined ? attachment?.[key] : row[key]]));
  const common = { ...row, ...period, applicationId: row.applicationId ?? history.application_id,
    organization: row.organization ?? row.organization_name, company: row.company ?? row.company_name };
  return kind === 'car'
    ? { ...common, plateNumber: row.car_number, mark: row.car_brand,
      unloadPlaces: (row.unload_place_ids ?? row.unload_places ?? []).map(place => typeof place === 'object' ? place.id : place) }
    : { ...common, citizenshipName: row.citizenshipName ?? row.citizenship_name,
      pass_time: [period.entry_time_from?.slice(0, 5), period.entry_time_to?.slice(0, 5)].filter(Boolean).join(' - ') };
}
