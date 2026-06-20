<template>
  <div
    class="fact-table-card"
    :class="[
      { 'config-not-ready': !configReady },
      `density-${rowDensity}`,
    ]"
    :style="{ '--table-font-size': tableFontSize + 'px' }"
    data-testid="fact-table"
  >
    <div class="card-header">
      <div class="card-header__title">
        <h3 class="card-title">
          {{ tableType === 'cars' ? 'Автомобили' : 'Люди' }} <span class="highlight-text">по факту</span>
        </h3>
      </div>
      <div class="card-header__settings">
        <RefreshButton
          @refresh="$emit('refresh-data')"
        />
      </div>
    </div>
    
    <div class="card-content">
      <!-- Заголовок таблицы -->
      <div class="fact-header">
        <div class="header-row">
          <!-- Служебные въезд/выезд - всегда первые (только cars) -->
          <div
            v-if="tableType === 'cars'"
            class="col entry-col"
            style="order: 0;"
          >
            Въезд
          </div>
          <div
            v-if="tableType === 'cars'"
            class="col exit-col"
            style="order: 1;"
          >
            Выезд
          </div>
          <!-- Конфигурируемые столбцы CarsTable-каталога -->
          <div
            v-if="tableType === 'cars' && isFieldVisible('car_number')"
            class="col number-col"
            :style="getColStyle('car_number')"
          >
            <p>Номер Т/С</p>
          </div>
          <div
            v-if="tableType === 'cars' && isFieldVisible('car_brand')"
            class="col brand-col"
            :style="getColStyle('car_brand')"
            @click="sortBy('car_brand')"
          >
            <p :class="{ 'active-sort': sortField === 'car_brand' }">Марка</p>
          </div>
          <div
            v-if="isFieldVisible('organization')"
            class="col organization-col"
            :style="getColStyle('organization')"
            @click="sortBy('organization')"
          >
            <p :class="{ 'active-sort': sortField === 'organization' }">Организация</p>
          </div>
          <div
            v-if="tableType === 'cars' && isFieldVisible('company')"
            class="col company-col"
            :style="getColStyle('company')"
          >
            <p>Компания</p>
          </div>
          <div
            v-if="isFieldVisible('application_id')"
            class="col application-col"
            :style="getColStyle('application_id')"
          >
            <p>Номер заявки</p>
          </div>
          <div
            v-if="tableType === 'cars' && isFieldVisible('unload_place')"
            class="col place-col"
            :style="getColStyle('unload_place')"
          >
            <p>Место разгрузки</p>
          </div>
          <div
            v-if="isFieldVisible('valid_until')"
            class="col date-col"
            :style="getColStyle('valid_until')"
            @click="sortBy('entry_date_to')"
          >
            <p :class="{ 'active-sort': sortField === 'entry_date_to' }">Действует до</p>
          </div>
          <div
            v-if="isFieldVisible(tableType === 'cars' ? 'time_range' : 'pass_time')"
            class="col time-col"
            :style="getColStyle(tableType === 'cars' ? 'time_range' : 'pass_time')"
            @click="sortBy('entry_time')"
          >
            <p :class="{ 'active-sort': sortField === 'entry_time' }">
              {{ tableType === 'cars' ? 'Время' : 'Время прохода' }}
            </p>
          </div>
          <div
            v-if="tableType === 'cars' && isFieldVisible('status')"
            class="col status-col"
            :style="getColStyle('status')"
          >
            <p>Статус</p>
          </div>
          <!-- People-only поля -->
          <div
            v-if="tableType === 'people' && isFieldVisible('last_name')"
            class="col last-name-col"
            :style="getColStyle('last_name')"
          >
            <p>Фамилия</p>
          </div>
          <div
            v-if="tableType === 'people' && isFieldVisible('first_name')"
            class="col first-name-col"
            :style="getColStyle('first_name')"
          >
            <p>Имя</p>
          </div>
          <div
            v-if="tableType === 'people' && isFieldVisible('middle_name')"
            class="col middle-name-col"
            :style="getColStyle('middle_name')"
          >
            <p>Отчество</p>
          </div>
          <div
            v-if="tableType === 'people' && isFieldVisible('position')"
            class="col position-col"
            :style="getColStyle('position')"
          >
            <p>Должность</p>
          </div>
          <div
            v-if="tableType === 'people' && isFieldVisible('citizenship_name')"
            class="col citizenship-col"
            :style="getColStyle('citizenship_name')"
          >
            <p>Гражданство</p>
          </div>
          <!-- Служебная actions всегда последняя -->
          <div
            class="col actions-col"
            style="order: 9999;"
          >
            <!-- Пустой заголовок для действий -->
          </div>
        </div>
      </div>
      
      <!-- Тело таблицы -->
      <div class="fact-container">
        <div
          v-if="loading"
          class="loading-message"
        >
          <LoaderSpinner :label="tableType === 'cars' ? 'Загрузка машин...' : 'Загрузка...'" />
        </div>
        <div
          v-else-if="filteredData.length > 0"
          class="fact-body"
        >
          <transition-group
            name="fade-list"
            tag="div"
          >
            <div 
              v-for="(item, index) in sortedData" 
              :key="item.id" 
              class="fact-item"
              :style="{ animationDelay: `${index * 0.1}s` }"
              @click="openItemDetails(item)"
            >
              <div class="fact-row">
                <!-- Служебные въезд/выезд - всегда первые (только cars) -->
                <div
                  v-if="tableType === 'cars'"
                  class="col entry-col"
                  style="order: 0;"
                  @click.stop
                >
                  <button
                    class="action-btn entry-btn"
                    :class="{ 'active': item.entry_checked }"
                    :disabled="item.entry_checked"
                    @click="handleEntryExit(item, 'entry')"
                  >
                    Въезд
                  </button>
                </div>
                <div
                  v-if="tableType === 'cars'"
                  class="col exit-col"
                  style="order: 1;"
                  @click.stop
                >
                  <button
                    class="action-btn exit-btn"
                    :class="{ 'active': item.exit_checked }"
                    :disabled="!item.entry_checked || item.exit_checked"
                    @click="handleEntryExit(item, 'exit')"
                  >
                    Выезд
                  </button>
                </div>
                <!-- Конфигурируемые столбцы -->
                <div
                  v-if="tableType === 'cars' && isFieldVisible('car_number')"
                  class="col number-col"
                  :style="getColStyle('car_number')"
                >
                  {{ item.car_number || '-' }}
                </div>
                <div
                  v-if="tableType === 'cars' && isFieldVisible('car_brand')"
                  class="col brand-col"
                  :style="getColStyle('car_brand')"
                >
                  {{ item.car_brand || '-' }}
                </div>
                <div
                  v-if="isFieldVisible('organization')"
                  class="col organization-col"
                  :style="getColStyle('organization')"
                >
                  {{ item.organization_name || '-' }}
                </div>
                <div
                  v-if="tableType === 'cars' && isFieldVisible('company')"
                  class="col company-col"
                  :style="getColStyle('company')"
                >
                  {{ item.company || '-' }}
                </div>
                <div
                  v-if="isFieldVisible('application_id')"
                  class="col application-col"
                  :style="getColStyle('application_id')"
                >
                  {{ item.applicationNumber || '-' }}
                </div>
                <div
                  v-if="tableType === 'cars' && isFieldVisible('unload_place')"
                  class="col place-col"
                  :style="getColStyle('unload_place')"
                >
                  {{ formatUnloadPlaces ? formatUnloadPlaces(item) : '-' }}
                </div>
                <div
                  v-if="isFieldVisible('valid_until')"
                  class="col date-col"
                  :style="getColStyle('valid_until')"
                >
                  {{ formatDate(item.entry_date_to) }}
                </div>
                <div
                  v-if="isFieldVisible(tableType === 'cars' ? 'time_range' : 'pass_time')"
                  class="col time-col"
                  :style="getColStyle(tableType === 'cars' ? 'time_range' : 'pass_time')"
                >
                  {{ tableType === 'cars'
                    ? formatTimeRange(item.entry_time_from, item.entry_time_to)
                    : formatPassTime(item.pass_time)
                  }}
                </div>
                <div
                  v-if="tableType === 'cars' && isFieldVisible('status')"
                  class="col status-col"
                  :style="getColStyle('status')"
                >
                  <StatusBadge :status="item.status" />
                </div>
                <!-- People-only поля -->
                <div
                  v-if="tableType === 'people' && isFieldVisible('last_name')"
                  class="col last-name-col"
                  :style="getColStyle('last_name')"
                >
                  {{ item.last_name || '-' }}
                </div>
                <div
                  v-if="tableType === 'people' && isFieldVisible('first_name')"
                  class="col first-name-col"
                  :style="getColStyle('first_name')"
                >
                  {{ item.first_name || '-' }}
                </div>
                <div
                  v-if="tableType === 'people' && isFieldVisible('middle_name')"
                  class="col middle-name-col"
                  :style="getColStyle('middle_name')"
                >
                  {{ item.middle_name || '-' }}
                </div>
                <div
                  v-if="tableType === 'people' && isFieldVisible('position')"
                  class="col position-col"
                  :style="getColStyle('position')"
                >
                  {{ item.position || '-' }}
                </div>
                <div
                  v-if="tableType === 'people' && isFieldVisible('citizenship_name')"
                  class="col citizenship-col"
                  :style="getColStyle('citizenship_name')"
                >
                  {{ item.citizenshipName || item.citizenship_name || '-' }}
                </div>
                <!-- Удалить - всегда в конце -->
                <div
                  class="col actions-col"
                  style="order: 9999;"
                  @click.stop
                >
                  <button
                    class="delete-btn"
                    @click="deleteItem(item)"
                  >
                    <img
                      src="@/assets/icons/trashcan.png"
                      alt="Удалить"
                      class="delete-icon"
                    >
                  </button>
                </div>
              </div>
            </div>
          </transition-group>
        </div>
        <p
          v-else
          class="no-data-message"
        >
          {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : `Заявок ${tableType === 'cars' ? 'на машины' : 'на людей'} по факту нет` }}
        </p>
      </div>
    </div>

    <!-- Модальное окно с деталями автомобиля (для машин) -->
    <VehicleDetailsModal
      v-if="tableType === 'cars'"
      :show="showDetailsModal"
      :vehicle="selectedVehicle"
      :all-unloading-places="allUnloadingPlaces"
      :license-plate-formats="licensePlateFormats"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      :show-car-features="true"
      :source="'facttable'"
      @close="closeDetailsModal"
      @open-application="$emit('open-application', $event)"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import RefreshButton from './RefreshButton.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import StatusBadge from '@/components/ui/StatusBadge.vue';
