<template>
  <section class="trash-view">
    <header class="trash-header">
      <div class="trash-titlebar">
        <h2 class="trash-title">
          <RouterLink
            :to="`/table/${tableName}`"
            class="trash-title__link"
          >
            <span class="trash-title__prefix">Таблица</span>
            <span class="trash-title__name">{{ displayName }}</span>
          </RouterLink>
          <span class="trash-title__sep">/ Корзина</span>
        </h2>
        <RouterLink
          :to="`/table/${tableName}`"
          class="trash-back-btn"
          data-testid="trash-back"
        >
          <svg
            class="trash-back-btn__icon"
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
          >
            <path
              d="M15 18L9 12L15 6"
              stroke="currentColor"
              stroke-width="2.5"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
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
          <div class="trash-filters__control">
            <BaseDropdown
              :model-value="filters.organizationIds"
              :options="organizations"
              placeholder="Все организации"
              summary-label="Организация"
              data-testid="trash-filter-organizations"
              multiple
              searchable
              teleport
              @update:model-value="onOrganizationsChange"
            />
          </div>
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
            class="trash-tool-btn"
            data-testid="trash-export"
            :disabled="!items.length || isExporting"
            @click="onExport"
          >
            <AppIcon
              name="export"
              class="trash-tool-btn__icon"
            />
            Экспорт
          </button>
          <button
            class="trash-tool-btn"
            data-testid="trash-clear"
            :disabled="!items.length"
            @click="onClearAll"
          >
            <AppIcon
              name="trashcan"
              class="trash-tool-btn__icon"
            />
            Очистить
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
          class="trash-restore-btn"
          data-testid="trash-restore-selected"
          :disabled="!selectedIds.length"
          @click="onRestoreSelected"
        >
          Восстановить
        </button>
        <button
          class="trash-history-btn"
          data-testid="trash-history"
          @click="openHistory"
        >
          История
        </button>
        <RefreshButton
          :loading="isLoading"
          @refresh="reload"
        />
      </div>

      <div
        ref="cardBody"
        class="trash-card__body"
        :style="bodyStyle"
      >
        <div
          v-if="isLoading"
          class="trash-overlay"
        >
          <span class="trash-spinner" />
        </div>
        <div
          v-if="error"
          class="trash-state trash-state--error"
        >
          {{ error }}
        </div>
        <div
          v-else-if="!items.length && !isLoading"
          class="trash-state"
        >
          Корзина пуста
        </div>
        <table
          v-else-if="items.length"
          class="trash-table rt-table"
          data-testid="trash-table"
        >
          <thead class="rt-head-row">
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
                <AppIcon
                  v-if="col.sortable"
                  name="sort"
                  class="trash-table__sort"
                  :class="{
                    'trash-table__sort--sorted': sortField === col.key,
                    'trash-table__sort--desc': sortField === col.key && sortDir === 'desc',
                  }"
                />
              </th>
              <th class="trash-table__th-actions" />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="item in sortedItems"
              :key="item.id"
              class="trash-table__row rt-row"
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
                  :aria-label="`Выбрать запись ${item.application_number || item.id}`"
                  data-testid="trash-row-check"
                  @change="toggleSelect(item.id)"
                >
              </td>
              <td
                class="trash-table__td trash-table__td--muted"
                data-label="Номер заявки"
              >
                {{ item.application_number || '—' }}
              </td>
              <td
                class="trash-table__td"
                data-label="Дата и время удаления"
              >
                {{ formatDateTime(item.deleted_at) }}
              </td>
              <template v-if="tableType === 'cars'">
                <td
                  class="trash-table__td"
                  data-label="Номер Т/С"
                >
                  {{ item.car_number || '—' }}
                </td>
                <td
                  class="trash-table__td"
                  data-label="Марка"
                >
                  {{ item.mark_name || '—' }}
                </td>
              </template>
              <template v-else>
                <td
                  class="trash-table__td"
                  data-label="Фамилия"
                >
                  {{ item.last_name || '—' }}
                </td>
                <td
                  class="trash-table__td"
                  data-label="Имя"
                >
                  {{ item.first_name || '—' }}
                </td>
                <td
                  class="trash-table__td"
                  data-label="Отчество"
                >
                  {{ item.middle_name || '—' }}
                </td>
              </template>
              <td
                class="trash-table__td"
                data-label="Организация"
              >
                {{ item.organization || '—' }}
              </td>
              <td
                class="trash-table__td"
                data-label="Действует до"
              >
                {{ formatDate(item.entry_date_to) }}
              </td>
              <td
                class="trash-table__td"
                data-label="Время"
              >
                {{ formatTimeRange(item) }}
              </td>
              <td
                class="trash-table__td"
                data-label="Статус"
              >
                <span class="trash-badge trash-badge--deleted">
                  {{ tableType === 'cars' ? 'Удалена' : 'Удалён' }}
                </span>
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
                  <AppIcon
                    name="trashcan"
                    class="trash-icon-btn__icon"
                  />
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
      :all-unloading-places="detailPlaces"
      :license-plate-formats="[]"
      :show-car-features="true"
      :source="'trash'"
      @close="closeDetails"
      @open-application="handleOpenApplication"
    />
    <EmployeeDetailsModal
      v-if="tableType === 'people' && showDetails"
      :show="showDetails"
      :employee="selectedDetail"
      :all-tables="detailPlaces"
      :source="'trash'"
      @close="closeDetails"
      @open-application="handleOpenApplication"
    />

    <!-- История корзины -->
    <TrashHistoryModal
      v-if="showHistory"
      :table-id="tableID"
      :table-display-name="displayName"
      :current-user-name="currentUserName"
      @close="closeHistory"
    />

    <!-- Подтверждение удаления (как в BlankSelector) -->
    <ConfirmationModal
      :show="confirmModal.show"
      title="Подтверждение удаления"
      :message="confirmModal.message"
      :confirm-text="confirmModal.confirmText"
      :confirm-button-style="{ background: '#ff4444', borderColor: '#ff4444' }"
      @confirm="onConfirmModal"
      @cancel="cancelConfirm"
    />

    <!-- Детали заявки (открывается поверх деталей машины/сотрудника) -->
    <ApplicationDetail
      v-if="showApplicationDetail && selectedApplication"
      :application="selectedApplication"
      :current-user-name="currentUserName"
      :mode="'center'"
      @close="closeApplicationDetail"
    />
  </section>
