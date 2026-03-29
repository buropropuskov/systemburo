<template>
  <div class="approvers-management-container dashboard-card">
    <div class="management-header">
      <h3 class="management-title">Принимающие заявки</h3>
      <div class="header-controls">
        <SearchComponent
          :title="'Поиск принимающих...'"
          v-model="searchQuery"
        />
        
        <button @click="showAddModal = true" class="add-header-button">
          Добавить принимающего
        </button>
        <RefreshButton @refresh="refreshData" />
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - список принимающих (50%) -->
      <div class="table-section" :class="{'with-details': selectedApprover}">
        <div class="table-container">
          <div class="table-header">
            <div class="header-col id-col" @click="sortBy('id')">
              <p :class="{ 'active-sort': sortField === 'id' }">ID</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'id',
                  'desc': sortField === 'id' && sortDirection === 'desc'
                }" 
              />
            </div>
            <div class="header-col name-col" @click="sortBy('full_name')">
              <p :class="{ 'active-sort': sortField === 'full_name' }">ФИО</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'full_name',
                  'desc': sortField === 'full_name' && sortDirection === 'desc'
                }" 
              />
            </div>
            <div class="header-col date-col" @click="sortBy('created_at')">
              <p :class="{ 'active-sort': sortField === 'created_at' }">Добавлен</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'created_at',
                  'desc': sortField === 'created_at' && sortDirection === 'desc'
                }" 
              />
            </div>
          </div>

          <div class="table-body">
            <div 
              v-for="approver in sortedApprovers" 
              :key="approver.id" 
              class="table-row"
              :class="{ 'selected': selectedApprover && selectedApprover.id === approver.id }"
              @click="selectApprover(approver)"
            >
              <div class="table-col id-col">
                <span class="cell-content id-value">{{ approver.id }}</span>
              </div>
              <div class="table-col name-col">
                <span class="truncate-text" :title="getFullName(approver)">
                  {{ getFullName(approver) }}
                </span>
              </div>
              <div class="table-col date-col">
                <span class="cell-content">{{ formatDate(approver.created_at) }}</span>
              </div>
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">
              Всего: {{ filteredApprovers.length }}
            </span>
          </div>
        </div>
      </div>

      <!-- Правая часть - детали принимающего (50%) -->
      <div v-if="selectedApprover" class="details-section">
        <div class="details-content">
          <div class="details-header">
            <h3 class="details-title">{{ getFullName(selectedApprover) }}</h3>
            <button 
              @click="confirmDeleteApprover(selectedApprover)"
              class="delete-btn"
              title="Удалить"
            >
              <img src="@/assets/icons/delete.png" class="delete-icon" />
            </button>
          </div>
          
          <div class="details-body">
            <div class="info-row">
              <span class="info-label">Должность:</span>
              <span class="info-value">{{ selectedApprover.position || '—' }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">Организация:</span>
              <span class="info-value">{{ selectedApprover.organization || '—' }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">Компания:</span>
              <span class="info-value">{{ selectedApprover.company || '—' }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">Добавлен:</span>
              <span class="info-value">{{ formatDate(selectedApprover.created_at) }}</span>
            </div>
          </div>
        </div>
      </div>
      
      <div v-else class="no-selection-message">
        <p>Выберите принимающего</p>
      </div>
    </div>

    <div v-if="filteredApprovers.length === 0 && !searchQuery" class="no-results">
      <p>Нет принимающих</p>
    </div>

    <!-- Модальное окно добавления принимающего -->
    <div v-if="showAddModal" class="modal-overlay" @click.self="closeAddModal">
      <div class="modal modal-compact">
        <div class="modal-header">
          <h3>Добавить принимающего</h3>
          <button class="modal-close" @click="closeAddModal">×</button>
        </div>
        
        <div class="modal-content">
          <div class="user-search-section">
            <input
              v-model="userSearchQuery"
              @input="searchUsers"
              @focus="showUserDropdown = true"
              @blur="onSearchBlur"
              class="search-input"
              placeholder="Поиск пользователей..."
              type="text"
              ref="searchInput"
              autocomplete="off"
            />
            <div v-if="showUserDropdown && filteredAvailableUsers.length > 0" class="user-dropdown">
              <div class="user-dropdown-content">
                <div 
                  v-for="user in filteredAvailableUsers" 
                  :key="user.id"
                  class="user-item"
                  @mousedown.prevent="addUser(user)"
                >
                  <div class="user-info">
                    <div class="user-name">{{ getFullName(user) }}</div>
                    <div class="user-details">
                      <span class="user-username">@{{ user.username }}</span>
                      <span v-if="user.position" class="user-position">{{ user.position }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <div v-else-if="showUserDropdown && filteredAvailableUsers.length === 0" class="user-dropdown no-results-dropdown">
              <div class="user-dropdown-content">
                <div class="no-results-message">
                  Нет доступных пользователей
                </div>
              </div>
            </div>
          </div>
          
          <div class="selected-users">
            <div class="selected-users-header">
              <span>Выбрано пользователей:</span>
              <span class="selected-count">{{ selectedUsers.length }}</span>
            </div>
            
            <div class="users-list-container">
              <div class="users-list">
                <div 
                  v-for="user in selectedUsers" 
                  :key="user.id"
                  class="selected-user"
                >
                  <div class="selected-user-info">
                    <span class="selected-user-name">{{ getFullName(user) }}</span>
                    <span class="selected-user-username">@{{ user.username }}</span>
                    <span v-if="user.position" class="selected-user-position">{{ user.position }}</span>
                  </div>
                  <button 
                    @click="removeUser(user)"
                    class="remove-user-btn"
                    title="Удалить из списка"
                  >
                    ×
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
        
        <div class="modal-footer">
          <button class="modal-cancel-btn" @click="closeAddModal">
            Отмена
          </button>
          <button 
            class="modal-add-btn"
            @click="addApprovers"
            :disabled="selectedUsers.length === 0 || loading"
          >
            {{ loading ? 'Добавление...' : `Добавить (${selectedUsers.length})` }}
          </button>
        </div>
      </div>
    </div>

    <!-- Уведомления -->
    <div v-if="notification.show" class="notification" :class="notification.type">
      {{ notification.message }}
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import SearchComponent from './SearchComponent.vue';
import RefreshButton from './RefreshButton.vue';

export default {
  name: 'ApplicationApproversManagement',
  components: {
    SearchComponent,
    RefreshButton
  },
  data() {
    return {
      searchQuery: '',
      approvers: [],
      allUsers: [],
      selectedApprover: null,
      showAddModal: false,
      userSearchQuery: '',
      showUserDropdown: false,
      selectedUsers: [],
      loading: false,
      sortField: 'full_name',
      sortDirection: 'asc',
      notification: {
        show: false,
        message: '',
        type: 'info'
      }
    };
  },
  computed: {
    filteredApprovers() {
      if (!this.searchQuery) return this.approvers;
      
      const query = this.searchQuery.toLowerCase();
      return this.approvers.filter(a => 
        this.getFullName(a).toLowerCase().includes(query)
      );
    },
    
    sortedApprovers() {
      const approvers = [...this.filteredApprovers];
      
      return approvers.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'id':
            valueA = a.id;
            valueB = b.id;
            break;
          case 'full_name':
            valueA = this.getFullName(a);
            valueB = this.getFullName(b);
            break;
          case 'created_at':
            valueA = a.created_at || '';
            valueB = b.created_at || '';
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
    
    // ID уже добавленных принимающих
    approverIds() {
      return this.approvers.map(a => a.user_id);
    },
    
    // ID выбранных для добавления
    selectedUserIds() {
      return this.selectedUsers.map(u => u.id);
    },
    
    // Доступные пользователи (не в списке принимающих и не выбранные)
    availableUsers() {
      return this.allUsers.filter(user => 
        !this.approverIds.includes(user.id) && 
        !this.selectedUserIds.includes(user.id)
      );
    },
    
    // Отфильтрованные по поиску
    filteredAvailableUsers() {
      if (!this.userSearchQuery.trim()) {
        return this.availableUsers.slice(0, 10);
      }
      
      const query = this.userSearchQuery.toLowerCase();
      return this.availableUsers.filter(user => {
        const fullName = this.getFullName(user).toLowerCase();
        const username = user.username.toLowerCase();
        const position = (user.position || '').toLowerCase();
        
        return fullName.includes(query) ||
               username.includes(query) ||
               position.includes(query);
      }).slice(0, 10);
    }
  },
  methods: {
    getFullName(user) {
      const parts = [];
      if (user.last_name) parts.push(user.last_name);
      if (user.first_name) parts.push(user.first_name);
      if (user.middle_name) parts.push(user.middle_name);
      return parts.length > 0 ? parts.join(' ') : user.username;
    },

    formatDate(dateString) {
      if (!dateString) return '—';
      const date = new Date(dateString);
      return date.toLocaleString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
      });
    },

    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'asc';
      }
    },

    selectApprover(approver) {
      this.selectedApprover = approver;
    },

    searchUsers() {
      this.showUserDropdown = true;
    },

    onSearchBlur() {
      setTimeout(() => {
        this.showUserDropdown = false;
      }, 200);
    },

    addUser(user) {
      this.selectedUsers.push(user);
      this.userSearchQuery = '';
      this.showUserDropdown = false;
      this.$refs.searchInput.blur();
    },

    removeUser(user) {
      this.selectedUsers = this.selectedUsers.filter(u => u.id !== user.id);
    },

    async refreshData() {
      await Promise.all([
        this.fetchApprovers(),
        this.fetchAllUsers()
      ]);
    },

    async fetchApprovers() {
      try {
        const response = await apiRequest('/application-approvers', {});
        if (response.ok) {
          this.approvers = await response.json();
        }
      } catch (error) {
        console.error('Error fetching approvers:', error);
      }
    },

    async fetchAllUsers() {
      try {
        const response = await apiRequest('/users/all', {});
        if (response.ok) {
          this.allUsers = await response.json();
        }
      } catch (error) {
        console.error('Error fetching users:', error);
      }
    },

    async addApprovers() {
      if (this.selectedUsers.length === 0) return;

      this.loading = true;
      let successCount = 0;
      let errorCount = 0;

      for (const user of this.selectedUsers) {
        try {
          const response = await apiRequest('/application-approvers', {
            method: 'POST',
            body: JSON.stringify({ user_id: user.id })
          });

          if (response.ok) {
            successCount++;
          } else {
            errorCount++;
          }
        } catch (error) {
          console.error('Error adding approver:', error);
          errorCount++;
        }
      }

      this.closeAddModal();
      await this.fetchApprovers();
      
      if (errorCount === 0) {
        this.showNotification(`Добавлено ${successCount} принимающих`, 'success');
      } else if (successCount > 0) {
        this.showNotification(`Добавлено ${successCount}, ошибок: ${errorCount}`, 'warning');
      } else {
        this.showNotification('Ошибка при добавлении', 'error');
      }
      
      this.loading = false;
    },

    confirmDeleteApprover(approver) {
      const fullName = this.getFullName(approver);
      if (confirm(`Вы уверены, что хотите удалить пользователя "${fullName}" из списка принимающих?`)) {
        this.deleteApprover(approver);
      }
    },

    async deleteApprover(approver) {
      try {
        const response = await apiRequest(`/application-approvers/${approver.id}`, {
          method: 'DELETE'});

        if (response.ok) {
          if (this.selectedApprover && this.selectedApprover.id === approver.id) {
            this.selectedApprover = null;
          }
          await this.fetchApprovers();
          this.showNotification('Принимающий удален', 'success');
        } else {
          this.showNotification('Ошибка при удалении', 'error');
        }
      } catch (error) {
        console.error('Error deleting approver:', error);
        this.showNotification('Ошибка сети', 'error');
      }
    },

    closeAddModal() {
      this.showAddModal = false;
      this.userSearchQuery = '';
      this.selectedUsers = [];
      this.showUserDropdown = false;
    },

    showNotification(message, type = 'info') {
      this.notification = { show: true, message, type };
      setTimeout(() => {
        this.notification.show = false;
      }, 3000);
    }
  },
  mounted() {
    this.refreshData();
  }
};
</script>

<style scoped>
.approvers-management-container {
  background: #fff;
  border-radius: 16px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 100%;
  height: 500px;
  position: relative;
}

.management-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid #e6e6e6;
  height: 50px;
}

