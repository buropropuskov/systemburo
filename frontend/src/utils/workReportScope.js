import { formatMoscowDateTime } from './serverTime';

export const workScopes = [
  { key: 'now', label: 'Активно' },
  { key: 'today', label: 'Сегодня' },
  { key: 'submitted', label: 'За период подачи' },
];

export function workSnapshotLabel(result) {
  const mode = workScopes.find(item => item.key === result?.work_scope);
  if (!mode || !result.as_of) return '';
  return mode.label + ' · ' + formatMoscowDateTime(new Date(result.as_of)).slice(0, 16) + ' МСК';
}
