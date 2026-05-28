<template>
  <section class="trash-view">
    <header class="trash-header">
      <div class="trash-titlebar">
        <h2 class="trash-title">
          Таблица
          <RouterLink
            :to="`/table/${tableName}`"
            class="trash-title__name"
          >
            {{ displayName }}
          </RouterLink>
          / Корзина
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
            data-testid="trash-history"
            @click="openHistory"
          >
            История
          </button>
          <button
            class="trash-btn trash-btn--ghost"
            data-testid="trash-export"
            :disabled="!items.length || isExporting"
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
            class="trash-btn trash-btn--ghost"
            data-testid="trash-clear"
            :disabled="!items.length"
            @click="onClearAll"
          >
            Очистить
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
              :class="{ 'trash-btn__icon--spin': isLoading }"
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

        <div class="trash-card__spacer" />

        <span
          v-if="selectedIds.length"
          class="trash-card__selected"
        >
          Выбрано: {{ selectedIds.length }}
        </span>
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
          <span class="trash-spinner" />
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
              <th class="trash-table__th trash-table__th-check">
                <input
                  type="checkbox"
                  class="trash-check"
                  :checked="allSelected"
                  data-testid="trash-select-all"
                  @change="toggleSelectAll"
                >
              </th>
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
              class="trash-table__row"
              data-testid="trash-row"
              @click="onRowClick(item)"
            >
              <td
                class="trash-table__td trash-table__td-check"
                @click.stop
              >
                <input
                  type="checkbox"
                  class="trash-check"
                  :checked="isSelected(item.id)"
                  data-testid="trash-row-check"
                  @change="toggleSelect(item.id)"
                >
              </td>
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
                <td class="trash-table__td">
                  {{ item.middle_name || '—' }}
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
              <td
                class="trash-table__td trash-table__td--actions"
                @click.stop
              >
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

    <!-- Детальная модалка -->
    <VehicleDetailsModal
      v-if="tableType === 'cars' && showDetails"
      :show="showDetails"
      :vehicle="selectedDetail"
      :all-unloading-places="[]"
      :license-plate-formats="[]"
      :show-car-features="false"
      :source="'trash'"
      @close="closeDetails"
    />
    <EmployeeDetailsModal
      v-if="tableType === 'people' && showDetails"
      :show="showDetails"
      :employee="selectedDetail"
      :all-tables="[]"
      :source="'trash'"
      @close="closeDetails"
      @open-application="() => {}"
    />

    <!-- Модалка истории корзины -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showHistory"
          class="trash-modal-overlay"
          @click.self="closeHistory"
        >
          <div class="trash-modal">
            <div class="trash-modal__header">
              <h3 class="trash-modal__title">
                История корзины
              </h3>
              <button
                class="trash-modal__close"
                data-testid="trash-history-close"
                @click="closeHistory"
              >
                <svg
                  width="12"
                  height="12"
                  viewBox="0 0 14 14"
                  fill="none"
                >
                  <path
                    d="M13 1L1 13M1 1L13 13"
                    stroke="#666"
                    stroke-width="2"
                    stroke-linecap="round"
                  />
                </svg>
              </button>
            </div>
            <div class="trash-modal__body">
              <div
                v-if="historyLoading"
                class="trash-state"
              >
                <span class="trash-spinner" />
                Загрузка...
              </div>
              <div
                v-else-if="!historyItems.length"
                class="trash-state"
              >
                История пуста
              </div>
              <ul
                v-else
                class="trash-log"
              >
                <li
                  v-for="h in historyItems"
                  :key="h.id"
                  class="trash-log__item"
                >
                  <span
                    class="trash-log__dot"
                    :class="h.action_type === 'bulk_restored' ? 'trash-log__dot--restore' : 'trash-log__dot--clear'"
                  />
                  <div class="trash-log__content">
                    <p class="trash-log__text">
                      {{ historyActionText(h) }}
                    </p>
                    <p class="trash-log__meta">
                      {{ h.user_name || 'Система' }} · {{ formatDateTime(h.created_at) }}
                    </p>
                  </div>
                </li>
              </ul>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>
  </section>
</template>

