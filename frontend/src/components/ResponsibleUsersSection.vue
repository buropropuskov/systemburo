<template>
  <div class="responsible-users-section card">
    <div class="sec-title">
      Ответственные
      <span
        v-if="selectedUsers.length > 0"
        class="count-badge"
      >{{ selectedUsers.length }}</span>
      <span
        v-if="hasSelectedUsers && !selectionMode"
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
              <!-- Подпись лежит соседним span, поэтому имя контрола задаём явно:
                   иначе тумблер, вернувшийся в порядок обхода, объявляется безымянным
                   и в списке из нескольких ответственных неразличим. -->
              <input
                v-model="user.required_approval"
                type="checkbox"
                :aria-label="`Обязательное согласование: ${getUserDisplayName(user)}`"
                @change="updateUserRequiredApproval(user)"
              >
              <span class="slider" />
            </label>
            <span class="toggle-txt">Обязательное согласование</span>
          </div>
        </div>
        <div class="resp-acts">
          <button
            v-if="!selectionMode && !user.is_primary"
            class="icon-btn up"
            title="Сделать главным"
            @click="setAsPrimary(user)"
          >
            ↑
          </button>
          <button
            v-else-if="!selectionMode"
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
import { useDeletionsStore } from '@/stores/deletions'
export default {
  name: 'ResponsibleUsersSection',
  props: {
    entity: {
      type: Object,
      default: null
    },
    entityType: {
      type: String,
      required: true,
      validator: value => ['organization', 'company'].includes(value)
    },
    // Режим «только выбор» (групповые операции): без fetch/save сущности и без
    // назначения главного; обязательное согласование - per-user (тумблер на каждой
    // карточке). Выбор через v-model = массив {username, required_approval}.
    selectionMode: {
      type: Boolean,
      default: false
    },
    modelValue: {
      type: Array,
      default: () => []
    }
  },
  emits: ['users-updated', 'dirty-change', 'count-change', 'update:modelValue'],
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
        if (this.selectionMode) return;
        if (newEntity && newEntity.id) {
          this.fetchEntityUsers(newEntity.id);
        }
      }
    },
    // Синк выбора из v-model (групповой режим): сброс/смена набора снаружи.
    modelValue: {
      immediate: true,
      handler(usernames) {
        if (!this.selectionMode) return;
        this.syncSelectedFromModel(usernames);
      }
    },
    // fix 5: поднимаем dirty-состояние в dirtyTracker родителя (предупреждение
    // о несохранённых ответственных при уходе с вкладки/смене сущности).
    hasSelectedUsers: {
      immediate: true,
      handler(dirty) {
        if (this.selectionMode) return;
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

    if (this.selectionMode) {
      this.syncSelectedFromModel(this.modelValue);
      return;
    }
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
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'список пользователей', type: 'error' });
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
          useDeletionsStore().notify({ prefix: 'Ответственные сохранены для ', bold: this.entity.name, type: 'success' });
          this.$emit('users-updated');
        } else {
          const error = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось сохранить ответственных: ', bold: error.message || 'ошибка сервера', type: 'error' });
          await this.fetchEntityUsers(this.entity.id);
        }
      } catch (error) {
        console.error("Error updating responsible users:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось сохранить ответственных: ', bold: 'ошибка сети', type: 'error' });
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
        if (this.selectionMode) this.emitSelection();
      }
      this.userSearchQuery = '';
      this.showUserDropdown = false;
    },

    removeResponsibleUser(user) {
      this.selectedUsers = this.selectedUsers.filter(u => u.username !== user.username);
      if (this.selectionMode) this.emitSelection();
    },

    syncSelectedFromModel(items) {
      // modelValue групповой операции - массив {username, required_approval}
      const norm = (Array.isArray(items) ? items : []).map(x => ({
        username: x.username,
        required_approval: !!x.required_approval,
      }));
      const current = this.selectedUsers.map(u => ({
        username: u.username,
        required_approval: !!u.required_approval,
      }));
      if (JSON.stringify(current) === JSON.stringify(norm)) return;
      this.selectedUsers = norm.map(({ username, required_approval }) => {
        const full = this.allUsers.find(u => u.username === username);
        return full
          ? { ...full, is_primary: false, required_approval }
          : { username, is_primary: false, required_approval };
      });
    },

    emitSelection() {
      this.$emit('update:modelValue', this.selectedUsers.map(u => ({
        username: u.username,
        required_approval: !!u.required_approval,
      })));
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
      if (this.selectionMode) this.emitSelection();
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
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 16px;
  background: var(--surface-sunken);
}