.management-title {
  font-size: 1.2em;
  margin: 0;
  font-weight: 600;
  color: #000;
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.add-header-button {
  padding: 8px 16px;
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 50px;
  cursor: pointer;
  font-size: 0.9em;
  white-space: nowrap;
}

.add-header-button:hover {
  background: #3a45b2;
}

.content-container {
  display: flex;
  height: 450px;
  width: 100%;
}

.table-section {
  width: 50%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #e6e6e6;
}

.table-section.with-details {
  width: 50%;
}

.table-container {
  background: #fff;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.table-header {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid #e6e6e6;
  background: #fff;
  height: 43px;
  align-items: center;
}

.header-col {
  padding: 0 8px;
  font-size: 14px;
  color: #a2a2a2;
  font-weight: 600;
  text-align: left;
  display: flex;
  align-items: center;
  gap: 5px;
  cursor: pointer;
  user-select: none;
}

.header-col:hover {
  color: #000;
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
  color: #000 !important;
}

.id-col {
  width: 10%;
  min-width: 60px;
}

.name-col {
  width: 60%;
  min-width: 200px;
}

.date-col {
  width: 30%;
  min-width: 120px;
}

.table-body {
  flex: 1;
  overflow-y: auto;
  max-height: 407px;
}

.table-row {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid #f0f0f0;
  align-items: center;
  cursor: pointer;
  height: 42px;
  font-size: 14px;
}

.table-row:hover {
  background-color: #fafafa;
}

.table-row.selected {
  background-color: #f8f9ff;
}

.table-col {
  padding: 0 8px;
}

.cell-content {
  display: block;
  padding: 4px 0;
}

.id-value {
  font-weight: 600;
  color: #000;
}

.truncate-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
  display: block;
}