</template>

<script>
import ExcelJS from 'exceljs';
import { apiRequest } from '@/api/client';
import { useDeletionsStore } from '@/stores/deletions';
import { listTrash, restoreItems, purgeItem, clearTrash } from '@/api/trash';
import SearchComponent from '@/components/SearchComponent.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import DateFilter from '@/components/DateFilter.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import VehicleDetailsModal from '@/components/CreateApplication/VehicleDetailsModal.vue';
import EmployeeDetailsModal from '@/components/CreateApplication/EmployeeDetailsModal.vue';
import TrashHistoryModal from '@/components/TrashHistoryModal.vue';
import ConfirmationModal from '@/components/ConfirmationModal.vue';
import ApplicationDetail from '@/components/ApplicationDetail/ApplicationDetail.vue';
import AppIcon from '@/components/icons/AppIcon.vue';
import { formatMoscowDateTime } from '@/utils/serverTime';

export default {
  name: 'TrashView',
  components: {
    SearchComponent, BaseDropdown, DateFilter, RefreshButton,
    VehicleDetailsModal, EmployeeDetailsModal, TrashHistoryModal, ConfirmationModal,
    ApplicationDetail,
    AppIcon,
  },
  data() {
    return {
      tableID: 0,
      tableType: '',
      displayName: '',
      currentUserName: '',
      items: [],
      selectedIds: [],
      isLoading: false,
      isExporting: false,
      error: '',
      filters: {
        search: '',
        organizationIds: [],
        selectedDate: null,
        dateFrom: null,
        dateTo: null,
      },
      searchTimer: null,
      sortField: '',
      sortDir: 'desc',
      showDetails: false,
      selectedDetail: null,
      detailPlaces: [],
      showHistory: false,
      organizations: [],
      lastHeight: 0,
      confirmModal: { show: false, message: '', confirmText: 'Удалить', action: null, itemId: null },
      showApplicationDetail: false,
      selectedApplication: null,
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
      const field = this.sortField || 'deleted_at';
      const dir = (this.sortField && this.sortDir === 'asc') ? 1 : -1;
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
    currentUserDisplayName() {
      if (!this.currentUserName) return 'Пользователь';
      const parts = this.currentUserName.split(' ').filter(p => p && p !== 'null' && p !== 'undefined');
      return parts.length > 0 ? parts.join(' ') : 'Пользователь';
    },
    formattedExportDateTime() {
      return formatMoscowDateTime();
    },
    bodyStyle() {
      return this.isLoading ? { minHeight: `${this.lastHeight || 200}px` } : {};
    },
  },
  watch: {
    '$route.params.tableName'() {
      this.fetchTable().then(() => this.reload());
    },
  },
  async mounted() {
    this.fetchCurrentUser();
    this.fetchOrganizations();
    await this.fetchTable();
    if (this.tableID) await this.reload();
  },
  methods: {
    async fetchCurrentUser() {
      try {
        const res = await apiRequest('/users/me');
        const data = await res.json();
        const parts = [data.last_name, data.first_name, data.middle_name].filter(Boolean);
        this.currentUserName = parts.join(' ') || data.username || '';
      } catch {
        this.currentUserName = '';
      }
    },
    async fetchOrganizations() {
      try {
        const res = await apiRequest('/organizations');
        const data = await res.json();
        this.organizations = Array.isArray(data) ? data : [];
      } catch {
        this.organizations = [];
      }
    },
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
      if (this.$refs.cardBody && this.$refs.cardBody.offsetHeight) {
        this.lastHeight = this.$refs.cardBody.offsetHeight;
      }
      this.isLoading = true;
      this.selectedIds = [];
      try {
        const params = {
          search: this.filters.search,
          dateFrom: this.filters.selectedDate || this.filters.dateFrom || '',
          dateTo: this.filters.selectedDate || this.filters.dateTo || '',
        };
        if (this.filters.organizationIds.length) {
          params.organizationIds = this.filters.organizationIds;
        }
        const data = await listTrash(this.tableID, params);
        this.items = Array.isArray(data) ? data : [];
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить корзину', type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },
    onSearchChange() {
      clearTimeout(this.searchTimer);
      this.searchTimer = setTimeout(() => this.reload(), 300);
    },
    onOrganizationsChange(ids) {
      this.filters.organizationIds = Array.isArray(ids) ? ids : [];
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
      const hhmm = (v) => (typeof v === 'string' && v.length >= 5 ? v.slice(0, 5) : v);
      const from = hhmm(item.entry_time_from || item.time_from);
      const to = hhmm(item.entry_time_to || item.time_to);
      if (from && to) return `${from} - ${to}`;
      if (from) return from;
      if (to) return to;
      return '—';
    },
    onRowClick(item) {
      const deletedAtText = this.formatDateTime(item.deleted_at);
      if (this.tableType === 'cars') {
        const places = Array.isArray(item.unload_places) ? item.unload_places : [];
        this.detailPlaces = places;
        this.selectedDetail = {
          id: item.id,
          plateNumber: item.car_number,
          mark: item.mark_name,
          formatId: null,
          organization: item.organization,
          organizationId: null,
          company: item.company || '',
          companyId: null,
          isExisting: true,
          unloadPlaces: places.map(p => p.id),
          entry_date_to: item.entry_date_to,
          entry_time_from: item.entry_time_from,
          entry_time_to: item.entry_time_to,
          applicationId: item.application_id || null,
          deletedByName: item.deleted_by_name,
          deletedAtText,
        };
      } else {
        const places = Array.isArray(item.pass_places) ? item.pass_places : [];
        this.detailPlaces = places;
        this.selectedDetail = {
          id: item.id,
          last_name: item.last_name,
          first_name: item.first_name,
          middle_name: item.middle_name,
          position: item.position || '',
          citizenshipName: item.citizenship_name || '',
          passport_series_number: item.passport_series_number || '',
          patent_number: item.patent_number || '',
          other_permission: item.other_permission || '',
          organization: item.organization,
          organizationId: null,
          company: item.company || '',
          companyId: null,
          entry_date_to: item.entry_date_to,
          pass_time: this.formatTimeRange(item),
          target_tables: places.map(p => p.id),
          territory_status: null,
          applicationId: item.application_id || null,
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
    async handleOpenApplication(applicationId) {
      if (!applicationId) return;
      try {
        const response = await apiRequest(`/applications/${applicationId}/details`, {});
        if (!response.ok) {
          useDeletionsStore().notify({ prefix: 'Не удалось загрузить заявку', type: 'error' });
          return;
        }
        this.selectedApplication = await response.json();
        this.showApplicationDetail = true;
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить заявку', type: 'error' });
      }
    },
    closeApplicationDetail() {
      this.showApplicationDetail = false;
      this.selectedApplication = null;
    },
    async onRestoreSelected() {
      if (!this.selectedIds.length) return;
      const firstItem = this.items.find(i => i.id === this.selectedIds[0]);
      try {
        const result = await restoreItems(this.tableID, this.selectedIds);
        const r = (result && result.restored) || 0;
        const req = (result && result.requested) || this.selectedIds.length;
        if (r >= 1) {
          this.notifyRestored(r, firstItem);
        }
        if (r < req) {
          useDeletionsStore().notify({ prefix: `Восстановлено ${r} из ${req}. У остальных нет активной согласованной заявки.`, type: 'error' });
        }
        await this.reload();
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось восстановить', type: 'error' });
      }
    },
    notifyRestored(count, firstItem) {
      const isCars = this.tableType === 'cars';
      if (count === 1 && firstItem) {
        const name = isCars
          ? firstItem.car_number
          : [firstItem.last_name, firstItem.first_name, firstItem.middle_name].filter(Boolean).join(' ');
        useDeletionsStore().notify({
          prefix: isCars ? 'Машина ' : 'Сотрудник ',
          bold: name || '',
          suffix: isCars ? ' восстановлена' : ' восстановлен',
        });
      } else {
        useDeletionsStore().notify({
          prefix: 'Восстановлено ',
          bold: String(count),
          suffix: ' элемент(ов)',
        });
      }
    },
    onPurgeOne(id) {
      this.confirmModal = {
        show: true,
        message: 'Удалить эту запись безвозвратно? Действие нельзя отменить.',
        confirmText: 'Удалить',
        action: 'purge',
        itemId: id,
      };
    },
    onClearAll() {
      this.confirmModal = {
        show: true,
        message: 'Очистить корзину целиком? Действие нельзя отменить.',
        confirmText: 'Очистить',
        action: 'clear',
        itemId: null,
      };
    },
    cancelConfirm() {
      this.confirmModal.show = false;
    },
    async onConfirmModal() {
      const { action, itemId } = this.confirmModal;
      this.confirmModal.show = false;
      if (action === 'purge') {
        await this.purgeOne(itemId);
      } else if (action === 'clear') {
        await this.clearAllConfirmed();
      }
    },
    async purgeOne(id) {
      try {
        await purgeItem(this.tableID, id);
        useDeletionsStore().notify({ prefix: 'Запись удалена безвозвратно' });
        await this.reload();
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось удалить', type: 'error' });
      }
    },
    async clearAllConfirmed() {
      try {
        const result = await clearTrash(this.tableID);
        useDeletionsStore().notify({ prefix: `Корзина очищена: ${(result && result.purged) || 0} запис(ей)` });
        await this.reload();
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось очистить', type: 'error' });
      }
    },
    openHistory() {
      this.showHistory = true;
    },
    closeHistory() {
      this.showHistory = false;
    },
    async onExport() {
      if (!this.items.length) return;
      this.isExporting = true;
      try {
        const isCars = this.tableType === 'cars';
        const headers = isCars
          ? ['Номер заявки', 'Дата и время удаления', 'Номер Т/С', 'Марка', 'Организация', 'Действует до', 'Время', 'Статус', 'Кто удалил']
          : ['Номер заявки', 'Дата и время удаления', 'Фамилия', 'Имя', 'Отчество', 'Организация', 'Действует до', 'Время', 'Статус', 'Кто удалил'];

        const status = isCars ? 'Удалена' : 'Удалён';
        const dataRows = this.sortedItems.map((item) => (isCars
          ? [item.application_number || '', this.formatDateTime(item.deleted_at), item.car_number || '', item.mark_name || '', item.organization || '', this.formatDate(item.entry_date_to), this.formatTimeRange(item), status, item.deleted_by_name || '']
          : [item.application_number || '', this.formatDateTime(item.deleted_at), item.last_name || '', item.first_name || '', item.middle_name || '', item.organization || '', this.formatDate(item.entry_date_to), this.formatTimeRange(item), status, item.deleted_by_name || '']));

        const workbook = new ExcelJS.Workbook();
        const worksheet = workbook.addWorksheet('Korzina');

        const headerRow = worksheet.addRow(headers);
        headerRow.height = 25;
        headerRow.eachCell((cell) => {
          cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF4F5BDF' } };
          cell.font = { name: 'Verdana', size: 11, bold: true, color: { argb: 'FFFFFFFF' } };
          cell.alignment = { vertical: 'middle', horizontal: 'center' };
        });

        dataRows.forEach((row, index) => {
          const r = worksheet.addRow(row);
          r.height = 20;
          const fillColor = index % 2 === 0 ? 'FFF0F5FF' : 'FFE0E9FF';
          r.eachCell((cell) => {
            cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: fillColor } };
            cell.font = { name: 'Verdana', size: 9, color: { argb: 'FF333333' } };
            cell.alignment = { vertical: 'middle' };
          });
        });

        // Авто-ширина по содержимому (через getColumn - применяется после addRow).
        headers.forEach((h, i) => {
          const maxLen = Math.max(h.length, ...dataRows.map(row => String(row[i] ?? '').length));
          worksheet.getColumn(i + 1).width = Math.min(Math.max(maxLen + 6, 16), 80);
        });
        // Первый столбец должен вмещать подписи футера.
        worksheet.getColumn(1).width = Math.max(worksheet.getColumn(1).width || 0, 22);

        worksheet.addRow([]);
        const infoRow1 = worksheet.addRow(['Отчёт сформировал:', this.currentUserDisplayName]);
        const infoRow2 = worksheet.addRow(['Дата формирования:', this.formattedExportDateTime]);
        [infoRow1, infoRow2].forEach((row) => {
          row.eachCell((cell) => {
            cell.font = { name: 'Verdana', size: 10, color: { argb: 'FF333333' } };
          });
        });

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
        useDeletionsStore().notify({ prefix: 'Ошибка при экспорте', type: 'error' });
      } finally {
        this.isExporting = false;
      }
    },
  },
};
</script>

<style scoped>
.trash-view {
  padding: 20px;
  font-family: 'Montserrat', sans-serif;
  position: relative;
}

.trash-header {
  display: flex;
  flex-direction: column;
  gap: 15px;
  margin-bottom: 15px;
}

.trash-titlebar {
  display: flex;
  align-items: center;
  gap: 10px;
}

.trash-title {
  margin: 0;
  font-family: 'Montserrat', sans-serif;
  font-weight: 700;
  font-size: 18px;
  line-height: 22px;
}

.trash-title__link {
  text-decoration: none;
}

.trash-title__prefix,
.trash-title__name {
  color: var(--text-muted);
  transition: color 0.2s ease;
}

.trash-title__prefix {
  margin-right: 0.35em;
}

.trash-title__sep {
  margin-left: 0.35em;
}

.trash-title__link:hover .trash-title__prefix {
  color: var(--text);
}

.trash-title__link:hover .trash-title__name {
  color: var(--accent-text);
}

.trash-title__sep {
  color: var(--text);
}

.trash-back-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 25px;
  padding: 0 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 50px;
  font-family: 'Montserrat', sans-serif;
  font-weight: 500;
  font-size: 14px;
  color: var(--accent-text);
  text-decoration: none;
  white-space: nowrap;
  transition: all 0.2s ease;
}

.trash-back-btn:hover {
  background: var(--surface-2);
  border-color: var(--accent);
}

.trash-back-btn__icon {
  color: var(--accent-text);
  width: 14px;
  height: 14px;
}

.trash-filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 15px;
  border-bottom: 1px solid var(--border);
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

/* Ширина ряда живёт на обёртке: у .base-dropdown своей ширины нет, и без неё кнопка
   дёргалась бы при смене подписи "Все организации" -> "Организация: 2". */
.trash-filters__control {
  width: 200px;
  max-width: 100%;
}

/* Кнопка дропдауна под контракт ряда: те же 35px, радиус 15px и отступы, что у поиска
   и календаря рядом. :deep обязателен - кнопка живёт внутри дочернего компонента и хэша
   этого файла не несёт; min-height:0 гасит собственные 30px BaseDropdown. */
.trash-filters__control :deep(.base-dropdown__button) {
  height: 35px;
  min-height: 0;
  border-radius: 15px;
  padding: 0 10px;
}

.trash-tool-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  height: 25px;
  padding: 0 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 10px;
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.2s ease;
}

.trash-tool-btn:hover:not(:disabled) {
  background: var(--surface-2);
}

.trash-tool-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.trash-tool-btn__icon {
  width: 15px;
  height: 15px;
}

.trash-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 30px;
  overflow: hidden;
  max-height: 600px;
  display: flex;
  flex-direction: column;
  margin-top: 15px;
}

