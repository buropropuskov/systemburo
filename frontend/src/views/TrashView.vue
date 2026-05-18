<template>
  <div class="trash-view">
    <header class="trash-header">
      <div class="trash-title">
        <RouterLink :to="`/table/${tableName}`" class="back-btn">← Назад</RouterLink>
        <h2>
          <RouterLink :to="`/table/${tableName}`" class="trash-table-name">{{ displayName }}</RouterLink>
          <span class="trash-slash"> / Корзина</span>
        </h2>
      </div>
      <div class="trash-controls">
        <input
          v-model="filters.search"
          class="lk-input"
          placeholder="Поиск..."
          @input="onFilterChange"
        >
        <input
          v-model="filters.dateFrom"
          type="date"
          class="lk-input"
          @change="reload"
        >
        <input
          v-model="filters.dateTo"
          type="date"
          class="lk-input"
          @change="reload"
        >
        <button class="lk-btn lk-btn--ghost" :disabled="isLoading" @click="reload">
          Обновить
        </button>
        <button
          class="lk-btn lk-btn--danger"
          :disabled="!items.length"
          @click="onClearAll"
        >
          Очистить корзину
        </button>
      </div>
    </header>

    <div class="trash-bulk-actions" v-if="selectedIds.length">
      <span>{{ selectedIds.length }} выбрано</span>
      <button class="lk-btn" @click="onRestoreSelected">Восстановить</button>
      <button class="lk-btn lk-btn--danger" @click="onPurgeSelected">Удалить безвозвратно</button>
    </div>

    <div v-if="isLoading" class="trash-loader">Загрузка...</div>
    <div v-else-if="error" class="trash-error">{{ error }}</div>
    <div v-else-if="!items.length" class="trash-empty">Корзина пуста</div>
    <table v-else class="trash-table">
      <thead>
        <tr>
          <th class="th-checkbox">
            <input
              type="checkbox"
              :checked="allSelected"
              @change="toggleAll"
            >
          </th>
          <th>Номер заявки</th>
          <th>Дата удаления</th>
          <template v-if="tableType === 'cars'">
            <th>Номер ТС</th>
            <th>Марка</th>
            <th>Организация</th>
            <th>Действует до</th>
          </template>
          <template v-else>
            <th>Фамилия</th>
            <th>Имя</th>
            <th>Отчество</th>
            <th>Организация</th>
          </template>
          <th>Статус</th>
          <th>Кто удалил</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in items" :key="item.id" data-testid="trash-row">
          <td>
            <input
              type="checkbox"
              :value="item.id"
              v-model="selectedIds"
            >
          </td>
          <td>{{ item.application_number || '—' }}</td>
          <td>{{ formatDate(item.deleted_at) }}</td>
          <template v-if="tableType === 'cars'">
            <td>{{ item.car_number || '—' }}</td>
            <td>{{ item.mark_name || '—' }}</td>
            <td>{{ item.organization || '—' }}</td>
            <td>{{ item.entry_date_to || '—' }}</td>
          </template>
          <template v-else>
            <td>{{ item.last_name || '—' }}</td>
            <td>{{ item.first_name || '—' }}</td>
            <td>{{ item.middle_name || '—' }}</td>
            <td>{{ item.organization || '—' }}</td>
          </template>
          <td class="status-trash">{{ tableType === 'cars' ? 'Удалена' : 'Удален' }}</td>
          <td>{{ item.deleted_by_name || '—' }}</td>
          <td>
            <button class="icon-btn danger" title="Удалить безвозвратно" @click="onPurgeOne(item.id)">
              ✕
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client';
import { useUiStore } from '@/stores/ui';
import { listTrash, restoreItems, purgeItem, clearTrash } from '@/api/trash';

