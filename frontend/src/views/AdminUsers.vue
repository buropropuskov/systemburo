<template>
  <section class="admin-users">
    <header class="admin-users__header">
      <h2 class="admin-users__title">
        Управление пользователями
      </h2>
      <span class="admin-users__count">{{ filteredUsers.length }} из {{ users.length }}</span>
    </header>

    <div class="admin-users__content">
      <!-- Левая панель: таблица -->
      <div class="admin-users__list-panel">
        <div class="admin-users__search">
          <input
            v-model="searchQuery"
            type="text"
            class="admin-users__search-input"
            placeholder="Поиск по логину, ФИО, организации..."
            aria-label="Поиск пользователей"
          >
        </div>

        <div class="admin-users__table-wrap">
          <table
            class="admin-users__table"
            aria-label="Список пользователей"
          >
            <thead>
              <tr>
                <th class="admin-users__th">
                  Логин
                </th>
                <th class="admin-users__th">
                  ФИО
                </th>
                <th class="admin-users__th">
                  Организация
                </th>
                <th class="admin-users__th admin-users__th--type">
                  Тип
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="user in filteredUsers"
                :key="user.id"
                class="admin-users__row"
                :class="{ 'admin-users__row--selected': selectedUser && selectedUser.id === user.id }"
                tabindex="0"
                @click="selectUser(user)"
                @keydown.enter="selectUser(user)"
              >
                <td class="admin-users__td">
                  {{ user.username }}
                </td>
                <td class="admin-users__td">
                  {{ formatUserName(user) }}
                </td>
                <td class="admin-users__td">
                  {{ user.organization || '—' }}
                </td>
                <td class="admin-users__td admin-users__td--type">
                  <StatusBadge
                    :status="user.user_type || 'Неизвестно'"
                    variant="badge"
                  />
                </td>
              </tr>
            </tbody>
          </table>

          <div
            v-if="filteredUsers.length === 0 && !loading"
            class="admin-users__empty"
          >
            <p class="admin-users__empty-text">
              {{ searchQuery ? 'Пользователи не найдены' : 'Нет пользователей' }}
            </p>
          </div>

          <SkeletonTransition :loading="loading">
            <template #skeleton>
              <SkeletonTable
                :rows="8"
                :columns="4"
              />
            </template>
            <span />
          </SkeletonTransition>
        </div>
      </div>

      <!-- Правая панель: детали -->
      <div class="admin-users__detail-panel">
        <div
          v-if="!selectedUser"
          class="admin-users__no-selection"
        >
          <p class="admin-users__no-selection-text">
            Выберите пользователя из списка
          </p>
        </div>

        <template v-else>
          <div class="admin-users__detail-header">
            <h3 class="admin-users__detail-title">
              {{ selectedUser.username }}
            </h3>
            <StatusBadge
              :status="getUserTypeName(editForm.type_id) || 'Неизвестно'"
              variant="badge"
            />
          </div>

          <div class="admin-users__tabs">
            <button
              class="admin-users__tab"
              :class="{ 'admin-users__tab--active': activeTab === 'info' }"
              @click="activeTab = 'info'"
            >
              Основные
            </button>
            <button
              class="admin-users__tab"
              :class="{ 'admin-users__tab--active': activeTab === 'permissions' }"
              @click="switchToPermissions"
            >
              Разрешения
            </button>
          </div>

          <!-- Таб "Основные" -->
          <div
            v-if="activeTab === 'info'"
            class="admin-users__tab-content"
          >
            <div class="admin-users__form">
              <div class="admin-users__form-row admin-users__form-row--three">
                <div class="admin-users__field">
                  <label class="admin-users__label">Фамилия</label>
                  <input
                    v-model="editForm.last_name"
                    type="text"
                    class="admin-users__input"
                  >
                </div>
                <div class="admin-users__field">
                  <label class="admin-users__label">Имя</label>
                  <input
                    v-model="editForm.first_name"
                    type="text"
                    class="admin-users__input"
                  >
                </div>
                <div class="admin-users__field">
                  <label class="admin-users__label">Отчество</label>
                  <input
                    v-model="editForm.middle_name"
                    type="text"
                    class="admin-users__input"
                  >
                </div>
              </div>

              <div class="admin-users__form-row">
                <div class="admin-users__field">
                  <label class="admin-users__label">Должность</label>
                  <input
                    v-model="editForm.position"
                    type="text"
                    class="admin-users__input"
                  >
                </div>
              </div>

              <div class="admin-users__form-row admin-users__form-row--two">
                <div class="admin-users__field">
                  <label class="admin-users__label">Email</label>
                  <input
                    v-model="editForm.email"
                    type="email"
                    class="admin-users__input"
                  >
                </div>
                <div class="admin-users__field">
                  <label class="admin-users__label">Телефон</label>
                  <input
                    v-model="editForm.phone"
                    type="tel"
                    class="admin-users__input"
                  >
                </div>
              </div>

              <div class="admin-users__form-row admin-users__form-row--three">
                <div class="admin-users__field">
                  <label class="admin-users__label">Организация</label>
                  <BaseDropdown
                    v-model="editForm.organization_id"
                    :options="organizations"
                    label-key="name"
                    value-key="id"
                    placeholder="Выберите организацию"
                    :searchable="true"
                  />
                </div>
                <div class="admin-users__field">
                  <label class="admin-users__label">Компания</label>
                  <BaseDropdown
                    v-model="editForm.company_id"
                    :options="companies"
                    label-key="name"
                    value-key="id"
                    placeholder="Выберите компанию"
                    :searchable="true"
                  />
                </div>
                <div class="admin-users__field">
                  <label class="admin-users__label">Тип пользователя</label>
                  <BaseDropdown
                    v-model="editForm.type_id"
                    :options="userTypes"
                    label-key="name"
                    value-key="id"
                    placeholder="Выберите тип"
                  />
                </div>
              </div>
            </div>

            <div class="admin-users__actions">
              <button
                class="admin-users__btn admin-users__btn--primary"
                :disabled="saving"
                @click="saveInfo"
              >
                {{ saving ? 'Сохранение...' : 'Сохранить' }}
              </button>
              <button
                class="admin-users__btn admin-users__btn--outline"
                @click="showPermissionsModal = true"
              >
                Роль и группы прав
              </button>
              <button
                class="admin-users__btn admin-users__btn--outline"
                @click="showPasswordModal = true"
              >
                Сбросить пароль
              </button>
              <button
                class="admin-users__btn admin-users__btn--danger"
                @click="showDeleteModal = true"
              >
                Удалить
              </button>
            </div>
          </div>

          <!-- Таб "Разрешения" -->
          <div
            v-if="activeTab === 'permissions'"
            class="admin-users__tab-content"
          >
            <SkeletonTransition :loading="loadingPermissions">
              <template #skeleton>
                <div style="padding: 16px; display: flex; flex-direction: column; gap: 16px;">
                  <SkeletonLine
                    width="40%"
                    height="14px"
                  />
                  <SkeletonLine
                    width="100%"
                    height="12px"
                  />
                  <SkeletonLine
                    width="100%"
                    height="12px"
                  />
                  <SkeletonLine
                    width="100%"
                    height="12px"
                  />
                  <SkeletonLine
                    width="35%"
                    height="14px"
                  />
                  <SkeletonLine
                    width="100%"
                    height="12px"
                  />
                  <SkeletonLine
                    width="100%"
                    height="12px"
                  />
                </div>
              </template>

              <div
                v-if="permissionTree.length === 0"
                class="admin-users__empty"
              >
                <p class="admin-users__empty-text">
                  Нет доступных разрешений
                </p>
              </div>

              <div
                v-else
                class="admin-users__permissions"
              >
                <div
                  v-for="category in permissionTree"
                  :key="category.category"
                  class="admin-users__perm-group"
                >
                  <h4 class="admin-users__perm-category">
                    {{ category.category }}
                  </h4>
                  <div class="admin-users__perm-list">
                    <label
                      v-for="perm in category.permissions"
                      :key="perm.key"
                      class="admin-users__perm-item"
                    >
                      <input
                        type="checkbox"
                        class="admin-users__checkbox"
                        :checked="userPermissions[perm.key] === true"
                        @change="onPermissionChange(perm.key, $event.target.checked)"
                      >
                      <span class="admin-users__perm-label">{{ perm.label }}</span>
                      <span
                        v-if="perm.description"
                        class="admin-users__perm-desc"
                      >{{ perm.description }}</span>
                    </label>
                  </div>
                </div>
              </div>
            </SkeletonTransition>

            <div class="admin-users__actions">
              <button
                class="admin-users__btn admin-users__btn--primary"
                :disabled="savingPermissions"
                @click="savePermissions"
              >
                {{ savingPermissions ? 'Сохранение...' : 'Сохранить разрешения' }}
              </button>
            </div>
          </div>
        </template>
      </div>
    </div>

    <!-- Модалка: сброс пароля -->
    <BaseModal
      :show="showPasswordModal"
      title="Сброс пароля"
      width="420px"
      @close="closePasswordModal"
    >
      <p class="admin-users__modal-text">
        Новый пароль для <strong>{{ selectedUser?.username }}</strong>:
      </p>
      <div class="admin-users__password-row">
        <input
          ref="passwordInput"
          v-model="newPassword"
          :type="showNewPass ? 'text' : 'password'"
          class="admin-users__input"
          placeholder="Введите новый пароль"
          @keydown.enter="resetPassword"
        >
        <button
          class="admin-users__btn admin-users__btn--icon"
          type="button"
          :aria-label="showNewPass ? 'Скрыть пароль' : 'Показать пароль'"
          @click="showNewPass = !showNewPass"
        >
          {{ showNewPass ? '🙈' : '👁' }}
        </button>
      </div>
      <button
        class="admin-users__btn admin-users__btn--outline admin-users__btn--small"
        type="button"
        @click="generatePassword"
      >
        Сгенерировать
      </button>

      <template #actions>
        <button
          class="admin-users__btn admin-users__btn--outline"
          @click="closePasswordModal"
        >
          Отмена
        </button>
        <button
          class="admin-users__btn admin-users__btn--primary"
          :disabled="!newPassword || savingPassword"
          @click="resetPassword"
        >
          {{ savingPassword ? 'Сохранение...' : 'Сохранить' }}
        </button>
      </template>
    </BaseModal>

    <!-- Модалка: подтверждение удаления -->
    <BaseModal
      :show="showDeleteModal"
      title="Удаление пользователя"
      width="420px"
      @close="showDeleteModal = false"
    >
      <p class="admin-users__modal-text">
        Вы уверены, что хотите удалить пользователя
        <strong>{{ selectedUser?.username }}</strong>?
        Это действие необратимо.
      </p>

      <template #actions>
        <button
          class="admin-users__btn admin-users__btn--outline"
          @click="showDeleteModal = false"
        >
          Отмена
        </button>
        <button
          class="admin-users__btn admin-users__btn--danger"
          :disabled="deleting"
          @click="confirmDelete"
        >
          {{ deleting ? 'Удаление...' : 'Удалить' }}
        </button>
      </template>
    </BaseModal>

    <UserPermissionsModal
      :show="showPermissionsModal"
      :user="selectedUser"
      @close="showPermissionsModal = false"
      @updated="handlePermissionsUpdated"
    />
  </section>
