<template>
  <div class="selected-table-card">
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
          <span class="blue">Люди</span> по заявке
        </h3>
      </div>
      <div class="card-header__settings">
        <span class="items-count">
          Людей на территории: {{ activeItemsCount }}
        </span>
        <RefreshButton @refresh="loadData" :disabled="isLoading" />
      </div>
    </div>
    
    <div class="card-content">
      <div class="items-header">
        <div class="header-row">
          <div class="header-col checkbox-col"></div>
          <div class="header-col name-col" @click="sortBy('last_name')">
            <p :class="{ 'active-sort': sortField === 'last_name' }">Фамилия</p>
            <img src="@/assets/icons/sort.png" class="sort-icon" :class="{ 'sorted': sortField === 'last_name', 'desc': sortField === 'last_name' && sortDirection === 'desc' }" />
          </div>
          <div class="header-col name-col" @click="sortBy('first_name')">
            <p :class="{ 'active-sort': sortField === 'first_name' }">Имя</p>
            <img src="@/assets/icons/sort.png" class="sort-icon" :class="{ 'sorted': sortField === 'first_name', 'desc': sortField === 'first_name' && sortDirection === 'desc' }" />
          </div>
          <div class="header-col name-col" @click="sortBy('middle_name')">
            <p :class="{ 'active-sort': sortField === 'middle_name' }">Отчество</p>
            <img src="@/assets/icons/sort.png" class="sort-icon" :class="{ 'sorted': sortField === 'middle_name', 'desc': sortField === 'middle_name' && sortDirection === 'desc' }" />
          </div>
          <div class="header-col organization-col" @click="sortBy('organization')">
            <p :class="{ 'active-sort': sortField === 'organization' }">Организация</p>
            <img src="@/assets/icons/sort.png" class="sort-icon" :class="{ 'sorted': sortField === 'organization', 'desc': sortField === 'organization' && sortDirection === 'desc' }" />
          </div>
          <div class="header-col date-col" @click="sortBy('entry_date_to')">
            <p :class="{ 'active-sort': sortField === 'entry_date_to' }">Действует до</p>
            <img src="@/assets/icons/sort.png" class="sort-icon" :class="{ 'sorted': sortField === 'entry_date_to', 'desc': sortField === 'entry_date_to' && sortDirection === 'desc' }" />
          </div>
          <div class="header-col time-col" @click="sortBy('entry_time')">
            <p :class="{ 'active-sort': sortField === 'entry_time' }">Время прохода</p>
            <img src="@/assets/icons/sort.png" class="sort-icon" :class="{ 'sorted': sortField === 'entry_time', 'desc': sortField === 'entry_time' && sortDirection === 'desc' }" />
          </div>
          <div class="header-col status-col" @click="sortBy('status')">
            <p :class="{ 'active-sort': sortField === 'status' }">Статус</p>
            <img src="@/assets/icons/sort.png" class="sort-icon" :class="{ 'sorted': sortField === 'status', 'desc': sortField === 'status' && sortDirection === 'desc' }" />
          </div>
          <div class="header-col actions-col"></div>
        </div>
      </div>
      
      <div class="items-container">
        <div v-if="isLoading" class="loading-message">
          <div class="loader"></div>
          <p>Загрузка сотрудников...</p>
        </div>
        
        <div v-else-if="displayItems.length > 0" class="items-body">
          <transition-group name="fade-list" tag="div">
            <div 
              v-for="(item, index) in displayItems" 
              :key="item.id" 
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
                <div class="item-col name-col">{{ item.last_name }}</div>
                <div class="item-col name-col">{{ item.first_name }}</div>
                <div class="item-col name-col">{{ item.middle_name || '-' }}</div>
                <div class="item-col organization-col">{{ item.organization_name }}</div>
                <div class="item-col date-col">{{ formatDate(item.entry_date_to) }}</div>
                <div class="item-col time-col">{{ formatPassTime(item.pass_time) }}</div>
                <div class="item-col status-col">
                  <span class="status-text">{{ item.status }}</span>
                </div>
                <div class="item-col actions-col">
                  <button @click="removeItemWithNotification(item)" class="delete-btn" :disabled="isLoading">
                    <img src="@/assets/icons/trashcan.png" alt="Удалить" class="delete-icon" />
                  </button>
                </div>
              </div>
            </div>
          </transition-group>
        </div>
        
        <div v-else class="no-data-message">
          {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Нет активных сотрудников' }}
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import RefreshButton from './RefreshButton.vue';

export default {
  name: 'PeopleTable',
  components: {
    RefreshButton
  },
  props: {
    tableName: {
      type: String,
      required: true
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
      isLoading: false,
      currentTableId: null,
      organizationsMap: {}
    };
  },
  computed: {
    displayItems() {
      let filtered = [...this.itemsData];
      
      // Поиск
      if (this.searchQuery) {
        const query = this.searchQuery.toLowerCase().trim();
        filtered = filtered.filter(item => {
          const searchFields = [
            item.last_name,
            item.first_name,
            item.middle_name || '',
            item.organization_name,
            this.formatDate(item.entry_date_to),
            item.status
          ];

          return searchFields.some(field => 
            field && field.toString().toLowerCase().includes(query)
          );
        });
      }

      // Фильтр по организации (теперь по organization_id)
      if (this.selectedOrganizationId) {
        filtered = filtered.filter(item => item.organization_id == this.selectedOrganizationId);
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
            case 'last_name':
            case 'first_name':
            case 'middle_name':
            case 'organization':
            case 'status':
              valueA = (a[this.sortField] || '').toString().toLowerCase();
              valueB = (b[this.sortField] || '').toString().toLowerCase();
              break;
              
            case 'entry_date_to':
              valueA = a.entry_date_to ? new Date(a.entry_date_to) : new Date(0);
              valueB = b.entry_date_to ? new Date(b.entry_date_to) : new Date(0);
              break;
              
            case 'entry_time':
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
    hasActiveFilters() {
      return !!(
        this.searchQuery ||
        this.selectedOrganizationId ||
        this.selectedDate ||
        (this.dateRangeStart && this.dateRangeEnd)
      );
    }
  },
  methods: {
    async loadData() {
      if (this.isLoading) return;
      
      this.isLoading = true;
      
      try {
        await this.fetchPeopleData();
        this.updateActiveItemsCount();
      } catch (error) {
        console.error('Ошибка при загрузке людей:', error);
      } finally {
        this.isLoading = false;
      }
    },

    async fetchPeopleData() {
  try {
    const token = localStorage.getItem("token");
    
    if (!this.tableName) {
      throw new Error('Table name is required');
    }
    
    // Получаем ID таблицы по имени
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
    this.currentTableId = table.id;
    
    // Получаем карту организаций
    await this.fetchOrganizations();
    
    // Создаем обратную карту: название организации → ID
    const nameToIdMap = {};
    Object.keys(this.organizationsMap).forEach(id => {
      nameToIdMap[this.organizationsMap[id]] = id;
    });
    
    // Получаем сотрудников для этой таблицы
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
        status: 'Активен',
        checked: false
      };
    });
    
    console.log('Загружены сотрудники:', this.itemsData);
    
  } catch (error) {
    console.error("Ошибка при загрузке данных сотрудников:", error);
    this.itemsData = [];
    throw error;
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
          // Создаем карту организаций для быстрого поиска по ID
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
      
      if (this.notification.message) {
        clearInterval(this.progressInterval);
      }
      
      this.notification = { 
        message: `Сотрудник ${item.last_name} удален.`, 
        item,
        originalItem: originalItem,
        itemIndex: itemIndex,
        undoFunction: () => {
          if (itemIndex !== -1) {
            this.itemsData[itemIndex] = { ...originalItem };
            this.updateActiveItemsCount();
          }
        }
      };

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
        // Для людей просто удаляем из массива (или делаем API запрос если нужно)
        if (itemIndex !== -1) {
          this.itemsData.splice(itemIndex, 1);
        }
        
        this.updateActiveItemsCount();
      } catch (error) {
        console.error("Ошибка при удалении:", error);
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
  mounted() {
    this.loadData();
  },
  watch: {
    tableName: {
      immediate: true,
      async handler(newVal) {
        console.log('Имя таблицы людей:', newVal);
        if (newVal) {
          await this.loadData();
        }
      }
    },
    selectedOrganizationId(newVal) {
      console.log('Фильтр по организации ID:', newVal);
    },
    searchQuery(newVal) {
      console.log('Поисковый запрос:', newVal);
    }
  },
  beforeUnmount() {
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