import { useDeletionsStore } from '@/stores/deletions';

/**
 * Выбор строк и групповые операции справочников (#871).
 *
 * Организации и компании написаны копипастой: после нормализации имён сущности
 * их файлы совпадают почти дословно, и вся эта логика существовала в двух
 * экземплярах. Различались только имена - `sortedOrganizations` против
 * `sortedCompanies`, - поэтому здесь параметризовано ровно одно: как добраться
 * до отсортированного списка.
 *
 * Отдаётся наборами для Options API, а не composable: шаблоны зовут методы по
 * коротким именам (`toggleSelectAll`, `onRowCheck`), и перевод на `bulk.*`
 * переписал бы разметку обоих файлов ради того же поведения.
 *
 * Ожидаемое состояние компонента: `selectedIds`, `lastSelectedId`,
 * `pendingBulkOp`, `bulkModalVisible`, `bulkConfirmVisible`, `bulkSubmitting`.
 * От компонента же нужны `refreshData()` и вычисляемый список.
 *
 * @param {{ sortedKey: string }} options ключ вычисляемого отсортированного списка
 */
export function directoryBulkComputed({ sortedKey }) {
  return {
    allSelected() {
      return this[sortedKey].length > 0 && this.selectedIds.length === this[sortedKey].length;
    },

    someSelected() {
      return this.selectedIds.length > 0 && !this.allSelected;
    },
  };
}

/** Подписи в уведомлении о результате: ключ - код операции. */
const RESULT_LABELS = {
  type: 'Тип изменён',
  'unload-places': 'Места разгрузки назначены',
  tables: 'Целевые таблицы назначены',
  users: 'Ответственные назначены',
  archive: 'Архивировано',
  restore: 'Восстановлено',
};

/**
 * @param {{ sortedKey: string }} options ключ вычисляемого отсортированного списка
 */
export function directoryBulkMethods({ sortedKey }) {
  return {
    isSelected(id) {
      return this.selectedIds.includes(id);
    },

    toggleSelect(id) {
      const i = this.selectedIds.indexOf(id);
      if (i === -1) this.selectedIds.push(id);
      else this.selectedIds.splice(i, 1);
    },

    onRowCheck(item, index, event) {
      // shift-клик не должен выделять текст (селект начинается на mousedown, .prevent его не гасит) -
      // гасим для любого shift-клика, включая fallback без валидного якоря.
      if (event.shiftKey && window.getSelection) window.getSelection().removeAllRanges();
      if (event.shiftKey && this.lastSelectedId != null && this.lastSelectedId !== item.id) {
        const anchor = this[sortedKey].findIndex(row => row.id === this.lastSelectedId);
        if (anchor !== -1) {
          const target = !this.isSelected(item.id);
          const [from, to] = anchor < index ? [anchor, index] : [index, anchor];
          for (let i = from; i <= to; i += 1) {
            const id = this[sortedKey][i].id;
            const sel = this.isSelected(id);
            if (target && !sel) this.selectedIds.push(id);
            else if (!target && sel) this.selectedIds.splice(this.selectedIds.indexOf(id), 1);
          }
          this.lastSelectedId = item.id;
          return;
        }
      }
      this.toggleSelect(item.id);
      this.lastSelectedId = item.id;
    },

    toggleSelectAll() {
      this.selectedIds = this.allSelected ? [] : this[sortedKey].map(row => row.id);
      this.lastSelectedId = null; // "выбрать всё" не задаёт якорь для shift-диапазона
    },

    clearSelection() {
      this.selectedIds = [];
      this.lastSelectedId = null;
      this.pendingBulkOp = null;
    },

    startBulkOperation(operation) {
      this.pendingBulkOp = operation;
      if (operation === 'archive' || operation === 'restore') {
        this.bulkConfirmVisible = true;
      } else {
        this.bulkModalVisible = true;
      }
    },

    closeBulkModal() {
      if (this.bulkSubmitting) return;
      this.bulkModalVisible = false;
      this.pendingBulkOp = null;
    },

    cancelBulkConfirm() {
      if (this.bulkSubmitting) return;
      this.bulkConfirmVisible = false;
      this.pendingBulkOp = null;
    },

    handleBulkResult(op, result, total) {
      if (!result || typeof result.success_count !== 'number') {
        useDeletionsStore().notify({ prefix: result?.message || 'Не удалось выполнить групповую операцию', type: 'error' });
        return false;
      }
      const label = RESULT_LABELS[op] || 'Готово';

      if (result.error_count > 0) {
        const failed = (result.errors || []).map(e => e.name || `#${e.id}`).join(', ');
        useDeletionsStore().notify({ prefix: 'Выполнено ', bold: `${result.success_count} из ${total}`, suffix: `. Не удалось: ${failed}`, type: 'warning' });
      } else {
        useDeletionsStore().notify({ prefix: `${label}: `, bold: String(result.success_count) });
      }
      this.clearSelection();
      this.refreshData();
      return true;
    },
  };
}
