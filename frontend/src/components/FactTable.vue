<template>
  <div class="fact-table-card">
    <div class="card-header">
      <div class="card-header__title">
        <h3 class="card-title">{{ tableType === 'cars' ? 'Автомобили' : 'Люди' }} <span class="highlight-text">по факту</span></h3>
      </div>
      <div class="card-header__settings">
        <RefreshButton @refresh="fetchData" />
      </div>
    </div>
    
    <div class="card-content">
      <!-- Заголовок таблицы -->
      <div class="fact-header">
        <div class="header-row">
          <div class="header-col checkbox-col" v-if="tableType === 'cars'">
            <!-- Пустой заголовок для чекбокса -->
          </div>
          <div class="header-col organization-col" @click="sortBy('organization')">
            <p :class="{ 'active-sort': sortField === 'organization' }">Организация</p>
            <img 
              src="@/assets/icons/sort.png" 
              class="sort-icon" 
              :class="{ 
                'sorted': sortField === 'organization',
                'desc': sortField === 'organization' && sortDirection === 'desc'
              }" 
            />
          </div>
          <div class="header-col place-col" @click="sortBy('unload_place')" v-if="tableType === 'cars'">
            <p :class="{ 'active-sort': sortField === 'unload_place' }">Место разгрузки</p>
            <img 
              src="@/assets/icons/sort.png" 
              class="sort-icon" 
              :class="{ 
                'sorted': sortField === 'unload_place',
                'desc': sortField === 'unload_place' && sortDirection === 'desc'
              }" 
            />
          </div>
          <div class="header-col date-col" @click="sortBy('entry_date_to')">
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
          <div class="header-col time-col" @click="sortBy('entry_time')">
            <p :class="{ 'active-sort': sortField === 'entry_time' }">{{ tableType === 'cars' ? 'Время' : 'Время прохода' }}</p>
            <img 
              src="@/assets/icons/sort.png" 
              class="sort-icon" 
              :class="{ 
                'sorted': sortField === 'entry_time',
                'desc': sortField === 'entry_time' && sortDirection === 'desc'
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
          <div class="header-col actions-col">
            <!-- Пустой заголовок для действий -->
          </div>
        </div>
      </div>
      
      <!-- Тело таблицы -->
      <div class="fact-container">
        <div v-if="filteredData.length > 0" class="fact-body">
          <transition-group name="fade-list" tag="div">
            <div 
              v-for="(item, index) in sortedData" 
              :key="item.id" 
              class="fact-item"
              :style="{ animationDelay: `${index * 0.1}s` }"
            >
              <div class="fact-row">
                <div class="fact-col checkbox-col" v-if="tableType === 'cars'">
                  <input 
                    type="checkbox" 
                    v-model="item.checked"
                    class="checkbox-input"
                  />
                </div>
                <div class="fact-col organization-col">
                  {{ item.organization_name }}
                </div>
                <div class="fact-col place-col" v-if="tableType === 'cars'">
                  {{ item.unload_place || '-' }}
                </div>
                <div class="fact-col date-col">
                  {{ formatDate(item.entry_date_to) }}
                </div>
                <div class="fact-col time-col">
                  {{ tableType === 'cars' 
                    ? formatTimeRange(item.entry_time_from, item.entry_time_to)
                    : formatPassTime(item.pass_time)
                  }}
                </div>
                <div class="fact-col status-col">
                  <span class="status-text">
                    {{ item.status }}
                  </span>
                </div>
                <div class="fact-col actions-col">
                  <button 
                    @click="deleteItem(item)" 
                    class="delete-btn"
                  >
                    <img 
                      src="@/assets/icons/trashcan.png" 
                      alt="Удалить" 
                      class="delete-icon"
                    />
                  </button>
                </div>
              </div>
            </div>
          </transition-group>
        </div>
        <p v-else class="no-data-message">
          {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : `Заявок ${tableType === 'cars' ? 'на машины' : 'на людей'} по факту нет` }}
        </p>
      </div>
    </div>
  </div>
</template>

<script>
import RefreshButton from './RefreshButton.vue';

