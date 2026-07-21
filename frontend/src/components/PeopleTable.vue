<template>
  <div
    class="selected-table-card"
    :class="[
      { 'enlarged': enlarged, 'grid-mode': grid, 'is-portrait': isCompact, 'config-not-ready': !configReady },
      `density-${rowDensity}`,
    ]"
    :style="{ '--table-font-size': tableFontSize + 'px' }"
    data-testid="people-table"
  >
    <div class="card-header">
      <div class="card-header__title">
        <h3 class="card-title">
          <span class="blue">Люди</span> по заявке
        </h3>
      </div>
      <div
        v-if="$slots['header-actions']"
        class="card-header__actions"
      >
        <slot name="header-actions" />
      </div>
      <div
        v-if="!preview"
        class="card-header__settings"
      >
        <span class="items-count">
          Людей зашло: {{ peopleOnTerritory }}
          <button
            v-if="can(`table.${tableName}.history`)"
            class="history-btn"
            @click="openEmployeesHistory"
          >История</button>
        </span>
        <SwitchToggle
          v-model="enlarged"
          data-testid="enlarged-toggle"
        />
        <SwitchToggle
          class="grid-toggle"
          :model-value="grid"
          label="Сетка"
          title="Показать границы ячеек таблицы"
          data-testid="grid-toggle"
          @update:model-value="$emit('update:grid', $event)"
        />
        <RefreshButton
          :loading="refreshing"
          @refresh="loadData"
        />
      </div>
    </div>

    <!-- Панель групповых операций (#1194) - оверлей поверх .card-header (не
         reflow, урок #510), появляется при ctrl/shift-выделении строк. -->
    <div
      v-if="!preview && selectedCount > 0"
      class="bulk-bar"
      data-testid="people-bulk-bar"
    >
      <span class="bulk-count">Выбрано: {{ selectedCount }}</span>
      <div class="bulk-actions">
        <!-- Экспорт выбранных (#1194 S6) - клиентский ExcelJS по уже загруженным
             строкам, read-only. Гейт - то же право, что у полного экспорта
             (table.<name>.export), НЕ page.admin: доступно любому, кто видит таблицу. -->
        <button
          v-if="can(`table.${tableName}.export`)"
          class="lk-button lk-button--secondary lk-button--sm"
          data-testid="people-bulk-export"
          @click="exportSelectedToExcel"
        >
          Экспорт выбранных
        </button>
        <!-- Перенос/добавление - BE гейтит requireAdmin (page.admin), FE-кнопки
             тем же правом (см. api/system-tables.js cleanupTableSnapshots), иначе
             "видно, но 403" при клике не-админом. -->
        <template v-if="can('page.admin')">
          <button
            class="lk-button lk-button--secondary lk-button--sm"
            data-testid="people-bulk-move"
            @click="openBulkModal('move')"
          >
            Перенести
          </button>
          <button
            class="lk-button lk-button--secondary lk-button--sm"
            data-testid="people-bulk-add"
            @click="openBulkModal('add')"
          >
            Добавить в таблицу
          </button>
          <button
            class="lk-button lk-button--danger lk-button--sm"
            data-testid="people-bulk-remove"
            @click="openBulkRemoveConfirm"
          >
            Убрать
          </button>
        </template>
        <button
          class="lk-button lk-button--ghost lk-button--sm bulk-clear"
          data-testid="people-bulk-clear"
          @click="clearSelection"
        >
          Снять выбор
        </button>
      </div>
    </div>

    <div class="card-content rt-table">
      <div class="items-header">
        <div class="header-row rt-head-row">
          <div
            class="col entry-col"
            style="order: 0;"
          >
            Вход
          </div>
          <div
            class="col exit-col"
            style="order: 1;"
          >
            Выход
          </div>
          <div
            v-if="isFieldInDom('last_name')"
            class="col last-name-col"
            :class="fieldColClass('last_name')"
            :style="getColStyle('last_name', true)"
            @click="sortBy('last_name')"
          >
            <p :class="{ 'active-sort': sortField === 'last_name' }">
              Фамилия
            </p>
            <img
              src="@/assets/icons/sort.png"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'last_name', 'desc': sortField === 'last_name' && sortDirection === 'desc' }"
            >
          </div>
          <div
            v-if="isFieldInDom('first_name')"
            class="col first-name-col"
            :class="fieldColClass('first_name')"
            :style="getColStyle('first_name', true)"
            @click="sortBy('first_name')"
          >
            <p :class="{ 'active-sort': sortField === 'first_name' }">
              Имя
            </p>
            <img
              src="@/assets/icons/sort.png"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'first_name', 'desc': sortField === 'first_name' && sortDirection === 'desc' }"
            >
          </div>
          <div
            v-if="isFieldInDom('middle_name')"
            class="col middle-name-col"
            :class="fieldColClass('middle_name')"
            :style="getColStyle('middle_name', true)"
            @click="sortBy('middle_name')"
          >
            <p :class="{ 'active-sort': sortField === 'middle_name' }">
              Отчество
            </p>
            <img
              src="@/assets/icons/sort.png"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'middle_name', 'desc': sortField === 'middle_name' && sortDirection === 'desc' }"
            >
          </div>
          <div
            v-if="isFieldInDom('position')"
            class="col position-col"
            :class="fieldColClass('position')"
            :style="getColStyle('position', true)"
            @click="sortBy('position')"
          >
            <p :class="{ 'active-sort': sortField === 'position' }">
              Должность
            </p>
            <img
              src="@/assets/icons/sort.png"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'position', 'desc': sortField === 'position' && sortDirection === 'desc' }"
            >
          </div>
          <div
            v-if="isFieldInDom('citizenship_name')"
            class="col citizenship-col"
            :class="fieldColClass('citizenship_name')"
            :style="getColStyle('citizenship_name', true)"
            @click="sortBy('citizenship_name')"
          >
            <p :class="{ 'active-sort': sortField === 'citizenship_name' }">
              Гражданство
            </p>
            <img
              src="@/assets/icons/sort.png"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'citizenship_name', 'desc': sortField === 'citizenship_name' && sortDirection === 'desc' }"
            >
          </div>
          <div
            v-if="isFieldInDom('organization')"
            class="col organization-col"
            :class="fieldColClass('organization')"
            :style="getColStyle('organization', true)"
            @click="sortBy('organization')"
          >
            <p :class="{ 'active-sort': sortField === 'organization' }">
              Организация
            </p>
            <img
              src="@/assets/icons/sort.png"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'organization', 'desc': sortField === 'organization' && sortDirection === 'desc' }"
            >
          </div>
          <div
            v-if="isFieldInDom('company')"
            class="col company-col"
            :class="fieldColClass('company')"
            :style="getColStyle('company', true)"
            @click="sortBy('company')"
          >
            <p :class="{ 'active-sort': sortField === 'company' }">
              Компания
            </p>
            <img
              src="@/assets/icons/sort.png"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'company', 'desc': sortField === 'company' && sortDirection === 'desc' }"
            >
          </div>
          <div
            v-if="isFieldInDom('valid_until')"
            class="col date-col"
            :class="fieldColClass('valid_until')"
            :style="getColStyle('valid_until', true)"
            @click="sortBy('entry_date_to')"
          >
            <p :class="{ 'active-sort': sortField === 'entry_date_to' }">
              Действует до
            </p>
            <img
              src="@/assets/icons/sort.png"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'entry_date_to', 'desc': sortField === 'entry_date_to' && sortDirection === 'desc' }"
            >
          </div>
          <div
            v-if="isFieldInDom('pass_time')"
            class="col time-col"
            :class="fieldColClass('pass_time')"
            :style="getColStyle('pass_time', true)"
            @click="sortBy('pass_time')"
          >
            <p :class="{ 'active-sort': sortField === 'pass_time' }">
              Время прохода
            </p>
            <img
              src="@/assets/icons/sort.png"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'pass_time', 'desc': sortField === 'pass_time' && sortDirection === 'desc' }"
            >
          </div>
          <div
            v-if="isFieldInDom('application_id')"
            class="col application-col"
            :class="fieldColClass('application_id')"
            :style="getColStyle('application_id', true)"
            @click="sortBy('application_id')"
          >
            <p :class="{ 'active-sort': sortField === 'application_id' }">
              Номер заявки
            </p>
            <img
              src="@/assets/icons/sort.png"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'application_id', 'desc': sortField === 'application_id' && sortDirection === 'desc' }"
            >
          </div>
          <!-- Пустой spacer-заголовок над chevron-кнопкой "Подробнее" в строке. -->
          <div
            v-if="isCompact && hiddenInPortraitFields().length"
            class="col expand-col"
            style="order: 9997;"
          />
          <div
            class="col status-col"
            style="order: 9998;"
            @click="sortBy('status')"
          >
            <p :class="{ 'active-sort': sortField === 'status' }">
              Статус
            </p>
            <img
              src="@/assets/icons/sort.png"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'status', 'desc': sortField === 'status' && sortDirection === 'desc' }"
            >
          </div>
          <div
            class="col actions-col"
            style="order: 9999;"
          />
        </div>
      </div>
      
      <div class="items-container">
        <div
          v-if="isLoading"
          class="loading-message"
        >
          <LoaderSpinner label="Загрузка сотрудников…" />
        </div>

        <div
          v-if="refreshing && !isLoading"
          class="refresh-overlay"
        >
          <LoaderSpinner label="Обновление…" />
        </div>

        <div
          v-if="!isLoading && displayItems.length > 0"
          class="items-body"
          :class="{ 'is-drag-selecting': isDragging }"
        >
          <transition-group
            name="fade-list"
            tag="div"
          >
            <div
              v-for="(item, index) in displayItems"
              :key="item.id"
              class="item-row"
              :class="{ 'item-row--expanded': expandedRows[item.id], 'item-row--selected': isSelected(item.id) }"
              :style="{ animationDelay: `${index * 0.05}s` }"
              @click="preview ? null : onRowClick($event, item)"
              @mousedown="preview ? null : onRowMouseDown($event, item)"
              @mouseenter="preview ? null : dragOver(item.id)"
            >
              <div class="item-data rt-row">
                <div
                  class="col entry-col"
                  style="order: 0;"
                  data-label="Вход"
                  @click.stop
                >
                  <button
                    class="action-btn entry-btn"
                    :class="{ 'active': item.entry_checked }"
                    :disabled="preview || item.entry_checked"
                    @click="preview ? null : handleEntryExit(item, 'entry')"
                  >
                    Вход
                  </button>
                </div>
                <div
                  class="col exit-col"
                  style="order: 1;"
                  data-label="Выход"
                  @click.stop
                >
                  <button
                    class="action-btn exit-btn"
                    :class="{ 'active': item.exit_checked }"
                    :disabled="preview || !item.entry_checked || item.exit_checked"
                    @click="preview ? null : handleEntryExit(item, 'exit')"
                  >
                    Выход
                  </button>
                </div>
                <div
                  v-if="isFieldInDom('last_name')"
                  class="col last-name-col"
                  :class="fieldColClass('last_name')"
                  :style="getColStyle('last_name')"
                  data-label="Фамилия"
                >
                  {{ item.last_name }}
                </div>
                <div
                  v-if="isFieldInDom('first_name')"
                  class="col first-name-col"
                  :class="fieldColClass('first_name')"
                  :style="getColStyle('first_name')"
                  data-label="Имя"
                >
                  {{ item.first_name }}
                </div>
                <div
                  v-if="isFieldInDom('middle_name')"
                  class="col middle-name-col"
                  :class="fieldColClass('middle_name')"
                  :style="getColStyle('middle_name')"
                  data-label="Отчество"
                >
                  {{ item.middle_name || '-' }}
                </div>
                <div
                  v-if="isFieldInDom('position')"
                  class="col position-col"
                  :class="fieldColClass('position')"
                  :style="getColStyle('position')"
                  data-label="Должность"
                >
                  {{ item.position || '-' }}
                </div>
                <div
                  v-if="isFieldInDom('citizenship_name')"
                  class="col citizenship-col"
                  :class="fieldColClass('citizenship_name')"
                  :style="getColStyle('citizenship_name')"
                  data-label="Гражданство"
                >
                  {{ item.citizenshipName || '-' }}
                </div>
                <div
                  v-if="isFieldInDom('organization')"
                  class="col organization-col"
                  :class="fieldColClass('organization')"
                  :style="getColStyle('organization')"
                  data-label="Организация"
                >
                  {{ item.organization_name }}
                </div>
                <div
                  v-if="isFieldInDom('company')"
                  class="col company-col"
                  :class="fieldColClass('company')"
                  :style="getColStyle('company')"
                  data-label="Компания"
                >
                  {{ item.company || '-' }}
                </div>
                <div
                  v-if="isFieldInDom('valid_until')"
                  class="col date-col"
                  :class="fieldColClass('valid_until')"
                  :style="getColStyle('valid_until')"
                  data-label="Действует до"
                >
                  {{ formatDate(item.entry_date_to) }}
                </div>
                <div
                  v-if="isFieldInDom('pass_time')"
                  class="col time-col"
                  :class="fieldColClass('pass_time')"
                  :style="getColStyle('pass_time')"
                  data-label="Время прохода"
                >
                  {{ formatPassTime(item.pass_time) }}
                </div>
                <div
                  v-if="isFieldInDom('application_id')"
                  class="col application-col"
                  :class="fieldColClass('application_id')"
                  :style="getColStyle('application_id')"
                  data-label="Номер заявки"
                >
                  <span
                    v-if="isManualItem(item)"
                    class="manual-badge"
                  >Добавлено вручную</span>
                  <template v-else>
                    {{ item.applicationNumber || '-' }}
                  </template>
                </div>
                <div
                  class="col status-col"
                  style="order: 9998;"
                  data-label="Статус"
                >
                  <StatusBadge :status="item.status" />
                </div>
                <div
                  v-if="isCompact && hiddenInPortraitFields().length"
                  class="col expand-col"
                  style="order: 9997;"
                  @click.stop
                >
                  <button
                    type="button"
                    class="expand-btn"
                    :class="{ 'expand-btn--open': expandedRows[item.id] }"
                    :aria-expanded="!!expandedRows[item.id]"
                    :aria-label="expandedRows[item.id] ? 'Скрыть' : 'Подробнее'"
                    @click="toggleRowExpand(item.id)"
                  >
                    <svg
                      width="14"
                      height="14"
                      viewBox="0 0 14 14"
                      fill="none"
                    >
                      <path
                        d="M3.5 5L7 8.5L10.5 5"
                        stroke="currentColor"
                        stroke-width="1.5"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                    </svg>
                  </button>
                </div>
                <div
                  v-if="can('entity.employees.delete')"
                  class="col actions-col"
                  style="order: 9999;"
                  @click.stop
                >
                  <!-- Сотрудник привязан к нескольким таблицам (#1194 S5) -
                       выбор между "убрать только отсюда" и глобальной
                       деактивацией. Единственная привязка ИЛИ не-админ - как
                       раньше (unbind-table гейтится page.admin на бэке). -->
                  <TableRowRemoveMenu
                    v-if="(item.target_tables_count || 0) > 1 && can('page.admin')"
                    :disabled="preview || isLoading"
                    @remove-current="removeFromCurrentTableWithNotification(item)"
                    @remove-all="removeItemWithNotification(item)"
                  />
                  <button
                    v-else
                    class="delete-btn"
                    :disabled="preview || isLoading"
                    @click="preview ? null : removeItemWithNotification(item)"
                  >
                    <img
                      src="@/assets/icons/trashcan.png"
                      alt="Удалить"
                      class="delete-icon"
                    >
                  </button>
                </div>
              </div>
              <div
                v-if="isCompact && expandedRows[item.id]"
                class="item-row__details"
                @click.stop
              >
                <div
                  v-for="name in hiddenInPortraitFields()"
                  :key="name"
                  class="detail-item"
                >
                  <span class="detail-item__label">{{ portraitFieldLabel(name) }}</span>
                  <span class="detail-item__value">{{ portraitFieldValue(item, name) }}</span>
                </div>
              </div>
            </div>
          </transition-group>
        </div>

        <div
          v-else-if="!isLoading"
          class="no-data-message"
        >
          {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Нет активных сотрудников' }}
        </div>
      </div>
    </div>

    <!-- Модальное окно деталей сотрудника -->
    <EmployeeDetailsModal
      v-if="!preview && showDetailsModal"
      :show="showDetailsModal"
      :employee="selectedEmployee"
      :all-tables="allTables"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      :source="'peopletable'"
      @close="closeDetailsModal"
      @open-application="openApplicationDetail"
    />

    <EmployeesTableHistoryModal
      v-if="!preview && showEmployeesHistory"
      :table-id="currentTableId"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      @close="showEmployeesHistory = false"
    />

    <!-- Групповые операции "Перенести"/"Добавить в таблицу" (#1194) -->
    <TableBulkTargetModal
      v-if="!preview"
      :show="bulkModalVisible"
      :mode="bulkModalMode"
      entity-type="people"
      :exclude-table-id="currentTableId"
      :selected-count="selectedCount"
      :submitting="bulkSubmitting"
      @close="closeBulkModal"
      @apply="applyBulkTableOp"
    />

    <!-- Групповое "Убрать" (#1194 S5): снимает привязку к текущей таблице у
         выделенных строк; последняя привязка -> BE деактивирует сотрудника сам. -->
    <ConfirmationModal
      v-if="!preview"
      :show="bulkRemoveConfirmVisible"
      title="Убрать из таблицы"
      :message="`Убрать выбранных сотрудников (${selectedCount}) из этой таблицы? Если это последняя таблица сотрудника, он будет деактивирован.`"
      confirm-text="Убрать"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
      @confirm="confirmBulkRemove"
      @cancel="cancelBulkRemove"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';
import { useOrientation } from '@/composables/useOrientation';
import { useRowSelection } from '@/composables/useRowSelection';
import eventStream from '@/services/eventStream';
import RefreshButton from './RefreshButton.vue';
import EmployeeDetailsModal from './CreateApplication/EmployeeDetailsModal.vue';
import EmployeesTableHistoryModal from './CreateApplication/EmployeesTableHistoryModal.vue';
import TableBulkTargetModal from './TableBulkTargetModal.vue';
import TableRowRemoveMenu from './TableRowRemoveMenu.vue';
import ConfirmationModal from './ConfirmationModal.vue';
import StatusBadge from './ui/StatusBadge.vue';
import SwitchToggle from './ui/SwitchToggle.vue';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import ExcelJS from 'exceljs';
import { bulkMoveEmployeesTable, bulkAddEmployeesTable, bulkUnbindEmployeesTable } from '@/api/employees';

const ENLARGED_KEY_PREFIX = 'enlarged-mode:people:';

export default {
  name: 'PeopleTable',
  components: {
    RefreshButton,
    EmployeeDetailsModal,
    EmployeesTableHistoryModal,
    TableBulkTargetModal,
    TableRowRemoveMenu,
    ConfirmationModal,
    StatusBadge,
    SwitchToggle,
    LoaderSpinner
  },
  setup() {
    const { isPortrait, isCompact } = useOrientation();
    const permissionsStore = usePermissionsStore();
    const rowSelection = useRowSelection();
    return { isPortrait, isCompact, permissionsStore, ...rowSelection };
  },
  props: {
    tableName: {
      type: String,
      default: ''
    },
    searchQuery: {
      type: String,
      default: ''
    },
    selectedOrganizationId: {
      type: [Number, String],
      default: null
    },
    selectedCompanyId: {
      type: [Number, String],
      default: null
    },
    selectedUnloadingPlace: {
      type: String,
      default: ''
    },
    dateRangeStart: {
      type: Date,
      default: null
    },
    dateRangeEnd: {
      type: Date,
      default: null
    },
    selectedDate: {
      type: Date,
      default: null
    },
    currentUserId: {
      type: Number,
      default: null
    },
    currentUserName: {
      type: String,
      default: ''
    },
    // Preview-режим для админ-вкладки "Колонки" (#345): seed-данные, без API и без кнопок.
    preview: { type: Boolean, default: false },
    previewFields: { type: Array, default: null },
    previewItems: { type: Array, default: null },
    // Режим "Сетка" (#1289): границы ячеек. Управляется одним тумблером страницы.
    grid: { type: Boolean, default: false }
  },
  emits: ['open-application', 'update:grid'],
  data() {
    return {
      sortField: null,
      sortDirection: 'desc',
      itemsData: [],
      pendingDeleteIds: [],
      isLoading: false,
      refreshing: false,
      currentTableId: null,
      organizationsMap: {},
      allTables: [],
      showDetailsModal: false,
      selectedEmployee: null,
      showEmployeesHistory: false,
      pollingInterval: null,
      // Real-time (#840): подписка на tables:<tableId>, статус SSE-соединения и seq-токен
      // против гонки конкурентных silentRefresh (таймер + SSE-сигнал, урок #632/#840).
      eventStreamOff: null,
      eventStreamStatusOff: null,
      sseConnected: false,
      refreshSeq: 0,
      enlarged: false,
      fieldsVisibility: {},
      fieldOrders: {},
      fieldWidths: {},
      fieldPriorities: {},
      fieldsEnlargedVisibility: {},
      fieldsEnlargedWidth: {},
      fieldsEnlargedWeight: {},
      tableFontSize: 14,
      rowDensity: 'normal',
      expandedRows: {},
      compactPriorityThreshold: 2,
      // false до первой загрузки конфига - класс config-not-ready на корне
      // подавляет transitions, чтобы шапка/столбцы не "ездили" при init.
      configReady: false,
      // Групповые операции "Перенести"/"Добавить в таблицу" (#1194 S4).
      bulkModalVisible: false,
      bulkModalMode: 'move',
      bulkSubmitting: false,
      // Групповое "Убрать" (#1194 S5).
      bulkRemoveConfirmVisible: false,
      bulkRemoveSubmitting: false,
    };
  },
  computed: {
    displayItems() {
      let filtered = this.itemsData.filter(i => !this.pendingDeleteIds.includes(i.id));

      if (this.searchQuery) {
        const variants = buildSearchVariants(this.searchQuery);
        filtered = filtered.filter(item => {
          const searchFields = [
            item.last_name,
            item.first_name,
            item.middle_name || '',
            item.organization_name,
            this.formatDate(item.entry_date_to),
            item.pass_time || '',
            item.status
          ];
          return matchesSearch(searchFields.filter(Boolean).join(' '), variants);
        });
      }

      if (this.selectedOrganizationId) {
        filtered = filtered.filter(item => item.organization_id == this.selectedOrganizationId);
      }

      if (this.selectedCompanyId) {
        filtered = filtered.filter(item => item.company_id == this.selectedCompanyId);
      }

      if (this.selectedDate) {
        const selectedDateStr = this.selectedDate.toISOString().split('T')[0];
        filtered = filtered.filter(item => item.entry_date_to === selectedDateStr);
      } else if (this.dateRangeStart && this.dateRangeEnd) {
        filtered = filtered.filter(item => {
          const itemDate = new Date(item.entry_date_to);
          return itemDate >= this.dateRangeStart && itemDate <= this.dateRangeEnd;
        });
      }

      if (this.sortField) {
        filtered.sort((a, b) => {
          let valueA, valueB;
          
          switch (this.sortField) {
            case 'last_name':
            case 'first_name':
            case 'middle_name':
            case 'organization':
            case 'status':
            case 'pass_places':
              valueA = (a[this.sortField] || '').toString().toLowerCase();
              valueB = (b[this.sortField] || '').toString().toLowerCase();
              break;
            case 'citizenship':
              valueA = (a.citizenshipName || '').toString().toLowerCase();
              valueB = (b.citizenshipName || '').toString().toLowerCase();
              break;
            case 'entry_date_to':
              valueA = a.entry_date_to ? new Date(a.entry_date_to) : new Date(0);
              valueB = b.entry_date_to ? new Date(b.entry_date_to) : new Date(0);
              break;
            case 'pass_time':
              valueA = this.extractPassTime(a.pass_time);
              valueB = this.extractPassTime(b.pass_time);
              break;
            default:
              return 0;
          }
          
          if (valueA < valueB) {
            return this.sortDirection === 'asc' ? -1 : 1;
          }
          if (valueA > valueB) {
            return this.sortDirection === 'asc' ? 1 : -1;
          }
          return 0;
        });
      }

      return filtered;
    },

    peopleOnTerritory() {
      return this.itemsData.filter(item => item.entry_checked && !item.exit_checked).length;
    },

    hasActiveFilters() {
      return !!(
        this.searchQuery ||
        this.selectedOrganizationId ||
        this.selectedCompanyId ||
        this.selectedDate ||
        (this.dateRangeStart && this.dateRangeEnd)
      );
    }
  },
  watch: {
    tableName: {
      handler() {
        if (this.preview) return;
        this.stopPolling();
        this.startPolling();
        this.loadEnlargedFromStorage();
      },
      immediate: true
    },
    enlarged(value) {
      if (this.preview) return;
      this.saveEnlargedToStorage(value);
    },
    // Строки, ушедшие из видимого списка (фильтр/поиск/удаление/поллинг),
    // убираем из выделения - счётчик "Выбрано: N" не должен врать (#1194).
    displayItems(items) {
      this.pruneSelection(items.map(i => i.id));
    },
    previewItems: {
      immediate: true,
      handler(newVal) {
        if (this.preview && Array.isArray(newVal)) {
          this.itemsData = newVal;
        }
      }
    },
    previewFields: {
      immediate: true,
      handler(newVal) {
        if (!this.preview || !Array.isArray(newVal)) return;
        const nextVis = {};
        const nextOrd = {};
        const nextW = {};
        const nextP = {};
        newVal.forEach((f, i) => {
          nextVis[f.field_name] = f.is_visible !== false;
          nextOrd[f.field_name] = typeof f.display_order === 'number' ? f.display_order : i;
          if (typeof f.width === 'number' && f.width > 0) nextW[f.field_name] = f.width;
          if (typeof f.priority === 'number' && f.priority > 0) nextP[f.field_name] = f.priority;
        });
        this.fieldsVisibility = nextVis;
        this.fieldOrders = nextOrd;
        this.fieldWidths = nextW;
        this.fieldPriorities = nextP;
        this.markConfigReady();
      }
    },
    // Real-time (#840): tableId для PeopleTable известен только после первого fetchPeopleData
    // (резолвится по tableName), поэтому подписываемся/пересобираем по watch, а не в mounted.
    currentTableId(newVal) {
      if (this.preview) return;
      this.subscribeTableScope(newVal);
    }
  },
  mounted() {
    if (this.preview) return;
    this.startPolling();
    this.loadEnlargedFromStorage();
    // Подгружаем настроенные длительности уведомлений после авторизации
    // (на холодном старте App.vue запрос мог уйти до получения токена).
    useDeletionsStore().loadDurations();

    // Real-time (#840): по сигналу продюсера tables.refresh тихо перезагружаем строки
    // вместо ожидания поллинга. Сама подписка на scope - в watch currentTableId.
    eventStream.connect();
    this.eventStreamStatusOff = eventStream.onStatus((status) => {
      this.sseConnected = status === 'connected';
    });

    // Drag-select (#1227 P4): mouseup может произойти вне строки (курсор ушёл
    // за пределы таблицы) - слушаем на window, иначе drag "залипнет".
    this.onGlobalMouseUp = () => this.endDrag();
    window.addEventListener('mouseup', this.onGlobalMouseUp);
  },
  beforeUnmount() {
    this.stopPolling();
    if (this.onGlobalMouseUp) {
      window.removeEventListener('mouseup', this.onGlobalMouseUp);
      this.onGlobalMouseUp = null;
    }
    if (this.preview) return; // preview никогда не подключался - нечего отключать
    if (this.eventStreamOff) {
      this.eventStreamOff();
      this.eventStreamOff = null;
    }
    if (this.eventStreamStatusOff) {
      this.eventStreamStatusOff();
      this.eventStreamStatusOff = null;
    }
    eventStream.disconnect();
  },
  methods: {
    // Гейтинг по правам (#187 Фаза 2). super -> всегда true, admin -> всё кроме
    // denied, обычный -> по эффективному гранту. Реактивно: читает стор прав.
    can(key) {
      return this.permissionsStore.hasPermission(key);
    },
    // Real-time (#840): подписка на scope конкретной таблицы, пересобирается при смене tableId.
    subscribeTableScope(tableId) {
      if (this.eventStreamOff) {
        this.eventStreamOff();
        this.eventStreamOff = null;
      }
      if (!tableId) return;
      this.eventStreamOff = eventStream.subscribe(`tables:${tableId}`, () => {
        this.silentRefresh();
      });
    },
    // seq-токен против гонки конкурентных вызовов (поллинг + SSE-сигнал, #632/#840):
    // устаревший (медленно резолвнутый) ответ не должен затирать более свежие данные.
    async _loadData(silent = false) {
      const seq = ++this.refreshSeq;
      if (!silent && this.isLoading) return;
      if (!silent) this.isLoading = true;
      try {
        await this.fetchAllTables();
        await this.fetchOrganizations();
        await this.fetchPeopleData(seq);
        await this.fetchEmployeesStatus(seq);
      } catch (error) {
        console.error('Ошибка при загрузке людей:', error);
      } finally {
        if (!silent) this.isLoading = false;
      }
    },

    async loadData() {
      this.refreshing = true;
      try {
        await this._loadData(true);
      } finally {
        this.refreshing = false;
      }
    },

    async silentRefresh() {
      await this._loadData(true);
    },

    async fetchAllTables() {
      try {
        const response = await apiRequest("/system-tables", { method: "GET" });
        if (response.ok) {
          this.allTables = await response.json();
        }
      } catch (error) {
        console.error("Ошибка при загрузке таблиц:", error);
      }
    },

    async fetchOrganizations() {
      try {
        const response = await apiRequest("/organizations", { method: "GET" });
        if (response.ok) {
          const data = await response.json();
          this.organizationsMap = {};
          data.forEach(org => { this.organizationsMap[org.id] = org.name; });
        }
      } catch (error) {
        console.error("Ошибка при загрузке организаций:", error);
      }
    },

    async fetchPeopleData(seq) {
      try {
        if (!this.tableName) return;

        const tableRes = await apiRequest(`/system-tables/name/${this.tableName}`, { method: "GET" });
        if (!tableRes.ok) return;
        const responseData = await tableRes.json();
        const table = responseData.table;
        this.currentTableId = table.id;

        // Подтягиваем конфиг видимости, порядка, ширины и приоритета столбцов
        // из того же ответа. Стиль таблицы (font_size/row_density) - из table.
        const nextVisibility = {};
        const nextOrders = {};
        const nextWidths = {};
        const nextPriorities = {};
        const nextEnlVis = {};
        const nextEnlW = {};
        const nextEnlWeight = {};
        (responseData.fields || []).forEach(f => {
          nextVisibility[f.field_name] = f.is_visible !== false;
          if (typeof f.display_order === 'number') {
            nextOrders[f.field_name] = f.display_order;
          }
          if (typeof f.width === 'number' && f.width > 0) {
            nextWidths[f.field_name] = f.width;
          }
          if (typeof f.priority === 'number' && f.priority > 0) {
            nextPriorities[f.field_name] = f.priority;
          }
          nextEnlVis[f.field_name] = f.enlarged_is_visible !== false;
          if (typeof f.enlarged_width === 'number' && f.enlarged_width > 0) {
            nextEnlW[f.field_name] = f.enlarged_width;
          }
          if (typeof f.enlarged_font_weight === 'number' && f.enlarged_font_weight > 0) {
            nextEnlWeight[f.field_name] = f.enlarged_font_weight;
          }
        });
        this.fieldsVisibility = nextVisibility;
        this.fieldOrders = nextOrders;
        this.fieldWidths = nextWidths;
        this.fieldPriorities = nextPriorities;
        this.fieldsEnlargedVisibility = nextEnlVis;
        this.fieldsEnlargedWidth = nextEnlW;
        this.fieldsEnlargedWeight = nextEnlWeight;
        const fs = Number(table.font_size);
        if (fs >= 10 && fs <= 24) this.tableFontSize = fs;
        const dens = table.row_density;
        if (['compact', 'normal', 'spacious'].includes(dens)) this.rowDensity = dens;
        this.markConfigReady();

        const employeesRes = await apiRequest(`/employees/active-for-table/${table.id}`, { method: "GET" });
        if (!employeesRes.ok) return;
        const employees = await employeesRes.json();

        const nameToIdMap = {};
        Object.keys(this.organizationsMap).forEach(id => {
          nameToIdMap[this.organizationsMap[id]] = id;
        });

        const newItems = employees.map(emp => {
          const orgName = emp.organization || '';
          const orgId = nameToIdMap[orgName] || emp.organization_id;
          return {
            id: emp.id,
            last_name: emp.last_name || '',
            first_name: emp.first_name || '',
            middle_name: emp.middle_name || '',
            organization_id: orgId,
            organization_name: orgName || 'Не указана',
            entry_date_to: emp.entry_date_to || '',
            pass_time: emp.pass_time || '',
          pass_places: emp.pass_places || '',
            status: 'Активен',
            applicationId: emp.application_id,
            applicationNumber: emp.application_number,
            target_tables: emp.target_tables || [],
            passport_series_number: emp.passport_series_number,
            patent_number: emp.patent_number,
            other_permission: emp.other_permission,
            citizenshipName: emp.citizenship_name,
            position: emp.position,
            company: emp.company,
            company_id: emp.company_id,
            entry_checked: false,
            exit_checked: false,
            territory_status: 0,
            // Число таблиц «Проход», к которым привязан сотрудник (#1194 S5) -
            // >1 включает per-row подменю «Убрать из этой/из всех».
            target_tables_count: emp.target_tables_count || 0
          };
        });
        if (seq !== undefined && seq !== this.refreshSeq) return; // устарел - новее уже в работе/загружен
        this.itemsData = newItems;
      } catch (error) {
        console.error("Ошибка при загрузке сотрудников:", error);
        if (seq === undefined || seq === this.refreshSeq) this.itemsData = [];
      }
    },

    async fetchEmployeesStatus(seq) {
      try {
        const response = await apiRequest("/employees/history/current-status", { method: "GET" });
        if (response.ok) {
          const statuses = await response.json();
          if (seq !== undefined && seq !== this.refreshSeq) return; // устарел
          const statusMap = {};
          statuses.forEach(status => { statusMap[status.employee_id] = status; });
          this.itemsData.forEach(item => {
            const status = statusMap[item.id];
            if (status) {
              item.territory_status = status.territory_status;
              item.entry_checked = status.territory_status === 1;
              item.exit_checked = status.territory_status === 2;
            }
          });
        }
      } catch (error) {
        console.error("Ошибка при загрузке статусов территории:", error);
      }
    },

    formatDate(dateString) {
      if (!dateString) return '';
      try {
        const [year, month, day] = dateString.split('-');
        const date = new Date(year, month - 1, day);
        return date.toLocaleDateString('ru-RU');
      } catch {
        return '';
      }
    },

    formatPassTime(passTime) {
      if (!passTime) return '-';
      const [timeFrom, timeTo] = passTime.split('-');
      const formatTime = (timeStr) => {
        if (!timeStr) return '';
        const parts = timeStr.trim().split(':');
        if (parts.length >= 2) return `${parts[0]}:${parts[1]}`;
        return timeStr;
      };
      const formattedFrom = formatTime(timeFrom);
      const formattedTo = formatTime(timeTo);
      if (!formattedTo) return formattedFrom;
      if (!formattedFrom) return formattedTo;
      return `${formattedFrom} - ${formattedTo}`;
    },

    extractPassTime(passTime) {
      if (!passTime || passTime === '-') return 0;
      const startTime = passTime.split('-')[0];
      const parts = startTime.split(':');
      if (parts.length >= 2) {
        const hours = parseInt(parts[0]) || 0;
        const minutes = parseInt(parts[1]) || 0;
        return hours * 60 + minutes;
      }
      return 0;
    },

    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'desc';
      }
    },

    async handleEntryExit(item, type) {
      if (!this.currentUserId || !this.currentTableId) return;
      const territory_status = type === 'entry' ? 1 : 2;
      try {
        const response = await apiRequest(`/employees/${item.id}/territory-status`, {
          method: "PUT",
          body: JSON.stringify({
            territory_status,
            user_id: this.currentUserId,
            table_id: this.currentTableId
          })
        });
        if (response.ok) {
          item.entry_checked = type === 'entry';
          item.exit_checked = type === 'exit';
        } else {
          console.error('Ошибка при обновлении статуса');
        }
      } catch (error) {
        console.error('Ошибка сети:', error);
      }
    },

    removeItemWithNotification(item) {
      if (this.isLoading) return;
      if (this.pendingDeleteIds.includes(item.id)) return;
      const empId = item.id;
      const tableId = this.currentTableId;
      const userId = this.currentUserId;
      const fullName = [item.last_name, item.first_name, item.middle_name].filter(Boolean).join(' ') || String(item.last_name || '');
      this.pendingDeleteIds.push(empId);
      useDeletionsStore().enqueue({
        prefix: 'Сотрудник ',
        bold: fullName,
        suffix: ' удалён',
        onConfirm: () => this.commitDelete(empId, tableId, userId),
        onUndo: () => this.unhidePending(empId),
      });
    },

    unhidePending(empId) {
      this.pendingDeleteIds = this.pendingDeleteIds.filter(id => id !== empId);
    },

    async commitDelete(empId, tableId, userId) {
      try {
        const response = await apiRequest(`/employees/${empId}/deactivate`, {
          method: "PUT",
          body: JSON.stringify({ status: 0, user_id: userId, table_id: tableId })
        });
        if (!response.ok) {
          console.error("Ошибка при удалении");
          this.unhidePending(empId);
          return;
        }
        await this._loadData(true);
        this.unhidePending(empId);
      } catch (error) {
        console.error("Ошибка сети при удалении:", error);
        this.unhidePending(empId);
      }
    },

    // Убрать ТОЛЬКО из текущей таблицы (#1194 S5) - альтернатива глобальной
    // деактивации, доступная per-row через TableRowRemoveMenu, когда сотрудник
    // привязан к нескольким таблицам. Тот же enqueue/undo UX, что и обычное
    // удаление, коммит идёт через bulkUnbindEmployeesTable ([id], tableId).
    removeFromCurrentTableWithNotification(item) {
      if (this.isLoading) return;
      if (this.pendingDeleteIds.includes(item.id)) return;
      const empId = item.id;
      const tableId = this.currentTableId;
      const fullName = [item.last_name, item.first_name, item.middle_name].filter(Boolean).join(' ') || String(item.last_name || '');
      this.pendingDeleteIds.push(empId);
      useDeletionsStore().enqueue({
        prefix: 'Сотрудник ',
        bold: fullName,
        suffix: ' убран из таблицы',
        onConfirm: () => this.commitUnbindFromCurrentTable(empId, tableId),
        onUndo: () => this.unhidePending(empId),
      });
    },

    async commitUnbindFromCurrentTable(empId, tableId) {
      try {
        const result = await bulkUnbindEmployeesTable([empId], tableId);
        if (!result || typeof result.success_count !== 'number' || result.error_count > 0) {
          const message = result?.errors?.[0]?.error || result?.message || 'ошибка сервера';
          useDeletionsStore().notify({ prefix: 'Не удалось убрать сотрудника: ', bold: message, type: 'error' });
        }
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось убрать сотрудника: ', bold: 'ошибка сети', type: 'error' });
      } finally {
        await this._loadData(true);
        this.unhidePending(empId);
      }
    },

    // Ctrl/Shift-клик по строке (#1194) - групповое выделение вместо открытия
    // детали; обычный клик поведение не меняет (handleRowClick вернёт false).
    onRowClick(event, item) {
      const orderedIds = this.displayItems.map(i => i.id);
      if (this.handleRowClick(event, item.id, orderedIds)) return;
      this.openEmployeeDetails(item);
    },

    // Ctrl(Cmd)+зажать курсор и вести (#1227 P4) - строки под курсором ДОБАВЛЯЮТСЯ
    // к выделению (dragOver на @mouseenter соседних строк, endDrag по глобальному
    // mouseup в mounted). preventDefault только когда drag реально стартовал -
    // иначе обычный клик/выделение текста браузером не ломаются.
    onRowMouseDown(event, item) {
      if (this.startDrag(item.id, event)) event.preventDefault();
    },

    // Групповые операции над выделенными строками (#1194 S4): 'move' - перенести
    // (снять с текущей таблицы, привязать к выбранным), 'add' - добавить в выбранные,
    // не снимая с текущей.
    openBulkModal(mode) {
      this.bulkModalMode = mode;
      this.bulkModalVisible = true;
    },
    closeBulkModal() {
      if (this.bulkSubmitting) return;
      this.bulkModalVisible = false;
    },
    async applyBulkTableOp(toTableIds) {
      const ids = [...this.selectedIds];
      const mode = this.bulkModalMode;
      if (!ids.length || !toTableIds.length) return;
      this.bulkSubmitting = true;
      let result;
      try {
        result = mode === 'move'
          ? await bulkMoveEmployeesTable(ids, this.currentTableId, toTableIds)
          : await bulkAddEmployeesTable(ids, toTableIds);
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось выполнить групповую операцию', type: 'error' });
        this.bulkSubmitting = false;
        return;
      }
      this.bulkSubmitting = false;
      if (this.handleBulkTableOpResult(mode, result, ids.length)) {
        this.bulkModalVisible = false;
      }
    },
    // Разбор BulkOpResult: полный успех -> notify success, частичный -> notify
    // warning с перечнем непрошедших имён (образец MarksManagement.handleBulkResult).
    // false при структурной ошибке-envelope (модалка остаётся открытой для повтора).
    handleBulkTableOpResult(mode, result, total) {
      if (!result || typeof result.success_count !== 'number') {
        useDeletionsStore().notify({ prefix: result?.message || 'Не удалось выполнить групповую операцию', type: 'error' });
        return false;
      }
      const label = mode === 'move' ? 'Перенесено сотрудников: ' : 'Добавлено сотрудников: ';
      if (result.error_count > 0) {
        const failed = (result.errors || []).map(e => e.name || `#${e.id}`).join(', ');
        useDeletionsStore().notify({ prefix: 'Выполнено ', bold: `${result.success_count} из ${total}`, suffix: `. Не удалось: ${failed}`, type: 'warning' });
      } else {
        useDeletionsStore().notify({ prefix: label, bold: String(result.success_count) });
      }
      this.clearSelection();
      this._loadData(true);
      return true;
    },

    // Групповое "Убрать" (#1194 S5): снимает привязку выделенных сотрудников к
    // ТЕКУЩЕЙ таблице (bulkUnbindEmployeesTable). Последняя привязка -> BE сам
    // деактивирует сотрудника (status=0) - фронту достаточно показать результат.
    openBulkRemoveConfirm() {
      this.bulkRemoveConfirmVisible = true;
    },
    cancelBulkRemove() {
      if (this.bulkRemoveSubmitting) return;
      this.bulkRemoveConfirmVisible = false;
    },
    async confirmBulkRemove() {
      const ids = [...this.selectedIds];
      if (!ids.length || this.bulkRemoveSubmitting) return;
      this.bulkRemoveSubmitting = true;
      let result;
      try {
        result = await bulkUnbindEmployeesTable(ids, this.currentTableId);
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось выполнить групповую операцию', type: 'error' });
        this.bulkRemoveSubmitting = false;
        return;
      }
      this.bulkRemoveSubmitting = false;
      if (this.handleBulkRemoveResult(result, ids.length)) {
        this.bulkRemoveConfirmVisible = false;
      }
    },
    // Разбор BulkOpResult для "Убрать" - тот же формат, что move/add (см.
    // handleBulkTableOpResult), отдельный метод из-за другой метки успеха.
    handleBulkRemoveResult(result, total) {
      if (!result || typeof result.success_count !== 'number') {
        useDeletionsStore().notify({ prefix: result?.message || 'Не удалось выполнить групповую операцию', type: 'error' });
        return false;
      }
      if (result.error_count > 0) {
        const failed = (result.errors || []).map(e => e.name || `#${e.id}`).join(', ');
        useDeletionsStore().notify({ prefix: 'Убрано ', bold: `${result.success_count} из ${total}`, suffix: `. Не удалось: ${failed}`, type: 'warning' });
      } else {
        useDeletionsStore().notify({ prefix: 'Убрано сотрудников: ', bold: String(result.success_count) });
      }
      this.clearSelection();
      this._loadData(true);
      return true;
    },

    openEmployeeDetails(item) {
      this.selectedEmployee = {
        id: item.id,
        last_name: item.last_name,
        first_name: item.first_name,
        middle_name: item.middle_name,
        position: item.position,
        citizenshipName: item.citizenshipName,
        passport_series_number: item.passport_series_number,
        patent_number: item.patent_number,
        other_permission: item.other_permission,
        organization: item.organization_name,
        organizationId: item.organization_id,
        company: item.company,
        companyId: item.company_id,
        entry_date_to: item.entry_date_to,
        pass_time: item.pass_time,
        target_tables: item.target_tables || [],
        territory_status: item.territory_status,
        applicationId: item.applicationId
      };
      this.showDetailsModal = true;
    },

    closeDetailsModal() {
      this.showDetailsModal = false;
      this.selectedEmployee = null;
    },

    // Сотрудник добавлен вручную без заявки (#1049): application_id === null (BE отдаёт
    // NULL для вложения-сироты). Строгий null - у обычных строк applicationId - число.
    isManualItem(item) {
      return item.applicationId === null;
    },

    openApplicationDetail(applicationId) {
      // Убрано закрытие модалки сотрудника
      this.$emit('open-application', applicationId);
    },

    openEmployeesHistory() {
      this.showEmployeesHistory = true;
    },

    startPolling() {
      if (this.pollingInterval) return;
      this.silentRefresh();
      this.pollingInterval = setInterval(() => {
        // На живом SSE поллинг молчит (обновление уже пришло сигналом tables.refresh) -
        // таймер остаётся подстраховкой на 60с и мгновенно подхватывает при разрыве (#840).
        if (this.sseConnected) return;
        this.silentRefresh();
      }, 60000);
    },

    stopPolling() {
      if (this.pollingInterval) {
        clearInterval(this.pollingInterval);
        this.pollingInterval = null;
      }
    },

    enlargedStorageKey() {
      return `${ENLARGED_KEY_PREFIX}${this.tableName || 'default'}`;
    },

    loadEnlargedFromStorage() {
      try {
        this.enlarged = localStorage.getItem(this.enlargedStorageKey()) === '1';
      } catch {
        this.enlarged = false;
      }
    },

    saveEnlargedToStorage(value) {
      try {
        localStorage.setItem(this.enlargedStorageKey(), value ? '1' : '0');
      } catch {
        /* localStorage недоступен - игнорируем */
      }
    },

    isFieldVisible(fieldName) {
      // Видимость определяется ОТДЕЛЬНО для каждого режима, иначе enlarged
      // молча "подтягивал" бы обычные настройки is_visible.
      if (this.enlarged) {
        const ev = this.fieldsEnlargedVisibility[fieldName];
        if (ev === false) return false;
      } else {
        const v = this.fieldsVisibility[fieldName];
        const visible = v === undefined ? true : v;
        if (!visible) return false;
      }
      if (this.isCompact) {
        const p = this.fieldPriorities[fieldName];
        if (typeof p === 'number' && p > this.compactPriorityThreshold) return false;
      }
      return true;
    },

    /**
     * Столбец ВСЕГДА в DOM - скрытие через класс col--collapsed с transition.
     */
    isFieldInDom() {
      return true;
    },

    fieldColClass(fieldName) {
      if (this.enlarged) {
        const ev = this.fieldsEnlargedVisibility[fieldName];
        if (ev === false) return 'col--collapsed';
      } else {
        const v = this.fieldsVisibility[fieldName];
        if (v === false) return 'col--collapsed';
      }
      if (this.isCompact) {
        const p = this.fieldPriorities[fieldName];
        if (typeof p === 'number' && p > this.compactPriorityThreshold) {
          return 'col--collapsed';
        }
      }
      return '';
    },

    /**
     * Стиль конфигурируемой ячейки:
     * - order: 10 + display_order (между фиксированными entry/exit/status/actions).
     * - flex-grow: width из конфига (если задан) - переопределяет дефолт из CSS.
     *
     * В enlarged НЕ задаём inline flex-grow ни одному столбцу - пусть CSS
     * пропорционально распределяет освободившееся от status пространство
     * по всем оставшимся столбцам.
     */
    getColStyle(fieldName, isHeader = false) {
      const order = this.fieldOrders[fieldName];
      const width = this.fieldWidths[fieldName];
      const style = {};
      if (order !== undefined) style.order = 10 + order;
      if (this.enlarged) {
        const ew = this.fieldsEnlargedWidth[fieldName];
        if (typeof ew === 'number' && ew > 0) style.flexGrow = ew;
        // enlarged_font_weight - применяется ТОЛЬКО к данным, не к заголовку.
        if (!isHeader) {
          const eweight = this.fieldsEnlargedWeight[fieldName];
          if (typeof eweight === 'number' && eweight > 0) style.fontWeight = eweight;
        }
      } else if (width !== undefined && width > 0) {
        style.flexGrow = width;
      }
      return Object.keys(style).length ? style : null;
    },

    hiddenInPortraitFields() {
      if (!this.isCompact) return [];
      return Object.keys(this.fieldsVisibility)
        .filter(name => this.fieldsVisibility[name] !== false)
        .filter(name => {
          const p = this.fieldPriorities[name];
          return typeof p === 'number' && p > this.compactPriorityThreshold;
        });
    },

    toggleRowExpand(rowId) {
      const next = { ...this.expandedRows };
      next[rowId] = !next[rowId];
      this.expandedRows = next;
    },

    markConfigReady() {
      // 2 rAF + 100ms - гарантия что layout с итоговыми ширинами применился
      // и transition не сработает на стартовом расхождении.
      if (typeof requestAnimationFrame !== 'function') {
        this.configReady = true;
        return;
      }
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          setTimeout(() => { this.configReady = true; }, 100);
        });
      });
    },

    portraitFieldLabel(name) {
      const LABELS = {
        last_name: 'Фамилия',
        first_name: 'Имя',
        middle_name: 'Отчество',
        position: 'Должность',
        citizenship_name: 'Гражданство',
        organization: 'Организация',
        company: 'Компания',
        valid_until: 'Действует до',
        pass_time: 'Время прохода',
        application_id: 'Номер заявки',
      };
      return LABELS[name] || name;
    },

    portraitFieldValue(item, name) {
      switch (name) {
        case 'last_name': return item.last_name || '-';
        case 'first_name': return item.first_name || '-';
        case 'middle_name': return item.middle_name || '-';
        case 'position': return item.position || '-';
        case 'citizenship_name': return item.citizenship_name || '-';
        case 'organization': return item.organization_name || '-';
        case 'company': return item.company || '-';
        case 'valid_until': return this.formatDate(item.entry_date_to);
        case 'pass_time': return item.pass_time || '-';
        case 'application_id': return this.isManualItem(item) ? 'Добавлено вручную' : (item.applicationNumber || '-');
        default: return '-';
      }
    },

    async exportToExcel() {
      await this.buildPeopleExcel(this.displayItems, 'Lyudi');
    },

    // Экспорт только выделенных строк (#1194 S6) - reuse форматирования полного
    // экспорта (buildPeopleExcel), фильтр по selectedIds (useRowSelection).
    async exportSelectedToExcel() {
      const rows = this.displayItems.filter(item => this.selectedIds.includes(item.id));
      if (!rows.length) return;
      await this.buildPeopleExcel(rows, 'Lyudi_vybrannye');
      useDeletionsStore().notify({ prefix: 'Выгружено строк: ', bold: String(rows.length) });
    },

    // Общий билдер книги people-таблицы: набор строк и префикс имени файла -
    // единственное, что различается между полным экспортом и экспортом выбранных.
    async buildPeopleExcel(rows, filenamePrefix) {
      if (!rows.length) return;

      const workbook = new ExcelJS.Workbook();
      const worksheet = workbook.addWorksheet('Lyudi');

      const headers = [
        'Въезд', 'Выезд', 'Фамилия', 'Имя', 'Отчество', 'Должность',
        'Организация', 'Компания', 'Дата до', 'Время прохода', '№ заявки', 'Статус',
      ];

      const headerRow = worksheet.addRow(headers);
      headerRow.height = 25;
      headerRow.eachCell((cell) => {
        cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF4F5BDF' } };
        cell.font = { name: 'Verdana', size: 11, bold: true, color: { argb: 'FFFFFFFF' } };
        cell.alignment = { vertical: 'middle', horizontal: 'center' };
        cell.border = {
          top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
          bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
          left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
          right: { style: 'thin', color: { argb: 'FFE6E6E6' } },
        };
      });

      rows.forEach((item, index) => {
        const row = worksheet.addRow([
          item.entry_checked ? 'Да' : 'Нет',
          item.exit_checked ? 'Да' : 'Нет',
          item.last_name || '-',
          item.first_name || '-',
          item.middle_name || '-',
          item.position || '-',
          item.organization_name || '-',
          item.company || '-',
          this.formatDate(item.entry_date_to),
          item.pass_time || '-',
          item.applicationNumber || '-',
          item.status || '-',
        ]);
        row.height = 20;
        const fillColor = index % 2 === 0 ? 'FFF0F5FF' : 'FFE0E9FF';
        row.eachCell((cell) => {
          cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: fillColor } };
          cell.font = { name: 'Verdana', size: 9, color: { argb: 'FF333333' } };
          cell.alignment = { vertical: 'middle' };
          cell.border = {
            top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            right: { style: 'thin', color: { argb: 'FFE6E6E6' } },
          };
        });
      });

      const colCount = headers.length;
      const lastDataRow = rows.length;
      for (let r = 1; r <= lastDataRow + 1; r++) {
        const rc = worksheet.getCell(r, colCount);
        rc.border = { ...rc.border, right: { style: 'medium', color: { argb: 'FF000000' } } };
        const lc = worksheet.getCell(r, 1);
        lc.border = { ...lc.border, left: { style: 'medium', color: { argb: 'FF000000' } } };
      }
      for (let c = 1; c <= colCount; c++) {
        const tc = worksheet.getCell(1, c);
        tc.border = { ...tc.border, top: { style: 'medium', color: { argb: 'FF000000' } } };
        const bc = worksheet.getCell(lastDataRow + 1, c);
        bc.border = { ...bc.border, bottom: { style: 'medium', color: { argb: 'FF000000' } } };
      }

      worksheet.addRow([]);
      const now = new Date();
      const dateStr = now.toLocaleString('ru-RU', {
        day: '2-digit', month: '2-digit', year: 'numeric',
        hour: '2-digit', minute: '2-digit', second: '2-digit',
      }).replace(',', '');
      const userDisplay = (this.currentUserName || '').trim() || 'Пользователь';
      [
        worksheet.addRow(['Отчёт сформировал:', userDisplay]),
        worksheet.addRow(['Дата формирования:', dateStr]),
      ].forEach(row => {
        row.eachCell((cell) => {
          cell.font = { name: 'Verdana', size: 10, color: { argb: 'FF333333' } };
          cell.alignment = { vertical: 'middle' };
          cell.border = {
            top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
            right: { style: 'thin', color: { argb: 'FFE6E6E6' } },
          };
        });
      });

      worksheet.columns = [
        { width: 10 }, { width: 10 }, { width: 22 }, { width: 18 }, { width: 18 }, { width: 22 },
        { width: 35 }, { width: 25 }, { width: 14 }, { width: 16 }, { width: 16 }, { width: 20 },
      ];

      const buffer = await workbook.xlsx.writeBuffer();
      const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.download = `${filenamePrefix}_${dateStr.replace(/[.:,\s]/g, '-')}.xlsx`;
      a.href = url;
      a.click();
      window.URL.revokeObjectURL(url);
    },
  }
};
</script>