.trash-card__header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 20px;
  height: 50px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.trash-card__title {
  margin: 0;
  font-family: 'Montserrat', sans-serif;
  font-weight: 600;
  font-size: 16px;
  color: var(--text);
}

.trash-history-btn {
  padding: 4px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 15px;
  font-family: 'Montserrat', sans-serif;
  font-size: 12px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
}

.trash-history-btn:hover {
  background: var(--surface-2);
  border-color: var(--accent);
}

.trash-card__spacer {
  flex: 1;
}

.trash-card__selected {
  font-size: 13px;
  color: var(--text-muted);
}

.trash-restore-btn {
  height: 25px;
  padding: 0 18px;
  background: var(--accent);
  border: 1px solid var(--accent);
  color: var(--accent-contrast);
  border-radius: 50px;
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s ease;
}

.trash-restore-btn:hover:not(:disabled) {
  background: var(--accent-hover);
}

.trash-restore-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.trash-card__body {
  padding: 0;
  flex-grow: 1;
  overflow-y: auto;
  position: relative;
}

.trash-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in srgb, var(--surface) 60%, transparent);
  z-index: 2;
}

.trash-state {
  padding: 60px 24px;
  text-align: center;
  font-family: 'Montserrat', sans-serif;
  color: var(--text-muted);
  font-size: 14px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.trash-state--error {
  color: var(--danger-text);
}

.trash-spinner {
  width: 28px;
  height: 28px;
  border: 3px solid var(--border);
  border-top-color: var(--accent-text);
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
  color: var(--text-muted);
  white-space: nowrap;
  user-select: none;
}

.trash-table__th.trash-table__th-check,
.trash-table__td.trash-table__td-check {
  width: 44px;
  padding: 0 0 0 20px;
  text-align: left;
  vertical-align: middle;
}

.trash-table__th--sortable {
  cursor: pointer;
}

.trash-table__th--sortable:hover {
  color: var(--text);
}

.trash-table__th--sortable:hover .trash-table__sort {
  color: var(--text);
}

.trash-table__th--active {
  color: var(--text);
}

.trash-table__sort {
  color: var(--text-muted);
  width: 12px;
  height: 12px;
  margin-left: 6px;
  vertical-align: middle;
  opacity: 0.7;
  transition: transform 0.2s ease;
}

.trash-table__sort--sorted {
  color: var(--text);
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
  background: var(--accent-tint);
}

.trash-table__td {
  padding: 12px;
  font-weight: 400;
  font-size: 14px;
  line-height: 17px;
  color: var(--text);
  border-top: 1px solid var(--border);
}

.trash-table__td--muted {
  color: var(--text-muted);
}

.trash-table__td--actions {
  text-align: right;
  padding-right: 20px;
}

.trash-badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 12px;
  border-radius: 50px;
  font-size: 11px;
  font-weight: 500;
  white-space: nowrap;
  border: 1px solid;
}

