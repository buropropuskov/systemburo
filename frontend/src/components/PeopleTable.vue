<template>
  <div class="selected-table-card">
    <transition name="slide-down">
      <div
        v-if="notification.message"
        class="notification"
      >
        <span>{{ notification.message }}</span>
        <button @click="undoDelete">
          Отменить
        </button>
        <div
          class="progress-bar"
          :style="{ width: `${progress}%` }"
        />
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
          Людей зашло: {{ peopleOnTerritory }}
          <button
            class="history-btn"
            @click="openEmployeesHistory"
          >История</button>
        </span>
        <RefreshButton
          :disabled="isLoading"
          @refresh="loadData"
        />
      </div>
    </div>
    
    <div class="card-content">
      <div class="items-header">
        <div class="header-row">
          <div class="col entry-col">
            Вход
          </div>
          <div class="col exit-col">
            Выход
          </div>
          <div
            class="col last-name-col"
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
            class="col first-name-col"
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
            class="col middle-name-col"
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
            class="col position-col"
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
          <div class="loader" />
          <p>Загрузка сотрудников...</p>
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
              @click="openEmployeeDetails(item)"
            >
              <div class="item-data">
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
                    Вход
                  </button>
                </div>
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
                    Выход
                  </button>
                </div>
                <div class="col last-name-col">
                  {{ item.last_name }}
                </div>
                <div class="col first-name-col">
                  {{ item.first_name }}
                </div>
                <div class="col middle-name-col">
                  {{ item.middle_name || '-' }}
                </div>
                <div class="col position-col">
                  {{ item.position || '-' }}
                </div>
                <div class="col organization-col">
                  {{ item.organization_name }}
                </div>
                <div class="col date-col">
                  {{ formatDate(item.entry_date_to) }}
                </div>
                <div class="col time-col">
                  {{ formatPassTime(item.pass_time) }}
                </div>
                <div class="col status-col">
                  <span class="status-text">{{ item.status }}</span>
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
          {{ hasActiveFilters ? 'Нет данных по выбранным фильтрам' : 'Нет активных сотрудников' }}
        </div>
      </div>
    </div>

    <!-- Модальное окно деталей сотрудника -->
    <EmployeeDetailsModal
      v-if="showDetailsModal"
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
      v-if="showEmployeesHistory"
      :table-id="currentTableId"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      @close="showEmployeesHistory = false"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client';
import RefreshButton from './RefreshButton.vue';
import EmployeeDetailsModal from './CreateApplication/EmployeeDetailsModal.vue';
import EmployeesTableHistoryModal from './CreateApplication/EmployeesTableHistoryModal.vue';

export default {
  name: 'PeopleTable',
  components: {
    RefreshButton,
    EmployeeDetailsModal,
    EmployeesTableHistoryModal
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
    },
    currentUserId: {
      type: Number,
      default: null
    },
    currentUserName: {
      type: String,
      default: ''
    }
  },
  emits: ['open-application'],
  data() {
    return {
      sortField: null,
      sortDirection: 'desc',
      itemsData: [],
      notification: { message: null, item: null },
      progress: 100,
      progressInterval: null,
      isLoading: false,
      currentTableId: null,
      organizationsMap: {},
      allTables: [],
      showDetailsModal: false,
      selectedEmployee: null,
      showEmployeesHistory: false,
      pollingInterval: null
    };
  },
  computed: {
    displayItems() {
      let filtered = [...this.itemsData];
      
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
            case 'position':
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
        this.stopPolling();
        this.startPolling();
      },
      immediate: true
    }
  },
  mounted() {
    this.startPolling();
  },
  beforeUnmount() {
    this.stopPolling();
    if (this.progressInterval) clearInterval(this.progressInterval);
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
      await this._loadData(false);
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
      const originalItem = { ...item };
      const itemIndex = this.itemsData.findIndex(i => i.id === item.id);
      if (this.notification.message) clearInterval(this.progressInterval);
      this.notification = { 
        message: `Сотрудник ${item.last_name} удален.`, 
        item,
        originalItem,
        itemIndex,
        undoFunction: () => {
          if (itemIndex !== -1) {
            this.itemsData[itemIndex] = { ...originalItem };
          }
        }
      };
      item.status = 'Удален';
      this.progress = 100;
      clearInterval(this.progressInterval);
      this.progressInterval = setInterval(() => {
        this.progress -= 1;
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
        const response = await apiRequest(`/employees/${item.id}/deactivate`, {
          method: "PUT",
          body: JSON.stringify({ status: 0, user_id: this.currentUserId, table_id: this.currentTableId })
        });
        if (response.ok) {
          this.itemsData.splice(itemIndex, 1);
        } else {
          console.error("Ошибка при удалении");
          if (itemIndex !== -1) this.itemsData[itemIndex] = { ...originalItem };
        }
      } catch (error) {
        console.error("Ошибка сети при удалении:", error);
        if (itemIndex !== -1) this.itemsData[itemIndex] = { ...originalItem };
      }
    },

    undoDelete() {
      clearInterval(this.progressInterval);
      if (this.notification.undoFunction) this.notification.undoFunction();
      this.notification = { message: null, item: null };
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

.entry-col { width: 6%; }
.exit-col { width: 6%; }
.last-name-col { width: 9%; }
.first-name-col { width: 8%; }
.middle-name-col { width: 8%; }
.position-col { width: 9%; }
.citizenship-col { width: 8%; }
.organization-col { width: 11%; }
.pass-places-col { width: 10%; }
.date-col { width: 8%; }
.time-col { width: 8%; }
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

.notification {
  position: fixed;
  top: 20px;
  right: 20px;
  display: flex;
  align-items: center;
  gap: 12px;
  background: #fff;
  border: 1px solid #e6e6e6;
  border-left: 4px solid #FF6668;
  border-radius: 15px;
  padding: 14px 18px;
  box-shadow: 0 3px 10px rgba(0,0,0,0.1);
  z-index: 1000;
  min-width: 280px;
  overflow: hidden;
  font-family: 'Montserrat', sans-serif;
  font-size: 14px;
  color: #000;
}

.notification button {
  margin-left: auto;
  background: #4F5BDF;
  color: white;
  border: none;
  padding: 6px 16px;
  border-radius: 50px;
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  font-weight: 500;
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
  background: #FF6668;
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
  .col.organization-col,
  .col.pass-places-col {
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

  .notification {
    left: 20px;
    right: 20px;
    min-width: auto;
  }

  .action-btn {
    width: 60px;
    height: 28px;
    font-size: 11px;
  }
}
</style>