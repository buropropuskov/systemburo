<template>
  <div class="applications-card">
    <div class="card-header">
      <div class="card-header__title">
        <h3 class="card-title">Заявки организации / отдела</h3>
        <p class="card-organization">{{ organization }}</p>
      </div>
      <div class="card-header__settings">
        <SearchComponent
          :title="'Поиск заявок..'"
          v-model="searchQuery"
        />
        <RefreshButton @refresh="$emit('refresh-applications')" />
      </div>
    </div>
    
    <div class="card-content">
      <div v-if="filteredApplications.length > 0" class="applications-container">
        <!-- Левая часть - таблица заявок -->
        <div class="applications-list" :class="{'with-details': selectedApplication}">
          <!-- Заголовок таблицы -->
          <div class="applications-header">
            <div class="header-row">
              <div class="header-col id-col" @click="sortBy('id')">
                <p :class="{ 'active-sort': sortField === 'id' }">Номер заявки</p>
                <img 
                  src="@/assets/icons/sort.png" 
                  class="sort-icon" 
                  :class="{ 
                    'sorted': sortField === 'id',
                    'desc': sortField === 'id' && sortDirection === 'desc'
                  }" 
                />
              </div>
              <div class="header-col date-col" @click="sortBy('submission_datetime')">
                <p :class="{ 'active-sort': sortField === 'submission_datetime' }">Дата подачи</p>
                <img 
                  src="@/assets/icons/sort.png" 
                  class="sort-icon" 
                  :class="{ 
                    'sorted': sortField === 'submission_datetime',
                    'desc': sortField === 'submission_datetime' && sortDirection === 'desc'
                  }" 
                />
              </div>
              <div class="header-col period-col" @click="sortBy('entry_date_to')">
                <p :class="{ 'active-sort': sortField === 'entry_date_to' }">Действует до</p>
                <img 
                  src="@/assets/icons/sort.png" 
                  class="sort-icon" 
                  :class="{ 
                    'sorted': sortField === 'entry_date_to',
                    'desc': sortField === 'entry_date_to' && sortDirection === 'desc'
                  }" 
                />
              </div>
              <div class="header-col status-col" @click="sortBy('status')">
                <p :class="{ 'active-sort': sortField === 'status' }">Статус</p>
                <img 
                  src="@/assets/icons/sort.png" 
                  class="sort-icon" 
                  :class="{ 
                    'sorted': sortField === 'status',
                    'desc': sortField === 'status' && sortDirection === 'desc'
                  }" 
                />
              </div>
            </div>
          </div>
          
          <!-- Тело таблицы -->
          <div class="applications-body">
            <div 
              v-for="application in sortedApplications" 
              :key="application.id" 
              class="application-item"
              :class="{'selected': selectedApplication && selectedApplication.id === application.id}"
              @click="selectApplication(application)"
            >
              <div class="application-row">
                <div class="application-col id-col">
                  <span class="application-id">№ {{ formatApplicationNumber(application.id, application.submission_datetime) }}</span>
                </div>
                <div class="application-col date-col">
                  {{ formatDate(application.submission_datetime) }}
                </div>
                <div class="application-col period-col">
                  {{ formatEntryDate(application.entry_date_to) || '-' }}
                </div>
                <div class="application-col status-col">
                  <span 
                    class="status-badge" 
                    :class="{'active': isApplicationActive(application), 'inactive': !isApplicationActive(application)}"
                  >
                    {{ isApplicationActive(application) ? 'Активна' : 'Неактивна' }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
        
        <!-- Правая часть - детали выбранной заявки -->
        <div v-if="selectedApplication" class="application-details-panel">
          <div class="details-content">
            <div class="details-header">
              <div class="details-title-wrapper">
                <h3 class="details-title">Заявка № {{ formatApplicationNumber(selectedApplication.id, selectedApplication.submission_datetime) }}</h3>
                <span 
                  class="status-badge-lg" 
                  :class="{'active': isApplicationActive(selectedApplication), 'inactive': !isApplicationActive(selectedApplication)}"
                >
                  {{ isApplicationActive(selectedApplication) ? 'Активна' : 'Неактивна' }}
                </span>
              </div>
              <button @click="closeDetails" class="close-btn">×</button>
            </div>
            
            <div class="details-section">
              <div class="details-grid">
                <div class="detail-item">
                  <span class="detail-label">Организация:</span>
                  <span class="detail-value">{{ selectedApplication.organization }}</span>
                </div>
                <div class="detail-item">
                  <span class="detail-label">Ответственное лицо:</span>
                  <span class="detail-value">{{ selectedApplication.responsible_person }}</span>
                </div>
                <div class="detail-item">
                  <span class="detail-label">Контактный телефон:</span>
                  <span class="detail-value">{{ selectedApplication.contact_phone }}</span>
                </div>
                <div class="detail-item">
                  <span class="detail-label">Дата подачи:</span>
                  <span class="detail-value">{{ formatDateTime(selectedApplication.submission_datetime) }}</span>
                </div>
                <div class="detail-item" v-if="selectedApplication.entry_date_to">
                  <span class="detail-label">Действует до:</span>
                  <span class="detail-value">{{ formatEntryDate(selectedApplication.entry_date_to) }}</span>
                </div>
                <div class="detail-item" v-if="selectedApplication.entry_time_from || selectedApplication.entry_time_to">
                  <span class="detail-label">Время:</span>
                  <span class="detail-value">{{ formatTimeRange(selectedApplication.entry_time_from, selectedApplication.entry_time_to) }}</span>
                </div>
              </div>
            </div>

            <div class="cars-section">
              <h4 class="section-subtitle">Автомобили</h4>
              <div class="cars-table">
                <div class="cars-header">
                  <div class="car-col car-number">Номер</div>
                  <div class="car-col car-brand">Марка</div>
                  <div class="car-col unload-place">Место разгрузки</div>
                  <div class="car-col car-status">Статус</div>
                </div>
                <div class="cars-body">
                  <div 
                    v-for="car in selectedApplication.cars" 
                    :key="car.id" 
                    class="car-row"
                    :class="{ 'inactive-row': car.status === 0 }"
                  >
                    <div class="car-col car-number">{{ car.car_number }}</div>
                    <div class="car-col car-brand">{{ car.car_brand }}</div>
                    <div class="car-col unload-place">{{ formatUnloadPlaces(car.unload_places) }}</div>
                    <div class="car-col car-status">
                      <span class="status-badge" :class="car.status === 1 ? 'active' : 'inactive'">
                        {{ car.status === 1 ? 'Активен' : 'Неактивен' }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        
        <div v-else class="no-selection-message">
          <p>Выберите заявку для просмотра</p>
        </div>
      </div>
      <p v-else class="no-data-message">
        {{ searchQuery ? 'Заявки не найдены' : 'Организация / отдел ещё не делала заявки' }}
      </p>
    </div>
  </div>
</template>

<script>
import RefreshButton from './RefreshButton.vue';
import SearchComponent from './SearchComponent.vue';

export default {
  components: {
    RefreshButton,
    SearchComponent
  },
  props: {
    applications: {
      type: Array,
      required: true
    },
    organization: {
      type: String,
      required: true
    }
  },
  emits: ['refresh-applications'],
  data() {
    return {
      selectedApplication: null,
      searchQuery: '',
      sortField: null,
      sortDirection: 'desc',
      unloadingPlacesMap: new Map()
    };
  },
  computed: {
    userApplications() {
      return this.applications
        .filter(application => application.organization === this.organization);
    },
    filteredApplications() {
      if (!this.searchQuery) return this.userApplications;
      
      const searchTerm = this.searchQuery.toLowerCase();
      return this.userApplications.filter(application => {
        // Проверяем все основные поля заявки
        const mainFieldsMatch = 
          application.organization?.toLowerCase().includes(searchTerm) ||
          application.responsible_person?.toLowerCase().includes(searchTerm) ||
          application.contact_phone?.toLowerCase().includes(searchTerm) ||
          application.submission_datetime?.toLowerCase().includes(searchTerm) ||
          application.entry_date_to?.toLowerCase().includes(searchTerm) ||
          application.entry_time_from?.toLowerCase().includes(searchTerm) ||
          application.entry_time_to?.toLowerCase().includes(searchTerm) ||
          this.formatApplicationNumber(application.id, application.submission_datetime).toLowerCase().includes(searchTerm);
        
        // Проверяем все поля автомобилей
        const carsMatch = application.cars?.some(car => 
          car.car_number?.toLowerCase().includes(searchTerm) ||
          car.car_brand?.toLowerCase().includes(searchTerm) ||
          this.formatUnloadPlaces(car.unload_places)?.toLowerCase().includes(searchTerm) ||
          (car.status === 1 ? 'активен' : 'неактивен').includes(searchTerm)
        );
        
        return mainFieldsMatch || carsMatch;
      });
    },
    sortedApplications() {
      const applications = [...this.filteredApplications];
      
      // Если сортировка не выбрана, возвращаем отсортированные по номеру заявки (по умолчанию)
      if (!this.sortField) {
        return applications.sort((a, b) => {
          const numA = this.parseApplicationNumber(a.id, a.submission_datetime);
          const numB = this.parseApplicationNumber(b.id, b.submission_datetime);
          
          // Сначала сравниваем даты (по убыванию - от новых к старым)
          if (numA.date !== numB.date) {
            return numB.date - numA.date;
          } else {
            // Если даты одинаковые, сравниваем номера (по убыванию)
            return numB.number - numA.number;
          }
        });
      }
      
      return applications.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'id': {
            // Специальная логика для сортировки по номеру заявки
            const numA = this.parseApplicationNumber(a.id, a.submission_datetime);
            const numB = this.parseApplicationNumber(b.id, b.submission_datetime);
            
            // Сначала сравниваем даты
            if (numA.date !== numB.date) {
              valueA = numA.date;
              valueB = numB.date;
            } else {
              // Если даты одинаковые, сравниваем номера
              valueA = numA.number;
              valueB = numB.number;
            }
            break;
          }
            
          case 'submission_datetime': {
            valueA = new Date(a.submission_datetime);
            valueB = new Date(b.submission_datetime);
            break;
          }
            
          case 'entry_date_to': {
            valueA = a.entry_date_to ? new Date(a.entry_date_to) : new Date(0);
            valueB = b.entry_date_to ? new Date(b.entry_date_to) : new Date(0);
            break;
          }
            
          case 'status': {
            valueA = this.isApplicationActive(a) ? 1 : 0;
            valueB = this.isApplicationActive(b) ? 1 : 0;
            break;
          }
            
          default:
            return 0;
        }
        
        // ИНВЕРТИРОВАННАЯ ЛОГИКА: теперь сортировка соответствует отображению иконки
        if (valueA < valueB) {
          return this.sortDirection === 'asc' ? 1 : -1;
        }
        if (valueA > valueB) {
          return this.sortDirection === 'asc' ? -1 : 1;
        }
        return 0;
      });
    }
  },
  methods: {
    async fetchUnloadingPlaces() {
      try {
        const response = await fetch("http://localhost:8080/unload-places");
        const places = await response.json();
        
        places.forEach(place => {
          this.unloadingPlacesMap.set(place.id, place.name);
        });
      } catch (error) {
        console.error("Ошибка при загрузке мест разгрузки:", error);
      }
    },

    isApplicationActive(application) {
      if (!application.entry_date_to) return false;
      
      const now = new Date();
      const endDate = new Date(application.entry_date_to);
      endDate.setHours(23, 59, 59, 999); // Устанавливаем конец дня
      
      return now <= endDate;
    },

    formatDate(dateString) {
      if (!dateString) return '';
      const date = new Date(dateString);
      return date.toLocaleDateString('ru-RU');
    },

    formatDateTime(dateString) {
      if (!dateString) return '';
      const date = new Date(dateString);
      return date.toLocaleString('ru-RU');
    },

    formatEntryDate(dateString) {
      if (!dateString) return '';
      const [year, month, day] = dateString.split('-');
      const date = new Date(year, month - 1, day);
      return date.toLocaleDateString('ru-RU');
    },

    formatTimeRange(timeFrom, timeTo) {
      if (!timeFrom && !timeTo) return '-';
      
      // Форматируем время, убирая секунды
      const formatTime = (timeStr) => {
        if (!timeStr) return '';
        // Если время в формате чч:мм:сс, обрезаем секунды
        if (timeStr.includes(':') && timeStr.split(':').length === 3) {
          return timeStr.substring(0, 5); // Берем только чч:мм
        }
        return timeStr;
      };

      const formattedTimeFrom = formatTime(timeFrom);
      const formattedTimeTo = formatTime(timeTo);
      
      if (!formattedTimeTo) return formattedTimeFrom;
      if (!formattedTimeFrom) return formattedTimeTo;
      return `${formattedTimeFrom} - ${formattedTimeTo}`;
    },

    formatUnloadPlaces(unloadPlaces) {
      if (!unloadPlaces || unloadPlaces.length === 0) return '-';
      
      // Получаем названия мест разгрузки
      const placeNames = unloadPlaces
        .map(place => {
          if (typeof place === 'object' && place.unload_place_name) {
            return place.unload_place_name;
          }
          // Если это ID, пытаемся найти название в кэше
          if (this.unloadingPlacesMap.has(place)) {
            return this.unloadingPlacesMap.get(place);
          }
          if (typeof place === 'object' && place.unload_place_id) {
            return this.unloadingPlacesMap.get(place.unload_place_id) || `Место ${place.unload_place_id}`;
          }
          return null;
        })
        .filter(name => name);
      
      if (placeNames.length === 0) return '-';
      if (placeNames.length === 1) return placeNames[0];
      
      return `${placeNames[0]} и др.`;
    },

    formatApplicationNumber(id, submissionDate) {
      const date = new Date(submissionDate);
      const datePart = date.toISOString().slice(0, 10).replace(/-/g, '');
      const numberPart = id.toString().padStart(3, '0');
      return `${datePart}/${numberPart}`;
    },

    parseApplicationNumber(id, submissionDate) {
      const date = new Date(submissionDate);
      return {
        date: date.getTime(),
        number: id
      };
    },

    selectApplication(application) {
      this.selectedApplication = application;
    },

    closeDetails() {
      this.selectedApplication = null;
    },

    sortBy(field) {
      if (this.sortField === field) {
        // Если уже сортируем по этому полю, меняем направление
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        // Если новое поле, устанавливаем его и направление по умолчанию (desc)
        this.sortField = field;
        this.sortDirection = 'desc';
      }
    }
  },
  mounted() {
    this.fetchUnloadingPlaces();
  }
};
</script>