</template>

<script>
import { getUsers, updateUserType, updateUserPassword, updateUserInfo, updateUserOrganization, updateUserCompany, deleteUser } from '@/api/users';
import { getPermissionTree, getUserPermissions, updateUserPermissions } from '@/api/permissions';
import { getOrganizations, getCompanies } from '@/api/organizations';
import { apiRequest } from '@/api/client';
import { useToast } from '@/composables/useToast';
import BaseModal from '@/components/ui/BaseModal.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import StatusBadge from '@/components/ui/StatusBadge.vue';
import SkeletonTransition from '@/components/ui/SkeletonTransition.vue';
import { formatShortName } from '@/utils/formatName';
import SkeletonTable from '@/components/ui/SkeletonTable.vue';
import SkeletonLine from '@/components/ui/SkeletonLine.vue';
import UserPermissionsModal from '@/components/admin/UserPermissionsModal.vue';

export default {
  name: 'AdminUsers',
  components: {
    BaseModal,
    BaseDropdown,
    StatusBadge,
    SkeletonTransition,
    SkeletonTable,
    SkeletonLine,
    UserPermissionsModal,
  },
  setup() {
    const toast = useToast();
    return { toast };
  },
  data() {
    return {
      users: [],
      organizations: [],
      companies: [],
      userTypes: [],
      loading: false,
      saving: false,
      savingPassword: false,
      savingPermissions: false,
      deleting: false,
      loadingPermissions: false,

      searchQuery: '',
      selectedUser: null,
      activeTab: 'info',

      editForm: {
        last_name: '',
        first_name: '',
        middle_name: '',
        position: '',
        email: '',
        phone: '',
        organization_id: null,
        company_id: null,
        type_id: null,
      },

      showPasswordModal: false,
      showDeleteModal: false,
      showPermissionsModal: false,
      newPassword: '',
      showNewPass: false,

      permissionTree: [],
      userPermissions: {},
    };
  },
  computed: {
    filteredUsers() {
      if (!this.searchQuery.trim()) return this.users;
      const q = this.searchQuery.toLowerCase();
      return this.users.filter(user => {
        const fullName = this.formatUserName(user).toLowerCase();
        const username = (user.username || '').toLowerCase();
        const org = (user.organization || '').toLowerCase();
        const company = (user.company || '').toLowerCase();
        return username.includes(q) || fullName.includes(q) || org.includes(q) || company.includes(q);
      });
    },
  },
  async created() {
    await Promise.all([
      this.fetchUsers(),
      this.fetchOrganizations(),
      this.fetchCompanies(),
      this.fetchUserTypes(),
    ]);
  },
  methods: {
    handlePermissionsUpdated() {
      this.fetchUsers();
      if (this.selectedUser) {
        const updated = this.users.find(u => u.id === this.selectedUser.id);
        if (updated) this.selectedUser = updated;
      }
    },
    async fetchUsers() {
      this.loading = true;
      try {
        const data = await getUsers();
        this.users = Array.isArray(data) ? data : [];
      } catch (e) {
        console.error('Ошибка загрузки пользователей:', e);
        this.toast.error('Не удалось загрузить пользователей');
      } finally {
        this.loading = false;
      }
    },

    async fetchOrganizations() {
      try {
        const data = await getOrganizations();
        this.organizations = Array.isArray(data) ? data : [];
      } catch (e) {
        console.error('Ошибка загрузки организаций:', e);
      }
    },

    async fetchCompanies() {
      try {
        const data = await getCompanies();
        this.companies = Array.isArray(data) ? data : [];
      } catch (e) {
        console.error('Ошибка загрузки компаний:', e);
      }
    },

    async fetchUserTypes() {
      try {
        const response = await apiRequest('/user-types');
        if (response.ok) {
          this.userTypes = await response.json();
        }
      } catch (e) {
        console.error('Ошибка загрузки типов:', e);
      }
    },

    selectUser(user) {
      this.selectedUser = { ...user };
      this.editForm = {
        last_name: user.last_name || '',
        first_name: user.first_name || '',
        middle_name: user.middle_name || '',
        position: user.position || '',
        email: user.email || '',
        phone: user.phone || '',
        organization_id: user.organization_id || null,
        company_id: user.company_id || null,
        type_id: user.type_id || null,
      };
      this.activeTab = 'info';
    },

    async saveInfo() {
      if (!this.selectedUser) return;
      this.saving = true;
      try {
        await updateUserInfo(this.selectedUser.username, {
          last_name: this.editForm.last_name || null,
          first_name: this.editForm.first_name || null,
          middle_name: this.editForm.middle_name || null,
          position: this.editForm.position || null,
          email: this.editForm.email || null,
          phone: this.editForm.phone || null,
        });

        if (this.editForm.organization_id !== this.selectedUser.organization_id) {
          await updateUserOrganization(this.selectedUser.username, this.editForm.organization_id);
        }
        if (this.editForm.company_id !== this.selectedUser.company_id) {
          await updateUserCompany(this.selectedUser.username, this.editForm.company_id);
        }
        if (this.editForm.type_id !== this.selectedUser.type_id) {
          await updateUserType(this.selectedUser.username, this.editForm.type_id);
        }

        this.toast.success('Данные пользователя сохранены');
        await this.fetchUsers();

        const updated = this.users.find(u => u.id === this.selectedUser.id);
        if (updated) this.selectUser(updated);
      } catch (e) {
        console.error('Ошибка сохранения:', e);
        this.toast.error('Не удалось сохранить данные');
      } finally {
        this.saving = false;
      }
    },

    async resetPassword() {
      if (!this.newPassword || !this.selectedUser) return;
      this.savingPassword = true;
      try {
        await updateUserPassword(this.selectedUser.username, this.newPassword);
        this.toast.success('Пароль успешно изменён');
        this.closePasswordModal();
      } catch (e) {
        console.error('Ошибка смены пароля:', e);
        this.toast.error('Не удалось сменить пароль');
      } finally {
        this.savingPassword = false;
      }
    },

    closePasswordModal() {
      this.showPasswordModal = false;
      this.newPassword = '';
      this.showNewPass = false;
    },

    generatePassword() {
      const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!#$%&?';
      const length = Math.floor(Math.random() * 6) + 8;
      let password = '';
      for (let i = 0; i < length; i++) {
        password += chars.charAt(Math.floor(Math.random() * chars.length));
      }
      this.newPassword = password;
      this.showNewPass = true;
    },

    async confirmDelete() {
      if (!this.selectedUser) return;
      this.deleting = true;
      try {
        await deleteUser(this.selectedUser.username);
        this.toast.success('Пользователь удалён');
        this.showDeleteModal = false;
        this.selectedUser = null;
        await this.fetchUsers();
      } catch (e) {
        console.error('Ошибка удаления:', e);
        this.toast.error('Не удалось удалить пользователя');
      } finally {
        this.deleting = false;
      }
    },

    async switchToPermissions() {
      this.activeTab = 'permissions';
      if (this.selectedUser) {
        await this.fetchPermissions();
      }
    },

    async fetchPermissions() {
      if (!this.selectedUser) return;
      this.loadingPermissions = true;
      try {
        const [treeData, permsData] = await Promise.all([
          getPermissionTree(),
          getUserPermissions(this.selectedUser.id),
        ]);
        this.permissionTree = Array.isArray(treeData) ? treeData : [];
        this.userPermissions = {};
        if (Array.isArray(permsData)) {
          permsData.forEach(p => { this.userPermissions[p.key] = p.value; });
        }
      } catch (e) {
        console.error('Ошибка загрузки разрешений:', e);
        this.toast.error('Не удалось загрузить разрешения');
      } finally {
        this.loadingPermissions = false;
      }
    },

    onPermissionChange(key, value) {
      this.userPermissions = { ...this.userPermissions, [key]: value };
    },

    async savePermissions() {
      if (!this.selectedUser) return;
      this.savingPermissions = true;
      const permissions = Object.entries(this.userPermissions).map(([key, value]) => ({ key, value }));
      try {
        await updateUserPermissions(this.selectedUser.id, { permissions });
        this.toast.success('Разрешения сохранены');
      } catch (e) {
        console.error('Ошибка сохранения разрешений:', e);
        this.toast.error('Не удалось сохранить разрешения');
      } finally {
        this.savingPermissions = false;
      }
    },

    formatUserName(user) {
      const formatted = formatShortName(user);
      return formatted || '—';
    },

    getUserTypeName(id) {
      if (!id) return null;
      const type = this.userTypes.find(t => t.id === id);
      return type ? type.name : null;
    },
  },
};
</script>

