<template>
  <div class="responsible-users-section card">
    <div class="sec-title">
      Ответственные
      <span
        v-if="selectedUsers.length > 0"
        class="count-badge"
      >{{ selectedUsers.length }}</span>
      <span
        v-if="hasSelectedUsers"
        class="sec-actions"
      >
        <span class="save-hint"><span class="dot" />несохранённые</span>
        <button
          class="btn-mini primary"
          :disabled="isSavingUsers"
          @click="saveResponsibleUsers"
        >
          {{ isSavingUsers ? 'Сохранение...' : 'Сохранить' }}
        </button>
        <button
          class="btn-mini"
          :disabled="isSavingUsers"
          @click="cancelResponsibleUsersChanges"
        >
          Отмена
        </button>
      </span>
    </div>

    <div class="stack">
      <!-- Поиск-добавление ответственного -->
      <div class="user-search-container">
        <input
          v-model="userSearchQuery"
          class="add-input"
          placeholder="＋ Добавить ответственного"
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
        v-for="user in sortedUsers"
        :key="user.username"
        class="resp-item"
        :class="{ primary: user.is_primary }"
      >
        <div class="avatar">
          {{ getInitials(user) }}
        </div>
        <div class="resp-body">
          <div class="resp-name-row">
            <span class="resp-name">{{ getUserDisplayName(user) }}</span>
            <span class="resp-user">@{{ user.username }}</span>
          </div>
          <div
            v-if="user.position || user.organization || user.company || user.is_primary"
            class="resp-tags"
          >
            <span
              v-if="user.position"
              class="tag tag-pos"
            >{{ user.position }}</span>
            <span
              v-if="user.organization"
              class="tag tag-neutral"
            >{{ user.organization }}</span>
            <span
              v-if="user.company"
              class="tag tag-neutral"
            >{{ user.company }}</span>
            <span
              v-if="user.is_primary"
              class="tag tag-main"
            >главный</span>
          </div>
          <div class="toggle-row">
            <label class="switch">
              <input
                v-model="user.required_approval"
                type="checkbox"
                @change="updateUserRequiredApproval(user)"
              >
              <span class="slider" />
            </label>
            <span class="toggle-txt">Обязательное согласование</span>
          </div>
        </div>
        <div class="resp-acts">
          <button
            v-if="!user.is_primary"
            class="icon-btn up"
            title="Сделать главным"
            @click="setAsPrimary(user)"
          >
            ↑
          </button>
          <button
            v-else
            class="icon-btn up"
            title="Убрать главного"
            @click="removePrimaryUser(user)"
          >
            ↓
          </button>
          <button
            class="icon-btn danger"
            title="Удалить"
            @click="removeResponsibleUser(user)"
          >
            ×
          </button>
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
  emits: ['users-updated', 'dirty-change', 'count-change'],
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
    },
    // fix 5: поднимаем dirty-состояние в dirtyTracker родителя (предупреждение
    // о несохранённых ответственных при уходе с вкладки/смене сущности).
    hasSelectedUsers: {
      immediate: true,
      handler(dirty) {
        this.$emit('dirty-change', dirty);
      }
    },
    // Счётчик для метаинформации в шапке деталей родителя.
    'selectedUsers.length': {
      immediate: true,
      handler(count) {
        this.$emit('count-change', count);
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

    getInitials(user) {
      const first = (user.last_name || user.first_name || user.username || '').trim();
      const second = (user.last_name ? user.first_name : '') || '';
      const a = first.charAt(0);
      const b = (second || '').charAt(0);
      return (a + b).toUpperCase() || '?';
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
  box-sizing: border-box;
}

/* карточка-секция (эталон мокапа .card) */
.card {
  border: 1px solid #e6e6e6;
  border-radius: 16px;
  padding: 16px;
  background: #fbfbfd;
}

.sec-title {
  font-size: 0.82em;
  font-weight: 700;
  color: #2a2f39;
  letter-spacing: 0.02em;
  text-transform: uppercase;
  margin: 0 0 12px;
  display: flex;
  align-items: center;
  /* резерв под появляющиеся счётчик/Сохранить/Отмена (btn-mini 28px) - чтобы
     их появление не двигало список ответственных ниже */
  min-height: 28px;
  gap: 8px;
}

.count-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 22px;
  height: 20px;
  padding: 0 7px;
  border-radius: 50px;
  background: #eef0ff;
  color: #4F5BDF;
  font-size: 11px;
  font-weight: 700;
}

.sec-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 8px;
  text-transform: none;
}

.save-hint {
  font-size: 11px;
  color: #b26a00;
  background: #fff4e5;
  border-radius: 8px;
  padding: 3px 9px;
  display: inline-flex;
  gap: 6px;
  align-items: center;
  font-weight: 600;
}

.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #f0a020;
  display: inline-block;
}

