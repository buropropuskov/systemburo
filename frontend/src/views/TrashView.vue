<template>
  <section class="trash-view">
    <header class="trash-header">
      <h2 class="trash-title">
        <RouterLink
          :to="`/table/${tableName}`"
          class="trash-title__name"
        >
          {{ displayName }}
        </RouterLink>
        <span class="trash-title__slash">/</span>
        <span class="trash-title__current">Корзина</span>
        <RouterLink
          :to="`/table/${tableName}`"
          class="trash-back-btn"
          data-testid="trash-back"
        >
          ← Назад
        </RouterLink>
      </h2>

      <div class="trash-filters">
        <SearchComponent
          v-model="filters.search"
          :title="'Поиск..'"
          class="trash-filters__search"
          @input="onSearchChange"
        />
        <OrganizationFilter
          ref="organizationFilter"
          v-model="filters.organizationId"
          @change="onOrganizationChange"
        />
        <DateFilter
          :selected-date="filters.selectedDate"
          :date-range-start="filters.dateFrom"
          :date-range-end="filters.dateTo"
          @update:selected-date="filters.selectedDate = $event"
          @update:date-range-start="filters.dateFrom = $event"
          @update:date-range-end="filters.dateTo = $event"
          @apply="reload"
          @clear="onDateClear"
        />
        <button
          class="trash-action-btn trash-action-btn--ghost"
          data-testid="trash-clear-filters"
          @click="clearFilters"
        >
          Очистить
        </button>
        <button
          class="trash-action-btn trash-action-btn--ghost"
          data-testid="trash-export"
          :disabled="!items.length"
          @click="onExport"
        >
          <img
            src="@/assets/icons/export.png"
            class="trash-action-btn__icon"
            alt=""
          >
          Экспорт
        </button>
        <RefreshButton
          :loading="isLoading"
          @refresh="reload"
        />
      </div>
    </header>

    <article class="trash-card">
      <div class="trash-card__header">
        <h3 class="trash-card__title">
          {{ tableType === 'cars' ? 'Удалённые автомобили' : 'Удалённые сотрудники' }}
        </h3>
        <button
          v-if="items.length"
          class="trash-card__link"
          data-testid="trash-restore-all-link"
          @click="onRestoreAll"
        >
          Восстановить
        </button>
        <div class="trash-card__spacer" />
        <button
          class="trash-action-btn trash-action-btn--primary"
          data-testid="trash-restore-selected"
          :disabled="!selectedIds.length"
          @click="onRestoreSelected"
        >
          Восстановить
        </button>
      </div>

      <div class="trash-card__body">
        <div
          v-if="isLoading"
          class="trash-state"
        >
          Загрузка...
        </div>
        <div
          v-else-if="error"
          class="trash-state trash-state--error"
        >
          {{ error }}
        </div>
        <div
          v-else-if="!items.length"
          class="trash-state"
        >
          Корзина пуста
        </div>
        <table
          v-else
          class="trash-table"
          data-testid="trash-table"
        >
          <thead>
            <tr>
              <th class="trash-table__th-checkbox">
                <input
                  type="checkbox"
                  :checked="allSelected"
                  data-testid="trash-select-all"
                  @change="toggleAll"
                >
              </th>
              <th>Номер заявки</th>
              <th>Дата и время удаления</th>
              <template v-if="tableType === 'cars'">
                <th>Номер Т/С</th>
                <th>Марка</th>
              </template>
              <template v-else>
                <th>Фамилия</th>
                <th>Имя</th>
              </template>
              <th>Организация</th>
              <th>Действует до</th>
              <th>Время</th>
              <th>Статус</th>
              <th class="trash-table__th-actions" />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="item in items"
              :key="item.id"
              data-testid="trash-row"
            >
              <td>
                <input
                  v-model="selectedIds"
                  type="checkbox"
                  :value="item.id"
                >
              </td>
              <td class="trash-table__app-number">
                {{ item.application_number || '—' }}
              </td>
              <td>{{ formatDateTime(item.deleted_at) }}</td>
              <template v-if="tableType === 'cars'">
                <td>{{ item.car_number || '—' }}</td>
                <td>{{ item.mark_name || '—' }}</td>
              </template>
              <template v-else>
                <td>{{ item.last_name || '—' }}</td>
                <td>{{ item.first_name || '—' }}</td>
              </template>
              <td>{{ item.organization || '—' }}</td>
              <td>{{ formatDate(item.entry_date_to) }}</td>
              <td>{{ formatTimeRange(item) }}</td>
              <td class="trash-table__status">
                {{ tableType === 'cars' ? 'Удалена' : 'Удалён' }}
              </td>
              <td class="trash-table__actions">
                <button
                  class="trash-icon-btn"
                  title="Удалить безвозвратно"
                  data-testid="trash-purge-one"
                  @click="onPurgeOne(item.id)"
                >
                  <img
                    src="@/assets/icons/trashcan.png"
                    alt=""
                  >
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </article>
  </section>