<style scoped>
.admin-users {
  padding: 12px;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.admin-users__header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-bottom: 12px;
}

.admin-users__title {
  font-size: 16px;
  font-weight: 600;
  color: #000;
  margin: 0;
}

.admin-users__count {
  font-size: 12px;
  color: var(--color-text-muted);
  background: var(--color-bg-secondary);
  padding: 2px 10px;
  border-radius: 12px;
}

/* Двухпанельный лейаут */
.admin-users__content {
  display: flex;
  gap: 12px;
  flex: 1;
  min-height: 0;
}

.admin-users__list-panel {
  width: 45%;
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: 20px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.admin-users__detail-panel {
  width: 55%;
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: 20px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* Поиск */
.admin-users__search {
  padding: 10px 12px;
  border-bottom: 1px solid var(--color-border);
}

.admin-users__search-input {
  width: 100%;
  padding: 8px 14px;
  border: 1px solid var(--color-border);
  border-radius: 50px;
  font-size: 13px;
  outline: none;
  transition: border-color 0.2s;
  box-sizing: border-box;
}

.admin-users__search-input:focus {
  border-color: var(--color-primary);
}

/* Таблица */
.admin-users__table-wrap {
  flex: 1;
  overflow-y: auto;
}

.admin-users__table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
}

.admin-users__th {
  position: sticky;
  top: 0;
  background: var(--color-bg-secondary);
  padding: 8px 12px;
  text-align: left;
  font-size: 12px;
  font-weight: 600;
  color: #666;
  border-bottom: 1px solid var(--color-border);
  white-space: nowrap;
}

.admin-users__th--type {
  width: 120px;
}

.admin-users__row {
  cursor: pointer;
  transition: background-color 0.15s;
}

.admin-users__row:hover {
  background-color: #f5f5f5;
}

.admin-users__row--selected {
  background-color: #f0f1ff;
}

.admin-users__row--selected:hover {
  background-color: #e8e9ff;
}

.admin-users__td {
  padding: 8px 12px;
  font-size: 13px;
  color: var(--color-text);
  border-bottom: 1px solid #f0f0f0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.admin-users__td--type {
  overflow: visible;
}

/* Пустое состояние и загрузка */
.admin-users__empty {
  padding: 40px 20px;
  text-align: center;
}

.admin-users__empty-text {
  color: var(--color-text-muted);
  font-size: 13px;
  margin: 0;
}


/* Нет выбранного пользователя */
.admin-users__no-selection {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1;
}

.admin-users__no-selection-text {
  color: var(--color-text-muted);
  font-size: 14px;
}

/* Заголовок деталей */
.admin-users__detail-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid var(--color-border);
}

