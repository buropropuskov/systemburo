<template>
  <section class="trash-view">
    <header class="trash-header">
      <div class="trash-titlebar">
        <h2 class="trash-title">
          Таблица <RouterLink :to="`/table/${tableName}`" class="trash-title__name">{{ displayName }}</RouterLink> / Корзина
        </h2>
        <RouterLink
          :to="`/table/${tableName}`"
          class="trash-back-btn"
          data-testid="trash-back"
        >
          <img
            src="@/assets/icons/arrow.png"
            class="trash-back-btn__icon"
            alt=""
          >
          Назад
        </RouterLink>
      </div>

      <div class="trash-filters">
        <div class="trash-filters__group trash-filters__group--left">
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
        </div>
        <div class="trash-filters__group trash-filters__group--right">
          <button
            class="trash-btn trash-btn--ghost"
            data-testid="trash-clear-filters"
            @click="clearFilters"
          >
            Очистить
          </button>
          <button
            class="trash-btn trash-btn--ghost"
            data-testid="trash-export"
            :disabled="!items.length"
            @click="onExport"
          >
            <img
              src="@/assets/icons/export.png"
              class="trash-btn__icon"
              alt=""
            >
            Экспорт
          </button>
          <button
            class="trash-btn trash-btn--refresh"
            data-testid="trash-refresh"
            :disabled="isLoading"
            @click="reload"
          >
            <img
              src="@/assets/icons/refresh.png"
              class="trash-btn__icon"
              alt=""
            >
            Обновить
          </button>
        </div>
      </div>
    </header>

    <article class="trash-card">
      <div class="trash-card__header">
        <h3 class="trash-card__title">
          {{ tableType === 'cars' ? 'Удаленные автомобили' : 'Удаленные сотрудники' }}
        </h3>
        <button
          class="trash-card__link"
          data-testid="trash-restore-all-link"
          :disabled="!items.length"
          @click="onRestoreAll"
        >
          Восстановить
        </button>

        <div class="trash-card__spacer" />

        <button
          class="trash-btn trash-btn--primary"
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
              <th
                v-for="col in columns"
                :key="col.key"
                :class="['trash-table__th', col.sortable && 'trash-table__th--sortable', sortField === col.key && 'trash-table__th--active']"
                @click="col.sortable && sortBy(col.key)"
              >
                <span>{{ col.label }}</span>
                <img
                  v-if="col.sortable"
                  src="@/assets/icons/sort.png"
                  class="trash-table__sort"
                  :class="{ 'trash-table__sort--desc': sortField === col.key && sortDir === 'desc' }"
                  alt=""
                >
              </th>
              <th class="trash-table__th-actions" />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="item in sortedItems"
              :key="item.id"
              data-testid="trash-row"
            >
              <td class="trash-table__td trash-table__td--muted">
                {{ item.application_number || '—' }}
              </td>
              <td class="trash-table__td">
                {{ formatDateTime(item.deleted_at) }}
              </td>
              <template v-if="tableType === 'cars'">
                <td class="trash-table__td">
                  {{ item.car_number || '—' }}
                </td>
                <td class="trash-table__td">
                  {{ item.mark_name || '—' }}
                </td>
              </template>
              <template v-else>
                <td class="trash-table__td">
                  {{ item.last_name || '—' }}
                </td>
                <td class="trash-table__td">
                  {{ item.first_name || '—' }}
                </td>
              </template>
              <td class="trash-table__td">
                {{ item.organization || '—' }}
              </td>
              <td class="trash-table__td">
                {{ formatDate(item.entry_date_to) }}
              </td>
              <td class="trash-table__td">
                {{ formatTimeRange(item) }}
              </td>
              <td class="trash-table__td trash-table__td--danger">
                {{ tableType === 'cars' ? 'Удалена' : 'Удалён' }}
              </td>
              <td class="trash-table__td trash-table__td--actions">
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