import VehicleDetailsModal from './CreateApplication/VehicleDetailsModal.vue';
import ExcelJS from 'exceljs';

export default {
  name: 'FactTable',
  components: {
    RefreshButton,
    LoaderSpinner,
    StatusBadge,
    VehicleDetailsModal
  },
  props: {
    tableType: { type: String, default: 'cars', validator: (v) => ['cars', 'people'].includes(v) },
    tableId: { type: Number, default: null },
    tableData: { type: Object, default: null },
    searchQuery: { type: String, default: '' },
    selectedOrganizationId: { type: [Number, String], default: null },
    selectedCompanyId: { type: [Number, String], default: null },
    selectedUnloadingPlaceId: { type: [Number, String], default: null },
    dateRangeStart: { type: Date, default: null },
    dateRangeEnd: { type: Date, default: null },
    selectedDate: { type: Date, default: null },
    currentUserId: { type: Number, default: null },
    currentUserName: { type: String, default: '' },
    loading: { type: Boolean, default: false }
  },
  emits: ['refresh-data', 'open-application'],
  data() {
    return {
      sortField: null,
      sortDirection: 'desc',
      factData: [],
      organizationsMap: {},
      factCarUnloadPlacesMap: {},
      allUnloadingPlaces: [],
      licensePlateFormats: [],
      showDetailsModal: false,
      selectedVehicle: null,
      pollingInterval: null,
      fieldsVisibility: {},
      fieldOrders: {},
      fieldWidths: {},
      tableFontSize: 14,
      rowDensity: 'normal',
      configReady: false,
    };
  },
  computed: {
    filteredData() {
      let filtered = [...this.factData];
      if (this.searchQuery) {
        const query = this.searchQuery.toLowerCase();
        filtered = filtered.filter(item => 
          item.organization_name.toLowerCase().includes(query) ||
          (this.tableType === 'cars' && item.car_brand?.toLowerCase().includes(query)) ||
          (this.tableType === 'cars' && (item.company || '').toLowerCase().includes(query)) ||
          // (this.tableType === 'cars' && this.formatUnloadPlaces(item).toLowerCase().includes(query)) || // Место разгрузки закомментировано
          item.status.toLowerCase().includes(query) ||
          (this.tableType === 'cars' 
            ? this.formatTimeRange(item.entry_time_from, item.entry_time_to).toLowerCase().includes(query)
            : this.formatPassTime(item.pass_time).toLowerCase().includes(query)
          ) ||
          this.formatDate(item.entry_date_to).toLowerCase().includes(query)
        );
      }
      if (this.selectedOrganizationId) {
        filtered = filtered.filter(item => item.organization_id == this.selectedOrganizationId);
      }
      if (this.selectedCompanyId) {
        filtered = filtered.filter(item => item.company_id == this.selectedCompanyId);
      }
      if (this.selectedUnloadingPlaceId && this.tableType === 'cars') {
        filtered = filtered.filter(item => {
          const carId = item.id;
          const unloadPlaces = this.factCarUnloadPlacesMap[carId] || [];
          return unloadPlaces.some(place => place.id == this.selectedUnloadingPlaceId);
        });
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
      return filtered;
    },

    sortedData() {
      const data = [...this.filteredData];
      if (!this.sortField) return data;
      return data.sort((a, b) => {
        let valueA, valueB;
        switch (this.sortField) {
          case 'organization':
          case 'status':
          case 'car_brand':
          case 'company':
            valueA = (a[this.sortField] || '').toString().toLowerCase();
            valueB = (b[this.sortField] || '').toString().toLowerCase();
            break;
          case 'unload_place':
            // Сортировка по месту разгрузки закомментирована вместе с колонкой
            // valueA = this.formatUnloadPlaces(a).toLowerCase();
            // valueB = this.formatUnloadPlaces(b).toLowerCase();
            // break;
            return 0;
          case 'entry_date_to':
            valueA = a.entry_date_to ? new Date(a.entry_date_to) : new Date(0);
            valueB = b.entry_date_to ? new Date(b.entry_date_to) : new Date(0);
            break;
          case 'entry_time':
            if (this.tableType === 'cars') {
              valueA = this.extractStartTime(a.entry_time_from);
              valueB = this.extractStartTime(b.entry_time_from);
            } else {
              valueA = this.extractPassTime(a.pass_time);
              valueB = this.extractPassTime(b.pass_time);
            }
            break;
          default: return 0;
        }
        if (valueA < valueB) return this.sortDirection === 'asc' ? -1 : 1;
        if (valueA > valueB) return this.sortDirection === 'asc' ? 1 : -1;
        return 0;
      });
    },

    hasActiveFilters() {
      return !!(
        this.searchQuery ||
        this.selectedOrganizationId ||
        this.selectedCompanyId ||
        (this.tableType === 'cars' && this.selectedUnloadingPlaceId) ||
        this.selectedDate ||
        (this.dateRangeStart && this.dateRangeEnd)
      );
    }
  },
  watch: {
    tableType: {
      handler() {
        this.stopPolling();
        this.startPolling();
      },
      immediate: true
    },
    // Применяем фактовые настройки таблицы (#345 PR-B).
    tableData: {
      immediate: true,
      deep: true,
      handler(newVal) {
        if (!newVal) return;
        const tbl = newVal.table || {};
        const factFields = newVal.fact_fields || [];
        const nextVis = {};
        const nextOrd = {};
        const nextW = {};
        factFields.forEach(f => {
          nextVis[f.field_name] = f.is_visible !== false;
          if (typeof f.display_order === 'number') nextOrd[f.field_name] = f.display_order;
          if (typeof f.width === 'number' && f.width > 0) nextW[f.field_name] = f.width;
        });
        this.fieldsVisibility = nextVis;
        this.fieldOrders = nextOrd;
        this.fieldWidths = nextW;
        const fs = Number(tbl.font_size_fact);
        if (fs >= 10 && fs <= 24) this.tableFontSize = fs;
        const dens = tbl.row_density_fact;
        if (['compact', 'normal', 'spacious'].includes(dens)) this.rowDensity = dens;
        this.markConfigReady();
      },
    },
  },
  mounted() {
    this.startPolling();
  },
  beforeUnmount() {
    this.stopPolling();
  },
  methods: {
    isFieldVisible(fieldName) {
      const v = this.fieldsVisibility[fieldName];
      return v === undefined ? true : v;
    },

    getColStyle(fieldName) {
      const order = this.fieldOrders[fieldName];
      const width = this.fieldWidths[fieldName];
      const style = {};
      if (order !== undefined) style.order = 10 + order;
      if (width !== undefined && width > 0) style.flexGrow = width;
      return Object.keys(style).length ? style : null;
    },

    markConfigReady() {
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

    async _loadData() {
      try {
        await this.fetchUnloadingPlaces();
        await this.fetchLicensePlateFormats();
        await this.fetchOrganizations();
        if (this.tableType === 'cars') {
          await this.fetchCarsData();
          await this.fetchFactCarUnloadPlaces();
          await this.fetchCarHistoryStatus();
        } else {
          await this.fetchPeopleData();
        }
      } catch (error) {
        console.error(`Ошибка при загрузке данных по факту (${this.tableType}):`, error);
      }
    },

    async loadData() {
      await this._loadData(false);
    },

    async silentRefresh() {
      await this._loadData(true);
    },

    async fetchUnloadingPlaces() {
      try {
        const response = await apiRequest("/unload-places", {});
        if (response.ok) this.allUnloadingPlaces = await response.json();
      } catch (error) {
        console.error("Ошибка при загрузке мест разгрузки:", error);
      }
    },

    async fetchLicensePlateFormats() {
      try {
        const response = await apiRequest("/license-plate-formats", {});
        if (response.ok) this.licensePlateFormats = await response.json();
      } catch (error) {
        console.error("Ошибка при загрузке форматов номеров:", error);
      }
    },

    async fetchOrganizations() {
      try {
        const response = await apiRequest("/organizations", {});
        if (response.ok) {
          const data = await response.json();
          this.organizationsMap = {};
          data.forEach(org => { this.organizationsMap[org.id] = org.name; });
        }
      } catch (error) {
        console.error("Ошибка при загрузке организаций:", error);
      }
    },

    getOrganizationName(organizationId) {
      if (!organizationId) return 'Не указана';
      return this.organizationsMap[organizationId] || `Организация ID: ${organizationId}`;
    },

    async fetchCarsData() {
      try {
        const response = await apiRequest("/cars/fact-for-tables", {});
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
        const factCars = await response.json();
        const nameToIdMap = {};
        Object.keys(this.organizationsMap).forEach(id => {
          nameToIdMap[this.organizationsMap[id]] = id;
        });
        const newData = factCars.map(car => {
          const orgName = car.organization || '';
          const orgId = nameToIdMap[orgName] || car.organization_id;
          return {
            id: car.id,
            car_number: car.car_number || 'по факту',
            car_brand: car.car_brand || '',
            organization_id: orgId,
            organization_name: orgName || 'Не указана',
            company: car.company || null,
            company_id: car.company_id,
            unload_place: car.unload_place || '-',
            unload_place_ids: car.unload_place_ids || [],
            entry_date_to: car.entry_date_to || '',
            entry_time_from: car.entry_time_from || '',
            entry_time_to: car.entry_time_to || '',
            status: 'В работе',
            entry_checked: false,
            exit_checked: false,
            applicationId: car.application_id,
            plateNumber: car.car_number,
            mark: car.car_brand,
            formatId: null,
            unloadPlaces: car.unload_place_ids || []
          };
        });
        this.factData = newData;
      } catch (error) {
        console.error("Ошибка при загрузке данных по факту:", error);
      }
    },

    async fetchCarHistoryStatus() {
      try {
        const response = await apiRequest("/cars/history/current-status", {});
        if (response.ok) {
          const statuses = await response.json();
          const statusMap = {};
          statuses.forEach(status => { statusMap[status.car_id] = status; });
          this.factData.forEach(item => {
            const status = statusMap[item.id];
            if (status) {
              item.entry_checked = status.territory_status === 1;
              item.exit_checked = status.territory_status === 2;
            }
          });
        }
      } catch (error) {
        console.error("Ошибка при загрузке статусов въезда/выезда:", error);
      }
    },

    async fetchFactCarUnloadPlaces() {
      try {
        const response = await apiRequest("/cars/fact-unload-places", {});
        if (response.ok) {
          const carUnloadPlaces = await response.json();
          this.factCarUnloadPlacesMap = {};
          carUnloadPlaces.forEach(cup => {
            if (!this.factCarUnloadPlacesMap[cup.car_id]) this.factCarUnloadPlacesMap[cup.car_id] = [];
            this.factCarUnloadPlacesMap[cup.car_id].push({
              id: cup.unload_place_id,
              name: cup.unload_place_name || `Место #${cup.unload_place_id}`
            });
            const car = this.factData.find(c => c.id === cup.car_id);
            if (car) {
              if (!car.unload_place_ids) car.unload_place_ids = [];
              if (!car.unload_place_ids.includes(cup.unload_place_id)) {
                car.unload_place_ids.push(cup.unload_place_id);
                car.unloadPlaces = car.unload_place_ids;
              }
            }
          });
        }
      } catch (error) {
        console.error("Ошибка при загрузке связей факт-машин с местами разгрузки:", error);
        this.factCarUnloadPlacesMap = {};
      }
    },

    async fetchPeopleData() {
      this.factData = [];
    },

    formatUnloadPlaces(item) {
      if (item.unload_place_ids && item.unload_place_ids.length > 0) {
        const placeNames = item.unload_place_ids
          .map(id => {
            const place = this.allUnloadingPlaces.find(p => p.id === id);
            return place ? place.name : null;
          })
          .filter(name => name);
        if (placeNames.length === 0) return '-';
        if (placeNames.length === 1) return placeNames[0];
        return `${placeNames[0]} и др.`;
      }
      return item.unload_place || '-';
    },

    formatDate(dateString) {
      if (!dateString) return '';
      const [year, month, day] = dateString.split('-');
      const date = new Date(year, month - 1, day);
      return date.toLocaleDateString('ru-RU');
    },

    formatTimeRange(timeFrom, timeTo) {
      if (!timeFrom && !timeTo) return '-';
      const formatTime = (timeStr) => {
        if (!timeStr) return '';
        if (timeStr.includes(':') && timeStr.split(':').length === 3) return timeStr.substring(0, 5);
        return timeStr;
      };
      const formattedTimeFrom = formatTime(timeFrom);
      const formattedTimeTo = formatTime(timeTo);
      if (!formattedTimeTo) return formattedTimeFrom;
      if (!formattedTimeFrom) return formattedTimeTo;
      return `${formattedTimeFrom} - ${formattedTimeTo}`;
    },

    formatPassTime(passTime) { return passTime || '-'; },
    
    async handleEntryExit(item, type) {
      if (!this.currentUserId) return;
      try {
        let territory_status = type === 'entry' ? 1 : 2;
        const response = await apiRequest(`/cars/${item.id}/territory-status`, {
          method: "PUT",
          body: JSON.stringify({ territory_status, user_id: this.currentUserId, table_id: this.tableId })
        });
        if (response.ok) {
          const index = this.factData.findIndex(i => i.id === item.id);
          if (index !== -1) {
            const updatedItem = { ...this.factData[index] };
            if (type === 'entry') {
              updatedItem.entry_checked = true;
              updatedItem.exit_checked = false;
            } else {
              updatedItem.entry_checked = false;
              updatedItem.exit_checked = true;
            }
            this.factData.splice(index, 1, updatedItem);
          }
        } else {
          const errorText = await response.text();
          console.error('Ошибка при обновлении статуса:', errorText);
        }
      } catch (error) {
        console.error('Ошибка сети:', error);
      }
    },

    async deleteItem(item) {
      if (!confirm(`Удалить запись?`)) return;
      try {
        if (this.tableType === 'cars') {
          const response = await apiRequest(`/cars/${item.id}/deactivate`, {
            method: "PUT",
            body: JSON.stringify({ status: 0, user_id: this.currentUserId })
          });
          if (response.ok) this.factData = this.factData.filter(i => i.id !== item.id);
        } else {
          this.factData = this.factData.filter(i => i.id !== item.id);
        }
      } catch (error) {
        console.error("Ошибка при удалении:", error);
      }
    },
    
    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'desc';
      }
    },
    
    extractStartTime(timeString) {
      if (!timeString || timeString === '-') return 0;
      const timeWithoutSeconds = timeString.split(':').slice(0, 2).join(':');
      const [hours, minutes] = timeWithoutSeconds.split(':').map(Number);
      return hours * 60 + minutes;
    },

    extractPassTime(passTime) {
      if (!passTime || passTime === '-') return 0;
      const startTime = passTime.split('-')[0];
      const [hours, minutes] = startTime.split(':').map(Number);
      return hours * 60 + minutes;
    },

    openItemDetails(item) {
      if (this.tableType !== 'cars') return;
      this.selectedVehicle = {
        id: item.id,
        plateNumber: item.car_number,
        mark: item.car_brand,
        formatId: null,
        organization: item.organization_name,
        organizationId: item.organization_id,
        company: item.company,
        companyId: item.company_id,
        isExisting: true,
        unloadPlaces: item.unload_place_ids || [],
        entry_date_to: item.entry_date_to,
        entry_time_from: item.entry_time_from,
        entry_time_to: item.entry_time_to,
        applicationId: item.applicationId,
        entry_checked: item.entry_checked,
        exit_checked: item.exit_checked,
        car_number: item.car_number,
        car_brand: item.car_brand
      };
      this.showDetailsModal = true;
    },

    closeDetailsModal() {
      this.showDetailsModal = false;
      this.selectedVehicle = null;
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

    async exportToExcel() {
      const rows = this.filteredData;
      if (!rows.length) return;

      const isCars = this.tableType === 'cars';
      const workbook = new ExcelJS.Workbook();
      const sheetName = isCars ? 'Fakt_Avtomobili' : 'Fakt_Lyudi';
      const worksheet = workbook.addWorksheet(sheetName);

      const headers = isCars
        ? ['Въезд', 'Выезд', 'Номер Т/С', 'Марка', 'Организация', 'Компания', 'Дата до', 'Время', 'Статус']
        : ['Въезд', 'Выезд', 'Фамилия', 'Имя', 'Отчество', 'Должность', 'Гражданство', 'Организация', 'Дата до', 'Время прохода', 'Статус'];

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
        const rowData = isCars
          ? [
              item.entry_checked ? 'Да' : 'Нет',
              item.exit_checked ? 'Да' : 'Нет',
              item.car_number || '-',
              item.car_brand || '-',
              item.organization_name || '-',
              item.company || '-',
              this.formatDate(item.entry_date_to),
              this.formatTimeRange(item.entry_time_from, item.entry_time_to),
              item.status || '-',
            ]
          : [
              item.entry_checked ? 'Да' : 'Нет',
              item.exit_checked ? 'Да' : 'Нет',
              item.last_name || '-',
              item.first_name || '-',
              item.middle_name || '-',
              item.position || '-',
              item.citizenshipName || item.citizenship_name || '-',
              item.organization_name || '-',
              this.formatDate(item.entry_date_to),
              this.formatPassTime(item.pass_time),
              item.status || '-',
            ];

        const row = worksheet.addRow(rowData);
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

      worksheet.columns = isCars
        ? [{ width: 10 }, { width: 10 }, { width: 18 }, { width: 22 }, { width: 35 }, { width: 25 }, { width: 14 }, { width: 18 }, { width: 20 }]
        : [{ width: 10 }, { width: 10 }, { width: 22 }, { width: 18 }, { width: 18 }, { width: 22 }, { width: 20 }, { width: 35 }, { width: 14 }, { width: 16 }, { width: 20 }];

      const buffer = await workbook.xlsx.writeBuffer();
      const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.download = `${sheetName}_${dateStr.replace(/[.:,\s]/g, '-')}.xlsx`;
      a.href = url;
      a.click();
      window.URL.revokeObjectURL(url);
    },
  }
};
</script>

<style scoped>
.fact-table-card {
  background-color: #fff;
  border-radius: 30px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 100%;
  min-height: 222px;
  max-height: 222px;
}

.card-header {
  border-bottom: 1px solid #e6e6e6;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0px 20px;
  height: 40px;
}

.card-header__title {
  display: flex;
  gap: 8px;
  align-items: center;
}

.card-header__settings {
  display: flex;
  gap: 8px;
  align-items: center;
}

.card-title {
  margin: 0;
  color: #000;
  font-weight: 600;
  font-size: 1.1em;
}

.highlight-text {
  color: #4F5BDF;
}

.card-content {
  padding: 0;
  height: calc(100% - 40px);
  display: flex;
  flex-direction: column;
}

/* fact-header повторяет геометрию fact-body (padding-right + margin-right 4px),
   чтобы доступная ширина колонок совпала и заголовки выровнялись с данными. */
.fact-header {
  border-bottom: 1px solid #e6e6e6;
  flex-shrink: 0;
  padding-right: 4px;
  margin-right: 4px;
}

/* header-row повторяет геометрию fact-row: padding 10/16 + flex + gap 4. */
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
}