.admin-users__detail-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text);
  margin: 0;
}

/* Табы */
.admin-users__tabs {
  display: flex;
  border-bottom: 1px solid var(--color-border);
}

.admin-users__tab {
  flex: 1;
  padding: 10px 16px;
  border: none;
  background: none;
  font-size: 13px;
  font-weight: 500;
  color: #666;
  cursor: pointer;
  transition: all 0.2s;
  border-bottom: 2px solid transparent;
}

.admin-users__tab:hover {
  color: var(--color-primary);
  background: #fafaff;
}

.admin-users__tab--active {
  color: var(--color-primary);
  border-bottom-color: var(--color-primary);
}

/* Контент таба */
.admin-users__tab-content {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

/* Форма */
.admin-users__form {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  flex: 1;
}

.admin-users__form-row {
  display: flex;
  gap: 10px;
}

.admin-users__form-row--two > .admin-users__field {
  flex: 1;
}

.admin-users__form-row--three > .admin-users__field {
  flex: 1;
}

.admin-users__field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
}

.admin-users__label {
  font-size: 11px;
  font-weight: 500;
  color: #666;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.admin-users__input {
  width: 100%;
  padding: 8px 14px;
  border: 1px solid var(--color-border);
  border-radius: 50px;
  font-size: 13px;
  outline: none;
  transition: border-color 0.2s;
  box-sizing: border-box;
}

