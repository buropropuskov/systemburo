<template>
  <div class="cars-fact-card">
    <div class="card-header">
      <div class="card-header__title">
        <h3 class="card-title">Автомобили <span class="highlight-text">по факту</span></h3>
      </div>
      <div class="card-header__settings">
        <RefreshButton @refresh="fetchCars" />
      </div>
    </div>
    
    <div class="card-content">
      <!-- Заголовок таблицы всегда отображается -->
      <div class="cars-header">
        <div class="header-row">
          <div class="header-col checkbox-col">
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
          <div class="header-col place-col" @click="sortBy('unload_place')">
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
            <p :class="{ 'active-sort': sortField === 'entry_time' }">Время</p>
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
      <div class="cars-container">
        <div v-if="filteredCars.length > 0" class="cars-body">
          <transition-group name="fade-list" tag="div">
            <div 
              v-for="(car, index) in sortedCars" 
              :key="car.id" 
              class="car-item"
              :style="{ animationDelay: `${index * 0.1}s` }"
            >
              <div class="car-row">
                <div class="car-col checkbox-col">
                  <input 
                    type="checkbox" 
                    v-model="car.checked"
                    class="checkbox-input"
                  />
                </div>
                <div class="car-col organization-col">
                  {{ car.organization }}
                </div>
                <div class="car-col place-col">
                  {{ formatUnloadPlaces(car.unload_places) }}
                </div>
                <div class="car-col date-col">
                  {{ formatDate(car.entry_date_to) }}
                </div>
                <div class="car-col time-col">
                  {{ formatTimeRange(car.entry_time_from, car.entry_time_to) }}
                </div>
                <div class="car-col status-col">
                  <span class="status-text">
                    {{ car.status }}
                  </span>
                </div>
                <div class="car-col actions-col">
                  <button 
                    @click="deleteCar(car)" 
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
          {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Заявок на машины по факту нет' }}
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
  emits: ['refresh-cars'],
  props: {
    searchQuery: {
      type: String,
      default: ''
    },
    selectedOrganization: {
      type: String,
      default: ''
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
    }
  },
  data() {
    return {
      sortField: null,
      sortDirection: 'desc',
      carsData: [],
      unloadingPlacesMap: new Map()
    };
  },
  computed: {
    filteredCars() {
      let filtered = [...this.carsData];

      // Поиск по всем полям
      if (this.searchQuery) {
        const query = this.searchQuery.toLowerCase();
        filtered = filtered.filter(car => 
          car.organization.toLowerCase().includes(query) ||
          this.formatUnloadPlaces(car.unload_places).toLowerCase().includes(query) ||
          car.status.toLowerCase().includes(query) ||
          this.formatTimeRange(car.entry_time_from, car.entry_time_to).toLowerCase().includes(query) ||
          this.formatDate(car.entry_date_to).toLowerCase().includes(query)
        );
      }

      // Фильтр по организации
      if (this.selectedOrganization) {
        filtered = filtered.filter(car => 
          car.organization === this.selectedOrganization
        );
      }

      // Фильтр по месту разгрузки
      if (this.selectedUnloadingPlace) {
        filtered = filtered.filter(car => {
          const places = this.formatUnloadPlaces(car.unload_places);
          return places.includes(this.selectedUnloadingPlace);
        });
      }

      // Фильтр по дате
      if (this.selectedDate) {
        const selectedDateStr = this.selectedDate.toISOString().split('T')[0];
        filtered = filtered.filter(car => car.entry_date_to === selectedDateStr);
      } else if (this.dateRangeStart && this.dateRangeEnd) {
        filtered = filtered.filter(car => {
          const carDate = new Date(car.entry_date_to);
          return carDate >= this.dateRangeStart && carDate <= this.dateRangeEnd;
        });
      }

      return filtered;
    },

    sortedCars() {
      const cars = [...this.filteredCars];
      
      if (!this.sortField) {
        return cars;
      }
      
      return cars.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'organization':
          case 'status':
            valueA = a[this.sortField]?.toLowerCase() || '';
            valueB = b[this.sortField]?.toLowerCase() || '';
            break;
            
          case 'unload_place':
            valueA = this.formatUnloadPlaces(a.unload_places)?.toLowerCase() || '';
            valueB = this.formatUnloadPlaces(b.unload_places)?.toLowerCase() || '';
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
        this.selectedOrganization ||
        this.selectedUnloadingPlace ||
        this.selectedDate ||
        (this.dateRangeStart && this.dateRangeEnd)
      );
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

    async fetchCars() {
      try {
        const response = await fetch("http://localhost:8080/applications/active-cars");
        const applications = await response.json();
        
        if (this.unloadingPlacesMap.size === 0) {
          await this.fetchUnloadingPlaces();
        }
        
        // Получаем все машины с номером "По факту"
        const factCars = applications.flatMap(application => 
          application.cars
            .filter(car => car.status !== 0) // Исключаем удаленные
            .filter(car => {
              const carNumber = car.car_number?.toLowerCase().trim();
              return carNumber === 'по факту';
            })
            .map(car => ({
              id: car.id,
              organization: application.organization,
              unload_places: car.unload_places || [],
              entry_date_from: application.entry_date_from,
              entry_date_to: application.entry_date_to,
              entry_time_from: application.entry_time_from,
              entry_time_to: application.entry_time_to,
              status: 'В работе',
              checked: false,
              applicationId: application.id
            }))
        );

        // Фильтруем по сроку действия
        const now = new Date();
        const validCars = factCars.filter(car => {
          const startDate = new Date(car.entry_date_from);
          const endDate = new Date(car.entry_date_to);
          endDate.setHours(23, 59, 59, 999);
          return startDate <= now && now <= endDate;
        });

        // Группируем по организации - оставляем только уникальные организации
        const uniqueOrganizations = new Map();
        validCars.forEach(car => {
          if (!uniqueOrganizations.has(car.organization)) {
            uniqueOrganizations.set(car.organization, car);
          } else {
            // Если организация уже есть, объединяем места разгрузки
            const existingCar = uniqueOrganizations.get(car.organization);
            const combinedPlaces = [...new Set([...existingCar.unload_places, ...car.unload_places])];
            existingCar.unload_places = combinedPlaces;
          }
        });

        this.carsData = Array.from(uniqueOrganizations.values());

        this.$nextTick(() => {
          setTimeout(() => {
            document.querySelectorAll('.car-item').forEach(item => {
              item.classList.add('animate-in');
            });
          }, 100);
        });
      } catch (error) {
        console.error("Ошибка при загрузке автомобилей по факту:", error);
      }
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

    formatUnloadPlaces(unloadPlaces) {
      if (!unloadPlaces || unloadPlaces.length === 0) return '-';
      
      const placeNames = unloadPlaces
        .map(place => {
          if (typeof place === 'object' && place.unload_place_name) {
            return place.unload_place_name;
          }
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
    
    async deleteCar(car) {
      try {
        const response = await fetch(`http://localhost:8080/applications/${car.applicationId}/cars/${car.id}`, {
          method: "PUT",
          headers: { 
            "Content-Type": "application/json",
            "Authorization": `Bearer ${localStorage.getItem("token")}`
          },
          body: JSON.stringify({ 
            car: {
              ...car,
              status: 0
            },
            unload_places: car.unload_places || []
          })
        });
        
        if (response.ok) {
          this.fetchCars();
        } else {
          console.error("Ошибка при удалении машины");
        }
      } catch (error) {
        console.error("Ошибка сети при удалении машины:", error);
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
    }
  },
  mounted() {
    this.fetchUnloadingPlaces().then(() => {
      this.fetchCars();
    });
    
    setTimeout(() => {
      document.querySelectorAll('.car-item').forEach(item => {
        item.classList.add('animate-in');
      });
    }, 100);
  }
};
</script>

<style scoped>
.cars-fact-card {
  background-color: #fff;
  border-radius: 30px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 65%;
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

.cars-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow-y: auto;
}

/* Заголовок таблицы */
.cars-header {
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

/* Колонки с фиксированной шириной - исправленные размеры */
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
.cars-body {
  overflow-y: auto;
  flex-grow: 1;
  padding-right: 4px;
  margin-right: 4px;
  scroll-behavior: smooth;
}

.car-item {
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

.car-item:hover {
  background-color: #fafafa;
}

.car-row {
  display: flex;
  width: 100%;
  padding: 10px 16px;
  align-items: center;
  border-top: 1px solid #f0f0f0;
}

.car-col {
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
.checkbox-col .car-col,
.actions-col .car-col {
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
.cars-body::-webkit-scrollbar {
  width: 6px;
}

.cars-body::-webkit-scrollbar-track {
  background: transparent;
  margin: 2px 0;
  border-radius: 3px;
}

.cars-body::-webkit-scrollbar-thumb {
  background: #D9E2FF;
  border-radius: 3px;
  border: 1px solid transparent;
  background-clip: content-box;
  transition: all 0.3s ease;
}

.cars-body::-webkit-scrollbar-thumb:hover {
  background: #C5D1FF;
  border: 1px solid transparent;
  background-clip: content-box;
  transform: scale(1.1);
}

.cars-body {
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
  .cars-fact-card {
    width: 100%;
    height: auto;
  }
  
  .header-row,
  .car-row {
    flex-wrap: wrap;
  }
  
  .header-col,
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