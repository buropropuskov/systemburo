import { apiRequest } from '@/api/client';

export async function loadApplicationChanges(applicationId) {
  const response = await apiRequest(`/notifications?application_id=${applicationId}`);
  if (!response.ok) throw new Error('Не удалось загрузить изменения заявки');
  const rows = await response.json();
  if (!Array.isArray(rows)) throw new Error('Не удалось загрузить изменения заявки');
  return rows.flatMap(notification => {
    let data;
    try { data = JSON.parse(notification.data || '{}'); } catch { return []; }
    if (data.application_id !== applicationId || !Array.isArray(data.changes)) return [];
    return data.changes.filter(change => !change.acknowledged).map(change => ({ ...change, notificationId: notification.id, key: `${notification.id}:${change.index}` }));
  });
}

export async function acknowledgeApplicationChanges(changes, selected) {
  // Acknowledge the displayed event and older events for this field only.
  // An event arriving concurrently with the click must remain highlighted.
  const targets = changes.filter(change => change.entity_id === selected.entity_id && change.entity_kind === selected.entity_kind && change.field === selected.field && change.notificationId <= selected.notificationId);
  for (const change of targets) {
    const response = await apiRequest(`/notifications/${change.notificationId}/read`, { method: 'PUT', body: JSON.stringify({ acknowledged_change: change.index }) });
    if (!response.ok) throw new Error('Не удалось подтвердить изменение');
  }
}