.admin-users__input:focus {
  border-color: var(--color-primary);
}

/* Кнопки действий */
.admin-users__actions {
  display: flex;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid var(--color-border);
  flex-wrap: wrap;
}

.admin-users__btn {
  padding: 8px 16px;
  border-radius: 50px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  border: 1px solid transparent;
  white-space: nowrap;
}

.admin-users__btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.admin-users__btn--primary {
  background: var(--color-primary);
  color: #fff;
  border-color: var(--color-primary);
}

.admin-users__btn--primary:hover:not(:disabled) {
  background: var(--color-primary-hover);
}

.admin-users__btn--outline {
  background: #fff;
  color: var(--color-primary);
  border-color: var(--color-primary);
}

.admin-users__btn--outline:hover:not(:disabled) {
  background: var(--color-primary);
  color: #fff;
}

.admin-users__btn--danger {
  background: #fff;
  color: var(--color-danger);
  border-color: var(--color-danger);
}

.admin-users__btn--danger:hover:not(:disabled) {
  background: var(--color-danger);
  color: #fff;
}

.admin-users__btn--small {
  padding: 4px 12px;
  font-size: 12px;
  margin-top: 8px;
}

.admin-users__btn--icon {
  width: 36px;
  height: 36px;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: 50%;
  flex-shrink: 0;
}

