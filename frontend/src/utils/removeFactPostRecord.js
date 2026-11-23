import { apiRequest } from '@/api/client';
import { useUiStore } from '@/stores/ui';
import { useDeletionsStore } from '@/stores/deletions';

export async function removeFactPostRecord({ kind, id, tableId, onRefresh }) {
  if (!tableId) return;
  const accepted = await useUiStore().confirm({
    title: 'Убрать с поста?',
    message: 'Запись будет снята только с текущего поста. Остальные посты, заявка и история сохранятся.',
    confirmText: 'Убрать', danger: true,
  });
  if (!accepted) return;
  try {
    const response = await apiRequest(`/${kind}/bulk/unbind-table`, {
      method: 'POST', body: JSON.stringify({ ids: [id], table_id: tableId }),
    });
    const result = await response.json();
    if (!response.ok || result.success_count !== 1 || result.error_count) {
      useDeletionsStore().notify({ prefix: 'Не удалось убрать запись: ', bold: result.errors?.[0]?.error || 'Обновите список и повторите действие', type: 'error' });
      return;
    }
    await onRefresh();
  } catch {
    useDeletionsStore().notify({ prefix: 'Не удалось убрать запись: ', bold: 'ошибка сети', type: 'error' });
  }
}