export default {
  name: 'TrashView',
  components: { SearchComponent, OrganizationFilter, DateFilter },
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
      sortField: 'deleted_at',
      sortDir: 'desc',
    };
  },
  computed: {
    tableName() {
      return this.$route.params.tableName;
    },
    columns() {
      const base = [
        { key: 'application_number', label: 'Номер заявки', sortable: true },
        { key: 'deleted_at', label: 'Дата и время удаления', sortable: true },
      ];
      const typeCols = this.tableType === 'cars'
        ? [
            { key: 'car_number', label: 'Номер Т/С', sortable: true },
            { key: 'mark_name', label: 'Марка', sortable: true },
          ]
        : [
            { key: 'last_name', label: 'Фамилия', sortable: true },
            { key: 'first_name', label: 'Имя', sortable: true },
          ];
      return [
        ...base,
        ...typeCols,
        { key: 'organization', label: 'Организация', sortable: true },
        { key: 'entry_date_to', label: 'Действует до', sortable: true },
        { key: 'time', label: 'Время', sortable: false },
        { key: 'status', label: 'Статус', sortable: true },
      ];
    },
    sortedItems() {
      const arr = [...this.items];
      const field = this.sortField;
      const dir = this.sortDir === 'asc' ? 1 : -1;
      arr.sort((a, b) => {
        const va = a[field] ?? '';
        const vb = b[field] ?? '';
        if (va < vb) return -1 * dir;
        if (va > vb) return 1 * dir;
        return 0;
      });
      return arr;
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
    sortBy(field) {
      if (this.sortField === field) {
        this.sortDir = this.sortDir === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDir = 'desc';
      }
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
      const ok = await useUiStore().confirm({
        title: 'Удаление записи',
        message: 'Удалить эту запись безвозвратно? Действие нельзя отменить.',
        confirmText: 'Удалить',
        danger: true,
      });
      if (!ok) return;
      try {
        await purgeItem(this.tableID, id);
        useUiStore().success('Запись удалена безвозвратно');
        await this.reload();
      } catch {
        useUiStore().error('Не удалось удалить');
      }
    },
    async onClearAll() {
      const ok = await useUiStore().confirm({
        title: 'Очистка корзины',
        message: 'Очистить корзину целиком? Действие нельзя отменить.',
        confirmText: 'Очистить',
        danger: true,
      });
      if (!ok) return;
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
  max-width: 1440px;
  margin: 0 auto;
  font-family: 'Montserrat', sans-serif;
}

.trash-header {
  display: flex;
  flex-direction: column;
  gap: 18px;
  margin-bottom: 20px;
}

.trash-titlebar {
  display: flex;
  align-items: center;
  gap: 16px;
}

.trash-title {
  margin: 0;
  font-family: 'Montserrat', sans-serif;
  font-weight: 700;
  font-size: 18px;
  line-height: 22px;
  color: #A2A2A2;
}

.trash-title__name {
  color: #A2A2A2;
  text-decoration: none;
}

.trash-title__name:hover {
  color: #4F5BDF;
}

.trash-back-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 0 18px;
  height: 35px;
  background: #FFFFFF;
  border: 1px solid #E6E6E6;
  border-radius: 50px;
  font-family: 'Montserrat', sans-serif;
  font-weight: 500;
  font-size: 14px;
  line-height: 17px;
  color: #4F5BDF;
  text-decoration: none;
  white-space: nowrap;
  transition: background 0.2s ease;
}

.trash-back-btn:hover {
  background: #f3f4ff;
}

.trash-back-btn__icon {
  width: 12px;
  height: 12px;
  transform: rotate(180deg);
}

.trash-filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.trash-filters__group {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

.trash-filters__group--left {
  flex: 1 1 auto;
}

.trash-filters__group--right {
  flex: 0 0 auto;
}

.trash-filters__search {
  flex: 0 0 200px;
}

.trash-filters :deep(.field) {
  height: 35px;
  width: 200px;
  background-color: #FFFFFF;
  border-radius: 10px;
  border: 1px solid #E6E6E6;
  padding: 0 10px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  position: relative;
  cursor: pointer;
  font-family: 'Montserrat', sans-serif;
  font-size: 14px;
}

.trash-filters :deep(.field--select) {
  width: 200px;
  min-width: 200px;
}

.trash-filters :deep(.field--select .select-text) {
  font-size: 14px;
  color: #A2A2A2;
}

.trash-filters :deep(.field--select .select-icon) {
  width: 10px;
  height: 10px;
}

.trash-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 35px;
  padding: 0 16px;
  background: #FFFFFF;
  border: 1px solid #E6E6E6;
  border-radius: 10px;
  font-family: 'Montserrat', sans-serif;
  font-weight: 500;
  font-size: 14px;
  line-height: 17px;
  color: #000000;
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.2s ease;
}

.trash-btn:hover:not(:disabled) {
  background: #f8f9fa;
}

.trash-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.trash-btn__icon {
  width: 15px;
  height: 15px;
}

.trash-btn--refresh {
  border-radius: 50px;
  color: #4F5BDF;
}

.trash-btn--primary {
  background: #4F5BDF;
  border-color: #4F5BDF;
  color: #FFFFFF;
  padding: 0 20px;
}

.trash-btn--primary:hover:not(:disabled) {
  background: #3a45b2;
}

.trash-card {
  background: #FFFFFF;
  border: 1px solid #E6E6E6;
  box-shadow: 0px 3px 10px rgba(0, 0, 0, 0.05);
  border-radius: 30px;
  overflow: hidden;
  max-height: 575px;
  display: flex;
  flex-direction: column;
}

.trash-card__header {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 0 20px;
  height: 50px;
  border-bottom: 1px solid #E6E6E6;
  flex-shrink: 0;
}

.trash-card__title {
  margin: 0;
  font-family: 'Montserrat', sans-serif;
  font-weight: 600;
  font-size: 1.1em;
  color: #000000;
}

.trash-card__link {
  background: none;
  border: none;
  cursor: pointer;
  padding: 0;
  font-family: 'Montserrat', sans-serif;
  font-weight: 500;
  font-size: 12px;
  line-height: 15px;
  color: #A2A2A2;
}

.trash-card__link:hover:not(:disabled) {
  color: #4F5BDF;
}

.trash-card__link:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.trash-card__spacer {
  flex: 1;
}

.trash-card__body {
  padding: 0;
  flex-grow: 1;
  overflow-y: auto;
}

.trash-state {
  padding: 60px 24px;
  text-align: center;
  font-family: 'Montserrat', sans-serif;
  color: #A2A2A2;
  font-size: 14px;
}

.trash-state--error {
  color: #FF6668;
}

.trash-table {
  width: 100%;
  border-collapse: collapse;
  font-family: 'Montserrat', sans-serif;
}

.trash-table__th {
  padding: 10px 12px;
  text-align: left;
  font-weight: 500;
  font-size: 14px;
  line-height: 17px;
  color: #A2A2A2;
  white-space: nowrap;
  user-select: none;
}

.trash-table__th:first-child {
  padding-left: 20px;
}

.trash-table__th--sortable {
  cursor: pointer;
}

.trash-table__th--sortable:hover {
  color: #333;
}

.trash-table__th--active {
  color: #4F5BDF;
}

.trash-table__sort {
  width: 12px;
  height: 12px;
  margin-left: 6px;
  vertical-align: middle;
  opacity: 0.6;
  transition: transform 0.2s ease;
}

.trash-table__th--active .trash-table__sort {
  opacity: 1;
}

.trash-table__sort--desc {
  transform: rotate(180deg);
}

.trash-table__th-actions {
  width: 60px;
}

.trash-table__td {
  padding: 12px;
  font-weight: 400;
  font-size: 14px;
  line-height: 17px;
  color: #000000;
  border-top: 1px solid #F0F0F0;
}

.trash-table__td:first-child {
  padding-left: 20px;
}

.trash-table__td--muted {
  color: #A2A2A2;
}

.trash-table__td--danger {
  color: #FF6668;
}

.trash-table__td--actions {
  text-align: right;
  padding-right: 20px;
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
  width: 20px;
  height: 20px;
}

@media (max-width: 1100px) {
  .trash-filters {
    flex-direction: column;
    align-items: stretch;
  }
  .trash-card__header {
    flex-wrap: wrap;
  }
}

@media (max-width: 768px) {
  .trash-view {
    padding: 16px;
  }
  .trash-title {
    font-size: 16px;
  }
  .trash-table th:nth-child(n+5),
  .trash-table td:nth-child(n+5) {
    display: none;
  }
}
</style>
