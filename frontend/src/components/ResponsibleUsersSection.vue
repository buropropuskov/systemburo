<template>
  <div class="responsible-users-section">
    <div class="detail-group">
      <div class="responsible-users__header">
        <div class="header-content">
          <label class="detail-label">Ответственные:</label>
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
      </div>
      
      <div class="responsible-users-container">
        <!-- Главный ответственный -->
        <div v-if="primaryUser" class="primary-responsible-section">
          <div class="section-title">
            <span class="title-text">Главный ответственный</span>
            <span class="primary-badge">ГЛАВНЫЙ</span>
          </div>
          <div class="primary-user-card">
            <div class="primary-user-info">
              <div class="primary-user-main">
                <span class="primary-user-name">{{ getUserDisplayName(primaryUser) }}</span>
                <span class="primary-user-username">@{{ primaryUser.username }}</span>
              </div>
              <div class="primary-user-details">
                <span v-if="primaryUser.position" class="primary-user-position">{{ primaryUser.position }}</span>
                <span v-if="primaryUser.organization" class="primary-user-organization">{{ primaryUser.organization }}</span>
                <span v-if="primaryUser.company" class="primary-user-company">{{ primaryUser.company }}</span>
              </div>
            </div>
            <div class="primary-user-actions">
              <button 
                @click="removePrimaryUser"
                class="remove-primary-btn"
                title="Убрать как главного"
              >
                ×
              </button>
            </div>
          </div>
          <div class="section-hint">
            Только один пользователь может быть главным ответственным
          </div>
        </div>

        <!-- Выбор нового главного ответственного -->
        <div v-if="!primaryUser && selectedUsers.length > 0" class="set-primary-section">
          <div class="section-title">
            <span class="title-text">Назначить главного ответственного</span>
          </div>
          <select 
            v-model="newPrimaryUserUsername"
            class="primary-select"
            @change="setNewPrimaryUser"
          >
            <option value="">Выберите пользователя...</option>
            <option 
              v-for="user in availableForPrimaryUsers" 
              :key="user.username"
              :value="user.username"
            >
              {{ getUserDisplayName(user) }} (@{{ user.username }})
            </option>
          </select>
        </div>

        <!-- Поисковая строка для добавления пользователей -->
        <div class="user-search-container">
          <input
            v-model="userSearchQuery"
            @focus="showUserDropdown = true"
            @blur="onUserSearchBlur"
            class="user-search-input"
            placeholder="Добавить ответственного пользователя..."
            type="text"
          />
          <div v-if="showUserDropdown" class="user-dropdown">
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
                  <span v-if="user.position" class="user-position">{{ user.position }}</span>
                  <span v-if="user.organization" class="user-organization">{{ user.organization }}</span>
                  <span v-if="user.company" class="user-company">{{ user.company }}</span>
                </div>
              </div>
            </div>
            <div v-if="sortedAvailableUsers.length === 0" class="no-users-dropdown">
              Пользователи не найдены
            </div>
          </div>
        </div>

        <!-- Список обычных ответственных -->
        <div v-if="regularUsers.length > 0" class="regular-users-section">
          <div class="section-title">
            <span class="title-text">Обычные ответственные</span>
            <span class="count-badge">{{ regularUsers.length }}</span>
          </div>
          <div class="selected-users-list">
            <div 
              v-for="user in regularUsers" 
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
              <div class="selected-user-actions">
                <button 
                  v-if="!primaryUser"
                  @click="setAsPrimary(user)"
                  class="set-primary-btn"
                  title="Сделать главным"
                >
                  ↑
                </button>
                <button 
                  @click="removeResponsibleUser(user)"
                  class="remove-user-btn"
                  title="Удалить"
                >
                  ×
                </button>
              </div>
            </div>
          </div>
        </div>

        <div v-if="selectedUsers.length === 0" class="no-selected-users">
          <p>Не выбрано ни одного ответственного лица для согласования.</p>
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
      isLoading: false,
      newPrimaryUserUsername: ''
    };
  },
  computed: {
    hasSelectedUsers() {
      return JSON.stringify(this.selectedUsers.map(u => ({
        username: u.username,
        is_primary: u.is_primary
      })).sort()) !== 
             JSON.stringify(this.originalSelectedUsers.map(u => ({
               username: u.username,
               is_primary: u.is_primary
             })).sort());
    },
    
    primaryUser() {
      return this.selectedUsers.find(user => user.is_primary === true);
    },
    
    regularUsers() {
      return this.selectedUsers.filter(user => !user.is_primary);
    },
    
    availableForPrimaryUsers() {
      return this.regularUsers.filter(user => !user.is_primary);
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
    },
    
    filteredAvailableUsers() {
      // Фильтруем пользователей, исключая уже выбранных
      return this.filteredUsers.filter(user => 
        !this.selectedUsers.some(selected => selected.username === user.username)
      );
    },
    
    sortedAvailableUsers() {
      return [...this.filteredAvailableUsers].sort((a, b) => {
        const companyA = (a.company || '').toLowerCase();
        const companyB = (b.company || '').toLowerCase();
        if (companyA !== companyB) {
          return companyA.localeCompare(companyB);
        }
        
        const orgA = (a.organization || '').toLowerCase();
        const orgB = (b.organization || '').toLowerCase();
        if (orgA !== orgB) {
          return orgA.localeCompare(orgB);
        }
        
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
          
          // Обрабатываем данные с учетом нового поля is_primary
          this.selectedUsers = users.map(entityUser => {
            const fullUserData = this.allUsers.find(u => u.username === entityUser.username);
            
            if (fullUserData) {
              return {
                ...entityUser,
                is_primary: entityUser.is_primary || false,
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
    
    // Подготавливаем данные в новом формате
    const usersData = this.selectedUsers.map(user => ({
      username: user.username,
      is_primary: user.is_primary || false
    }));
    
    const response = await fetch(endpoint, {
      method: "PUT",
      headers: {
        "Authorization": `Bearer ${token}`,
        "Content-Type": "application/json",
      },
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
      this.newPrimaryUserUsername = '';
    },

    addResponsibleUser(user) {
      const isAlreadySelected = this.selectedUsers.some(u => u.username === user.username);
      if (!isAlreadySelected) {
        const userWithDetails = {
          ...user,
          is_primary: false,
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
    
    removePrimaryUser() {
      if (this.primaryUser) {
        this.primaryUser.is_primary = false;
      }
    },
    
    setAsPrimary(user) {
      // Сбрасываем всех остальных
      this.selectedUsers.forEach(u => {
        u.is_primary = false;
      });
      
      // Устанавливаем выбранного как главного
      user.is_primary = true;
    },
    
    setNewPrimaryUser() {
      if (!this.newPrimaryUserUsername) return;
      
      const user = this.selectedUsers.find(u => u.username === this.newPrimaryUserUsername);
      if (user) {
        this.setAsPrimary(user);
        this.newPrimaryUserUsername = '';
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
  async mounted() {
    await this.fetchAllUsers();
    
    if (this.entity && this.entity.id) {
      await this.fetchEntityUsers(this.entity.id);
    }
  },
};
</script>

<style scoped>
.responsible-users-section {
  width: 100%;
}

.responsible-users__header {
  margin-bottom: 5px;
}

.header-content {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  height: 18px;
  padding-bottom: 5px;
}

.users-actions {
  display: flex;
  height: 18px;
  width: 118px;
  gap: 6px;
}

.responsible-users-container {
  width: 100%;
  background: #FFF;
  border-radius: 12px;
}

/* Секции */
.primary-responsible-section,
.regular-users-section,
.set-primary-section {
  margin-bottom: 15px;
}

.section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  padding-bottom: 4px;
  border-bottom: 1px solid #f0f0f0;
}

.title-text {
  font-size: 0.75em;
  font-weight: 600;
  color: #333;
}

.primary-badge {
  font-size: 0.6em;
  font-weight: 700;
  color: #fff;
  background: linear-gradient(135deg, #ff6b6b, #ff4757);
  padding: 2px 8px;
  border-radius: 10px;
  text-transform: uppercase;
}

.count-badge {
  font-size: 0.65em;
  font-weight: 600;
  color: #fff;
  background: #4F5BDF;
  padding: 2px 6px;
  border-radius: 8px;
}

.section-hint {
  font-size: 0.65em;
  color: #888;
  font-style: italic;
  margin-top: 4px;
}

/* Карточка главного ответственного */
.primary-user-card {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 10px;
  border-radius: 8px;
  background: linear-gradient(135deg, #fff9e6, #fff0cc);
  border: 2px solid #ffd700;
  box-shadow: 0 2px 8px rgba(255, 215, 0, 0.1);
}

.primary-user-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.primary-user-main {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.primary-user-name {
  font-weight: 700;
  font-size: 0.8em;
  color: #333;
}

.primary-user-username {
  font-size: 0.65em;
  color: #666;
  font-weight: 500;
}

.primary-user-details {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 2px;
}

.primary-user-position,
.primary-user-organization,
.primary-user-company {
  font-size: 0.65em;
  padding: 2px 6px;
  border-radius: 4px;
  white-space: nowrap;
}

.primary-user-position {
  background: #4F5BDF;
  color: white;
  font-weight: 600;
}

.primary-user-organization {
  background: #10b981;
  color: white;
}

.primary-user-company {
  background: #8b5cf6;
  color: white;
}

.primary-user-actions {
  display: flex;
  align-items: center;
}

.remove-primary-btn {
  background: none;
  border: none;
  color: #ff4444;
  font-size: 1.2em;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 4px;
  transition: background-color 0.2s ease;
}

.remove-primary-btn:hover {
  background-color: rgba(255, 68, 68, 0.1);
}

/* Выбор главного ответственного */
.primary-select {
  width: 100%;
  padding: 6px 10px;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  font-size: 0.7em;
  background: #fff;
  color: #333;
  cursor: pointer;
  transition: border-color 0.2s ease;
}

.primary-select:focus {
  border-color: #4F5BDF;
  outline: none;
}

.primary-select option {
  font-size: 0.7em;
  padding: 4px;
}

/* Поиск пользователей */
.user-search-container {
  position: relative;
  margin-bottom: 12px;
}

.user-search-input {
  width: 100%;
  padding: 6px 10px;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  font-size: 0.7em;
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
  border-radius: 12px;
  max-height: 200px;
  overflow-y: auto;
  z-index: 1000;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  margin-top: 8px;
}

.user-dropdown-item {
  padding: 8px;
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
  gap: 3px;
}

.user-main-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-bottom: 3px;
}

.user-name {
  font-weight: 600;
  font-size: 0.7em;
  color: #000;
}

.user-username {
  font-size: 0.65em;
  color: #6b7280;
}

.user-details {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.user-position,
.user-organization,
.user-company,
.user-phone {
  font-size: 0.7em;
  color: #6b7280;
}

.user-position {
  font-weight: 500;
  color: #4F5BDF;
}

.no-users-dropdown {
  padding: 8px;
  text-align: center;
  color: #6b7280;
  font-style: italic;
  font-size: 0.7em;
}

/* Обычные ответственные */
.selected-users-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 200px;
  overflow-y: auto;
}

.selected-user-item {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 8px;
  border-radius: 6px;
  border: 1px solid #e6e6e6;
  background: #fff;
  transition: all 0.2s ease;
}

.selected-user-item:hover {
  border-color: #4F5BDF;
  box-shadow: 0 2px 4px rgba(79, 91, 223, 0.1);
}

.selected-user-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.selected-user-main {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.selected-user-name {
  font-weight: 600;
  font-size: 0.7em;
  color: #000;
}

.selected-user-username {
  font-size: 0.65em;
  color: #6b7280;
}

.selected-user-details {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.selected-user-position,
.selected-user-organization,
.selected-user-company {
  font-size: 0.7em;
  padding: 1px 6px;
  border-radius: 50px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 160px;
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

.selected-user-actions {
  display: flex;
  gap: 4px;
  align-items: center;
}

.set-primary-btn {
  background: #f0f9ff;
  border: 1px solid #0ea5e9;
  color: #0ea5e9;
  font-size: 0.8em;
  font-weight: bold;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 4px;
  transition: all 0.2s ease;
}

.set-primary-btn:hover {
  background: #0ea5e9;
  color: white;
}

.remove-user-btn {
  background: none;
  border: none;
  color: #ef4444;
  font-size: 1em;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 3px;
  transition: background-color 0.2s ease;
}

.remove-user-btn:hover {
  background-color: #fee;
}

.no-selected-users {
  text-align: center;
  padding: 15px;
  color: #6b7280;
  font-size: 0.7em;
  border: 1px dashed #e6e6e6;
  border-radius: 8px;
  background: #fafafa;
}

/* Кнопки сохранения/отмены */
.save-users-btn {
  padding: 2px 6px;
  background: #4F5BDF;
  color: white;
  border: none;
  border-radius: 12px;
  font-size: 0.55em;
  font-weight: 600;
  cursor: pointer;
  transition: background-color 0.2s ease;
  height: 18px;
}

.save-users-btn:hover:not(:disabled) {
  background: #3a45b2;
}

.save-users-btn:disabled {
  background: #9ca3af;
  cursor: not-allowed;
}

.cancel-users-btn {
  padding: 2px 6px;
  font-weight: 600;
  background: #6b7280;
  color: white;
  border: none;
  border-radius: 12px;
  font-size: 0.55em;
  cursor: pointer;
  transition: background-color 0.2s ease;
  height: 18px;
}

.cancel-users-btn:hover:not(:disabled) {
  background: #4b5563;
}

.cancel-users-btn:disabled {
  background: #9ca3af;
  cursor: not-allowed;
}

.detail-label {
  font-size: 0.75em;
  color: #a2a2a2;
  font-weight: 400;
  margin-bottom: 0;
}

@media (max-width: 768px) {
  .responsible-users-section {
    width: 100%;
  }
  
  .header-content {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }
  
  .users-actions {
    justify-content: center;
  }
  
  .selected-user-item {
    flex-direction: column;
    gap: 8px;
  }
  
  .selected-user-actions {
    align-self: flex-end;
  }
  
  .primary-user-card {
    flex-direction: column;
    gap: 8px;
  }
  
  .primary-user-actions {
    align-self: flex-end;
  }
}
</style>