.table-footer {
  padding: 6px 20px;
  border-top: 1px solid #e6e6e6;
  text-align: end;
  background: #f8fafc;
}

.items-count {
  font-size: 12px;
  color: #a2a2a2;
  font-weight: 500;
}

.details-section {
  width: 50%;
  padding: 20px;
  overflow-y: auto;
  background: #fafafa;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.details-title {
  margin: 0;
  color: #000;
  font-size: 1.2em;
  font-weight: 600;
}

.delete-btn {
  outline: none;
  border: none;
  width: 36px;
  height: 36px;
  padding: 8px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f8f9fa;
  cursor: pointer;
  transition: all 0.2s ease;
}

.delete-btn:hover {
  background-color: #fee2e2;
}

.delete-icon {
  width: 20px;
  height: 20px;
  opacity: 0.6;
}

.delete-btn:hover .delete-icon {
  opacity: 1;
}

.details-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.info-row {
  display: flex;
  padding: 8px 0;
  border-bottom: 1px solid #f0f0f0;
  gap: 16px;
}

.info-row:last-child {
  border-bottom: none;
}

.info-label {
  width: 120px;
  font-size: 0.85em;
  color: #a2a2a2;
  flex-shrink: 0;
}

.info-value {
  flex: 1;
  font-size: 0.95em;
  color: #000;
}