.trash-badge--deleted {
  background-color: var(--danger-bg);
  color: var(--danger-text);
  border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.trash-check {
  width: 16px;
  height: 16px;
  cursor: pointer;
  accent-color: var(--accent-text);
  display: block;
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
  background: var(--border);
}

.trash-icon-btn__icon {
  width: 20px;
  height: 20px;
}

@media (max-width: 1100px) {
  .trash-filters {
    flex-direction: column;
    align-items: stretch;
  }
  /* При переносе кнопок (Восстановить/История/Обновить) на вторую строку
     фиксированная height:50px обрезала бы шапку - вторая строка налезала на
     список. Высота по контенту + min 50px, чтобы тело начиналось ниже. */
  .trash-card__header {
    flex-wrap: wrap;
    height: auto;
    min-height: 50px;
    padding: 10px 20px;
    row-gap: 8px;
  }
}

/* На <768 реальная <table> конвертируется в карточки (rt-* из
   responsive-tables.css: thead скрыт, каждый tr -> flex-карточка с data-label
   подписями). Брейкпоинт 767.98 = как в responsive-tables.css, иначе на ровно
   768px card-конверсия не включится, а тач-правки да -> гибрид (урок #1097 S8). */
@media (max-width: 767.98px) {
  .trash-view {
    padding: 12px;
  }

  .trash-title {
    font-size: 16px;
  }

  /* table-layout:auto меряет ширину <table> по самому длинному неразрывному
     токену даже когда tr/td переопределены во flex (rt-row/[data-label]) - без
     fixed карточка распирается шире вьюпорта (эталон AccessDenialsLog). */
  .trash-table {
    table-layout: fixed;
  }

  /* Длинные значения без пробелов (номер Т/С, организация) переносятся внутри
     карточки, а не уходят в скрытый горизонт-скролл обёртки. */
  .trash-table .rt-row > [data-label] {
    overflow-wrap: anywhere;
  }

  /* Десктопный разделитель строк (border-top на ячейке) в card-режиме дал бы
     ВТОРУЮ линию поверх dashed border-bottom подписей (rt-инфра) - гасим, чтобы
     между полями была ровно одна пунктирная линия, а над чекбоксом/под Статусом
     не висел лишний разделитель. */
  .trash-table__td {
    border-top: none;
  }

  /* Ячейки без data-label (чекбокс выбора, кнопка безвозвратного удаления)
     идут своими строками карточки - тач-таргет 44px. */
  .trash-table__td-check,
  .trash-table__td--actions {
    min-height: 44px;
    display: flex;
    align-items: center;
  }

  .trash-table__td--actions {
    justify-content: flex-end;
    padding: 6px 0 0;
  }

  /* Последнее подписанное поле (Статус) стоит перед ячейкой действий без
     подписи - его dashed-разделитель повис бы; убираем. */
  .trash-table .rt-row > [data-label]:nth-last-child(2) {
    border-bottom: none;
  }

  /* Фильтры на всю ширину телефона (в столбце они иначе 200px слева). */
  .trash-filters__control {
    width: 100%;
  }
}
</style>