export default {
  components: {
    RefreshButton
  },
  props: {
    tableType: {
      type: String,
      default: 'cars',
      validator: (value) => ['cars', 'people'].includes(value)
    },
    searchQuery: {
      type: String,
      default: ''
    },
    selectedOrganizationId: {
      type: [Number, String],
      default: null
    },
    selectedUnloadingPlaceId: {
      type: [Number, String],
      default: null
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
    }
  },
  data() {
    return {
      sortField: null,
      sortDirection: 'desc',
      factData: [],
      organizationsMap: {},
      factCarUnloadPlacesMap: {}
    };
  },
  computed: {
    filteredData() {
      let filtered = [...this.factData];

      // Поиск по всем полям
      if (this.searchQuery) {
        const query = this.searchQuery.toLowerCase();
        filtered = filtered.filter(item => 
          item.organization_name.toLowerCase().includes(query) ||
          (this.tableType === 'cars' && (item.unload_place || '-').toLowerCase().includes(query)) ||
          item.status.toLowerCase().includes(query) ||
          (this.tableType === 'cars' 
            ? this.formatTimeRange(item.entry_time_from, item.entry_time_to).toLowerCase().includes(query)
            : this.formatPassTime(item.pass_time).toLowerCase().includes(query)
          ) ||
          this.formatDate(item.entry_date_to).toLowerCase().includes(query)
        );
      }

      // Фильтр по организации (теперь по organization_id)
      if (this.selectedOrganizationId) {
        filtered = filtered.filter(item => item.organization_id == this.selectedOrganizationId);
      }

      // Фильтр по месту разгрузки (только для машин, по ID из car_unload_places)
      if (this.selectedUnloadingPlaceId && this.tableType === 'cars') {
        filtered = filtered.filter(item => {
          const carId = item.id;
          const unloadPlaces = this.factCarUnloadPlacesMap[carId] || [];
          return unloadPlaces.some(place => place.id == this.selectedUnloadingPlaceId);
        });
      }

      // Фильтр по дате
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
      
      if (!this.sortField) {
        return data;
      }
      
      return data.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'organization':
          case 'status':
            valueA = a[this.sortField]?.toLowerCase() || '';
            valueB = b[this.sortField]?.toLowerCase() || '';
            break;
            
          case 'unload_place':
            valueA = a.unload_place?.toLowerCase() || '';
            valueB = b.unload_place?.toLowerCase() || '';
            break;
            
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
    },

    hasActiveFilters() {
      return !!(
        this.searchQuery ||
        this.selectedOrganizationId ||
        (this.tableType === 'cars' && this.selectedUnloadingPlaceId) ||
        this.selectedDate ||
        (this.dateRangeStart && this.dateRangeEnd)
      );
    }
  },
  methods: {
    async fetchData() {
      try {
        await this.fetchOrganizations();
        
        if (this.tableType === 'cars') {
          await this.fetchCarsData();
          await this.fetchFactCarUnloadPlaces();
        } else {
          await this.fetchPeopleData();
        }
      } catch (error) {
        console.error(`Ошибка при загрузке данных по факту (${this.tableType}):`, error);
      }
    },

    async fetchOrganizations() {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch("http://localhost:8080/organizations", {
          method: "GET",
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });

        if (response.ok) {
          const data = await response.json();
          this.organizationsMap = {};
          data.forEach(org => {
            this.organizationsMap[org.id] = org.name;
          });
        } else {
          console.error("Ошибка при загрузке организаций");
        }
      } catch (error) {
        console.error("Ошибка сети при загрузке организаций:", error);
      }
    },

    getOrganizationName(organizationId) {
      if (!organizationId) return 'Не указана';
      return this.organizationsMap[organizationId] || `Организация ID: ${organizationId}`;
    },

    async fetchCarsData() {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch("http://localhost:8080/cars/fact-for-tables", {
          method: "GET",
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });
        
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const factCars = await response.json();
        console.log('Получены машины "по факту":', factCars);
        
        const nameToIdMap = {};
        Object.keys(this.organizationsMap).forEach(id => {
          nameToIdMap[this.organizationsMap[id]] = id;
        });
        
        this.factData = factCars.map(car => {
          const orgName = car.organization || '';
          const orgId = nameToIdMap[orgName] || car.organization_id;
          
          return {
            id: car.id,
            organization_id: orgId,
            organization_name: orgName || 'Не указана',
            unload_place: car.unload_place || '-',
            entry_date_to: car.entry_date_to || '',
            entry_time_from: car.entry_time_from || '',
            entry_time_to: car.entry_time_to || '',
            status: 'В работе',
            checked: false,
            applicationId: car.application_id
          };
        });
        
      } catch (error) {
        console.error("Ошибка при загрузке данных по факту:", error);
      }
    },

    async fetchFactCarUnloadPlaces() {
      try {
        const token = localStorage.getItem("token");
        
        const response = await fetch("http://localhost:8080/cars/fact-unload-places", {
          method: "GET",
          headers: {
            "Authorization": `Bearer ${token}`,
          }
        });
        
        if (response.ok) {
          const carUnloadPlaces = await response.json();
          
          this.factCarUnloadPlacesMap = {};
          
          carUnloadPlaces.forEach(cup => {
            if (!this.factCarUnloadPlacesMap[cup.car_id]) {
              this.factCarUnloadPlacesMap[cup.car_id] = [];
            }
            this.factCarUnloadPlacesMap[cup.car_id].push({
              id: cup.unload_place_id,
              name: cup.unload_place_name || `Место #${cup.unload_place_id}`
            });
          });
          
          console.log('Загружены связи факт-машин с местами разгрузки:', this.factCarUnloadPlacesMap);
        } else {
          console.error("Ошибка при загрузке связей факт-машин с местами разгрузки");
          this.factCarUnloadPlacesMap = {};
        }
      } catch (error) {
        console.error("Ошибка сети при загрузке связей факт-машин с местами разгрузки:", error);
        this.factCarUnloadPlacesMap = {};
      }
    },

    async fetchPeopleData() {
      // Заглушка для данных о людях (аналогично нужно будет адаптировать API)
      this.factData = [
        {
          id: 1,
          organization_id: 1,
          organization_name: 'ООО "Ромашка"',
          entry_date_to: '2024-12-31',
          pass_time: '08:00-17:00',
          status: 'Активен'
        },
        {
          id: 2,
          organization_id: 2,
          organization_name: 'ИП Иванов',
          entry_date_to: '2024-11-30',
          pass_time: '09:00-18:00',
          status: 'Активен'
        }
      ];
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
        if (timeStr.includes(':') && timeStr.split(':').length === 3) {
          return timeStr.substring(0, 5);
        }
        return timeStr;
      };

      const formattedTimeFrom = formatTime(timeFrom);
      const formattedTimeTo = formatTime(timeTo);
      
      if (!formattedTimeTo) return formattedTimeFrom;
      if (!formattedTimeFrom) return formattedTimeTo;
      return `${formattedTimeFrom} - ${formattedTimeTo}`;
    },

    formatPassTime(passTime) {
      return passTime || '-';
    },
    
    async deleteItem(item) {
      try {
        if (this.tableType === 'cars') {
          const response = await fetch(`http://localhost:8080/applications/${item.applicationId}/cars/${item.id}`, {
            method: "PUT",
            headers: { 
              "Content-Type": "application/json",
              "Authorization": `Bearer ${localStorage.getItem("token")}`
            },
            body: JSON.stringify({ 
              car: {
                ...item,
                status: 0
              },
              unload_places: item.unload_places || []
            })
          });
          
          if (response.ok) {
            this.fetchData();
          }
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
    }
  },
  mounted() {
    this.fetchData();
    
    setTimeout(() => {
      document.querySelectorAll('.fact-item').forEach(item => {
        item.classList.add('animate-in');
      });
    }, 100);
  },
  watch: {
    tableType: {
      handler() {
        this.fetchData();
      },
      immediate: true
    },
    selectedOrganizationId(newVal) {
      console.log('Фильтр FactTable по организации ID:', newVal);
    },
    selectedUnloadingPlaceId(newVal) {
      console.log('Фильтр FactTable по месту разгрузки ID:', newVal);
    },
    searchQuery(newVal) {
      console.log('Поисковый запрос FactTable:', newVal);
    }
  }
};
</script>