</template>

<script>
import { apiRequest } from '@/api/client';
import { useUiStore } from '@/stores/ui';
import { listTrash, restoreItems, purgeItem, clearTrash } from '@/api/trash';
import SearchComponent from '@/components/SearchComponent.vue';
import OrganizationFilter from '@/components/OrganizationFilter.vue';
import DateFilter from '@/components/DateFilter.vue';
import RefreshButton from '@/components/RefreshButton.vue';

export default {
  name: 'TrashView',
  components: { SearchComponent, OrganizationFilter, DateFilter, RefreshButton },
  data() {
    return {
      tableID: 0,
      tableType: '',
      displayName: '',
      items: [],
      selectedIds: [],
      isLoading: false,
      error: '',
      filters: {
        search: '',
        organizationId: null,
        selectedDate: null,
        dateFrom: null,
        dateTo: null,
      },
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
  watch: {
    '$route.params.tableName'() {
      this.fetchTable().then(() => this.reload());
    },
  },
  async mounted() {
    await this.fetchTable();
    if (this.tableID) await this.reload();
  },
  methods: {
    async fetchTable() {
      this.error = '';
      try {
        const res = await apiRequest(`/system-tables/name/${this.tableName}`);
        const data = await res.json();
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
        const params = {
          search: this.filters.search,
          dateFrom: this.filters.selectedDate || this.filters.dateFrom || '',
          dateTo: this.filters.selectedDate || this.filters.dateTo || '',
        };
        if (this.filters.organizationId) {
          params.organizationId = this.filters.organizationId;
        }
        const data = await listTrash(this.tableID, params);
        this.items = Array.isArray(data) ? data : [];
      } catch {
        useUiStore().error('Не удалось загрузить корзину');
      } finally {
        this.isLoading = false;
      }
    },
    onSearchChange() {
      clearTimeout(this.searchTimer);
      this.searchTimer = setTimeout(() => this.reload(), 300);
    },
    onOrganizationChange() {
      this.reload();
    },
    onDateClear() {
      this.filters.selectedDate = null;
      this.filters.dateFrom = null;
      this.filters.dateTo = null;
      this.reload();
    },
    clearFilters() {
      this.filters.search = '';
      this.filters.organizationId = null;
      this.filters.selectedDate = null;
      this.filters.dateFrom = null;
      this.filters.dateTo = null;
      if (this.$refs.organizationFilter?.reset) this.$refs.organizationFilter.reset();
      this.reload();
    },
    toggleAll(e) {
      this.selectedIds = e.target.checked ? this.items.map(i => i.id) : [];
    },
    formatDateTime(s) {
      if (!s) return '—';
      try {
        const d = new Date(s);
        const date = d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
        const time = d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
        return `${date} ${time}`;
      } catch {
        return s;
      }
    },
    formatDate(s) {
      if (!s) return '—';
      try {
        return new Date(s).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
      } catch {
        return s;
      }
    },
    formatTimeRange(item) {
      const from = item.entry_time_from || item.time_from;
      const to = item.entry_time_to || item.time_to;
      if (from && to) return `${from} - ${to}`;
      if (from) return from;
      return '—';
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
    async onRestoreAll() {
      if (!this.items.length) return;
      const ids = this.items.map(i => i.id);
      try {
        const result = await restoreItems(this.tableID, ids);
        const r = (result && result.restored) || 0;
        useUiStore().success(`Восстановлено: ${r}`);
        await this.reload();
      } catch {
        useUiStore().error('Не удалось восстановить');
      }
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
    onExport() {
      useUiStore().info('Экспорт корзины - скоро');
    },
  },
};
</script>

<style scoped>
.trash-view {
  padding: 24px;
  max-width: 1600px;
  margin: 0 auto;
}

.trash-header {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-bottom: 16px;
}

.trash-title {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 24px;
  font-weight: 600;
  margin: 0;
  flex-wrap: wrap;
}

.trash-title__name {
  color: #000;
  text-decoration: none;
}

.trash-title__name:hover {
  color: #4F5BDF;
}

.trash-title__slash {
  color: #999;
  font-weight: 400;
}

.trash-title__current {
  color: #000;
}

.trash-back-btn {
  display: inline-flex;
  align-items: center;
  padding: 6px 14px;
  background: #fff;
  color: #4F5BDF;
  border: 1px solid #e6e6e6;
  border-radius: 50px;
  font-size: 0.85em;
  text-decoration: none;
  transition: all 0.2s ease;
  margin-left: 8px;
}

.trash-back-btn:hover {
  background: #f3f4ff;
  border-color: #4F5BDF;
}

.trash-filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

.trash-filters__search {
  flex: 1 1 220px;
  min-width: 200px;
}

.trash-action-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 0.9em;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.trash-action-btn--ghost {
  background: #fff;
  color: #333;
  border: 1px solid #e6e6e6;
}

.trash-action-btn--ghost:hover:not(:disabled) {
  background: #f8f9fa;
}

.trash-action-btn--primary {
  background: #4F5BDF;
  color: #fff;
  border: none;
  font-weight: 600;
}

.trash-action-btn--primary:hover:not(:disabled) {
  background: #3a45b2;
}

.trash-action-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.trash-action-btn__icon {
  width: 16px;
  height: 16px;
}

.trash-card {
  background: #fff;
  border-radius: 16px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06);
  overflow: hidden;
}

.trash-card__header {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px 20px;
  border-bottom: 1px solid #f0f0f0;
}

.trash-card__title {
  margin: 0;
  font-size: 1.05em;
  font-weight: 600;
  color: #000;
}

.trash-card__link {
  background: none;
  border: none;
  color: #4F5BDF;
  cursor: pointer;
  font-size: 0.9em;
  padding: 0;
}

.trash-card__link:hover {
  text-decoration: underline;
}

.trash-card__spacer {
  flex: 1;
}

.trash-card__body {
  padding: 0;
}

.trash-state {
  padding: 40px 20px;
  text-align: center;
  color: #999;
  font-size: 0.95em;
}

.trash-state--error {
  color: #d73a3a;
}

.trash-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.9em;
}

.trash-table th,
.trash-table td {
  padding: 12px 16px;
  text-align: left;
  border-bottom: 1px solid #f0f0f0;
}

.trash-table th {
  background: #fafafa;
  font-weight: 500;
  color: #666;
  font-size: 0.85em;
}

.trash-table tbody tr:hover {
  background: #fafafa;
}

.trash-table tbody tr:last-child td {
  border-bottom: none;
}

.trash-table__th-checkbox {
  width: 40px;
}

.trash-table__th-actions {
  width: 50px;
}

.trash-table__app-number {
  color: #999;
}

.trash-table__status {
  color: #d73a3a;
  font-weight: 500;
}

.trash-table__actions {
  text-align: right;
}

.trash-icon-btn {
  background: none;
  border: none;
  padding: 4px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  transition: background-color 0.2s ease;
}

.trash-icon-btn:hover {
  background: #f0f0f0;
}

.trash-icon-btn img {
  width: 18px;
  height: 18px;
}

@media (max-width: 768px) {
  .trash-view {
    padding: 16px;
  }

  .trash-title {
    font-size: 1.2em;
  }

  .trash-filters {
    flex-direction: column;
    align-items: stretch;
  }

  .trash-filters__search,
  .trash-action-btn {
    width: 100%;
    justify-content: center;
  }

  .trash-table th:nth-child(n+4),
  .trash-table td:nth-child(n+4) {
    display: none;
  }
}
</style>