<style scoped>
.applications-card {
  background-color: #fff;
  border-radius: 30px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 100%;
  box-shadow: 0 3px 10px rgba(0,0,0,0.05);
}

.card-header {
  border-bottom: 1px solid #e6e6e6;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0px 20px;
  height: 50px;
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

.card-organization {
  font-size: 14px;
  color: #a2a2a2;
  margin: 0;
}

.card-title {
  margin: 0;
  color: #000;
  font-weight: 600;
  font-size: 1.1em;
}

.card-content {
  padding: 0;
}

.applications-container {
  display: flex;
  height: fit-content;
  max-height: 292px;
  width: 100%;
}

/* Левая часть - таблица заявок */
.applications-list {
  width: 100%;
  display: flex;
  flex-direction: column;
  transition: width 0.3s ease;
  border-right: 1px solid #e6e6e6;
}

.applications-list.with-details {
  width: 50%;
}

/* Заголовок таблицы */
.applications-header {
  border-bottom: 1px solid #e6e6e6;
  padding: 12px 16px;
  flex-shrink: 0;
}

.header-row {
  display: flex;
  width: 100%;
}

.header-col {
  font-weight: 500;
  color: #a2a2a2;
  text-align: left;
  padding: 0 4px;
  font-size: 14px;
  display: flex;
  align-items: center;
  gap: 5px;
  transition: .2s;
  cursor: pointer;
  user-select: none;
}

.header-col:hover {
  color: #333;
}

.header-col:hover .sort-icon {
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

/* Колонки с фиксированной шириной */
.id-col {
  width: 30%;
  min-width: 100px;
}

.date-col {
  width: 25%;
  min-width: 100px;
}

.period-col {
  width: 25%;
  min-width: 100px;
}

.status-col {
  width: 20%;
  min-width: 90px;
}

/* Тело таблицы */
.applications-body {
  overflow-y: auto;
  flex-grow: 1;
}

.application-item {
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.application-item.selected {
  background-color: #f8f9ff;
}

.application-item:hover {
  background-color: #fafafa;
}

.application-row {
  display: flex;
  width: 100%;
  padding: 12px 16px;
  align-items: center;
}

.application-col {
  padding: 0 4px;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
}

.application-id {
  color: #4F5BDF;
  font-weight: 600;
}

.status-badge {
  display: inline-block;
  padding: 4px 8px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
  min-width: 70px;
  text-align: center;
}

.status-badge.active {
  background-color: #f0f9ff;
  color: #0369a1;
  border: 1px solid #bae6fd;
}

.status-badge.inactive {
  background-color: #fef2f2;
  color: #991b1b;
  border: 1px solid #fecaca;
}

.status-badge-lg {
  display: inline-block;
  padding: 6px 12px;
  border-radius: 14px;
  font-size: 13px;
  font-weight: 500;
  min-width: 80px;
  text-align: center;
  margin-left: 12px;
}

.status-badge-lg.active {
  background-color: #f0f9ff;
  color: #0369a1;
  border: 1px solid #bae6fd;
}

.status-badge-lg.inactive {
  background-color: #fef2f2;
  color: #991b1b;
  border: 1px solid #fecaca;
}

/* Правая часть - детали заявки */
.application-details-panel {
  width: 50%;
  padding: 20px;
  overflow-y: auto;
  flex-shrink: 0;
  background-color: #fafafa;
}

.no-selection-message {
  width: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a2a2a2;
  font-weight: 400;
  flex-shrink: 0;
  font-size: 14px;
}

.details-content {
  height: 100%;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
  padding-bottom: 16px;
  border-bottom: 1px solid #e6e6e6;
}

.details-title-wrapper {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.details-title {
  margin: 0;
  color: #1a1a1a;
  font-size: 1.2em;
  font-weight: 600;
}

.close-btn {
  background: none;
  border: none;
  font-size: 20px;
  cursor: pointer;
  color: #999;
  padding: 0;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: 4px;
  transition: background-color 0.2s ease;
}

.close-btn:hover {
  background-color: #f0f0f0;
  color: #666;
}

.details-section {
  margin-bottom: 24px;
}

.section-subtitle {
  margin: 0 0 16px 0;
  font-size: 1em;
  color: #4F5BDF;
  font-weight: 600;
}

.details-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}

.detail-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.detail-label {
  font-weight: 500;
  color: #a2a2a2;
  font-size: 13px;
}

.detail-value {
  color: #1a1a1a;
  font-size: 14px;
  font-weight: 500;
}

/* Таблица автомобилей */
.cars-section {
  margin-top: 24px;
  padding-bottom: 20px;
}

.cars-table {
  width: 100%;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  overflow: hidden;
  background-color: #fff;
}

.cars-header {
  display: flex;
  background-color: #fafafa;
  padding: 12px 16px;
  font-weight: 600;
  color: #666;
  border-bottom: 1px solid #e6e6e6;
  font-size: 13px;
}

.cars-body {
  max-height: 200px;
  overflow-y: auto;
}

.car-row {
  display: flex;
  padding: 12px 16px;
  border-bottom: 1px solid #f0f0f0;
  align-items: center;
  transition: background-color 0.2s ease;
}

.car-row:hover {
  background-color: #fafafa;
}

.car-row:last-child {
  border-bottom: none;
}

.car-col {
  padding: 0 4px;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}

.car-number {
  width: 25%;
}

.car-brand {
  width: 25%;
}

.unload-place {
  width: 30%;
}

.car-status {
  width: 20%;
}

.inactive-row {
  opacity: 0.6;
}

.inactive-row .status-badge {
  opacity: 0.7;
}

.no-data-message {
  text-align: center;
  color: #a2a2a2;
  padding: 40px 20px;
  margin: 0;
  font-size: 14px;
}

@media (max-width: 768px) {
  .applications-container {
    flex-direction: column;
    height: auto;
  }
  
  .applications-list,
  .application-details-panel,
  .no-selection-message {
    width: 100% !important;
  }
  
  .applications-list.with-details {
    border-right: none;
    border-bottom: 1px solid #e6e6e6;
    height: 300px;
  }
  
  .header-row,
  .application-row {
    flex-wrap: wrap;
  }
  
  .header-col,
  .application-col {
    width: 50% !important;
    margin-bottom: 4px;
  }
  
  .details-title-wrapper {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  
  .status-badge-lg {
    margin-left: 0;
  }
  
  .cars-header,
  .car-row {
    flex-wrap: wrap;
  }
  
  .car-col {
    width: 50% !important;
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
}
</style>