.sec-title {
  font-size: 0.82em;
  font-weight: 700;
  color: var(--accent-text);
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
  background: var(--surface);
  color: var(--accent-text);
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
  color: var(--warning-text);
  background: var(--warning-bg);
  border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
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
  background: var(--warning);
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
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  white-space: nowrap;
}

.btn-mini.primary {
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
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
  border: 1px dashed color-mix(in srgb, var(--accent) 25%, var(--surface));
  border-radius: 12px;
  padding: 0 14px;
  font-size: 13px;
  color: var(--text);
  background: var(--surface);
  outline: none;
  box-sizing: border-box;
  transition: border-color 0.2s ease;
}

.add-input::placeholder {
  color: var(--text-muted);
}

.add-input:focus {
  border-style: solid;
  border-color: var(--accent);
}

.user-dropdown {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  right: 0;
  background: var(--surface);
  border: 1px solid var(--border);
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
  border-bottom: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  cursor: pointer;
  transition: background-color 0.2s ease;
}

/* Подсветка строки списка - тот же токен, что у пунктов BaseDropdown. Раньше здесь
   стоял --surface-2, то есть ровно фон чипов внутри строки, и при наведении чипы
   должности с организацией пропадали (#1894). */
.user-dropdown-item:hover {
  background-color: var(--row-hover);
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
  color: var(--accent-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-username {
  font-size: 0.68rem;
  color: var(--text-muted);
}

.user-details {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

/* Чип на полупрозрачном акценте, а не на непрозрачном слое: строка под ним меняет
   цвет при наведении, и только примесь к подложке даёт одинаковый отрыв в обоих
   состояниях и в обеих палитрах. Текст --text, а не --text-muted: на тонированном
   чипе поверх подсвеченной строки приглушённый давал 4.2 в тёмной теме. */
.user-tag {
  font-size: 0.65rem;
  padding: 1px 6px;
  border-radius: 6px;
  background: var(--accent-tint);
  color: var(--text);
  white-space: nowrap;
}

.no-users-dropdown {
  padding: 12px;
  text-align: center;
  color: var(--text-muted);
  font-style: italic;
  font-size: 0.72rem;
}

/* карточка ответственного (эталон мокапа .resp-item) */
.resp-item {
  border: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  border-radius: 12px;
  padding: 10px 12px;
  background: var(--surface);
  display: flex;
  gap: 10px;
  align-items: flex-start;
}

.resp-item.primary {
  border-color: color-mix(in srgb, var(--success) 30%, var(--surface));
  background: var(--success-bg);
}

.avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--surface-2);
  color: var(--accent-text);
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
  color: var(--accent-text);
}

.resp-user {
  font-size: 11px;
  color: var(--text-muted);
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
  background: var(--surface-2);
  color: var(--text);
}

.tag-neutral {
  background: var(--surface-2);
  color: var(--text-muted);
}

.tag-main {
  background: var(--success-bg);
  color: var(--success-text);
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

/* Инпут прячем визуально, но оставляем в порядке обхода: display:none выбрасывал
   тумблер из tab-order, и переключить его с клавиатуры было нельзя (эталоны
   SwitchToggle/ToggleSwitch прячут так же). */
.switch input {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: -1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0 0 0 0);
  border: 0;
}

/* Кольцо фокуса рисует дорожка - у скрытого инпута его не видно. */
.switch input:focus-visible + .slider {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.slider {
  position: absolute;
  inset: 0;
  background: var(--border);
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
  background: var(--surface);
  border-radius: 50%;
  transition: 0.2s;
}

.switch input:checked + .slider {
  background: var(--accent);
}

.switch input:checked + .slider::before {
  transform: translateX(15px);
}

.toggle-txt {
  font-size: 11px;
  color: var(--text-muted);
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
  border: 1px solid var(--border);
  background: var(--surface);
  cursor: pointer;
  font-size: 14px;
  line-height: 1;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
}

.icon-btn.danger:hover {
  border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
  color: var(--danger-text);
}

.icon-btn.up:hover {
  border-color: var(--accent);
  color: var(--accent-text);
}

.no-selected-users {
  text-align: center;
  padding: 16px 8px;
  color: var(--text-muted);
  font-size: 0.75rem;
  border: 1px dashed var(--border);
  border-radius: 12px;
  background: var(--surface);
}

.no-selected-users p {
  margin: 0;
}
</style>
