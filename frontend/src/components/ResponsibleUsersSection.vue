<template>
  <div class="responsible-users-section">
    <div class="detail-group">
      <div class="responsible-users__header">
        <div class="header-content">
          <div class="title-with-count">
            <label class="detail-label">Ответственные:</label>
            <span
              v-if="selectedUsers.length > 0"
              class="count-badge"
            >{{ selectedUsers.length }}</span>
          </div>
          <div
            v-if="hasSelectedUsers"
            class="users-actions"
          >
            <button 
              class="save-users-btn" 
              :disabled="isSavingUsers"
              @click="saveResponsibleUsers"
            >
              {{ isSavingUsers ? 'Сохранение...' : 'Сохранить' }}
            </button>
            <button 
              class="cancel-users-btn" 
              :disabled="isSavingUsers"
              @click="cancelResponsibleUsersChanges"
            >
              Отмена
            </button>
          </div>
        </div>
      </div>
      
      <div class="responsible-users-container">
        <!-- Поисковая строка для добавления пользователей -->
        <div class="user-search-container">
          <input
            v-model="userSearchQuery"
            class="user-search-input"
            placeholder="Добавить ответственного"
            type="text"
            @focus="showUserDropdown = true"
            @blur="onUserSearchBlur"
            @input="handleSearchInput"
          >
          <div
            v-if="showUserDropdown && sortedAvailableUsers.length > 0"
            class="user-dropdown"
          >
            <div class="user-dropdown-content">
              <div 
                v-for="user in sortedAvailableUsers" 
                :key="user.username"
                class="user-dropdown-item"
                @mousedown="addResponsibleUser(user)"
              >
                <div class="user-dropdown-info">
                  <div class="user-main-info">
                    <span class="user-name">{{ getUserDisplayName(user) }}</span>
                    <span class="user-username">@{{ user.username }}</span>
                  </div>
                  <div class="user-details">
                    <span
                      v-if="user.position"
                      class="user-tag position-tag"
                    >{{ user.position }}</span>
                    <span
                      v-if="user.organization"
                      class="user-tag org-tag"
                    >{{ user.organization }}</span>
                    <span
                      v-if="user.company"
                      class="user-tag company-tag"
                    >{{ user.company }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div
            v-if="showUserDropdown && userSearchQuery && sortedAvailableUsers.length === 0"
            class="user-dropdown"
          >
            <div class="no-users-dropdown">
              Пользователи не найдены
            </div>
          </div>
        </div>

        <!-- Список ответственных -->
        <div
          v-if="selectedUsers.length > 0"
          class="users-list-section"
        >
          <div class="selected-users-list">
            <div 
              v-for="user in sortedUsers" 
              :key="user.username"
              class="selected-user-item"
              :class="{ 'is-primary': user.is_primary }"
            >
              <div class="selected-user-info">
                <div class="selected-user-main">
                  <span class="selected-user-name">{{ getUserDisplayName(user) }}</span>
                  <span class="selected-user-username">@{{ user.username }}</span>
                </div>
                <div class="selected-user-details">
                  <span
                    v-if="user.position"
                    class="user-tag position-tag"
                  >{{ user.position }}</span>
                  <span
                    v-if="user.organization"
                    class="user-tag org-tag"
                  >{{ user.organization }}</span>
                  <span
                    v-if="user.company"
                    class="user-tag company-tag"
                  >{{ user.company }}</span>
                </div>
                <div class="user-settings">
                  <label class="toggle-switch">
                    <input 
                      v-model="user.required_approval" 
                      type="checkbox"
                      @change="updateUserRequiredApproval(user)"
                    >
                    <span class="toggle-slider" />
                  </label>
                  <span class="toggle-label-text">Обязательное согласование</span>
                </div>
              </div>
              <div class="selected-user-actions">
                <button 
                  v-if="!user.is_primary"
                  class="primary-btn"
                  title="Сделать главным"
                  @click="setAsPrimary(user)"
                >
                  ↑
                </button>
                <button 
                  v-if="user.is_primary"
                  class="unprimary-btn"
                  title="Убрать главного"
                  @click="removePrimaryUser(user)"
                >
                  ↓
                </button>
                <button 
                  class="remove-btn"
                  title="Удалить"
                  @click="removeResponsibleUser(user)"
                >
                  ×
                </button>
              </div>
            </div>
          </div>
        </div>

        <div
          v-if="selectedUsers.length === 0"
          class="no-selected-users"
        >
          <p>Нет ответственных</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
export default {
  name: 'ResponsibleUsersSection',
  props: {
    entity: {
      type: Object,
      required: true
    },
    entityType: {
      type: String,
      required: true,
      validator: value => ['organization', 'company'].includes(value)
    }
  },
  emits: ['users-updated'],
  data() {
    return {
      allUsers: [],
      selectedUsers: [],
      originalSelectedUsers: [],
      isSavingUsers: false,
      userSearchQuery: '',
      showUserDropdown: false,
      isLoading: false
    };
  },
  computed: {
    hasSelectedUsers() {
      return JSON.stringify(this.selectedUsers.map(u => ({
        username: u.username,
        is_primary: u.is_primary,
        required_approval: u.required_approval
      })).sort()) !== 
             JSON.stringify(this.originalSelectedUsers.map(u => ({
               username: u.username,
               is_primary: u.is_primary,
               required_approval: u.required_approval
             })).sort());
    },
    
    sortedUsers() {
      return [...this.selectedUsers].sort((a, b) => {
        if (a.is_primary && !b.is_primary) return -1;
        if (!a.is_primary && b.is_primary) return 1;
        
        const lastNameA = (a.last_name || '').toLowerCase();
        const lastNameB = (b.last_name || '').toLowerCase();
        return lastNameA.localeCompare(lastNameB);
      });
    },
    
    filteredUsers() {
      if (!this.userSearchQuery) return this.allUsers;
      
      const query = this.userSearchQuery.toLowerCase();
      return this.allUsers.filter(user => {
        const fullName = this.getUserDisplayName(user).toLowerCase();
        const username = user.username.toLowerCase();
        const position = (user.position || '').toLowerCase();
        const organization = (user.organization || '').toLowerCase();
        const company = (user.company || '').toLowerCase();
        
        return fullName.includes(query) ||
               username.includes(query) ||
               position.includes(query) ||
               organization.includes(query) ||
               company.includes(query);
      });
    },
    
    filteredAvailableUsers() {
      return this.filteredUsers.filter(user => 
        !this.selectedUsers.some(selected => selected.username === user.username)
      );
    },
    
    sortedAvailableUsers() {
      return [...this.filteredAvailableUsers].sort((a, b) => {
        const lastNameA = (a.last_name || '').toLowerCase();
        const lastNameB = (b.last_name || '').toLowerCase();
        return lastNameA.localeCompare(lastNameB);
      });
    }
  },
  watch: {
    entity: {
      immediate: true,
      handler(newEntity) {
        if (newEntity && newEntity.id) {
          this.fetchEntityUsers(newEntity.id);
        }
      }
    }
  },
  async mounted() {
    await this.fetchAllUsers();
    
    if (this.entity && this.entity.id) {
      await this.fetchEntityUsers(this.entity.id);
    }
  },
  methods: {
    async fetchAllUsers() {
      try {
        const response = await apiRequest("/users/all", {
        });
        if (response.ok) {
          const users = await response.json();
          this.allUsers = users;
        }
      } catch (error) {
        console.error("Error fetching users:", error);
        this.showNotification("Ошибка при загрузке пользователей", "error");
      }
    },

    async fetchEntityUsers(entityId) {
      if (this.isLoading) return;
      
      this.isLoading = true;
      try {
        if (this.allUsers.length === 0) {
          await this.fetchAllUsers();
        }
        const endpoint = this.entityType === 'organization'
          ? `/organizations/${entityId}/users`
          : `/companies/${entityId}/users`;
        
        const response = await apiRequest(endpoint, {
        });
        if (response.ok) {
          const users = await response.json();
          
          this.selectedUsers = users.map(entityUser => {
            const fullUserData = this.allUsers.find(u => u.username === entityUser.username);
            
            if (fullUserData) {
              return {
                ...entityUser,
                is_primary: entityUser.is_primary || false,
                required_approval: entityUser.required_approval || false,
                position: entityUser.position || fullUserData.position || '',
                organization: fullUserData.organization || '',
                company: fullUserData.company || '',
                phone: entityUser.phone || fullUserData.phone || '',
                email: entityUser.email || fullUserData.email || '',
                last_name: entityUser.last_name || fullUserData.last_name,
                first_name: entityUser.first_name || fullUserData.first_name,
                middle_name: entityUser.middle_name || fullUserData.middle_name
              };
            } else {
              return {
                ...entityUser,
                is_primary: entityUser.is_primary || false,
                required_approval: entityUser.required_approval || false,
                position: entityUser.position || '',
                organization: '',
                company: ''
              };
            }
          });
          
          this.originalSelectedUsers = JSON.parse(JSON.stringify(this.selectedUsers));
        } else {
          this.selectedUsers = [];
          this.originalSelectedUsers = [];
        }
      } catch (error) {
        console.error(`Error fetching ${this.entityType} users:`, error);
        this.selectedUsers = [];
        this.originalSelectedUsers = [];
      } finally {
        this.isLoading = false;
      }
    },

    async saveResponsibleUsers() {
      if (!this.entity) return;
      
      this.isSavingUsers = true;
      try {
        const endpoint = this.entityType === 'organization'
          ? `/organizations/${this.entity.id}/users`
          : `/companies/${this.entity.id}/users`;
        
        const usersData = this.selectedUsers.map(user => ({
          username: user.username,
          is_primary: user.is_primary || false,
          required_approval: user.required_approval || false
        }));
        
        const response = await apiRequest(endpoint, {
          method: "PUT",
          body: JSON.stringify({
            users: usersData
          }),
        });
        
        if (response.ok) {
          this.originalSelectedUsers = JSON.parse(JSON.stringify(this.selectedUsers));
          this.showNotification("Ответственные лица успешно обновлены", "success");
          this.$emit('users-updated');
        } else {
          const errorText = await response.text();
          let errorMessage = "Ошибка при обновлении ответственных лиц";
          
          try {
            const errorJson = JSON.parse(errorText);
            errorMessage = errorJson.message || errorMessage;
          } catch {
            errorMessage = errorText || errorMessage;
          }
          
          this.showNotification(errorMessage, "error");
          await this.fetchEntityUsers(this.entity.id);
        } 
      } catch (error) {
        console.error("Error updating responsible users:", error);
        this.showNotification("Ошибка сети", "error");
        await this.fetchEntityUsers(this.entity.id);
      } finally {
        this.isSavingUsers = false;
      }
    },

    cancelResponsibleUsersChanges() {
      this.selectedUsers = JSON.parse(JSON.stringify(this.originalSelectedUsers));
    },

    addResponsibleUser(user) {
      const isAlreadySelected = this.selectedUsers.some(u => u.username === user.username);
      if (!isAlreadySelected) {
        const userWithDetails = {
          ...user,
          is_primary: false,
          required_approval: false,
          position: user.position || '',
          organization: user.organization || '',
          company: user.company || ''
        };
        this.selectedUsers.push(userWithDetails);
      }
      this.userSearchQuery = '';
      this.showUserDropdown = false;
    },

    removeResponsibleUser(user) {
      this.selectedUsers = this.selectedUsers.filter(u => u.username !== user.username);
    },
    
    setAsPrimary(user) {
      this.selectedUsers.forEach(u => {
        u.is_primary = false;
      });
      
      user.is_primary = true;
    },

    removePrimaryUser(user) {
      user.is_primary = false;
    },
    
    updateUserRequiredApproval(user) {
      const selectedUser = this.selectedUsers.find(u => u.username === user.username);
      if (selectedUser) {
        selectedUser.required_approval = user.required_approval;
      }
    },

    handleSearchInput() {
      if (this.userSearchQuery.trim()) {
        this.showUserDropdown = true;
      } else {
        this.showUserDropdown = false;
      }
    },

    onUserSearchBlur() {
      setTimeout(() => {
        this.showUserDropdown = false;
      }, 200);
    },

    getUserDisplayName(user) {
      const names = [user.last_name, user.first_name, user.middle_name].filter(Boolean);
      return names.length > 0 ? names.join(' ') : user.username;
    },

    showNotification(message, type = 'info') {
      const notification = document.createElement('div');
      notification.className = `notification ${type}`;
      notification.textContent = message;
      notification.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        padding: 12px 20px;
        border-radius: 8px;
        color: white;
        font-weight: 500;
        z-index: 1000;
      `;
      
      if (type === 'success') notification.style.backgroundColor = '#10b981';
      if (type === 'error') notification.style.backgroundColor = '#ef4444';
      if (type === 'warning') notification.style.backgroundColor = '#f59e0b';
      if (type === 'info') notification.style.backgroundColor = '#3b82f6';
      
      document.body.appendChild(notification);
      
      setTimeout(() => {
        notification.remove();
      }, 3000);
    }
  },
};
</script>

<style scoped>
.responsible-users-section {
  width: 250px;
  max-width: 250px;
  min-width: 250px;
  box-sizing: border-box;
}

.responsible-users__header {
  margin-bottom: 6px;
}

.header-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 22px;
}

.title-with-count {
  display: flex;
  align-items: center;
  gap: 6px;
}

.detail-label {
  font-size: 0.7rem;
  color: #94a3b8;
  font-weight: 400;
}

.count-badge {
  font-size: 0.6rem;
  font-weight: 600;
  color: #fff;
  background: #4F5BDF;
  padding: 1px 5px;
  border-radius: 8px;
  min-width: 16px;
  text-align: center;
}

.selected-user-main {
  display: flex;
  flex-direction: column;
}

.users-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.responsible-users-container {
  width: 250px;
  background: #FFF;
  border-radius: 8px;
  padding: 4px 0;
  max-height: 800px;
  box-sizing: border-box;
}

/* Поиск */
.user-search-container {
  position: relative;
  margin-bottom: 12px;
  padding: 0 6px;
}

.user-search-input {
  width: 100%;
  padding: 6px 8px;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  font-size: 0.7rem;
  transition: all 0.2s ease;
  background: #fff;
  box-sizing: border-box;
}

.user-search-input:focus {
  border-color: #4F5BDF;
  outline: none;
}

.user-search-input::placeholder {
  color: #94a3b8;
  font-size: 0.65rem;
}

.user-dropdown {
  position: absolute;
  top: calc(100% + 6px);
  left: 6px;
  right: 6px;
  background: white;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  max-height: 300px;
  overflow-y: auto;
  z-index: 1000;
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
}

.user-dropdown-content {
  max-height: 300px;
  overflow-y: auto;
}

.user-dropdown-item {
  padding: 6px 8px;
  border-bottom: 1px solid #f1f5f9;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.user-dropdown-item:hover {
  background-color: #f8fafc;
}

.user-dropdown-item:last-child {
  border-bottom: none;
}

.user-dropdown-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.user-main-info {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.user-name {
  font-weight: 600;
  font-size: 0.7rem;
  color: #1e293b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 200px;
}

.user-username {
  font-size: 0.6rem;
  color: #64748b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 200px;
}

.user-details {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
}

/* Убираем подсветку тегов в дропдауне */
.user-dropdown .user-tag {
  opacity: 0.9;
  background: #f1f5f9;
  color: #475569;
}

.user-dropdown .position-tag {
  background: #f1f5f9;
  color: #475569;
}

.user-dropdown .org-tag {
  background: #f1f5f9;
  color: #475569;
}

.user-dropdown .company-tag {
  background: #f1f5f9;
  color: #475569;
}

.no-users-dropdown {
  padding: 12px;
  text-align: center;
  color: #94a3b8;
  font-style: italic;
  font-size: 0.7rem;
}

/* Список пользователей */
.users-list-section {
  padding: 0 6px;
}

.selected-users-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 350px;
  overflow-y: auto;
  padding-right: 2px;
  padding-bottom: 15px;
}

.selected-user-item {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 8px;
  border-radius: 15px;
  border: 1px solid #e2e8f0;
  background: #fff;
  transition: all 0.2s ease;
  width: 100%;
  box-sizing: border-box;
}

.selected-user-item.is-primary {
  background: linear-gradient(135deg, #fef9e7, #fff3d6);
  border: 1px solid #fcd34d;
}

.selected-user-item:hover:not(.is-primary) {
  border-color: #4F5BDF;
}

.selected-user-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.selected-user-name {
  font-weight: 600;
  font-size: 0.75rem;
  color: #1e293b;
  max-width: 160px;
}

.selected-user-username {
  font-size: 0.6rem;
  color: #64748b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.primary-badge {
  font-size: 0.55rem;
  font-weight: 600;
  color: #b45309;
  background: #fff3cd;
  padding: 1px 4px;
  border-radius: 4px;
  border: 1px solid #fcd34d;
  white-space: nowrap;
}

.selected-user-details {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.user-tag {
  font-size: 0.65rem;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 500;
  white-space: nowrap;
  display: inline-block;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.position-tag {
  background: #e0f2fe;
  color: #0369a1;
}

.org-tag {
  background: #dcfce7;
  color: #166534;
}

.company-tag {
  background: #f1f5f9;
  color: #475569;
}

.user-settings {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 2px;
}

.toggle-switch {
  position: relative;
  display: inline-block;
  width: 28px;
  height: 16px;
  flex-shrink: 0;
}

.toggle-switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.toggle-slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: #cbd5e1;
  transition: 0.2s;
  border-radius: 16px;
}

.toggle-slider:before {
  position: absolute;
  content: "";
  height: 12px;
  width: 12px;
  left: 2px;
  bottom: 2px;
  background-color: white;
  transition: 0.2s;
  border-radius: 50%;
}

input:checked + .toggle-slider {
  background-color: #4F5BDF;
}

input:checked + .toggle-slider:before {
  transform: translateX(12px);
}

.toggle-label-text {
  font-size: 0.6rem;
  color: #475569;
  white-space: nowrap;
}

.selected-user-actions {
  display: flex;
  gap: 2px;
  align-items: center;
  margin-left: 4px;
  flex-shrink: 0;
}

.primary-btn {
  background: #fef9e7;
  border: 1px solid #fcd34d;
  color: #b45309;
  font-size: 0.8rem;
  font-weight: bold;
  cursor: pointer;
  width: 22px;
  height: 22px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
  flex-shrink: 0;
}

.primary-btn:hover {
  background: #fcd34d;
  color: #1e293b;
}

.unprimary-btn {
  background: #fff3cd;
  border: 1px solid #fcd34d;
  color: #b45309;
  font-size: 0.8rem;
  font-weight: bold;
  cursor: pointer;
  width: 22px;
  height: 22px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
  flex-shrink: 0;
}

.unprimary-btn:hover {
  background: #fcd34d;
  color: #1e293b;
}

.remove-btn {
  background: none;
  border: none;
  color: #94a3b8;
  font-size: 1rem;
  cursor: pointer;
  width: 22px;
  height: 22px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
  flex-shrink: 0;
}

.remove-btn:hover {
  background: #fee2e2;
  color: #ef4444;
}

.no-selected-users {
  text-align: center;
  padding: 16px 8px;
  color: #94a3b8;
  font-size: 0.7rem;
  border: 1px dashed #e2e8f0;
  border-radius: 6px;
  background: #f8fafc;
  margin: 8px 6px;
}

.save-users-btn,
.cancel-users-btn {
  padding: 2px 8px;
  border: none;
  border-radius: 12px;
  font-size: 0.6rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  height: 20px;
  white-space: nowrap;
}

.save-users-btn {
  background: #4F5BDF;
  color: white;
}

.save-users-btn:hover:not(:disabled) {
  background: #3a45b2;
}

.cancel-users-btn {
  background: #e2e8f0;
  color: #475569;
}

.cancel-users-btn:hover:not(:disabled) {
  background: #cbd5e1;
}

.save-users-btn:disabled,
.cancel-users-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

@media (max-width: 768px) {
  .responsible-users-section {
    width: 100%;
    max-width: 100%;
    min-width: auto;
  }
  
  .responsible-users-container {
    width: 100%;
  }
}
</style>