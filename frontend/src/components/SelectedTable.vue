<template>
  <div class="selected-table-card">
    <!-- Уведомление об удалении -->
    <transition name="slide-down">
      <div v-if="notification.message" class="notification">
        <span>{{ notification.message }}</span>
        <button @click="undoDelete">Отменить</button>
        <div class="progress-bar" :style="{ width: `${progress}%` }"></div>
      </div>
    </transition>

    <div class="card-header">
      <div class="card-header__title">
        <h3 class="card-title"><span class="blue">Номера</span> автомобилей по заявке</h3>
      </div>
      <div class="card-header__settings">
        <span class="cars-count">Машин на территории: {{ activeCarsCount }}</span>
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
          <div class="header-col number-col" @click="sortBy('car_number')">
            <p :class="{ 'active-sort': sortField === 'car_number' }">Номер машины</p>
            <img 
              src="@/assets/icons/sort.png" 
              class="sort-icon" 
              :class="{ 
                'sorted': sortField === 'car_number',
                'desc': sortField === 'car_number' && sortDirection === 'desc'
              }" 
            />
          </div>
          <div class="header-col brand-col" @click="sortBy('car_brand')">
            <p :class="{ 'active-sort': sortField === 'car_brand' }">Марка</p>
            <img 
              src="@/assets/icons/sort.png" 
              class="sort-icon" 
              :class="{ 
                'sorted': sortField === 'car_brand',
                'desc': sortField === 'car_brand' && sortDirection === 'desc'
              }" 
            />
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
              :key="car.id || index" 
              class="car-item"
              :style="{ animationDelay: `${index * 0.1}s` }"
            >
              <div class="car-row">
                <div class="car-col checkbox-col">
                  <input 
                    type="checkbox" 
                    v-model="car.checked"
                    class="checkbox-input"
                    @change="updateActiveCarsCount"
                  />
                </div>
                <div class="car-col number-col">
                  {{ car.car_number }}
                </div>
                <div class="car-col brand-col">
                  {{ car.car_brand }}
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
                    @click="removeCarWithNotification(car.applicationId, car)" 
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
          {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Нет заявок на автомобили' }}
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
      notification: { message: null, car: null },
      progress: 100,
      progressInterval: null,
      activeCarsCount: 0,
      unloadingPlacesMap: new Map() // Кэш для мест разгрузки
    };
  },
  computed: {
    filteredCars() {
      let filtered = [...this.carsData];

      // Поиск по всем полям
      if (this.searchQuery) {
        const query = this.searchQuery.toLowerCase();
        filtered = filtered.filter(car => 
          car.car_number.toLowerCase().includes(query) ||
          car.car_brand.toLowerCase().includes(query) ||
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
      
      // Если сортировка не выбрана, возвращаем исходный массив
      if (!this.sortField) {
        return cars;
      }
      
      return cars.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'car_number':
          case 'car_brand':
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
            // Для времени извлекаем начальное время для сравнения
            valueA = this.extractStartTime(a.entry_time_from);
            valueB = this.extractStartTime(b.entry_time_from);
            break;
            
          default:
            return 0;
        }
        
        // Логика сортировки
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
        
        // Сохраняем места разгрузки в Map для быстрого доступа
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
        
        // Загружаем места разгрузки если еще не загружены
        if (this.unloadingPlacesMap.size === 0) {
          await this.fetchUnloadingPlaces();
        }
        
        // Преобразуем данные в формат для таблицы
        const allCars = applications.flatMap(application => 
          application.cars
            .filter(car => car.status !== 0) // Исключаем уже удаленные машины
            .filter(car => {
              // Фильтруем машины с номером "По факту"
              const carNumber = car.car_number?.toLowerCase().trim();
              return carNumber !== 'по факту';
            })
            .map(car => ({
              id: car.id,
              car_number: car.car_number,
              car_brand: car.car_brand,
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

        // Фильтруем машины по сроку действия
        const now = new Date();
        
        const validCars = allCars.filter(car => {
          // Проверяем, действует ли заявка на текущую дату
          const startDate = new Date(car.entry_date_from);
          const endDate = new Date(car.entry_date_to);
          endDate.setHours(23, 59, 59, 999); // Устанавливаем конец дня
          
          return startDate <= now && now <= endDate;
        });

        // Удаляем просроченные машины из БД
        const overdueCars = allCars.filter(car => {
          const endDate = new Date(car.entry_date_to);
          endDate.setHours(23, 59, 59, 999);
          return now > endDate;
        });

        if (overdueCars.length > 0) {
          await this.deleteOverdueCars(overdueCars);
        }

        this.carsData = validCars;
        
        // Обновляем счетчик активных машин после загрузки данных
        this.updateActiveCarsCount();

        // Добавляем класс для анимации после обновления данных
        this.$nextTick(() => {
          setTimeout(() => {
            document.querySelectorAll('.car-item').forEach(item => {
              item.classList.add('animate-in');
            });
          }, 100);
        });
      } catch (error) {
        console.error("Ошибка при загрузке автомобилей:", error);
      }
    },

    async deleteOverdueCars(overdueCars) {
      try {
        const deletePromises = overdueCars.map(async (car) => {
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
          
          if (!response.ok) {
            console.error(`Ошибка при удалении просроченной машины ${car.car_number}`);
          }
          return response.ok;
        });

        await Promise.all(deletePromises);
        console.log(`Удалено ${overdueCars.length} просроченных машин`);
      } catch (error) {
        console.error("Ошибка при удалении просроченных машин:", error);
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

    sortBy(field) {
      if (this.sortField === field) {
        // Если уже сортируем по этому полю, меняем направление
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        // Если новое поле, устанавливаем его и направление по умолчанию (desc)
        this.sortField = field;
        this.sortDirection = 'desc';
      }
    },

    extractStartTime(timeString) {
      if (!timeString || timeString === '-') return 0;
      
      // Убираем секунды перед парсингом
      const timeWithoutSeconds = timeString.split(':').slice(0, 2).join(':');
      const [hours, minutes] = timeWithoutSeconds.split(':').map(Number);
      return hours * 60 + minutes; // Возвращаем время в минутах для сравнения
    },

    async actuallyDeleteCar(applicationId, car) {
      try {
        const response = await fetch(`http://localhost:8080/applications/${applicationId}/cars/${car.id}`, {
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
        
        if (!response.ok) {
          console.error("Ошибка при удалении машины");
          // Если не удалось удалить, возвращаем предыдущий статус
          car.status = this.notification.originalStatus;
        }
        this.fetchCars(); // Обновляем данные после удаления
      } catch (error) {
        console.error("Ошибка сети при удалении машины:", error);
        car.status = this.notification.originalStatus;
      }
    },

    removeCarWithNotification(applicationId, car) {
      // Сохраняем исходное состояние машины для возможного восстановления
      const originalCar = { ...car };
      
      this.notification = { 
        message: `Машина ${car.car_number} удалена.`, 
        car,
        applicationId,
        originalStatus: car.status,
        undoFunction: async () => {
          try {
            car.status = originalCar.status;
            const response = await fetch(`http://localhost:8080/applications/${applicationId}/cars/${car.id}`, {
              method: "PUT",
              headers: { 
                "Content-Type": "application/json",
                "Authorization": `Bearer ${localStorage.getItem("token")}`
              },
              body: JSON.stringify({ 
                car: { ...car, status: originalCar.status },
                unload_places: car.unload_places || []
              })
            });
            
            if (!response.ok) throw new Error("Ошибка восстановления");
            this.fetchCars(); // Обновляем данные
          } catch (error) {
            console.error("Ошибка при отмене удаления:", error);
          }
        }
      };

      // Помечаем как удаленную локально, но не отправляем на сервер
      car.status = 0;

      this.progress = 100;
      clearInterval(this.progressInterval);
      this.progressInterval = setInterval(() => {
        this.progress -= 10;
        if (this.progress <= 0) {
          clearInterval(this.progressInterval);
          // Только после завершения таймера отправляем запрос на сервер
          this.actuallyDeleteCar(applicationId, car);
          if (this.notification.car?.id === car.id) {
            this.notification = { message: null, car: null };
          }
        }
      }, 100);
    },

    undoDelete() {
      clearInterval(this.progressInterval);
      if (this.notification.undoFunction) {
        this.notification.undoFunction();
      }
      this.notification = { message: null, car: null };
    },

    updateActiveCarsCount() {
      // Считаем количество активных чекбоксов
      this.activeCarsCount = this.carsData.filter(car => car.checked).length;
    }
  },
  mounted() {
    this.fetchUnloadingPlaces().then(() => {
      this.fetchCars();
    });
    
    // Добавляем класс для анимации после монтирования компонента
    this.$nextTick(() => {
      setTimeout(() => {
        document.querySelectorAll('.car-item').forEach(item => {
          item.classList.add('animate-in');
        });
      }, 100);
    });
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

.cars-count {
  color: #4F5BDF;
  font-weight: 500;
  font-size: 0.9em;
}

.card-content {
  padding: 0;
  flex-grow: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.cars-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
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
.checkbox-col {
  width: 3%;
  min-width: 25px;
}

.number-col {
  width: 14%;
  min-width: 90px;
}

.brand-col {
  width: 12%;
  min-width: 80px;
}

.organization-col {
  width: 22%;
  min-width: 100px;
}

.place-col {
  width: 20%;
  min-width: 110px;
}

.date-col {
  width: 12%;
  min-width: 90px;
}

.time-col {
  width: 12%;
  min-width: 90px;
}

.status-col {
  width: 12%;
  min-width: 90px;
}

.actions-col {
  width: 9%;
  min-width: 40px;
}

/* Тело таблицы */
.cars-body {
  overflow-y: auto;
  flex-grow: 1;
  /* Добавляем отступ справа для скроллбара */
  padding-right: 4px;
  margin-right: 4px;
  min-height: 80px;

}



/* Для Firefox - более тонкий и плавный скролл */

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
  border-bottom: 1px solid #e6e6e6;
}

.car-col {
  padding: 0 4px;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 15px; /* Увеличен размер шрифта */
}

.checkbox-input {
  width: 13px;
  height: 13px;
  cursor: pointer;
}

.status-text {
  color: #079D1D; /* Цвет текста статуса */
  font-weight: 500;
  /* Убраны все бэджи, фон и бордеры */
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

.delete-btn:hover {
  background-color: transparent;
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

/* Стили для уведомления */
.notification {
  position: fixed;
  top: 20px;
  right: 20px;
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  padding: 12px 16px;
  box-shadow: 0 3px 10px rgba(0,0,0,0.1);
  z-index: 1000;
  min-width: 250px;
}

.notification button {
  margin-left: 10px;
  background: #4F5BDF;
  color: white;
  border: none;
  padding: 4px 8px;
  border-radius: 4px;
  cursor: pointer;
}

.progress-bar {
  position: absolute;
  bottom: 0;
  left: 0;
  height: 3px;
  background: #4F5BDF;
  transition: width 0.1s linear;
}

.slide-down-enter-active,
.slide-down-leave-active {
  transition: all 0.3s ease;
}

.slide-down-enter-from {
  transform: translateY(-100%);
  opacity: 0;
}

.slide-down-leave-to {
  transform: translateY(-100%);
  opacity: 0;
}

@media (max-width: 768px) {
  .selected-table-card {
    max-height: none;
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
  
  .notification {
    left: 20px;
    right: 20px;
    min-width: auto;
  }
}
</style>