.no-selection-message {
  width: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a2a2a2;
  font-size: 14px;
}

.no-results {
  text-align: center;
  padding: 40px 20px;
  color: #a2a2a2;
  width: 100%;
}

/* Модальное окно */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 20000;
  animation: fadeIn 0.2s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.modal-compact {
  background: white;
  border-radius: 16px;
  width: 480px;
  max-width: 90%;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.2);
  animation: scaleIn 0.2s ease-out;
}

@keyframes scaleIn {
  from {
    opacity: 0;
    transform: scale(0.95);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid #e6e6e6;
  flex-shrink: 0;
}

.modal-header h3 {
  font-size: 16px;
  font-weight: 600;
  color: #333;
  margin: 0;
}

.modal-close {
  background: none;
  border: none;
  font-size: 22px;
  color: #999;
  cursor: pointer;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.modal-close:hover {
  background: #f0f0f0;
  color: #333;
}

.modal-content {
  flex: 1;
  padding: 16px 20px;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.user-search-section {
  position: relative;
  flex-shrink: 0;
}

.search-input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  font-size: 14px;
  transition: border-color 0.2s ease;
}

.search-input:focus {
  border-color: #4F5BDF;
  outline: none;
}

.user-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  max-height: 250px;
  overflow-y: auto;
  z-index: 1000;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  margin-top: 4px;
}

.user-dropdown-content {
  max-height: 250px;
  overflow-y: auto;
}

.user-item {
  padding: 8px 12px;
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.user-item:hover {
  background-color: #f5f5f5;
}

.user-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.user-name {
  font-weight: 500;
  font-size: 14px;
  color: #000;
}

.user-details {
  display: flex;
  gap: 8px;
  font-size: 12px;
  color: #666;
}

.user-username {
  color: #4F5BDF;
}

.user-position {
  color: #999;
}

.no-results-dropdown {
  padding: 12px;
  text-align: center;
  color: #999;
  font-size: 14px;
}

.selected-users {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.selected-users-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 14px;
  color: #666;
  flex-shrink: 0;
}

.selected-count {
  background: #4F5BDF;
  color: white;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
}

.users-list-container {
  flex: 1;
  overflow-y: auto;
  max-height: 230px;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  background: #fafafa;
}

.users-list {
  display: flex;
  flex-direction: column;
}

.selected-user {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  border-bottom: 1px solid #e6e6e6;
  transition: background-color 0.15s ease;
}

.selected-user:last-child {
  border-bottom: none;
}

.selected-user:hover {
  background-color: #f0f0f0;
}

.selected-user-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.selected-user-name {
  font-weight: 500;
  font-size: 14px;
  color: #000;
}

.selected-user-username {
  font-size: 12px;
  color: #4F5BDF;
}

.selected-user-position {
  font-size: 11px;
  color: #999;
  margin-top: 2px;
}

.remove-user-btn {
  background: none;
  border: none;
  color: #999;
  font-size: 18px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 4px;
  transition: all 0.15s ease;
}

.remove-user-btn:hover {
  background-color: #fee2e2;
  color: #ef4444;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 20px;
  border-top: 1px solid #e6e6e6;
  flex-shrink: 0;
}

.modal-cancel-btn {
  padding: 8px 16px;
  background: #f0f0f0;
  color: #333;
  border: none;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s ease;
}

.modal-cancel-btn:hover {
  background: #e0e0e0;
}

.modal-add-btn {
  padding: 8px 16px;
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s ease;
  min-width: 100px;
}

.modal-add-btn:hover:not(:disabled) {
  background: #3a45c0;
}

.modal-add-btn:disabled {
  background: #ccc;
  cursor: not-allowed;
}

/* Уведомления */
.notification {
  position: fixed;
  top: 0;
  left: 50%;
  transform: translateX(-50%) translateY(-100%);
  padding: 10px 20px;
  border-radius: 0 0 8px 8px;
  color: white;
  font-weight: 500;
  z-index: 21000;
  font-size: 14px;
  animation: slideDown 0.2s ease-out forwards;
}

.notification.success { background: #10b981; }
.notification.error { background: #ef4444; }
.notification.warning { background: #f59e0b; }
.notification.info { background: #3b82f6; }

@keyframes slideDown {
  from { transform: translateX(-50%) translateY(-100%); }
  to { transform: translateX(-50%) translateY(0); }
}

/* Скроллбары */
.user-dropdown-content::-webkit-scrollbar,
.users-list-container::-webkit-scrollbar,
.table-body::-webkit-scrollbar {
  width: 4px;
}

.user-dropdown-content::-webkit-scrollbar-track,
.users-list-container::-webkit-scrollbar-track,
.table-body::-webkit-scrollbar-track {
  background: #f1f1f1;
}

.user-dropdown-content::-webkit-scrollbar-thumb,
.users-list-container::-webkit-scrollbar-thumb,
.table-body::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 4px;
}
</style>