/* Веса колонок (flex-grow) - как в CarsTable. Inline-style из getColStyle
   перебивает базовые веса при наличии пользовательской ширины в конфиге. */
.entry-col { flex: 6.5 0 0; }
.exit-col { flex: 8 0 0; }
.number-col { flex: 10 0 0; }
.brand-col { flex: 10 0 0; }
.organization-col { flex: 18 0 0; }
.company-col { flex: 12 0 0; }
.application-col { flex: 12 0 0; }
.place-col { flex: 15 0 0; }
.date-col { flex: 12 0 0; }
.time-col { flex: 10 0 0; }
.status-col { flex: 7 0 0; }
.last-name-col { flex: 14 0 0; }
.first-name-col { flex: 9 0 0; }
.middle-name-col { flex: 11 0 0; }
.position-col { flex: 11 0 0; }
.citizenship-col { flex: 10 0 0; }
.actions-col { flex: 2 0 0; }

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

.fact-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.fact-body {
  overflow-y: auto;
  flex-grow: 1;
  padding-right: 4px;
  margin-right: 4px;
  scroll-behavior: smooth;
}

.fact-item {
  transition: background-color 0.2s ease;
  opacity: 0;
  transform: translateY(10px);
  animation: fadeInUp 0.3s ease forwards;
  cursor: pointer;
}