<style scoped>
/* Стили остаются без изменений */
.fact-table-card {
  background-color: #fff;
  border-radius: 30px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 100%;
  min-height: 222px;
  max-height: 222px;
  box-shadow: 0 3px 10px rgba(0,0,0,0.05);
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

.fact-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow-y: auto;
}

/* Заголовок таблицы */
.fact-header {
  border-bottom: 1px solid #e6e6e6;
  padding: 12px 16px;
  flex-shrink: 0;
}

.header-row {
  display: flex;
  width: 100%;
  align-items: center;
}

.header-col {
  font-weight: 500;
  color: #a2a2a2;
  text-align: left;
  padding: 0 0px;
  font-size: 14px;
  display: flex;
  align-items: center;
  gap: 5px;
  transition: .2s;
  cursor: pointer;
  user-select: none;
  height: 20px;
  box-sizing: border-box;
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
.checkbox-col {
  width: 4%;
  min-width: 25px;
  justify-content: center;
}

.organization-col {
  width: 35%;
  min-width: 95px;
}

.place-col {
  width: 30%;
  min-width: 115px;
}

.date-col {
  width: 22%;
  min-width: 90px;
}

.time-col {
  width: 18%;
  min-width: 90px;
}

.status-col {
  width: 15%;
  min-width: 90px;
}

.actions-col {
  width: 2%;
  min-width: 40px;
  justify-content: center;
}

/* Тело таблицы */
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
  animation: fadeInUp 0.5s ease forwards;
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

.fact-item:hover {
  background-color: #fafafa;
}

.fact-row {
  display: flex;
  width: 100%;
  padding: 10px 16px;
  align-items: center;
  border-top: 1px solid #f0f0f0;
}

.fact-col {
  padding: 0 8px;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  display: flex;
  align-items: center;
  height: 100%;
  box-sizing: border-box;
}

/* Выравнивание содержимого колонок */
.checkbox-col .fact-col,
.actions-col .fact-col {
  justify-content: center;
}

.checkbox-input {
  width: 13px;
  height: 13px;
  cursor: pointer;
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
  border-radius: 4px;
  transition: background-color 0.2s ease;
}

.delete-btn:hover {
  background-color: #f5f5f5;
}

.delete-icon {
  width: 16px;
  height: 16px;
  opacity: 0.7;
  transition: opacity 0.2s ease;
}

.delete-btn:hover .delete-icon {
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

/* Стилизация скроллбара */
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

/* Анимация для списка */
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
  }
  
  .header-row,
  .fact-row {
    flex-wrap: wrap;
  }
  
  .header-col,
  .fact-col {
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