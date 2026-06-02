<template>
  <div
    class="selected-table-card"
    :class="[
      { 'enlarged': enlarged, 'is-portrait': isCompact, 'config-not-ready': !configReady },
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
        v-if="!preview"
        class="card-header__settings"
      >
        <span class="items-count">
          Людей зашло: {{ peopleOnTerritory }}
          <button
            class="history-btn"
            @click="openEmployeesHistory"
          >История</button>
        </span>
        <EnlargedToggle
          v-model="enlarged"
          data-testid="enlarged-toggle"
        />
        <RefreshButton
          :loading="refreshing"
          @refresh="loadData"
        />
      </div>
    </div>
    
    <div class="card-content">
      <div class="items-header">
        <div class="header-row">
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
            v-if="isFieldVisible('last_name')"
            class="col last-name-col"
            :style="getColStyle('last_name')"
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
            v-if="isFieldVisible('first_name')"
            class="col first-name-col"
            :style="getColStyle('first_name')"
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
            v-if="isFieldVisible('middle_name')"
            class="col middle-name-col"
            :style="getColStyle('middle_name')"
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
            v-if="isFieldVisible('position')"
            class="col position-col"
            :style="getColStyle('position')"
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
            v-if="isFieldVisible('citizenship_name')"
            class="col citizenship-col"
            :style="getColStyle('citizenship_name')"
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
            v-if="isFieldVisible('organization')"
            class="col organization-col"
            :style="getColStyle('organization')"
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
            v-if="isFieldVisible('company')"
            class="col company-col"
            :style="getColStyle('company')"
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
            v-if="isFieldVisible('valid_until')"
            class="col date-col"
            :style="getColStyle('valid_until')"
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
            v-if="isFieldVisible('pass_time')"
            class="col time-col"
            :style="getColStyle('pass_time')"
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
            v-if="isFieldVisible('application_id')"
            class="col application-col"
            :style="getColStyle('application_id')"
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
          <div class="loader" />
          <p>Загрузка сотрудников...</p>
        </div>

        <div
          v-if="refreshing && !isLoading"
          class="refresh-overlay"
        >
          <div class="loader" />
          <p>Обновление...</p>
        </div>

        <div
          v-if="!isLoading && displayItems.length > 0"
          class="items-body"
        >
          <transition-group
            name="fade-list"
            tag="div"
          >
            <div
              v-for="(item, index) in displayItems"
              :key="item.id"
              class="item-row"
              :class="{ 'item-row--expanded': expandedRows[item.id] }"
              :style="{ animationDelay: `${index * 0.05}s` }"
              @click="preview ? null : openEmployeeDetails(item)"
            >
              <div class="item-data">
                <div
                  class="col entry-col"
                  style="order: 0;"
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
                  v-if="isFieldVisible('last_name')"
                  class="col last-name-col"
                  :style="getColStyle('last_name')"
                >
                  {{ item.last_name }}
                </div>
                <div
                  v-if="isFieldVisible('first_name')"
                  class="col first-name-col"
                  :style="getColStyle('first_name')"
                >
                  {{ item.first_name }}
                </div>
                <div
                  v-if="isFieldVisible('middle_name')"
                  class="col middle-name-col"
                  :style="getColStyle('middle_name')"
                >
                  {{ item.middle_name || '-' }}
                </div>
                <div
                  v-if="isFieldVisible('position')"
                  class="col position-col"
                  :style="getColStyle('position')"
                >
                  {{ item.position || '-' }}
                </div>
                <div
                  v-if="isFieldVisible('citizenship_name')"
                  class="col citizenship-col"
                  :style="getColStyle('citizenship_name')"
                >
                  {{ item.citizenshipName || '-' }}
                </div>
                <div
                  v-if="isFieldVisible('organization')"
                  class="col organization-col"
                  :style="getColStyle('organization')"
                >
                  {{ item.organization_name }}
                </div>
                <div
                  v-if="isFieldVisible('company')"
                  class="col company-col"
                  :style="getColStyle('company')"
                >
                  {{ item.company || '-' }}
                </div>
                <div
                  v-if="isFieldVisible('valid_until')"
                  class="col date-col"
                  :style="getColStyle('valid_until')"
                >
                  {{ formatDate(item.entry_date_to) }}
                </div>
                <div
                  v-if="isFieldVisible('pass_time')"
                  class="col time-col"
                  :style="getColStyle('pass_time')"
                >
                  {{ formatPassTime(item.pass_time) }}
                </div>
                <div
                  v-if="isFieldVisible('application_id')"
                  class="col application-col"
                  :style="getColStyle('application_id')"
                >
                  {{ item.applicationNumber || '-' }}
                </div>
                <div
                  class="col status-col"
                  style="order: 9998;"
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
                  class="col actions-col"
                  style="order: 9999;"
                  @click.stop
                >
                  <button
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
  </div>
</template>

<script>
import { apiRequest } from '@/api/client';
import { useDeletionsStore } from '@/stores/deletions';
import { useOrientation } from '@/composables/useOrientation';
import RefreshButton from './RefreshButton.vue';
import EmployeeDetailsModal from './CreateApplication/EmployeeDetailsModal.vue';
import EmployeesTableHistoryModal from './CreateApplication/EmployeesTableHistoryModal.vue';
import StatusBadge from './ui/StatusBadge.vue';
import EnlargedToggle from './ui/EnlargedToggle.vue';