<style scoped>
.selected-table-card {
  position: relative; /* контекст для оверлей-панели .bulk-bar поверх .card-header (#1194) */
  background-color: #fff;
  border-radius: 30px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 100%;
  max-height: 575px;
  display: flex;
  flex-direction: column;
}

.card-header {
  border-bottom: 1px solid #e6e6e6;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 8px 20px;
  min-height: 50px;
  flex-shrink: 0;
  flex-wrap: wrap;
}

/* Панель групповых операций (#1194) - оверлей поверх .card-header (не
   reflow, урок #510). Высота = высоте шапки (50px). */
.bulk-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 6;
  display: flex;
  align-items: center;
  gap: 14px;
  height: 50px;
  padding: 0 20px;
  border-bottom: 1px solid #e6e6e6;
  background: #f0f2ff;
}

.bulk-count {
  font-size: 14px;
  font-weight: 600;
  color: #4F5BDF;
  white-space: nowrap;
}

.bulk-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

.card-header__title {
  display: flex;
  gap: 12px;
  align-items: center;
  min-width: 0;
  flex-shrink: 1;
}

/* Слот для действий в шапке (напр. поиск в preview версий) - прижат вправо. */
.card-header__actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 12px;
}

.card-header__settings {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.card-title {
  margin: 0;
  color: #000;
  font-weight: 600;
  font-size: 1.1em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.blue {
  color: #4F5BDF;
}

.items-count {
  color: #4F5BDF;
  font-weight: 500;
  font-size: 0.9em;
  display: flex;
  align-items: center;
  gap: 10px;
  white-space: nowrap;
}

@media (max-width: 1100px) {
  .card-header {
    gap: 8px;
    padding: 6px 12px;
  }
  .card-title {
    font-size: 0.95em;
  }
  .items-count {
    font-size: 0.8em;
    gap: 6px;
  }
  .history-btn {
    padding: 2px 8px;
    font-size: 11px;
  }
}

.history-btn {
  padding: 4px 12px;
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  font-size: 12px;
  color: #333;
  cursor: pointer;
  transition: all 0.2s ease;
}

.history-btn:hover {
  background: #f5f5f5;
  border-color: #4F5BDF;
}

.card-content {
  padding: 0;
  flex-grow: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* items-header повторяет геометрию items-body (padding-right + margin-right 4px),
   чтобы доступная ширина для колонок совпала и заголовки выровнялись с данными. */
.items-header {
  border-bottom: 1px solid #e6e6e6;
  flex-shrink: 0;
  padding-right: 4px;
  margin-right: 4px;
}

/* header-row повторяет геометрию item-data: padding 10/16 + flex + gap 4. */
.header-row {
  padding: 10px 10px;
  display: flex;
  width: 100%;
  align-items: center;
  gap: 4px;
}

.col {
  flex-shrink: 0;
  box-sizing: border-box;
  text-align: left;
  /* Данные не липнут к границам столбца (заметно в режиме "Сетка", #1289).
     Боковой отступ строки уменьшен на столько же, поэтому крайние столбцы
     остались на прежнем расстоянии от края карточки. */
  padding-left: 6px;
  padding-right: 6px;

  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding-right: 8px;
}

/* Веса колонок (flex-grow). Браузер делит доступную ширину пропорционально весам
   видимых колонок. При скрытии любых через v-if остальные автоматически расширяются. */
.entry-col { flex: 6 0 0; }
.exit-col { flex: 6 0 0; }
.last-name-col { flex: 14 0 0; }
.first-name-col { flex: 9 0 0; }
.middle-name-col { flex: 11 0 0; }
.position-col { flex: 9 0 0; }
.citizenship-col { flex: 10 0 0; }
.organization-col { flex: 16 0 0; }
.company-col { flex: 11 0 0; }
.date-col { flex: 11 0 0; }
.time-col { flex: 13 0 0; }
.application-col { flex: 11 0 0; }
.status-col { flex: 8 0 0; }
.actions-col { flex: 2 0 0; padding-right: 0; }

.manual-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 11px;
  font-weight: 500;
  line-height: 1.4;
  color: #4f5bdf;
  background: rgba(79, 91, 223, 0.1);
  white-space: nowrap;
}

.header-row .col {
  font-weight: 500;
  color: #a2a2a2;
  cursor: pointer;
  user-select: none;
  display: flex;
  align-items: center;
  gap: 5px;
}

.header-row .col:hover {
  color: #333;
}

.header-row .col:hover .sort-icon {
  filter: brightness(0);
}

.sort-icon {
  width: 12px;
  height: 12px;
  transition: .2s;
}

.sort-icon.sorted {
  filter: brightness(0);
}

.sort-icon.desc {
  transform: rotate(180deg);
}

.active-sort {
  color: #333 !important;
  font-weight: 500 !important;
}

.items-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  position: relative;
}

/* Overlay-лоадер при refresh - сохраняет высоту таблицы. */
.refresh-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.75);
  backdrop-filter: blur(1px);
  z-index: 2;
  pointer-events: none;
}