.admin-users__btn--icon:hover {
  background: #f5f5f5;
}

/* Модалка пароля */
.admin-users__modal-text {
  font-size: 14px;
  color: var(--color-text);
  margin: 0 0 12px;
  line-height: 1.5;
}

.admin-users__password-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.admin-users__password-row .admin-users__input {
  flex: 1;
}

/* Разрешения */
.admin-users__permissions {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1;
}

.admin-users__perm-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.admin-users__perm-category {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  margin: 0;
  padding-bottom: 4px;
  border-bottom: 1px solid #f0f0f0;
}

.admin-users__perm-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.admin-users__perm-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: background 0.15s;
}

.admin-users__perm-item:hover {
  background: var(--color-bg-secondary);
}

.admin-users__checkbox {
  width: 16px;
  height: 16px;
  accent-color: var(--color-primary);
  cursor: pointer;
  flex-shrink: 0;
}

.admin-users__perm-label {
  font-size: 13px;
  color: var(--color-text);
}

.admin-users__perm-desc {
  font-size: 11px;
  color: var(--color-text-muted);
  margin-left: auto;
}

/* Скроллбар */
.admin-users__table-wrap::-webkit-scrollbar,
.admin-users__tab-content::-webkit-scrollbar {
  width: 4px;
}

.admin-users__table-wrap::-webkit-scrollbar-track,
.admin-users__tab-content::-webkit-scrollbar-track {
  background: transparent;
}

.admin-users__table-wrap::-webkit-scrollbar-thumb,
.admin-users__tab-content::-webkit-scrollbar-thumb {
  background: #D9E2FF;
  border-radius: 2px;
}

.admin-users__table-wrap,
.admin-users__tab-content {
  scrollbar-width: thin;
  scrollbar-color: #D9E2FF transparent;
}

/* Адаптивность */
@media (max-width: 1024px) {
  .admin-users__content {
    flex-direction: column;
  }

  .admin-users__list-panel,
  .admin-users__detail-panel {
    width: 100%;
  }

  .admin-users__list-panel {
    max-height: 300px;
  }
}

@media (max-width: 768px) {
  .admin-users__form-row--three,
  .admin-users__form-row--two {
    flex-direction: column;
  }

  .admin-users__actions {
    flex-direction: column;
  }

  .admin-users__btn {
    width: 100%;
    text-align: center;
  }
}
</style>
