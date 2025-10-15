<template>
  <div class="responsible-users-section">
    <div class="detail-group">
      <div class="responsible-users__header">
        <label class="detail-label">Ответственные за согласование:</label>
        <div v-if="hasSelectedUsers" class="users-actions">
          <button 
            @click="saveResponsibleUsers" 
            class="save-users-btn"
            :disabled="isSavingUsers"
          >
            {{ isSavingUsers ? 'Сохранение...' : 'Сохранить' }}
          </button>
          <button 
            @click="cancelResponsibleUsersChanges" 
            class="cancel-users-btn"
            :disabled="isSavingUsers"
          >
            Отмена
          </button>
        </div>
      </div>
      
      <div class="responsible-users-container">
        <!-- Поисковая строка -->
        <div class="user-search-container">
          <input
            v-model="userSearchQuery"
            @focus="showUserDropdown = true"
            @blur="onUserSearchBlur"
            class="user-search-input"
            placeholder="Поиск пользователей..."
            type="text"
          />
          <div v-if="showUserDropdown" class="user-dropdown">
            <div 
              v-for="user in filteredUsers" 
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
                  <span v-if="user.position" class="user-position">{{ user.position }}</span>
                  <span v-if="user.organization" class="user-organization">{{ user.organization }}</span>
                  <span v-if="user.company" class="user-company">{{ user.company }}</span>
                </div>
              </div>
            </div>
            <div v-if="filteredUsers.length === 0" class="no-users-dropdown">
              Пользователи не найдены
            </div>
          </div>
        </div>

        <!-- Список выбранных пользователей -->
        <div class="selected-users-list">
          <div 
            v-for="user in selectedUsers" 
            :key="user.username"
            class="selected-user-item"
          >
            <div class="selected-user-info">
              <div class="selected-user-main">
                <span class="selected-user-name">{{ getUserDisplayName(user) }}</span>
                <span class="selected-user-username">@{{ user.username }}</span>
              </div>
              <div class="selected-user-details">
                <span v-if="user.position" class="selected-user-position">{{ user.position }}</span>
                <span v-if="user.organization" class="selected-user-organization">{{ user.organization }}</span>
                <span v-if="user.company" class="selected-user-company">{{ user.company }}</span>
              </div>
            </div>
            <button 
              @click="removeResponsibleUser(user)"
              class="remove-user-btn"
            >
              ×
            </button>
          </div>
          <div v-if="selectedUsers.length === 0" class="no-selected-users">
            <p>Нет выбранных ответственных лиц</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
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
      return JSON.stringify(this.selectedUsers.map(u => u.username).sort()) !== 
             JSON.stringify(this.originalSelectedUsers.map(u => u.username).sort());
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
        const phone = (user.phone || '').toLowerCase();
        
        return fullName.includes(query) ||
               username.includes(query) ||
               position.includes(query) ||
               organization.includes(query) ||
               company.includes(query) ||
               phone.includes(query);
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
  methods: {
    async fetchAllUsers() {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch("http://localhost:8080/users/all", {
          headers: {
            "Authorization": `Bearer ${token}`,
          },
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
        // Убедимся, что allUsers загружены перед обработкой
        if (this.allUsers.length === 0) {
          await this.fetchAllUsers();
        }

        const token = localStorage.getItem("token");
        const endpoint = this.entityType === 'organization'
          ? `http://localhost:8080/organizations/${entityId}/users`
          : `http://localhost:8080/companies/${entityId}/users`;
        
        const response = await fetch(endpoint, {
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });
        if (response.ok) {
          const users = await response.json();
          
          // Обогащаем данные пользователей полной информацией из allUsers
          this.selectedUsers = users.map(entityUser => {
            // Находим полные данные пользователя в allUsers
            const fullUserData = this.allUsers.find(u => u.username === entityUser.username);
            
            if (fullUserData) {
              // Объединяем данные: базовые из entityUser и полные из allUsers
              return {
                ...entityUser,
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
              // Если не нашли в allUsers, используем только базовые данные
              return {
                ...entityUser,
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
        const token = localStorage.getItem("token");
        const endpoint = this.entityType === 'organization'
          ? `http://localhost:8080/organizations/${this.entity.id}/users`
          : `http://localhost:8080/companies/${this.entity.id}/users`;
        
        const response = await fetch(endpoint, {
          method: "PUT",
          headers: {
            "Authorization": `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            user_ids: this.selectedUsers.map(u => u.username),
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
          // Перезагружаем данные в случае ошибки
          await this.fetchEntityUsers(this.entity.id);
        } 
      } catch (error) {
        console.error("Error updating responsible users:", error);
        this.showNotification("Ошибка сети", "error");
        // Перезагружаем данные в случае ошибки
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
        // Используем полные данные пользователя
        const userWithDetails = {
          ...user,
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

    onUserSearchBlur() {
      // Небольшая задержка чтобы клик по элементу dropdown успел обработаться
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
  async mounted() {
    // Сначала загружаем всех пользователей
    await this.fetchAllUsers();
    
    // Если сущность уже выбрана, загружаем ее пользователей
    if (this.entity && this.entity.id) {
      await this.fetchEntityUsers(this.entity.id);
    }
  },
};
</script>

<style scoped>
.detail-label {
  font-size: 0.85em;
  color: #a2a2a2;
  font-weight: 400;
}
.responsible-users-section {
  margin-top: 5px;
}

.responsible-users__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 35px;
}

.responsible-users-container {
  width: 100%;
  background: #FFF;
  border-radius: 15px;
  border: 1px solid #e6e6e6;
  padding: 12px;
}

/* Поиск пользователей */
.user-search-container {
  position: relative;
  margin-bottom: 15px;
}

.user-search-input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  font-size: 0.9em;
  transition: border-color 0.2s ease;
  background: #fff;
}

.user-search-input:focus {
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
  border-radius: 15px;
  max-height: 300px;
  overflow-y: auto;
  z-index: 1000;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  margin-top: 10px;
}

.user-dropdown-item {
  padding: 12px;
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.user-dropdown-item:hover {
  background-color: #f8f9ff;
}

.user-dropdown-item:last-child {
  border-bottom: none;
}

.user-dropdown-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.user-main-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.user-name {
  font-weight: 600;
  font-size: 0.9em;
  color: #000;
}

.user-username {
  font-size: 0.8em;
  color: #6b7280;
}

.user-details {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.user-position,
.user-organization,
.user-company,
.user-phone {
  font-size: 0.8em;
  color: #6b7280;
}

.user-position {
  font-weight: 500;
  color: #4F5BDF;
}

.no-users-dropdown {
  padding: 12px;
  text-align: center;
  color: #6b7280;
  font-style: italic;
}

/* Выбранные пользователи */
.selected-users-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.selected-user-item {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 10px;
  
  border-radius: 8px;
  border: 1px solid #e6e6e6;
}

.selected-user-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.selected-user-main {
  display: flex;
  align-items: center;
  gap: 8px;
}

.selected-user-name {
  font-weight: 600;
  font-size: 0.9em;
  color: #000;
}

.selected-user-username {
  font-size: 0.8em;
  color: #6b7280;
}

.selected-user-details {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
  max-width: 375px;
  flex-wrap: wrap;
}

.selected-user-position,
.selected-user-organization,
.selected-user-company {
  font-size: 0.75em;
  color: #6b7280;
  background: #e6e6e6;
  padding: 2px 6px;
  border-radius: 6px;
  white-space: nowrap;
}

.selected-user-position {
  background: #e0e7ff;
  color: #4F5BDF;
  font-weight: 500;
}

.selected-user-organization {
  background: #f0f9ff;
  color: #0369a1;
}

.selected-user-company {
  background: #f0f0f0;
  color: #666;
}

.remove-user-btn {
  background: none;
  border: none;
  color: #ef4444;
  font-size: 1.2em;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 4px;
  transition: background-color 0.2s ease;
  flex-shrink: 0;
  margin-left: 8px;
}

.remove-user-btn:hover {
  background-color: #fee;
}

.no-selected-users {
  text-align: center;
  color: #6b7280;
  font-size: 14px;

}

.no-users-message {
  text-align: center;
  padding: 20px;
  color: #6b7280;
  font-style: italic;
}

.users-actions {
  display: flex;
  gap: 8px;
}

.save-users-btn {
  padding: 0px 8px;
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 15px;
  font-size: 0.6em;
  font-weight: 600;
  cursor: pointer;
  transition: background-color 0.2s ease;
  height: 20px;
}

.save-users-btn:hover:not(:disabled) {
  background: #3a45b2;
}

.save-users-btn:disabled {
  background: #9ca3af;
  cursor: not-allowed;
}

.cancel-users-btn {
  padding: 0px 8px;
  font-weight: 600;
  background: #6b7280;
  color: white;
  border: none;
  border-radius: 15px;
  font-size: 0.6em;
  cursor: pointer;
  transition: background-color 0.2s ease;
  height: 20px;
}

.cancel-users-btn:hover:not(:disabled) {
  background: #4b5563;
}

.cancel-users-btn:disabled {
  background: #9ca3af;
  cursor: not-allowed;
}

@media (max-width: 768px) {
  .users-actions {
    flex-direction: column;
  }
  
  .selected-user-details {
    flex-direction: column;
    gap: 4px;
    align-items: flex-start;
  }
  
  .selected-user-item {
    flex-direction: column;
    gap: 8px;
  }
  
  .remove-user-btn {
    align-self: flex-end;
    margin-left: 0;
  }
}
</style>