.items-body {
  overflow-y: auto;
  flex-grow: 1;
  padding-right: 4px;
  margin-right: 4px;
  min-height: 80px;
}

/* Drag-select (#1227 P4): пока ведём курсор с зажатым Ctrl - не даём браузеру
   выделить текст строк под курсором. */
.items-body.is-drag-selecting,
.items-body.is-drag-selecting * {
  user-select: none;
}

.item-row {
  transition: background-color 0.2s ease;
  opacity: 0;
  transform: translateY(10px);
  animation: fadeInUp 0.3s ease forwards;
  cursor: pointer;
}

.item-row:hover {
  background-color: #f5f5f5;
}

/* Подсветка ctrl/shift-выделенной строки (#1194) - тот же тон, что фон
   .bulk-bar, чтобы связь выбора с панелью читалась визуально. */
.item-row--selected {
  background-color: #eef0ff;
}

.item-row--selected:hover {
  background-color: #e4e7fd;
}

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.item-data {
  display: flex;
  width: 100%;
  padding: 10px 10px;
  align-items: center;
  border-bottom: 1px solid #e6e6e6;
  gap: 4px;
}

.entry-col, .exit-col {
  display: flex;
}

.action-btn {
  width: 70px;
  height: 30px;
  border-radius: 50px;
  border: 1px solid #e6e6e6;
  background: white;
  color: #000;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}

.action-btn:hover:not(:disabled) {
  background: #f5f5f5;
  border-color: #a2a2a2;
}

.action-btn:disabled {
  cursor: not-allowed;
  opacity: 0.7;
}

.action-btn.entry-btn.active {
  background: #e6f7e6;
  color: #2e7d32;
  border-color: #a5d6a7;
  font-weight: 600;
}

.action-btn.exit-btn.active {
  background: #ffebee;
  color: #c62828;
  border-color: #ef9a9a;
  font-weight: 600;
}

.status-text {
  color: #079D1D;
  font-weight: 500;
}

.delete-btn {
  background: none;
  border: none;
  cursor: pointer;
  padding: 2px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.delete-btn:hover:not(:disabled) {
  background-color: transparent;
}

.delete-btn:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.delete-icon {
  width: 14px;
  height: 14px;
  opacity: 0.7;
  transition: opacity 0.2s ease;
}

.delete-btn:hover:not(:disabled) .delete-icon {
  opacity: 1;
}

.no-data-message {
  text-align: center;
  color: #a2a2a2;
  padding: 40px 20px;
  margin: 0;
  font-size: 14px;
  flex-grow: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.loading-message {
  text-align: center;
  color: #a2a2a2;
  padding: 40px 20px;
  margin: 0;
  font-size: 14px;
  flex-grow: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
}

.fade-list-enter-active,
.fade-list-leave-active {
  transition: all 0.3s ease;
}

.fade-list-enter-from,
.fade-list-leave-to {
  opacity: 0;
  transform: translateY(10px);
}

.fade-list-move {
  transition: transform 0.3s ease;
}

/* "Увеличенный режим": плавный переход в обе стороны.
   Transition объединён на .col (font-size + width + opacity), чтобы избежать
   конфликта специфичности (раньше .items-body .col перекрывал отдельные
   правила .status-col / .organization-col и данные не анимировались). */
.selected-table-card .col {
  transition:
    font-size 0.4s ease-in-out,
    flex-grow 0.4s ease-in-out,
    flex-basis 0.4s ease-in-out,
    max-width 0.4s ease-in-out,
    min-width 0.4s ease-in-out,
    padding 0.4s ease-in-out,
    opacity 0.3s ease-in-out;
}

/* Скрытый в "Увеличенный режим" столбец - не удаляем из DOM, а схлопываем.
   Транзишен на .col даёт плавную анимацию ширины/прозрачности. */
.selected-table-card .col.col--collapsed {
  flex-grow: 0 !important;
  flex-basis: 0 !important;
  min-width: 0 !important;
  max-width: 0 !important;
  padding-left: 0 !important;
  padding-right: 0 !important;
  opacity: 0;
  pointer-events: none;
  overflow: hidden;
  white-space: nowrap;
}

/* Пока конфиг не загружен - запрещаем transitions на всех потомках, чтобы
   шапка/столбцы не "ездили" между дефолтными и сохранёнными значениями
   при первом рендере. */
.selected-table-card.config-not-ready,
.selected-table-card.config-not-ready * {
  transition: none !important;
}

.selected-table-card .item-data {
  transition: min-height 0.4s ease-in-out;
}

.selected-table-card .status-col {
  overflow: hidden;
}

.selected-table-card.enlarged .items-body .col {
  font-size: 18px;
}

/* В enlarged status-col схлопывается; освободившиеся 8 grow распределяются
   автоматически по ВСЕМ оставшимся столбцам пропорционально базовым весам -
   inline flex-grow для них не задаётся (getColStyle пропускает). */
.selected-table-card.enlarged .status-col {
  flex-grow: 0;
  opacity: 0;
  pointer-events: none;
}

.selected-table-card.enlarged .item-data {
  min-height: 36px;
}

@media (max-width: 767.98px) {
  .selected-table-card {
    max-height: none;
    height: auto;
  }

  .card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
    height: auto;
    padding: 16px;
  }

  /* Шапка стала колоночной auto-высоты - фиксированный оверлей .bulk-bar (50px)
     накрыл бы только верх, хвост торчал бы под ним. Возвращаем панель в поток
     (образец CompaniesManagement, #1194). */
  .bulk-bar {
    position: static;
    height: auto;
    padding: 12px 16px;
    flex-wrap: wrap;
  }

  /* Три кнопки операций + "Снять выбор" не помещаются в строку на узком экране -
     переносим, чтобы не утекали в горизонтальный скролл (#1097 S8/#1114). */
  .bulk-actions {
    flex-wrap: wrap;
    margin-left: 0;
    width: 100%;
  }

  .card-header__settings,
  .card-header__actions {
    width: 100%;
    justify-content: flex-end;
  }

  /* rt-row (#1097 S8) сидит на .item-data, а не на v-for-корне .item-row -
     сиблинг-селектор ".rt-row + .rt-row" из responsive-tables.css поэтому не
     матчит (соседние .item-row, не .item-data), спейсинг карточек добираем тут. */
  .item-row + .item-row {
    margin-top: 8px;
  }

  /* Значения в карточке не обрезаем многоточием - там больше горизонтального
     места, чем в узкой табличной колонке. */
  .selected-table-card .rt-row > [data-label]:not(.col--collapsed) {
    white-space: normal;
    overflow: visible;
    text-overflow: clip;
  }

  /* Приоритет-схлопнутые (col--collapsed) колонки в card-режиме прячем совсем -
     их поля доступны в панели "Подробнее". Иначе card-override overflow:visible
     перебивает overflow:hidden схлопывания (равная специфичность, правило ниже) и
     колонка становится пустой раздутой строкой карточки вместо исчезновения. */
  .selected-table-card .rt-row > .col--collapsed {
    display: none !important;
  }

  /* Тач-таргет >=44px (WCAG) для кнопок Вход/Выход/удаления/раскрытия. */
  .action-btn {
    min-width: 70px;
    height: 44px;
    font-size: 13px;
  }

  .delete-btn {
    width: 44px;
    height: 44px;
  }

  .expand-btn {
    width: 44px;
    height: 44px;
  }
}

/* #345 Phase 1D: размер шрифта строк через CSS-переменную. */
.selected-table-card .items-body .col {
  font-size: var(--table-font-size, 14px);
}

/* #345 Phase 1E: плотность строк - вертикальный padding ячейки. */
.selected-table-card.density-compact .item-data,
.selected-table-card.density-compact .header-row {
  padding-top: 4px;
  padding-bottom: 4px;
}

.selected-table-card.density-spacious .item-data,
.selected-table-card.density-spacious .header-row {
  padding-top: 16px;
  padding-bottom: 16px;
}

/* На мобилке строки показываются карточками - сетка не применяется, тумблер
   там не нужен. */
@media (max-width: 767.98px) {
  .grid-toggle {
    display: none;
  }
}

/* Режим "Сетка" (#1289): вертикальные линии между колонками. Горизонтальные
   линии и внешний контур уже дают border-bottom строк и рамка карточки.
   На мобилке строки превращаются в карточки (rt-row), поэтому блок живёт от 768px.

   Включение режима ничего не двигает: отступы, выравнивание и размеры ячеек
   остаются прежними при любых настройках размера шрифта и плотности строк.
   Линия - псевдоэлемент; ячейке разрешён вынос за свои границы (overflow: clip
   вместо hidden, обрезка содержимого и ellipsis при этом сохраняются), а по
   высоте строки линию подрезает сама строка. Так линия идёт от края до края
   независимо от того, насколько содержимое ячейки ниже строки. */
@media (min-width: 768px) {
  .selected-table-card.grid-mode .header-row,
  .selected-table-card.grid-mode .item-data {
    overflow: clip;
  }

  /* overflow: clip, в отличие от hidden, не создаёт скролл-контейнер, поэтому
     ячейка перестаёт занулять свой автоматический минимум ширины и начинает
     распираться содержимым. Возвращаем нулевой минимум явно - слабым селектором,
     чтобы фиксированные min-width увеличенного режима остались в силе. */
  .selected-table-card.grid-mode .col {
    min-width: 0;
  }

  .selected-table-card.grid-mode .header-row > .col,
  .selected-table-card.grid-mode .item-data > .col {
    position: relative;
    overflow: clip;
    /* Заведомо больше любой строки - лишнее подрежет строка. */
    overflow-clip-margin: 200px;
  }

  .selected-table-card.grid-mode .header-row > .col::after,
  .selected-table-card.grid-mode .item-data > .col::after {
    content: '';
    position: absolute;
    top: -200px;
    right: 0;
    bottom: -200px;
    width: 1px;
    background: #e6e6e6;
  }

  /* У схлопнутой колонки (увеличенный режим, приоритет) нулевая ширина - её
     линия легла бы поверх соседней. Последняя колонка упирается в рамку
     карточки, своя линия ей не нужна. */
  .selected-table-card.grid-mode .col--collapsed::after,
  .selected-table-card.grid-mode .header-row > .col:last-child::after,
  .selected-table-card.grid-mode .item-data > .col:last-child::after {
    display: none;
  }
}

/* #345 Phase 1F: портретный режим. */
.expand-col {
  flex: 2.5 0 0;
  display: flex;
  justify-content: center;
  align-items: center;
}

.expand-btn {
  width: 22px;
  height: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: 1px solid #e6e6e6;
  border-radius: 6px;
  color: #6b7280;
  cursor: pointer;
  transition: transform 0.2s ease, color 0.15s ease, background 0.15s ease;
}

.expand-btn:hover {
  background: #f5f5f5;
  color: #4F5BDF;
}

.expand-btn--open {
  transform: rotate(180deg);
  color: #4F5BDF;
  background: #eef0ff;
}

/* Раскрытие "Подробнее" - стиль карточек label/value как в EmployeeDetailsModal.
   Auto-fit grid, каждый item - flex-column с label сверху и value снизу. */
.item-row__details {
  padding: 12px 16px 14px;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 10px;
  background: #fafafa;
  border-top: 1px dashed #e6e6e6;
}

.detail-item {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 4px;
  min-width: 0;
  padding: 10px 14px;
  background: #fff;
  border-radius: 20px;
  border: 1px solid #ececec;
}

.detail-item__label {
  color: #a2a2a2;
  font-size: 11px;
  font-weight: 400;
  letter-spacing: 0.3px;
  white-space: nowrap;
}

.detail-item__value {
  color: #333;
  font-size: 14px;
  font-weight: 500;
  word-break: break-word;
  min-width: 0;
}
</style>