.btn-mini {
  height: 28px;
  border-radius: 8px;
  padding: 0 12px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  font-family: inherit;
  border: 1px solid #e6e6e6;
  background: #fff;
  color: #4a5361;
  white-space: nowrap;
}

.btn-mini.primary {
  background: #4F5BDF;
  color: #fff;
  border-color: #4F5BDF;
}

.btn-mini:hover:not(:disabled) {
  filter: brightness(0.97);
}

.btn-mini:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.stack {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* поиск-добавление */
.user-search-container {
  position: relative;
}

.add-input {
  width: 100%;
  height: 36px;
  border: 1px dashed #d5d9e0;
  border-radius: 12px;
  padding: 0 14px;
  font-size: 13px;
  color: #3a3f49;
  background: #fff;
  outline: none;
  box-sizing: border-box;
  transition: border-color 0.2s ease;
}

.add-input::placeholder {
  color: #a2a2a2;
}

.add-input:focus {
  border-style: solid;
  border-color: #4F5BDF;
}

.user-dropdown {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  right: 0;
  background: white;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  max-height: 300px;
  overflow-y: auto;
  z-index: 1000;
  box-shadow: 0 8px 24px rgba(20, 25, 40, 0.14);
}

.user-dropdown-content {
  max-height: 300px;
  overflow-y: auto;
}

.user-dropdown-item {
  padding: 8px 12px;
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
  gap: 3px;
}

.user-main-info {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.user-name {
  font-weight: 600;
  font-size: 0.78rem;
  color: #1e293b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-username {
  font-size: 0.68rem;
  color: #94a3b8;
}

.user-details {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.user-tag {
  font-size: 0.65rem;
  padding: 1px 6px;
  border-radius: 6px;
  background: #f1f5f9;
  color: #475569;
  white-space: nowrap;
}

.no-users-dropdown {
  padding: 12px;
  text-align: center;
  color: #94a3b8;
  font-style: italic;
  font-size: 0.72rem;
}

/* карточка ответственного (эталон мокапа .resp-item) */
.resp-item {
  border: 1px solid #eef0f3;
  border-radius: 12px;
  padding: 10px 12px;
  background: #fff;
  display: flex;
  gap: 10px;
  align-items: flex-start;
}

.resp-item.primary {
  border-color: #bfe6cf;
  background: #f4fbf7;
}

.avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: #e7e9ff;
  color: #4F5BDF;
  font-weight: 700;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.resp-body {
  flex: 1;
  min-width: 0;
}

.resp-name-row {
  display: flex;
  align-items: baseline;
  gap: 6px;
  flex-wrap: wrap;
}

.resp-name {
  font-size: 13px;
  font-weight: 600;
  color: #111318;
}

.resp-user {
  font-size: 11px;
  color: #a2a2a2;
}

.resp-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 5px 0;
}

.tag {
  font-size: 10px;
  font-weight: 600;
  border-radius: 6px;
  padding: 2px 7px;
  white-space: nowrap;
}

.tag-pos {
  background: #eef1f6;
  color: #4a5361;
}

.tag-neutral {
  background: #f1f5f9;
  color: #475569;
}

.tag-main {
  background: #e7f8ee;
  color: #12854a;
}

.toggle-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
}

.switch {
  position: relative;
  width: 34px;
  height: 19px;
  flex-shrink: 0;
}

.switch input {
  display: none;
}

.slider {
  position: absolute;
  inset: 0;
  background: #d3d7de;
  border-radius: 20px;
  transition: 0.2s;
  cursor: pointer;
}

.slider::before {
  content: "";
  position: absolute;
  width: 15px;
  height: 15px;
  left: 2px;
  top: 2px;
  background: #fff;
  border-radius: 50%;
  transition: 0.2s;
}

.switch input:checked + .slider {
  background: #4F5BDF;
}

.switch input:checked + .slider::before {
  transform: translateX(15px);
}

.toggle-txt {
  font-size: 11px;
  color: #5a6472;
}

.resp-acts {
  display: flex;
  flex-direction: column;
  gap: 5px;
  flex-shrink: 0;
}

.icon-btn {
  width: 26px;
  height: 26px;
  border-radius: 8px;
  border: 1px solid #e6e6e6;
  background: #fff;
  cursor: pointer;
  font-size: 14px;
  line-height: 1;
  color: #5a6472;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
}

.icon-btn.danger:hover {
  border-color: #fecaca;
  color: #dc3545;
}

.icon-btn.up:hover {
  border-color: #4F5BDF;
  color: #4F5BDF;
}

.no-selected-users {
  text-align: center;
  padding: 16px 8px;
  color: #94a3b8;
  font-size: 0.75rem;
  border: 1px dashed #e2e8f0;
  border-radius: 12px;
  background: #f8fafc;
}

.no-selected-users p {
  margin: 0;
}
</style>
