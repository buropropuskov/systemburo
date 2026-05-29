<template>
  <div
    class="selected-table-card"
    data-testid="cars-table"
  >
    <div class="card-header">
      <div class="card-header__title">
        <h3 class="card-title">
          <span class="blue">Номера автомобилей</span> по заявке
        </h3>
      </div>
      <div class="card-header__settings">
        <span class="items-count">
          Машин на территории: {{ carsOnTerritory }}
          <button
            class="history-btn"
            @click="openCarsTableHistory"
          >
            История
          </button>
        </span>
        <RefreshButton
          :loading="refreshing"
          @refresh="loadData"
        />
      </div>
    </div>
    
    <div class="card-content">
      <div class="items-header">
        <div class="header-row">
          <!-- Въезд - отдельная колонка -->
          <div class="col entry-col">
            Въезд
          </div>
          <!-- Выезд - отдельная колонка -->
          <div class="col exit-col">
            Выезд
          </div>
          <div
            class="col number-col"
            @click="sortBy('car_number')"
          >
            <p :class="{ 'active-sort': sortField === 'car_number' }">
              Номер Т/С
            </p>
            <img
              src="@/assets/icons/sort.png"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'car_number', 'desc': sortField === 'car_number' && sortDirection === 'desc' }"
            >
          </div>
          <div
            class="col brand-col"
            @click="sortBy('car_brand')"
          >
            <p :class="{ 'active-sort': sortField === 'car_brand' }">
              Марка
            </p>
            <img
              src="@/assets/icons/sort.png"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'car_brand', 'desc': sortField === 'car_brand' && sortDirection === 'desc' }"
            >
          </div>
          <div
            class="col organization-col"
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
          <!-- Компания скрыта, но класс оставлен для возможного использования -->
          <div
            class="col company-col"
            style="display: none;"
          />
          <div
            class="col place-col"
            @click="sortBy('unload_place')"
          >
            <p :class="{ 'active-sort': sortField === 'unload_place' }">
              Место разгрузки
            </p>
            <img
              src="@/assets/icons/sort.png"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'unload_place', 'desc': sortField === 'unload_place' && sortDirection === 'desc' }"
            >
          </div>
          <div
            class="col date-col"
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
            class="col time-col"
            @click="sortBy('entry_time')"
          >
            <p :class="{ 'active-sort': sortField === 'entry_time' }">
              Время
            </p>
            <img
              src="@/assets/icons/sort.png"
              class="sort-icon"
              :class="{ 'sorted': sortField === 'entry_time', 'desc': sortField === 'entry_time' && sortDirection === 'desc' }"
            >
          </div>
          <div
            class="col status-col"
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
          <div class="col actions-col" />
        </div>
      </div>
      
      <div class="items-container">
        <div
          v-if="isLoading"
          class="loading-message"
        >
          <LoaderSpinner label="Загрузка машин…" />
        </div>
        
        <div
          v-else-if="displayItems.length > 0"
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
              :style="{ animationDelay: `${index * 0.05}s` }"
              @click="openVehicleDetails(item)"
            >
              <div class="item-data">
                <!-- Въезд - кнопка -->
                <div
                  class="col entry-col"
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
                <!-- Выезд - кнопка -->
                <div
                  class="col exit-col"
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
                <div class="col number-col">
                  {{ item.car_number }}
                </div>
                <div class="col brand-col">
                  {{ item.car_brand }}
                </div>
                <div class="col organization-col">
                  {{ item.organization_name }}
                </div>
                <!-- Компания скрыта -->
                <div
                  class="col company-col"
                  style="display: none;"
                />
                <div class="col place-col">
                  {{ formatUnloadPlaces(item) }}
                </div>
                <div class="col date-col">
                  {{ formatDate(item.entry_date_to) }}
                </div>
                <div class="col time-col">
                  {{ formatTimeRange(item.entry_time_from, item.entry_time_to) }}
                </div>
                <div class="col status-col">
                  <StatusBadge :status="item.status" />
                </div>
                <div
                  class="col actions-col"
                  @click.stop
                >
                  <button
                    class="delete-btn"
                    :disabled="isLoading"
                    @click="removeItemWithNotification(item)"
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
        
        <div
          v-else
          class="no-data-message"
        >
          {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Нет активных автомобилей' }}
        </div>
      </div>
    </div>

    <!-- Модальное окно с деталями автомобиля -->
    <VehicleDetailsModal
      :show="showVehicleDetails"
      :vehicle="selectedVehicle"
      :all-unloading-places="allUnloadingPlaces"
      :license-plate-formats="licensePlateFormats"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      :show-car-features="true"
      :source="'carstable'"
      @close="closeVehicleDetails"
      @open-application="$emit('open-application', $event)"
    />

    <!-- Модальное окно истории всех машин -->
    <CarsTableHistoryModal
      v-if="showCarsTableHistory"
      :cars="itemsData"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      @close="showCarsTableHistory = false"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useDeletionsStore } from '@/stores/deletions';
