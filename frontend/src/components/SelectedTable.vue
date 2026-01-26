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
        <h3 class="card-title">
          <span class="blue">{{ tableType === 'cars' ? 'Номера автомобилей' : 'Люди' }}</span> по заявке
        </h3>
      </div>
      <div class="card-header__settings">
        <span class="items-count">
          {{ tableType === 'cars' ? 'Машин' : 'Людей' }} на территории: {{ activeItemsCount }}
        </span>
        <RefreshButton @refresh="fetchData" />
      </div>
    </div>
    
    <div class="card-content">
      <!-- Заголовок таблицы всегда отображается -->
      <div class="items-header">
        <div class="header-row">
          <div class="header-col checkbox-col">
            <!-- Пустой заголовок для чекбокса -->
          </div>
          
          <!-- Поля для машин -->
          <template v-if="tableType === 'cars'">
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
          </template>
          
          <!-- Поля для людей -->
          <template v-else>
            <div class="header-col name-col" @click="sortBy('last_name')">
              <p :class="{ 'active-sort': sortField === 'last_name' }">Фамилия</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'last_name',
                  'desc': sortField === 'last_name' && sortDirection === 'desc'
                }" 
              />
            </div>
            <div class="header-col name-col" @click="sortBy('first_name')">
              <p :class="{ 'active-sort': sortField === 'first_name' }">Имя</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'first_name',
                  'desc': sortField === 'first_name' && sortDirection === 'desc'
                }" 
              />
            </div>
            <div class="header-col name-col" @click="sortBy('middle_name')">
              <p :class="{ 'active-sort': sortField === 'middle_name' }">Отчество</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'middle_name',
                  'desc': sortField === 'middle_name' && sortDirection === 'desc'
                }" 
              />
            </div>
          </template>

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
            <p :class="{ 'active-sort': sortField === 'entry_time' }">
              {{ tableType === 'cars' ? 'Время' : 'Время прохода' }}
            </p>
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
      <div class="items-container">
        <div v-if="isLoading && itemsData.length === 0" class="loading-message">
            <div class="loader"></div>
            <p>Загрузка данных...</p>
          </div>
          <div v-else-if="filteredItems.length > 0" class="items-body">
        <div v-if="filteredItems.length > 0" class="items-body">
          <transition-group name="fade-list" tag="div">
            <div 
              v-for="(item, index) in sortedItems" 
              :key="item.id || index" 
              class="item-row"
              :style="{ animationDelay: `${index * 0.1}s` }"
            >
              <div class="item-data">
                <div class="item-col checkbox-col">
                  <input 
                    type="checkbox" 
                    v-model="item.checked"
                    class="checkbox-input"
                    @change="updateActiveItemsCount"
                  />
                </div>
                
                <!-- Данные для машин -->
                <template v-if="tableType === 'cars'">
                  <div class="item-col number-col">
                    {{ item.car_number }}
                  </div>
                  <div class="item-col brand-col">
                    {{ item.car_brand }}
                  </div>
                </template>
                
                <!-- Данные для людей -->
                <template v-else>
                  <div class="item-col name-col">
                    {{ item.last_name }}
                  </div>
                  <div class="item-col name-col">
                    {{ item.first_name }}
                  </div>
                  <div class="item-col name-col">
                    {{ item.middle_name || '-' }}
                  </div>
                </template>

                <div class="item-col organization-col">
                  {{ item.organization }}
                </div>
                
                <div class="item-col place-col" v-if="tableType === 'cars'">
                  <!-- Используем unload_place вместо formatUnloadPlaces -->
                  {{ item.unload_place || formatUnloadPlaces(item) }}
                </div>
                
                <div class="item-col date-col">
                  {{ formatDate(item.entry_date_to) }}
                </div>
                
                <div class="item-col time-col">
                  {{ tableType === 'cars' 
                    ? formatTimeRange(item.entry_time_from, item.entry_time_to)
                    : formatPassTime(item.pass_time)
                  }}
                </div>
                
                <div class="item-col status-col">
                  <span class="status-text">
                    {{ item.status }}
                  </span>
                </div>
                
                <div class="item-col actions-col">
                  <button 
                    @click="removeItemWithNotification(item)" 
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
        </div>
        <p v-else class="no-data-message">
          {{ hasActiveFilters 
            ? 'Нет данных по выбранным фильтрам' 
            : `Нет заявок на ${tableType === 'cars' ? 'автомобили' : 'людей'}` 
          }}
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
    tableName: {
      type: String,
      default: ''
    },
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
      itemsData: [],
      notification: { message: null, item: null },
      progress: 100,
      progressInterval: null,
      activeItemsCount: 0,
      unloadingPlacesMap: new Map(),
      isLoading: false, // Добавляем флаг загрузки
      isInitialLoad: true // Флаг первой загрузки
    };
  },
  computed: {
    filteredItems() {
      let filtered = [...this.itemsData];

      // Поиск по всем полям
      if (this.searchQuery) {
        const query = this.searchQuery.toLowerCase();
        filtered = filtered.filter(item => {
          const searchFields = [
            item.organization,
            this.formatDate(item.entry_date_to),
            item.status,
            this.tableType === 'cars' 
              ? this.formatTimeRange(item.entry_time_from, item.entry_time_to)
              : this.formatPassTime(item.pass_time)
          ];

          if (this.tableType === 'cars') {
            searchFields.push(
              item.car_number,
              item.car_brand,
              // Используем unload_place вместо formatUnloadPlaces
              item.unload_place || ''
            );
          } else {
            searchFields.push(
              item.last_name,
              item.first_name,
              item.middle_name
            );
          }

          return searchFields.some(field => 
            field?.toLowerCase().includes(query)
          );
        });
      }

      // Фильтр по организации
      if (this.selectedOrganization) {
        filtered = filtered.filter(item => 
          item.organization === this.selectedOrganization
        );
      }

      // Фильтр по месту разгрузки (только для машин)
      if (this.selectedUnloadingPlace && this.tableType === 'cars') {
        filtered = filtered.filter(item => {
          // Используем поле unload_place для фильтрации
          const place = item.unload_place || '';
          return place.toLowerCase().includes(this.selectedUnloadingPlace.toLowerCase());
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

    sortedItems() {
      const items = [...this.filteredItems];
      
      if (!this.sortField) {
        return items;
      }
      
      return items.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'car_number':
          case 'car_brand':
          case 'organization':
          case 'status':
          case 'last_name':
          case 'first_name':
          case 'middle_name':
            valueA = a[this.sortField]?.toLowerCase() || '';
            valueB = b[this.sortField]?.toLowerCase() || '';
            break;
            
          case 'unload_place':
            // Используем поле unload_place для сортировки
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
        this.selectedOrganization ||
        (this.tableType === 'cars' && this.selectedUnloadingPlace) ||
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

    async fetchData() {
  // Если загрузка уже идет, не начинаем новую
  if (this.isLoading) return;
  
  // Если это таблица людей и нет имени таблицы - не загружаем
  if (this.tableType === 'people' && !this.tableName) {
    console.warn('Table name is required for people table');
    this.itemsData = [];
    this.isLoading = false;
    return;
  }
  
  this.isLoading = true;
  
  // Сначала сбрасываем данные, чтобы не показывать старые данные другого типа
  this.itemsData = [];
  
  try {
    // Сохраняем текущий тип таблицы для проверки после загрузки
    const currentTableType = this.tableType;
    
    let newData = [];
    
    if (currentTableType === 'cars') {
      newData = await this.fetchCarsData();
    } else if (currentTableType === 'people') {
      newData = await this.fetchPeopleData();
    } else {
      console.warn(`Unknown table type: ${currentTableType}`);
      newData = [];
    }
    
    // Проверяем, не изменился ли тип таблицы во время загрузки
    if (this.tableType !== currentTableType) {
      console.log('Тип таблицы изменился во время загрузки, игнорируем результат');
      this.itemsData = [];
    } else {
      // Устанавливаем данные только если тип таблицы не изменился
      this.itemsData = newData;
    }
    
    this.updateActiveItemsCount();
  } catch (error) {
    console.error(`Ошибка при загрузке данных (${this.tableType}):`, error);
    this.itemsData = [];
  } finally {
    this.isLoading = false;
    this.isInitialLoad = false;
  }
},

    async fetchCarsData() {
  try {
    const token = localStorage.getItem("token");
    
    console.log('Загрузка машин для таблицы с типом cars...');
    
    const response = await fetch("http://localhost:8080/cars/active-for-tables", {
        method: "GET",
        headers: {
            "Authorization": `Bearer ${token}`,
        },
    });
    
    if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
    }
    
    const cars = await response.json();
    console.log('Получены все активные машины:', cars.length, 'шт.');
    
    // Фильтруем машины
    const regularCars = cars.filter(car => {
        if (car.status !== 1) {
            return false;
        }
        
        const carNumber = car.car_number?.toLowerCase().trim();
        return carNumber !== 'по факту';
    });
    
    console.log('Машины для отображения в таблице:', regularCars.length, 'шт.');
    
    // Возвращаем отформатированные данные
    return regularCars.map(car => ({
        id: car.id,
        car_number: car.car_number || '',
        car_brand: car.car_brand || '',
        organization: car.organization || 'Не указана',
        unload_place: car.unload_place || '-',
        unload_places: car.unload_places || [],
        entry_date_to: car.entry_date_to || '',
        entry_time_from: car.entry_time_from || '',
        entry_time_to: car.entry_time_to || '',
        status: 'В работе',
        checked: false
    }));
    
  } catch (error) {
    console.error("Ошибка при загрузке данных машин:", error);
    return [];
  }
},

    async fetchPeopleData() {
  try {
    const token = localStorage.getItem("token");
    
    if (!this.tableName) {
        console.error("Table name is required for fetching people data");
        return [];
    }
    
    const tableResponse = await fetch(`http://localhost:8080/system-tables/name/${this.tableName}`, {
        method: "GET",
        headers: {
            "Authorization": `Bearer ${token}`,
        },
    });
    
    if (!tableResponse.ok) {
        throw new Error(`Failed to get table: ${tableResponse.status}`);
    }
    
    const table = await tableResponse.json();
    
    const response = await fetch(`http://localhost:8080/employees/active-for-table/${table.id}`, {
        method: "GET",
        headers: {
            "Authorization": `Bearer ${token}`,
        },
    });
    
    if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
    }
    
    const employees = await response.json();
    
    return employees.map(emp => ({
        id: emp.id,
        last_name: emp.last_name || '',
        first_name: emp.first_name || '',
        middle_name: emp.middle_name || '',
        organization: emp.organization || 'Не указана',
        entry_date_to: emp.entry_date_to || '',
        pass_time: emp.pass_time || '',
        status: 'Активен',
        checked: false
    }));
    
  } catch (error) {
    console.error("Ошибка при загрузке данных сотрудников:", error);
    return [];
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

    formatPassTime(passTime) {
      return passTime || '-';
    },

    // Обновляем функцию formatUnloadPlaces для использования unload_place
    formatUnloadPlaces(item) {
      // Используем поле unload_place из данных
      if (item.unload_place) {
        return item.unload_place;
      }
      
      // Для обратной совместимости с массивом unload_places
      if (item.unload_places && item.unload_places.length > 0) {
        const placeNames = item.unload_places
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
      }
      
      return '-';
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

    async removeItemWithNotification(item) {
      const originalItem = { ...item };
      
      this.notification = { 
        message: `${this.tableType === 'cars' ? 'Машина' : 'Человек'} ${this.tableType === 'cars' ? item.car_number : item.last_name} удален.`, 
        item,
        originalStatus: item.status,
        undoFunction: async () => {
          try {
            item.status = originalItem.status;
            if (this.tableType === 'cars') {
              const response = await fetch(`http://localhost:8080/applications/${item.applicationId}/cars/${item.id}`, {
                method: "PUT",
                headers: { 
                  "Content-Type": "application/json",
                  "Authorization": `Bearer ${localStorage.getItem("token")}`
                },
                body: JSON.stringify({ 
                  car: { ...item, status: originalItem.status },
                  unload_places: item.unload_places || []
                })
              });
              
              if (!response.ok) throw new Error("Ошибка восстановления");
            }
            this.fetchData();
          } catch (error) {
            console.error("Ошибка при отмене удаления:", error);
          }
        }
      };

      item.status = 0;

      this.progress = 100;
      clearInterval(this.progressInterval);
      this.progressInterval = setInterval(() => {
        this.progress -= 10;
        if (this.progress <= 0) {
          clearInterval(this.progressInterval);
          this.actuallyDeleteItem(item);
          if (this.notification.item?.id === item.id) {
            this.notification = { message: null, item: null };
          }
        }
      }, 100);
    },

    async actuallyDeleteItem(item) {
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
          
          if (!response.ok) {
            console.error("Ошибка при удалении");
            item.status = this.notification.originalStatus;
          }
        } else {
          // Логика удаления для людей
          this.itemsData = this.itemsData.filter(i => i.id !== item.id);
        }
        this.fetchData();
      } catch (error) {
        console.error("Ошибка сети при удалении:", error);
        item.status = this.notification.originalStatus;
      }
    },

    undoDelete() {
      clearInterval(this.progressInterval);
      if (this.notification.undoFunction) {
        this.notification.undoFunction();
      }
      this.notification = { message: null, item: null };
    },

    updateActiveItemsCount() {
      this.activeItemsCount = this.itemsData.filter(item => item.checked).length;
    }
  },
  mounted() {
  // Загружаем данные только если указан tableType
  if (this.tableType === 'cars') {
    this.fetchUnloadingPlaces().then(() => {
      this.fetchData();
    });
  } else if (this.tableType === 'people' && this.tableName) {
    // Для людей ждем, пока tableName будет установлен
    this.fetchData();
  }
  },
  watch: {
  tableType: {
    handler(newVal, oldVal) {
      console.log('Тип таблицы изменился:', oldVal, '->', newVal);
      console.log('Имя таблицы:', this.tableName);
      
      // Сбрасываем данные при смене типа таблицы
      this.itemsData = [];
      this.isInitialLoad = true;
      
      // Отменяем предыдущие уведомления
      if (this.notification.message) {
        clearInterval(this.progressInterval);
        this.notification = { message: null, item: null };
      }
      
      // Загружаем данные для нового типа
      this.fetchData();
    }
  },
  tableName: {
    handler(newVal, oldVal) {
      if (newVal && this.tableType === 'people') {
        console.log('Имя таблицы изменилось:', oldVal, '->', newVal);
        
        // Сбрасываем данные при смене имени таблицы
        this.itemsData = [];
        this.isInitialLoad = true;
        
        this.fetchData();
      }
    }
  }
},
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

.blue {
  color: #4F5BDF;
}

.items-count {
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

.items-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.items-header {
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
  width: 18%;
  min-width: 90px;
}

.brand-col {
  width: 12%;
  min-width: 80px;
}

.name-col {
  width: 12%;
  min-width: 80px;
}

.organization-col {
  width: 25%;
  min-width: 100px;
}

.place-col {
  width: 20%;
  min-width: 110px;
}

.date-col {
  width: 15%;
  min-width: 90px;
}

.time-col {
  width: 15%;
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

.item-row:hover {
  background-color: #fafafa;
}

.item-data {
  display: flex;
  width: 100%;
  padding: 10px 16px;
  align-items: center;
  border-bottom: 1px solid #e6e6e6;
}

.item-col {
  padding: 0 4px;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 15px;
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
  .item-data {
    flex-wrap: wrap;
  }
  
  .header-col,
  .item-col {
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