.fact-item:hover {
  background-color: #fafafa;
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

.fact-row {
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
  padding: 4px;
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
  width: 16px;
  height: 16px;
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

.fact-body::-webkit-scrollbar {
  width: 6px;
}

.fact-body::-webkit-scrollbar-track {
  background: transparent;
  margin: 2px 0;
  border-radius: 3px;
}

.fact-body::-webkit-scrollbar-thumb {
  background: #D9E2FF;
  border-radius: 3px;
  border: 1px solid transparent;
  background-clip: content-box;
  transition: all 0.3s ease;
}

.fact-body::-webkit-scrollbar-thumb:hover {
  background: #C5D1FF;
  border: 1px solid transparent;
  background-clip: content-box;
  transform: scale(1.1);
}

.fact-body {
  scrollbar-width: thin;
  scrollbar-color: #D9E2FF transparent;
  scroll-behavior: smooth;
  overscroll-behavior: contain;
}

.fade-list-enter-active,
.fade-list-leave-active {
  transition: all 0.5s ease;
}

.fade-list-enter-from,
.fade-list-leave-to {
  opacity: 0;
  transform: translateY(10px);
}

.fade-list-move {
  transition: transform 0.5s ease;
}

@media (max-width: 768px) {
  .fact-table-card {
    width: 100%;
    height: auto;
    max-height: none;
  }
  
  .header-row,
  .fact-row {
    flex-wrap: wrap;
    gap: 8px;
  }
  
  .col {
    width: calc(50% - 4px) !important;
    margin-bottom: 4px;
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
  
  .entry-col, .exit-col {
    width: calc(50% - 4px) !important;
    justify-content: flex-start;
  }
  
  .action-btn {
    width: 60px;
    height: 28px;
    font-size: 11px;
  }
}

.loading-message {
  text-align: center;
  color: #a2a2a2;
  padding: 40px 20px;
  font-size: 14px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
}

/* #345 PR-B: размер шрифта строк через CSS-переменную (только тело). */
.fact-table-card .fact-body .col {
  font-size: var(--table-font-size, 14px);
}

/* Плотность строк через padding row. */
.fact-table-card.density-compact .fact-row,
.fact-table-card.density-compact .header-row {
  padding-top: 4px;
  padding-bottom: 4px;
}

.fact-table-card.density-spacious .fact-row,
.fact-table-card.density-spacious .header-row {
  padding-top: 16px;
  padding-bottom: 16px;
}

/* Пока конфиг не загружен - запрещаем transitions на всех потомках, чтобы
   шапка/строки не "ездили" между дефолтами и сохранёнными значениями. */
.fact-table-card.config-not-ready,
.fact-table-card.config-not-ready * {
  transition: none !important;
}
</style>