export default {
  name: 'TrashView',
  data() {
    return {
      tableID: 0,
      tableType: '',
      displayName: '',
      items: [],
      selectedIds: [],
      isLoading: false,
      error: '',
      filters: { search: '', dateFrom: '', dateTo: '' },
      searchTimer: null,
    };
  },
  computed: {
    tableName() {
      return this.$route.params.tableName;
    },
    allSelected() {
      return this.items.length > 0 && this.selectedIds.length === this.items.length;
    },
  },
  async mounted() {
    await this.fetchTable();
    if (this.tableID) await this.reload();
  },
  watch: {
    '$route.params.tableName'() {
      this.fetchTable().then(() => this.reload());
    },
  },
  methods: {
    async fetchTable() {
      this.error = '';
      try {
        const res = await apiRequest(`/system-tables/name/${this.tableName}`);
        const data = await res.json();
        // API возвращает { table: {...}, ... } - сам объект таблицы вложен.
        // Падаем обратно на data если структура изменилась.
        const tbl = (data && data.table) || data;
        if (!tbl || !tbl.id) {
          this.error = 'Таблица не найдена';
          return;
        }
        this.tableID = tbl.id;
        this.tableType = tbl.table_type;
        this.displayName = tbl.display_name || tbl.name || this.tableName;
        if (this.tableType !== 'cars' && this.tableType !== 'people') {
          this.error = 'Этот тип таблицы не поддерживает корзину';
        }
      } catch {
        this.error = 'Ошибка загрузки таблицы';
      }
    },
    async reload() {
      if (!this.tableID || this.error) return;
      this.isLoading = true;
      this.selectedIds = [];
      try {
        const data = await listTrash(this.tableID, {
          search: this.filters.search,
          dateFrom: this.filters.dateFrom,
          dateTo: this.filters.dateTo,
        });
        this.items = Array.isArray(data) ? data : [];
      } catch {
        useUiStore().error('Не удалось загрузить корзину');
      } finally {
        this.isLoading = false;
      }
    },
    onFilterChange() {
      // Debounce поиска: ждём 300мс после последнего ввода.
      clearTimeout(this.searchTimer);
      this.searchTimer = setTimeout(() => this.reload(), 300);
    },
    toggleAll(e) {
      this.selectedIds = e.target.checked ? this.items.map(i => i.id) : [];
    },
    formatDate(s) {
      if (!s) return '—';
      try {
        return new Date(s).toLocaleString('ru-RU');
      } catch {
        return s;
      }
    },
    async onRestoreSelected() {
      if (!this.selectedIds.length) return;
      try {
        const result = await restoreItems(this.tableID, this.selectedIds);
        const r = (result && result.restored) || 0;
        const req = (result && result.requested) || this.selectedIds.length;
        if (r < req) {
          useUiStore().warning(`Восстановлено ${r} из ${req}. У остальных нет активной согласованной заявки.`);
        } else {
          useUiStore().success(`Восстановлено: ${r}`);
        }
        await this.reload();
      } catch {
        useUiStore().error('Не удалось восстановить');
      }
    },
    async onPurgeSelected() {
      if (!this.selectedIds.length) return;
      if (!confirm(`Удалить безвозвратно ${this.selectedIds.length} элемент(ов)?`)) return;
      let purged = 0;
      for (const id of this.selectedIds) {
        try {
          await purgeItem(this.tableID, id);
          purged++;
        } catch {
          // Одиночный fail не прерывает массовое.
        }
      }
      useUiStore().success(`Удалено: ${purged}`);
      await this.reload();
    },
    async onPurgeOne(id) {
      if (!confirm('Удалить эту запись безвозвратно?')) return;
      try {
        await purgeItem(this.tableID, id);
        useUiStore().success('Запись удалена безвозвратно');
        await this.reload();
      } catch {
        useUiStore().error('Не удалось удалить');
      }
    },
    async onClearAll() {
      if (!confirm('Очистить корзину целиком? Это действие необратимо.')) return;
      try {
        const result = await clearTrash(this.tableID);
        useUiStore().success(`Корзина очищена: ${(result && result.purged) || 0} запис(ей)`);
        await this.reload();
      } catch {
        useUiStore().error('Не удалось очистить');
      }
    },
  },
};
</script>

<style scoped>
.trash-view {
  padding: 20px;
  max-width: 1400px;
  margin: 0 auto;
}

.trash-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 16px;
  margin-bottom: 20px;
}

.trash-title {
  display: flex;
  align-items: center;
  gap: 16px;
}

.back-btn {
  color: #666;
  text-decoration: none;
  font-size: 14px;
}

.back-btn:hover {
  color: var(--color-primary);
}

.trash-title h2 {
  margin: 0;
  font-size: 22px;
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.trash-table-name {
  color: #a2a2a2;
  text-decoration: none;
}

.trash-table-name:hover {
  color: var(--color-primary);
}

.trash-slash {
  color: #000;
}

.trash-controls {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.lk-input {
  padding: 6px 10px;
  border: 1px solid var(--color-border);
  border-radius: 6px;
  font-size: 13px;
}

.lk-btn {
  padding: 6px 14px;
  border-radius: 6px;
  border: 0;
  background: var(--color-primary);
  color: #fff;
  cursor: pointer;
  font-size: 13px;
}

.lk-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.lk-btn--ghost {
  background: transparent;
  border: 1px solid var(--color-border);
  color: #333;
}

.lk-btn--danger {
  background: #d73a3a;
}

.trash-bulk-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  background: var(--color-bg-secondary);
  border-radius: 8px;
  margin-bottom: 12px;
}

.trash-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.trash-table th,
.trash-table td {
  padding: 8px 10px;
  text-align: left;
  border-bottom: 1px solid var(--color-border);
}

.trash-table th {
  background: var(--color-bg-secondary);
  color: #666;
  font-weight: 600;
}

.th-checkbox {
  width: 40px;
}

.status-trash {
  color: #d73a3a;
  font-weight: 600;
}

.icon-btn {
  background: none;
  border: 0;
  cursor: pointer;
  font-size: 16px;
  color: #999;
}

.icon-btn.danger:hover {
  color: #d73a3a;
}

.trash-empty,
.trash-loader,
.trash-error {
  text-align: center;
  padding: 60px 20px;
  color: #888;
}

.trash-error {
  color: #d73a3a;
}
</style>