import RefreshButton from './RefreshButton.vue';
import VehicleDetailsModal from './CreateApplication/VehicleDetailsModal.vue';
import CarsTableHistoryModal from './CarsTableHistoryModal.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import StatusBadge from '@/components/ui/StatusBadge.vue';

export default {
  name: 'CarsTable',
  components: {
    RefreshButton,
    VehicleDetailsModal,
    CarsTableHistoryModal,
    LoaderSpinner,
    StatusBadge
  },
  props: {
    tableName: { type: String, default: '' },
    tableId: { type: Number, default: null },
    searchQuery: { type: String, default: '' },
    selectedOrganizationId: { type: [Number, String], default: null },
    selectedUnloadingPlaceId: { type: [Number, String], default: null },
    dateRangeStart: { type: Date, default: null },
    dateRangeEnd: { type: Date, default: null },
    selectedDate: { type: Date, default: null },
    currentUserId: { type: Number, default: null },
    currentUserName: { type: String, default: '' }
  },
  data() {
    return {
      sortField: null,
      sortDirection: 'desc',
      itemsData: [],
      pendingDeleteIds: [],
      isLoading: false,
      refreshing: false,
      organizationsMap: {},
      carUnloadPlacesMap: {},
      allUnloadingPlaces: [],
      licensePlateFormats: [],
      showVehicleDetails: false,
      selectedVehicle: null,
      showCarsTableHistory: false,
      pollingInterval: null
    };
  },
  computed: {
    displayItems() {
      let filtered = this.itemsData.filter(i => !this.pendingDeleteIds.includes(i.id));
      if (this.searchQuery) {
        const query = this.searchQuery.toLowerCase().trim();
        filtered = filtered.filter(item => {
          const searchFields = [
            item.car_number,
            item.car_brand,
            item.organization_name,
            item.company,
            this.formatUnloadPlaces(item),
            this.formatDate(item.entry_date_to),
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
      if (this.selectedUnloadingPlaceId) {
        filtered = filtered.filter(item => {
          const carId = item.id;
          const unloadPlaces = this.carUnloadPlacesMap[carId] || [];
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
      if (this.sortField) {
        filtered.sort((a, b) => {
          let valueA, valueB;
          switch (this.sortField) {
            case 'car_number':
            case 'car_brand':
            case 'organization':
            case 'company':
            case 'status':
              valueA = (a[this.sortField] || '').toString().toLowerCase();
              valueB = (b[this.sortField] || '').toString().toLowerCase();
              break;
            case 'unload_place':
              valueA = this.formatUnloadPlaces(a).toLowerCase();
              valueB = this.formatUnloadPlaces(b).toLowerCase();
              break;
            case 'entry_date_to':
              valueA = a.entry_date_to ? new Date(a.entry_date_to) : new Date(0);
              valueB = b.entry_date_to ? new Date(b.entry_date_to) : new Date(0);
              break;
            case 'entry_time':
              valueA = this.extractStartTime(a.entry_time_from);
              valueB = this.extractStartTime(b.entry_time_from);
              break;
            default:
              return 0;
          }
          if (valueA < valueB) return this.sortDirection === 'asc' ? -1 : 1;
          if (valueA > valueB) return this.sortDirection === 'asc' ? 1 : -1;
          return 0;
        });
      }
      return filtered;
    },
    carsOnTerritory() {
      return this.itemsData.filter(item => item.entry_checked && !item.exit_checked).length;
    },
    hasActiveFilters() {
      return !!(
        this.searchQuery ||
        this.selectedOrganizationId ||
        this.selectedUnloadingPlaceId ||
        this.selectedDate ||
        (this.dateRangeStart && this.dateRangeEnd)
      );
    }
  },
  watch: {
    tableName: {
      immediate: true,
      handler(newVal) {
        if (newVal) {
          this.stopPolling();
          this.startPolling();
        }
      }
    }
  },
  mounted() {
    this.startPolling();
    // Подгружаем настроенные длительности уведомлений после авторизации
    // (на холодном старте App.vue запрос мог уйти до получения токена).
    useDeletionsStore().loadDurations();
  },
  beforeUnmount() {
    this.stopPolling();
  },
  methods: {
    // Основной метод загрузки данных с флагом silent (без показа лоадера)
    async _loadData(silent = false) {
      if (!silent && this.isLoading) return;
      if (!silent) this.isLoading = true;
      try {
        await this.fetchUnloadingPlaces();
        await this.fetchLicensePlateFormats();
        await this.fetchCarsData();
        await this.fetchCarUnloadPlaces();
        await this.fetchCarHistoryStatus();
      } catch (error) {
        console.error('Ошибка при загрузке машин:', error);
      } finally {
        if (!silent) this.isLoading = false;
      }
    },

    // Для внешнего вызова (кнопка Refresh) - тихо, без скачка высоты таблицы.
    async loadData() {
      this.refreshing = true;
      try {
        await this._loadData(true);
      } finally {
        this.refreshing = false;
      }
    },

    // Для тихого обновления по таймеру
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

    async fetchCarsData() {
      try {
        const response = await apiRequest("/cars/active-for-tables", {});
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
        const cars = await response.json();
        await this.fetchOrganizations();
        const nameToIdMap = {};
        Object.keys(this.organizationsMap).forEach(id => {
          nameToIdMap[this.organizationsMap[id]] = id;
        });
        const regularCars = cars.filter(car => {
          if (car.status !== 1) return false;
          const carNumber = car.car_number?.toLowerCase().trim();
          return carNumber !== 'по факту';
        });
        // Преобразуем в нужный формат
        const newItems = regularCars.map(car => {
          const orgName = car.organization || '';
          const orgId = nameToIdMap[orgName] || car.organization_id;
          return {
            id: car.id,
            car_number: car.car_number || '',
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
            checked: false,
            entry_checked: false,
            exit_checked: false,
            entry_time: null,
            exit_time: null,
            applicationId: car.application_id,
            territory_status: car.territory_status || 0,
            plateNumber: car.car_number,
            mark: car.car_brand,
            formatId: null,
            unloadPlaces: car.unload_place_ids || []
          };
        });
        // Заменяем массив целиком – Vue отреагирует оптимально
        this.itemsData = newItems;
      } catch (error) {
        console.error("Ошибка при загрузке данных машин:", error);
        this.itemsData = [];
        throw error;
      }
    },

    async fetchCarHistoryStatus() {
      try {
        const response = await apiRequest("/cars/history/current-status", {});
        if (response.ok) {
          const statuses = await response.json();
          const statusMap = {};
          statuses.forEach(status => { statusMap[status.car_id] = status; });
          this.itemsData.forEach(item => {
            const status = statusMap[item.id];
            if (status) {
              item.entry_checked = status.territory_status === 1;
              item.exit_checked = status.territory_status === 2;
              item.entry_time = status.entry_time;
              item.exit_time = status.last_exit_time;
            }
          });
        }
      } catch (error) {
        console.error("Ошибка при загрузке статусов въезда/выезда:", error);
      }
    },

    async fetchCarUnloadPlaces() {
      try {
        const response = await apiRequest("/cars/unload-places", {});
        if (response.ok) {
          const carUnloadPlaces = await response.json();
          this.carUnloadPlacesMap = {};
          carUnloadPlaces.forEach(cup => {
            if (!this.carUnloadPlacesMap[cup.car_id]) this.carUnloadPlacesMap[cup.car_id] = [];
            this.carUnloadPlacesMap[cup.car_id].push({
              id: cup.unload_place_id,
              name: cup.unload_place_name || `Место #${cup.unload_place_id}`
            });
            const car = this.itemsData.find(c => c.id === cup.car_id);
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
        console.error("Ошибка при загрузке связей машин с местами разгрузки:", error);
        this.carUnloadPlacesMap = {};
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
      try {
        const [year, month, day] = dateString.split('-');
        const date = new Date(year, month - 1, day);
        return date.toLocaleDateString('ru-RU');
      } catch {
        return '';
      }
    },

    formatTimeRange(timeFrom, timeTo) {
      if (!timeFrom && !timeTo) return '-';
      const formatTime = (timeStr) => {
        if (!timeStr) return '';
        const parts = timeStr.split(':');
        if (parts.length >= 2) return `${parts[0]}:${parts[1]}`;
        return timeStr;
      };
      const formattedTimeFrom = formatTime(timeFrom);
      const formattedTimeTo = formatTime(timeTo);
      if (!formattedTimeTo) return formattedTimeFrom;
      if (!formattedTimeFrom) return formattedTimeTo;
      return `${formattedTimeFrom} - ${formattedTimeTo}`;
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
      const parts = timeString.split(':');
      if (parts.length >= 2) {
        const hours = parseInt(parts[0]) || 0;
        const minutes = parseInt(parts[1]) || 0;
        return hours * 60 + minutes;
      }
      return 0;
    },

    async handleEntryExit(item, type) {
      if (!this.currentUserId) return;
      try {
        let territory_status = type === 'entry' ? 1 : 2;
        const response = await apiRequest(`/cars/${item.id}/territory-status`, {
          method: "PUT",
          body: JSON.stringify({ territory_status, user_id: this.currentUserId })
        });
        if (response.ok) {
          const index = this.itemsData.findIndex(i => i.id === item.id);
          if (index !== -1) {
            const updatedItem = { ...this.itemsData[index] };
            if (type === 'entry') {
              updatedItem.entry_checked = true;
              updatedItem.exit_checked = false;
            } else {
              updatedItem.entry_checked = false;
              updatedItem.exit_checked = true;
            }
            this.itemsData.splice(index, 1, updatedItem);
          }
          this.showNotification(`Машина ${item.car_number} ${type === 'entry' ? 'отмечена о прибытии' : 'уехала'}`, 'success');
        } else {
          const errorText = await response.text();
          console.error('Ошибка при обновлении статуса:', errorText);
          this.showNotification(`Ошибка: ${errorText}`, 'error');
        }
      } catch (error) {
        console.error('Ошибка сети:', error);
        this.showNotification('Ошибка сети', 'error');
      }
    },

    removeItemWithNotification(item) {
      if (this.isLoading) return;
      if (this.pendingDeleteIds.includes(item.id)) return;
      const carId = item.id;
      const tableId = this.tableId;
      const userId = this.currentUserId;
      // Прячем строку через displayItems-фильтр (устойчиво к polling), пока идёт окно отмены.
      this.pendingDeleteIds.push(carId);
      useDeletionsStore().enqueue({
        prefix: 'Машина ',
        bold: item.car_number,
        suffix: ' удалена',
        onConfirm: () => this.commitDelete(carId, tableId, userId),
        onUndo: () => this.unhidePending(carId),
      });
    },

    unhidePending(carId) {
      this.pendingDeleteIds = this.pendingDeleteIds.filter(id => id !== carId);
    },

    async commitDelete(carId, tableId, userId) {
      try {
        const response = await apiRequest(`/cars/${carId}/deactivate`, {
          method: "PUT",
          body: JSON.stringify({ status: 0, user_id: userId, table_id: tableId })
        });
        if (!response.ok) {
          console.error("Ошибка при удалении");
          this.unhidePending(carId);
          return;
        }
        await this._loadData(true);
        this.unhidePending(carId);
      } catch (error) {
        console.error("Ошибка сети при удалении:", error);
        this.unhidePending(carId);
      }
    },

    openVehicleDetails(item) {
      this.selectedVehicle = {
        ...item,
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
        exit_checked: item.exit_checked
      };
      this.showVehicleDetails = true;
    },

    closeVehicleDetails() {
      this.showVehicleDetails = false;
      this.selectedVehicle = null;
    },

    openCarsTableHistory() {
      this.showCarsTableHistory = true;
    },

    showNotification(message, type = 'success') {
      console.log(`[${type}] ${message}`);
    },

    startPolling() {
      if (this.pollingInterval) return;
      this.silentRefresh(); // сразу загружаем без лоадера
      this.pollingInterval = setInterval(() => {
        this.silentRefresh();
      }, 10000); // каждые 10 секунд
    },

    stopPolling() {
      if (this.pollingInterval) {
        clearInterval(this.pollingInterval);
        this.pollingInterval = null;
      }
    }
  }
};
</script>

<style scoped>
/* Стили остаются без изменений (как в оригинале) */
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
  padding: 0px 20px;
  height: 50px;
  flex-shrink: 0;
}

.card-header__title {
  display: flex;
  gap: 12px;
  align-items: center;
}

.card-header__settings {
  display: flex;
  gap: 12px;
  align-items: center;
}

.card-title {
  margin: 0;
  color: #000;
  font-weight: 600;
  font-size: 1.1em;
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

.items-header {
  padding: 10px 16px;
  border-bottom: 1px solid #e6e6e6;
  flex-shrink: 0;
}

.header-row {
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

.entry-col { width: 6.5%; }
.exit-col { width: 8%; }
.number-col { width: 10%; }
.brand-col { width: 8.5%; }
.organization-col { width: 18%; }
.company-col { width: 0%; }
.place-col { width: 15%; }
.date-col { width: 11.5%; }
.time-col { width: 10%; }
.status-col { width: 7%; }
.actions-col { width: 2%; }

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

@media (max-width: 768px) {
  .selected-table-card {
    max-height: none;
    height: auto;
  }

  /* Синхронный horizontal scroll - scroll на .card-content */
  .card-content {
    overflow-x: auto !important;
    overflow-y: visible !important;
  }

  .items-header,
  .items-body {
    overflow: visible !important;
    min-width: 900px;
  }

  .header-row,
  .item-data {
    flex-wrap: nowrap !important;
    gap: 0;
    min-width: 900px;
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

  .col.number-col,
  .col.brand-col,
  .col.organization-col,
  .col.company-col,
  .col.place-col {
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
</style>