<script>
import ExcelJS from 'exceljs';
import { apiRequest } from '@/api/client';
import { useUiStore } from '@/stores/ui';
import { listTrash, restoreItems, purgeItem, clearTrash, getTrashHistory } from '@/api/trash';
import SearchComponent from '@/components/SearchComponent.vue';
import OrganizationFilter from '@/components/OrganizationFilter.vue';
import DateFilter from '@/components/DateFilter.vue';
import VehicleDetailsModal from '@/components/CreateApplication/VehicleDetailsModal.vue';
import EmployeeDetailsModal from '@/components/CreateApplication/EmployeeDetailsModal.vue';

export default {
  name: 'TrashView',
  components: { SearchComponent, OrganizationFilter, DateFilter, VehicleDetailsModal, EmployeeDetailsModal },
  data() {
    return {
      tableID: 0,
      tableType: '',
      displayName: '',
      items: [],
      selectedIds: [],
      isLoading: false,
      isExporting: false,
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
      showDetails: false,
      selectedDetail: null,
      showHistory: false,
      historyItems: [],
      historyLoading: false,
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
            { key: 'middle_name', label: 'Отчество', sortable: true },
          ];
      return [
        ...base,
        ...typeCols,
        { key: 'organization', label: 'Организация', sortable: true },
        { key: 'entry_date_to', label: 'Действует до', sortable: true },
        { key: 'time', label: 'Время', sortable: false },
        { key: 'status', label: 'Статус', sortable: false },
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
    sortBy(field) {
      if (this.sortField === field) {
        this.sortDir = this.sortDir === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDir = 'desc';
      }
    },
    isSelected(id) {
      return this.selectedIds.includes(id);
    },
    toggleSelect(id) {
      const i = this.selectedIds.indexOf(id);
      if (i === -1) this.selectedIds.push(id);
      else this.selectedIds.splice(i, 1);
    },
    toggleSelectAll() {
      if (this.allSelected) this.selectedIds = [];
      else this.selectedIds = this.items.map(i => i.id);
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
      if (to) return to;
      return '—';
    },
    onRowClick(item) {
      const deletedAtText = this.formatDateTime(item.deleted_at);
      if (this.tableType === 'cars') {
        this.selectedDetail = {
          plateNumber: item.car_number,
          mark: item.mark_name,
          formatId: null,
          organization: item.organization,
          organizationId: null,
          company: '',
          companyId: null,
          isExisting: true,
          unloadPlaces: [],
          entry_date_to: item.entry_date_to,
          entry_time_from: item.entry_time_from,
          entry_time_to: item.entry_time_to,
          applicationId: null,
          deletedByName: item.deleted_by_name,
          deletedAtText,
        };
      } else {
        this.selectedDetail = {
          id: item.id,
          last_name: item.last_name,
          first_name: item.first_name,
          middle_name: item.middle_name,
          position: '',
          citizenshipName: '',
          passport_series_number: '',
          patent_number: '',
          other_permission: '',
          organization: item.organization,
          organizationId: null,
          company: '',
          companyId: null,
          entry_date_to: item.entry_date_to,
          pass_time: this.formatTimeRange(item),
          target_tables: [],
          territory_status: null,
          applicationId: null,
          deletedByName: item.deleted_by_name,
          deletedAtText,
        };
      }
      this.showDetails = true;
    },
    closeDetails() {
      this.showDetails = false;
      this.selectedDetail = null;
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
    async openHistory() {
      this.showHistory = true;
      this.historyLoading = true;
      try {
        const data = await getTrashHistory(this.tableID);
        this.historyItems = Array.isArray(data) ? data : [];
      } catch {
        useUiStore().error('Не удалось загрузить историю');
        this.historyItems = [];
      } finally {
        this.historyLoading = false;
      }
    },
    closeHistory() {
      this.showHistory = false;
    },
    historyActionText(h) {
      if (h.action_type === 'cleared') {
        return `Корзина очищена (${h.affected_count} запис.)`;
      }
      if (h.action_type === 'bulk_restored') {
        return `Восстановлено ${h.affected_count} элемент(ов)`;
      }
      return h.action_type;
    },
    async onExport() {
      if (!this.items.length) return;
      this.isExporting = true;
      try {
        const isCars = this.tableType === 'cars';
        const headers = isCars
          ? ['Номер заявки', 'Дата и время удаления', 'Номер Т/С', 'Марка', 'Организация', 'Действует до', 'Время', 'Статус', 'Кто удалил']
          : ['Номер заявки', 'Дата и время удаления', 'Фамилия', 'Имя', 'Отчество', 'Организация', 'Действует до', 'Время', 'Статус', 'Кто удалил'];

        const workbook = new ExcelJS.Workbook();
        const worksheet = workbook.addWorksheet('Korzina');

        const headerRow = worksheet.addRow(headers);
        headerRow.height = 25;
        headerRow.eachCell((cell) => {
          cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF4F5BDF' } };
          cell.font = { name: 'Verdana', size: 11, bold: true, color: { argb: 'FFFFFFFF' } };
          cell.alignment = { vertical: 'middle', horizontal: 'center' };
        });

        this.sortedItems.forEach((item, index) => {
          const status = isCars ? 'Удалена' : 'Удалён';
          const row = isCars
            ? [item.application_number || '', this.formatDateTime(item.deleted_at), item.car_number || '', item.mark_name || '', item.organization || '', this.formatDate(item.entry_date_to), this.formatTimeRange(item), status, item.deleted_by_name || '']
            : [item.application_number || '', this.formatDateTime(item.deleted_at), item.last_name || '', item.first_name || '', item.middle_name || '', item.organization || '', this.formatDate(item.entry_date_to), this.formatTimeRange(item), status, item.deleted_by_name || ''];
          const r = worksheet.addRow(row);
          r.height = 20;
          const fillColor = index % 2 === 0 ? 'FFF0F5FF' : 'FFE0E9FF';
          r.eachCell((cell) => {
            cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: fillColor } };
            cell.font = { name: 'Verdana', size: 9, color: { argb: 'FF333333' } };
            cell.alignment = { vertical: 'middle' };
          });
        });

        worksheet.columns = headers.map(() => ({ width: 22 }));

        const buffer = await workbook.xlsx.writeBuffer();
        const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.download = `korzina_${this.displayName}_${new Date().toISOString().slice(0, 10)}.xlsx`;
        a.href = url;
        a.click();
        window.URL.revokeObjectURL(url);
      } catch (e) {
        console.error('Ошибка экспорта корзины', e);
        useUiStore().error('Ошибка при экспорте');
      } finally {
        this.isExporting = false;
      }
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

.trash-btn__icon--spin {
  animation: trash-spin 0.8s linear infinite;
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

.trash-card__spacer {
  flex: 1;
}

.trash-card__selected {
  font-size: 13px;
  color: #A2A2A2;
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
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.trash-state--error {
  color: #FF6668;
}

.trash-spinner {
  width: 28px;
  height: 28px;
  border: 3px solid #E6E6E6;
  border-top-color: #4F5BDF;
  border-radius: 50%;
  animation: trash-spin 0.8s linear infinite;
}

@keyframes trash-spin {
  to { transform: rotate(360deg); }
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

.trash-table__th-check,
.trash-table__td-check {
  width: 44px;
  padding-left: 20px;
  padding-right: 0;
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

.trash-table__row {
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.trash-table__row:hover {
  background: #f8f9ff;
}

.trash-table__td {
  padding: 12px;
  font-weight: 400;
  font-size: 14px;
  line-height: 17px;
  color: #000000;
  border-top: 1px solid #F0F0F0;
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

.trash-check {
  width: 16px;
  height: 16px;
  cursor: pointer;
  accent-color: #4F5BDF;
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

/* История корзины */
.trash-modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.trash-modal {
  background: #FFFFFF;
  border-radius: 30px;
  width: 480px;
  max-width: calc(100vw - 40px);
  max-height: 70vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  font-family: 'Montserrat', sans-serif;
}

.trash-modal__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px;
  border-bottom: 1px solid #E6E6E6;
}

.trash-modal__title {
  margin: 0;
  font-weight: 600;
  font-size: 16px;
  color: #000;
}

.trash-modal__close {
  background: none;
  border: none;
  cursor: pointer;
  padding: 4px;
}

.trash-modal__body {
  padding: 12px 24px 24px;
  overflow-y: auto;
}

.trash-log {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.trash-log__item {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}

.trash-log__dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  margin-top: 5px;
  flex-shrink: 0;
}

.trash-log__dot--restore {
  background: #34C759;
}

.trash-log__dot--clear {
  background: #FF6668;
}

.trash-log__text {
  margin: 0;
  font-size: 14px;
  color: #000;
}

.trash-log__meta {
  margin: 2px 0 0;
  font-size: 12px;
  color: #A2A2A2;
}

.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
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
}
</style>