const ENLARGED_KEY_PREFIX = 'enlarged-mode:people:';

export default {
  name: 'PeopleTable',
  components: {
    RefreshButton,
    EmployeeDetailsModal,
    EmployeesTableHistoryModal,
    StatusBadge,
    EnlargedToggle
  },
  setup() {
    const { isPortrait, isCompact } = useOrientation();
    return { isPortrait, isCompact };
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
    previewItems: { type: Array, default: null }
  },
  emits: ['open-application'],
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
    };
  },
  computed: {
    displayItems() {
      let filtered = this.itemsData.filter(i => !this.pendingDeleteIds.includes(i.id));

      if (this.searchQuery) {
        const query = this.searchQuery.toLowerCase().trim();
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
          return searchFields.some(field => 
            field && field.toString().toLowerCase().includes(query)
          );
        });
      }

      if (this.selectedOrganizationId) {
        filtered = filtered.filter(item => item.organization_id == this.selectedOrganizationId);
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
    }
  },
  mounted() {
    if (this.preview) return;
    this.startPolling();
    this.loadEnlargedFromStorage();
    // Подгружаем настроенные длительности уведомлений после авторизации
    // (на холодном старте App.vue запрос мог уйти до получения токена).
    useDeletionsStore().loadDurations();
  },
  beforeUnmount() {
    this.stopPolling();
  },
  methods: {
    async _loadData(silent = false) {
      if (!silent && this.isLoading) return;
      if (!silent) this.isLoading = true;
      try {
        await this.fetchAllTables();
        await this.fetchOrganizations();
        await this.fetchPeopleData();
        await this.fetchEmployeesStatus();
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

    async fetchPeopleData() {
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

        this.itemsData = employees.map(emp => {
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
            territory_status: 0
          };
        });
      } catch (error) {
        console.error("Ошибка при загрузке сотрудников:", error);
        this.itemsData = [];
      }
    },

    async fetchEmployeesStatus() {
      try {
        const response = await apiRequest("/employees/history/current-status", { method: "GET" });
        if (response.ok) {
          const statuses = await response.json();
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
        this.silentRefresh();
      }, 10000);
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
      if (this.enlarged) {
        const ev = this.fieldsEnlargedVisibility[fieldName];
        if (ev === false) return false;
      }
      // Пока конфиг не загружен - показываем всё (предотвращает мигание при инициализации).
      const v = this.fieldsVisibility[fieldName];
      const visible = v === undefined ? true : v;
      if (!visible) return false;
      if (this.isCompact) {
        const p = this.fieldPriorities[fieldName];
        if (typeof p === 'number' && p > this.compactPriorityThreshold) return false;
      }
      return true;
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
    getColStyle(fieldName) {
      const order = this.fieldOrders[fieldName];
      const width = this.fieldWidths[fieldName];
      const style = {};
      if (order !== undefined) style.order = 10 + order;
      if (this.enlarged) {
        const ew = this.fieldsEnlargedWidth[fieldName];
        if (typeof ew === 'number' && ew > 0) style.flexGrow = ew;
        const eweight = this.fieldsEnlargedWeight[fieldName];
        if (typeof eweight === 'number' && eweight > 0) style.fontWeight = eweight;
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
        case 'application_id': return item.applicationNumber || '-';
        default: return '-';
      }
    },
  }
};
</script>

<style scoped>
.selected-table-card {
  background-color: #fff;
  border-radius: 30px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 100%;
  max-height: 575px;
  box-shadow: 0 3px 10px rgba(0,0,0,0.05);
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

.card-header__title {
  display: flex;
  gap: 12px;
  align-items: center;
  min-width: 0;
  flex-shrink: 1;
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
  padding: 10px 16px;
  display: flex;
  width: 100%;
  align-items: center;
  gap: 4px;
}

.col {
  flex-shrink: 0;
  box-sizing: border-box;
  text-align: left;
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
  padding: 10px 16px;
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

.loader {
  width: 30px;
  height: 30px;
  border: 3px solid #f3f3f3;
  border-top: 3px solid #4F5BDF;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
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
  transition: font-size 0.4s ease-in-out, flex-grow 0.4s ease-in-out, opacity 0.3s ease-in-out;
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

.selected-table-card.enlarged .items-body .last-name-col {
  font-weight: 700;
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

@media (max-width: 768px) {
  .selected-table-card {
    max-height: none;
    height: auto;
  }

  /*
   * Синхронный horizontal scroll: scroll на .card-content, header и body
   * имеют overflow visible и наследуют scroll от parent'а.
   */
  .card-content {
    overflow-x: auto !important;
    overflow-y: visible !important;
  }

  .items-header,
  .items-body {
    overflow: visible !important;
    min-width: 800px;
  }

  .header-row,
  .item-data {
    flex-wrap: nowrap !important;
    gap: 0;
    min-width: 800px;
  }

  .col {
    width: auto !important;
    min-width: 90px !important;
    flex: 1 1 auto !important;
    margin-bottom: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .col.last-name-col,
  .col.first-name-col,
  .col.organization-col {
    min-width: 110px !important;
  }

  .entry-col, .exit-col {
    width: auto !important;
    min-width: 60px !important;
    justify-content: flex-start;
  }

  .card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
    height: auto;
    padding: 16px;
  }

  .card-header__settings {
    width: 100%;
    justify-content: flex-end;
  }

  .action-btn {
    width: 60px;
    height: 28px;
    font-size: 11px;
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