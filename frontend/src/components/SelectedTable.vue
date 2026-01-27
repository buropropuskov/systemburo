<template>
  <div class="selected-table-card">
    <!-- Уведомление об удаления -->
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
        <RefreshButton @refresh="loadData" :disabled="isLoading" />
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
        <div v-if="isLoading" class="loading-message">
          <div class="loader"></div>
          <p>Загрузка данных...</p>
        </div>
        
        <template v-else>
          <div v-if="displayItems.length > 0" class="items-body">
            <transition-group name="fade-list" tag="div">
              <div 
                v-for="(item, index) in displayItems" 
                :key="item.id || index" 
                class="item-row"
                :style="{ animationDelay: `${index * 0.05}s` }"
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
                    {{ item.unload_place || '-' }}
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
                      :disabled="isLoading"
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
          
          <div v-else class="no-data-message">
            {{ hasActiveFilters 
              ? 'Нет данных по выбранным фильтрам' 
              : tableType === 'cars' 
                ? 'Нет активных автомобилей' 
                : 'Нет активных сотрудников'
            }}
          </div>
        </template>
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
      itemsData: [], // Общий массив данных
      notification: { message: null, item: null },
      progress: 100,
      progressInterval: null,
      activeItemsCount: 0,
      isLoading: false,
      hasDataLoaded: false // Флаг, что данные были загружены
    };
  },
  computed: {
    displayItems() {
      if (!this.hasDataLoaded || this.isLoading) {
        return [];
      }
      
      let filtered = [...this.itemsData];
      
      // Поиск по всем полям
      if (this.searchQuery) {
        const query = this.searchQuery.toLowerCase().trim();
        filtered = filtered.filter(item => {
          const searchFields = [
            item.organization,
            this.formatDate(item.entry_date_to),
            item.status
          ];

          if (this.tableType === 'cars') {
            searchFields.push(
              item.car_number,
              item.car_brand,
              item.unload_place || ''
            );
          } else {
            searchFields.push(
              item.last_name,
              item.first_name,
              item.middle_name || ''
            );
          }

          return searchFields.some(field => 
            field && field.toString().toLowerCase().includes(query)
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

      // Сортировка
      if (this.sortField) {
        filtered.sort((a, b) => {
          let valueA, valueB;
          
          switch (this.sortField) {
            case 'car_number':
            case 'car_brand':
            case 'organization':
            case 'status':
            case 'last_name':
            case 'first_name':
            case 'middle_name':
              valueA = (a[this.sortField] || '').toString().toLowerCase();
              valueB = (b[this.sortField] || '').toString().toLowerCase();
              break;
              
            case 'unload_place':
              valueA = (a.unload_place || '').toString().toLowerCase();
              valueB = (b.unload_place || '').toString().toLowerCase();
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
      }

      return filtered;
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
    async loadData() {
      if (this.isLoading) return;
      
      this.isLoading = true;
      this.hasDataLoaded = false;
      
      try {
        console.log('Загрузка данных для таблицы типа:', this.tableType);
        
        if (this.tableType === 'cars') {
          await this.fetchCarsData();
        } else {
          await this.fetchPeopleData();
        }
        
        this.hasDataLoaded = true;
        this.updateActiveItemsCount();
      } catch (error) {
        console.error(`Ошибка при загрузке данных (${this.tableType}):`, error);
        this.hasDataLoaded = true;
      } finally {
        this.isLoading = false;
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
          }
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
        
        // Сохраняем данные
        this.itemsData = regularCars.map(car => ({
          id: car.id,
          car_number: car.car_number || '',
          car_brand: car.car_brand || '',
          organization: car.organization || 'Не указана',
          unload_place: car.unload_place || '-',
          entry_date_to: car.entry_date_to || '',
          entry_time_from: car.entry_time_from || '',
          entry_time_to: car.entry_time_to || '',
          status: 'В работе',
          checked: false,
          applicationId: car.application_id
        }));
        
      } catch (error) {
        console.error("Ошибка при загрузке данных машин:", error);
        this.itemsData = [];
        throw error;
      }
    },

    async fetchPeopleData() {
      try {
        const token = localStorage.getItem("token");
        
        if (!this.tableName) {
          console.warn('Table name не указан для загрузки людей');
          this.itemsData = [];
          return;
        }
        
        const tableResponse = await fetch(`http://localhost:8080/system-tables/name/${this.tableName}`, {
          method: "GET",
          headers: {
            "Authorization": `Bearer ${token}`,
          }
        });
        
        if (!tableResponse.ok) {
          throw new Error(`Failed to get table: ${tableResponse.status}`);
        }
        
        const table = await tableResponse.json();
        
        const response = await fetch(`http://localhost:8080/employees/active-for-table/${table.id}`, {
          method: "GET",
          headers: {
            "Authorization": `Bearer ${token}`,
          }
        });
        
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const employees = await response.json();
        
        this.itemsData = employees.map(emp => ({
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
        this.itemsData = [];
        throw error;
      }
    },

    formatDate(dateString) {
      if (!dateString) return '';
      try {
        const [year, month, day] = dateString.split('-');
        const date = new Date(year, month - 1, day);
        return date.toLocaleDateString('ru-RU');
      } catch (error) {
        console.error('Ошибка форматирования даты:', error);
        return '';
      }
    },

    formatTimeRange(timeFrom, timeTo) {
      if (!timeFrom && !timeTo) return '-';
      
      const formatTime = (timeStr) => {
        if (!timeStr) return '';
        const parts = timeStr.split(':');
        if (parts.length >= 2) {
          return `${parts[0]}:${parts[1]}`;
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
      if (!passTime) return '-';
      
      const [timeFrom, timeTo] = passTime.split('-');
      
      if (!timeFrom && !timeTo) return '-';
      
      const formatTime = (timeStr) => {
        if (!timeStr) return '';
        const parts = timeStr.trim().split(':');
        if (parts.length >= 2) {
          return `${parts[0]}:${parts[1]}`;
        }
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

    removeItemWithNotification(item) {
      if (this.isLoading) return;
      
      const originalItem = { ...item };
      const itemIndex = this.itemsData.findIndex(i => i.id === item.id);
      
      // Отменяем предыдущее уведомление, если оно есть
      if (this.notification.message) {
        clearInterval(this.progressInterval);
      }
      
      this.notification = { 
        message: `${this.tableType === 'cars' ? 'Машина' : 'Человек'} ${this.tableType === 'cars' ? item.car_number : item.last_name} удален.`, 
        item,
        originalItem: originalItem,
        itemIndex: itemIndex,
        undoFunction: () => {
          try {
            // Восстанавливаем исходные данные
            if (itemIndex !== -1) {
              this.itemsData[itemIndex] = { ...originalItem };
              this.updateActiveItemsCount();
            }
          } catch (error) {
            console.error("Ошибка при отмене удаления:", error);
          }
        }
      };

      // Временно скрываем элемент
      item.status = 'Удален';
      item.checked = false;
      this.updateActiveItemsCount();

      this.progress = 100;
      clearInterval(this.progressInterval);
      this.progressInterval = setInterval(() => {
        this.progress -= 10;
        if (this.progress <= 0) {
          clearInterval(this.progressInterval);
          this.actuallyDeleteItem(item, originalItem, itemIndex);
          if (this.notification.item?.id === item.id) {
            this.notification = { message: null, item: null };
          }
        }
      }, 100);
    },

    async actuallyDeleteItem(item, originalItem, itemIndex) {
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
                ...originalItem,
                status: 0
              }
            })
          });
          
          if (!response.ok) {
            console.error("Ошибка при удалении");
            // Восстанавливаем статус в случае ошибки
            if (itemIndex !== -1) {
              this.itemsData[itemIndex] = { ...originalItem };
            }
            return;
          }
          
          // Удаляем элемент из массива после успешного удаления
          if (itemIndex !== -1) {
            this.itemsData.splice(itemIndex, 1);
          }
        } else {
          // Для людей просто удаляем из массива
          if (itemIndex !== -1) {
            this.itemsData.splice(itemIndex, 1);
          }
        }
        
        this.updateActiveItemsCount();
      } catch (error) {
        console.error("Ошибка сети при удалении:", error);
        // Восстанавливаем статус в случае ошибки
        if (itemIndex !== -1) {
          this.itemsData[itemIndex] = { ...originalItem };
        }
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
      this.activeItemsCount = this.itemsData.filter(item => 
        item.checked && item.status !== 'Удален'
      ).length;
    }
  },
  watch: {
    tableType: {
      immediate: true,
      async handler(newVal, oldVal) {
        if (newVal !== oldVal || !this.hasDataLoaded) {
          console.log('Тип таблицы изменился:', oldVal, '->', newVal);
          
          // Очищаем данные
          this.itemsData = [];
          this.sortField = null;
          this.sortDirection = 'desc';
          this.hasDataLoaded = false;
          
          // Отменяем предыдущие уведомления
          if (this.notification.message) {
            clearInterval(this.progressInterval);
            this.notification = { message: null, item: null };
          }
          
          // Загружаем данные только если тип определен
          if (newVal) {
            await this.loadData();
          }
        }
      }
    },
    tableName: {
      async handler(newVal, oldVal) {
        if (newVal !== oldVal && this.tableType !== 'cars') {
          console.log('Имя таблицы для людей изменилось:', oldVal, '->', newVal);
          this.itemsData = [];
          this.hasDataLoaded = false;
          await this.loadData();
        }
      }
    }
  },
  mounted() {
    console.log('SelectedTable mounted с типом:', this.tableType, 'и именем:', this.tableName);
    
    // Загружаем данные при монтировании
    if (this.tableType) {
      this.loadData();
    }
  },
  beforeUnmount() {
    // Очищаем интервалы
    if (this.progressInterval) {
      clearInterval(this.progressInterval);
    }
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
  animation: fadeInUp 0.3s ease forwards;
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

/* Анимация для списка */
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
  transition: background-color 0.2s;
}

.notification button:hover {
  background: #3a46d2;
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