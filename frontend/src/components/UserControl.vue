<template>
  <div class="user-management dashboard-card">
    <div class="management-header">
      <h3 class="management-title">
        Учётные записи пользователей
      </h3>
      <div class="search-container">
        <BaseDropdown
          class="archive-dropdown"
          :model-value="showArchive ? 'archive' : 'active'"
          :options="archiveOptions"
          label-key="label"
          value-key="value"
          @update:model-value="onArchiveModeChange"
        />
        <SearchComponent
          v-model="userSearch"
          :title="'Поиск пользователей...'"
        />
        <button
          class="lk-button lk-button--primary"
          @click="openCreateModal"
        >
          Создать
        </button>
        <RefreshButton
          :loading="refreshing"
          @refresh="refreshAllData"
        />
      </div>
    </div>

    <div class="users-container">
      <div class="users-list">
        <!-- Заголовок таблицы -->
        <div class="users-header">
          <div class="header-row">
            <div
              class="header-col login-col"
              @click="sortBy('username')"
            >
              <p :class="{ 'active-sort': sortField === 'username' }">
                Логин
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'username',
                  'desc': sortField === 'username' && sortDirection === 'desc'
                }" 
              >
            </div>
            <div
              class="header-col name-col"
              @click="sortBy('full_name')"
            >
              <p :class="{ 'active-sort': sortField === 'full_name' }">
                Фамилия И.О.
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'full_name',
                  'desc': sortField === 'full_name' && sortDirection === 'desc'
                }" 
              >
            </div>
            <div
              class="header-col org-col"
              @click="sortBy('organization')"
            >
              <p :class="{ 'active-sort': sortField === 'organization' }">
                Организация / Отдел
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'organization',
                  'desc': sortField === 'organization' && sortDirection === 'desc'
                }" 
              >
            </div>
            <div
              class="header-col company-col"
              @click="sortBy('company')"
            >
              <p :class="{ 'active-sort': sortField === 'company' }">
                Компания
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'company',
                  'desc': sortField === 'company' && sortDirection === 'desc'
                }" 
              >
            </div>
            <div
              class="header-col position-col"
              @click="sortBy('position')"
            >
              <p :class="{ 'active-sort': sortField === 'position' }">
                Должность
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'position',
                  'desc': sortField === 'position' && sortDirection === 'desc'
                }"
              >
            </div>
            <div
              class="header-col type-col"
              @click="sortBy('user_type')"
            >
              <p :class="{ 'active-sort': sortField === 'user_type' }">
                Тип
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'user_type',
                  'desc': sortField === 'user_type' && sortDirection === 'desc'
                }"
              >
            </div>
          </div>
        </div>
        
        <!-- Тело таблицы -->
        <div class="users-body">
          <div
            v-for="user in sortedUsers"
            :key="user.username"
            class="user-item"
            :class="{ 'inactive': user.is_active === false }"
            @click="selectUser(user)"
          >
            <div class="user-row">
              <div class="user-col login-col">
                <span class="user-login">{{ user.username }}</span>
                <span
                  v-if="user.is_active === false"
                  class="inactive-badge"
                >(архив)</span>
              </div>
              <div class="user-col name-col">
                {{ formatUserName(user) }}
              </div>
              <div class="user-col org-col">
                <span
                  class="truncate-text"
                  :title="user.organization || '-'"
                >
                  {{ user.organization || '-' }}
                </span>
              </div>
              <div class="user-col company-col">
                <span
                  class="truncate-text"
                  :title="user.company || '-'"
                >
                  {{ user.company || '-' }}
                </span>
              </div>
              <div class="user-col position-col">
                <span
                  class="truncate-text"
                  :title="user.position || '-'"
                >
                  {{ user.position || '-' }}
                </span>
              </div>
              <div class="user-col type-col">
                <span
                  v-if="user.user_type"
                  class="type-badge"
                >{{ user.user_type }}</span>
                <span v-else>-</span>
              </div>
            </div>
          </div>
        </div>
        <div class="users-footer">
          <span class="items-count">{{ showArchive ? 'В архиве' : 'Всего пользователей' }}: {{ sortedUsers.length }}</span>
        </div>
      </div>
    </div>

    <div
      v-if="filteredUsers.length === 0"
      class="no-users"
    >
      <p>{{ userSearch ? 'Пользователи не найдены' : 'Пользователи отсутствуют' }}</p>
    </div>

    <!-- Модальное окно редактирования пользователя -->
    <BaseModal
      :show="showEditModal && !!selectedUser"
      width="880px"
      content-class="user-edit-modal"
      :z-index="900"
      @close="closeEditModal"
    >
      <template #header>
        <div class="modal-title-group">
          <h3 class="modal-title">
            Редактирование
          </h3>
          <p class="modal-subtitle">
            учётной записи <strong>{{ selectedUser && selectedUser.username }}</strong>
          </p>
        </div>
        <div
          v-if="selectedUser"
          class="modal-header-actions"
        >
          <button
            class="lk-button lk-button--secondary"
            @click="openHistory(selectedUser)"
          >
            История
          </button>
          <template v-if="selectedUser.is_active !== false">
            <button
              class="lk-button lk-button--secondary"
              data-testid="user-reset-onboarding"
              @click="resetOnboarding(selectedUser)"
            >
              Сбросить обучение
            </button>
          </template>
          <template v-else>
            <span class="archive-badge">В архиве</span>
            <button
              class="lk-button lk-button--primary"
              @click="restoreUser(selectedUser)"
            >
              Восстановить
            </button>
          </template>
        </div>
      </template>

      <div
        v-if="selectedUser"
        class="modal-body-inner"
      >
        <nav
          class="modal-tabs"
          role="tablist"
        >
          <button
            type="button"
            class="modal-tab"
            :class="{ 'modal-tab--active': activeTab === 'profile' }"
            role="tab"
            :aria-selected="activeTab === 'profile'"
            @click="activeTab = 'profile'"
          >
            Профиль
          </button>
          <button
            v-if="selectedUser.is_active !== false"
            type="button"
            class="modal-tab"
            :class="{ 'modal-tab--active': activeTab === 'access' }"
            role="tab"
            :aria-selected="activeTab === 'access'"
            @click="activeTab = 'access'"
          >
            Доступ
          </button>
          <button
            type="button"
            class="modal-tab"
            :class="{ 'modal-tab--active': activeTab === 'logins' }"
            role="tab"
            :aria-selected="activeTab === 'logins'"
            data-testid="user-tab-logins"
            @click="activeTab = 'logins'"
          >
            История входов
          </button>
        </nav>

        <div
          v-show="activeTab === 'profile'"
          class="tab-panel"
        >
          <div class="details-grid-two-columns">
            <div class="details-column">
              <div class="detail-group">
                <label class="detail-label">Фамилия:</label>
                <input
                  v-model="selectedUser.last_name"
                  class="lk-input"
                  placeholder="Введите фамилию"
                  autocomplete="new-password"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  @change="updateUserInfo(selectedUser)"
                >
              </div>

              <div class="detail-group">
                <label class="detail-label">Отчество:</label>
                <input
                  v-model="selectedUser.middle_name"
                  class="lk-input"
                  placeholder="Введите отчество"
                  autocomplete="new-password"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  @change="updateUserInfo(selectedUser)"
                >
              </div>

              <div class="detail-group">
                <label class="detail-label">Организация:</label>
                <BaseDropdown
                  :model-value="selectedUser.organization_id"
                  :options="orgOptionsWithNone"
                  label-key="name"
                  value-key="id"
                  placeholder="Не выбрано"
                  searchable
                  @update:model-value="onSelectOrganization"
                />
              </div>

              <div class="detail-group">
                <label class="detail-label">Должность:</label>
                <input
                  v-model="selectedUser.position"
                  class="lk-input"
                  placeholder="Введите должность"
                  autocomplete="new-password"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  @change="updateUserInfo(selectedUser)"
                >
              </div>
            </div>

            <div class="details-column">
              <div class="detail-group">
                <label class="detail-label">Имя:</label>
                <input
                  v-model="selectedUser.first_name"
                  class="lk-input"
                  placeholder="Введите имя"
                  autocomplete="new-password"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  @change="updateUserInfo(selectedUser)"
                >
              </div>

              <div class="detail-group">
                <label class="detail-label">Телефон:</label>
                <input
                  :value="selectedUser.phone"
                  class="lk-input"
                  placeholder="+7 (___) ___ __-__"
                  type="tel"
                  autocomplete="new-password"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  @input="onPhoneInput($event, 'selectedUser')"
                  @change="updateUserInfo(selectedUser)"
                >
              </div>

              <div class="detail-group">
                <label class="detail-label">Компания:</label>
                <BaseDropdown
                  :model-value="selectedUser.company_id"
                  :options="companyOptionsWithNone"
                  label-key="name"
                  value-key="id"
                  placeholder="Не выбрано"
                  searchable
                  @update:model-value="onSelectCompany"
                />
              </div>

              <div class="detail-group">
                <label class="detail-label">Email:</label>
                <input
                  v-model="selectedUser.email"
                  class="lk-input"
                  placeholder="Введите email"
                  type="email"
                  autocomplete="new-password"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  @change="updateUserInfo(selectedUser)"
                >
              </div>
            </div>
          </div>

          <div class="full-width-groups">
            <div class="detail-group detail-group--checkbox">
              <ToggleSwitch
                :model-value="!!selectedUser.is_important"
                @update:model-value="val => { selectedUser.is_important = val; updateUserInfo(selectedUser); }"
              >
                Важный пользователь
              </ToggleSwitch>
            </div>

            <div class="detail-group">
              <label class="detail-label">Тип пользователя:</label>
              <BaseDropdown
                :model-value="selectedUser.type_id"
                :options="userTypes"
                label-key="name"
                value-key="id"
                placeholder="Не выбрано"
                @update:model-value="onSelectUserType"
              />
            </div>

            <div class="detail-group password-group">
              <label class="detail-label">Новый пароль:</label>
              <div class="password-input-container">
                <input
                  v-model="selectedUser.newPassword"
                  :type="showNewPass ? 'text' : 'password'"
                  class="password-input-sm"
                  placeholder="Новый пароль"
                  autocomplete="new-password"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  @keyup="checkInputLanguage($event)"
                  @keyup.enter="changeUserPassword(selectedUser)"
                >
                <div class="password-actions">
                  <button
                    class="generate-password-btn"
                    type="button"
                    @click="generatePassword(selectedUser)"
                  >
                    <img
                      src="@/assets/icons/random.png"
                      class="generate-icon"
                    >
                    Генерировать
                  </button>
                  <button
                    :disabled="!changePasswordValid"
                    class="save-password-btn"
                    @click="changeUserPassword(selectedUser)"
                  >
                    <img
                      src="@/assets/icons/save.png"
                      class="save-icon"
                    >
                  </button>
                </div>
              </div>
              <div class="input-hints">
                <span
                  class="language-hint"
                  :class="{ 'warning': isCapsLockOn }"
                >
                  {{ currentLanguage }} {{ isCapsLockOn ? '| CAPS LOCK' : '' }}
                </span>
                <ul
                  v-if="selectedUser && selectedUser.newPassword"
                  class="password-checklist"
                >
                  <li
                    v-for="rule in changePasswordRules"
                    :key="rule.key"
                    :class="{ 'password-checklist__item--ok': rule.ok }"
                    class="password-checklist__item"
                  >
                    {{ rule.ok ? '✓' : '○' }} {{ rule.label }}
                  </li>
                </ul>
              </div>
            </div>
          </div>

          <div
            v-if="selectedUser.is_active !== false"
            class="danger-zone"
          >
            <span class="danger-zone__hint">Удаление переносит учётную запись в архив и блокирует вход.</span>
            <button
              class="lk-button lk-button--danger"
              @click="confirmDeleteUser(selectedUser)"
            >
              Удалить учётную запись
            </button>
          </div>
        </div>

        <div
          v-if="selectedUser.is_active !== false"
          v-show="activeTab === 'access'"
          class="tab-panel"
        >
          <div class="access-card">
            <div class="access-card__body">
              <strong>Права доступа</strong>
              <p>Индивидуальные права, роли и группы пользователя.</p>
            </div>
            <button
              class="lk-button lk-button--secondary"
              @click="openAccess(selectedUser)"
            >
              Настроить
            </button>
          </div>
          <div
            v-if="selectedUserIsSecurity"
            class="access-card"
          >
            <div class="access-card__body">
              <strong>Места доступа</strong>
              <p>Площадки и точки прохода для сотрудника охраны.</p>
            </div>
            <button
              class="lk-button lk-button--secondary"
              data-testid="user-access-places"
              @click="openAccessPlaces(selectedUser)"
            >
              Настроить
            </button>
          </div>
        </div>

        <div
          v-show="activeTab === 'logins'"
          class="tab-panel"
        >
          <UserLoginHistory
            v-if="activeTab === 'logins'"
            :username="selectedUser.username"
            :current-user-name="currentUserName"
          />
        </div>
      </div>
    </BaseModal>

    <!-- Модальное окно создания пользователя -->
    <BaseModal
      :show="showCreateModal"
      title="Создание пользователя"
      width="600px"
      content-class="user-create-modal"
      @close="closeCreateModal"
    >
      <div class="create-user-body">
        <div class="form-wrap">
          <div class="input-group half">
            <label class="input-label">Логин <span class="required">*</span></label>
            <input
              ref="usernameInput"
              v-model="newUser.username"
              placeholder="Введите логин"
              class="modal-input"
              @input="saveDraft"
            >
          </div>
          <div class="input-group half">
            <label class="input-label">Пароль <span class="required">*</span></label>
            <PasswordInput
              v-model="newUser.password"
              placeholder="Введите пароль"
              @input="saveDraft"
            />
            <ul
              v-if="newUser.password"
              class="password-checklist"
            >
              <li
                v-for="rule in createPasswordRules"
                :key="rule.key"
                :class="{ 'password-checklist__item--ok': rule.ok }"
                class="password-checklist__item"
              >
                {{ rule.ok ? '✓' : '○' }} {{ rule.label }}
              </li>
            </ul>
          </div>
          <div class="input-group half">
            <label class="input-label">Организация <span class="required">*</span></label>
            <BaseDropdown
              :model-value="newUser.organization_id"
              :options="orgOptionsWithNone"
              label-key="name"
              value-key="id"
              placeholder="Не выбрано"
              searchable
              @update:model-value="onSelectNewUserOrg"
            />
          </div>
          <div class="input-group half">
            <label class="input-label">Компания <span class="required">*</span></label>
            <BaseDropdown
              :model-value="newUser.company_id"
              :options="companyOptionsWithNone"
              label-key="name"
              value-key="id"
              placeholder="Не выбрано"
              searchable
              @update:model-value="onSelectNewUserCompany"
            />
          </div>
          <div class="input-group full">
            <label class="input-label">Тип пользователя <span class="required">*</span></label>
            <BaseDropdown
              :model-value="newUser.type_id"
              :options="userTypes"
              label-key="name"
              value-key="id"
              placeholder="Выберите тип"
              @update:model-value="onSelectNewUserType"
            />
          </div>
          <div class="input-group half">
            <label class="input-label">Фамилия</label>
            <input
              v-model="newUser.last_name"
              placeholder="Введите фамилию"
              class="modal-input"
              @input="saveDraft"
            >
          </div>
          <div class="input-group half">
            <label class="input-label">Имя</label>
            <input
              v-model="newUser.first_name"
              placeholder="Введите имя"
              class="modal-input"
              @input="saveDraft"
            >
          </div>
          <div class="input-group half">
            <label class="input-label">Отчество</label>
            <input
              v-model="newUser.middle_name"
              placeholder="Введите отчество"
              class="modal-input"
              @input="saveDraft"
            >
          </div>
          <div class="input-group half">
            <label class="input-label">Должность</label>
            <input
              v-model="newUser.position"
              placeholder="Введите должность"
              class="modal-input"
              @input="saveDraft"
            >
          </div>
          <div class="input-group half">
            <label class="input-label">Email</label>
            <input
              v-model="newUser.email"
              placeholder="Введите email"
              class="modal-input"
              type="email"
              @input="saveDraft"
            >
          </div>
          <div class="input-group half">
            <label class="input-label">Телефон</label>
            <input
              :value="newUser.phone"
              placeholder="+7 (___) ___ __-__"
              class="modal-input"
              type="tel"
              @input="onPhoneInput($event, 'newUser')"
            >
          </div>
        </div>
      </div>

      <template #actions>
        <button
          class="modal-btn modal-btn--cancel"
          @click="closeCreateModal"
        >
          Отмена
        </button>
        <button
          :disabled="!canCreateUser"
          class="modal-btn modal-btn--confirm"
          :class="{'modal-btn--disabled': !canCreateUser}"
          @click="createUser"
        >
          Создать
        </button>
      </template>
    </BaseModal>

    <ConfirmationModal
      :show="!!deleteConfirmUser"
      title="Удаление пользователя"
      :message="deleteConfirmUser ? `Удалить учётную запись «${deleteConfirmUser.username}»? Действие необратимо.` : ''"
      confirm-text="Удалить"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
      @confirm="performDeleteUser"
      @cancel="deleteConfirmUser = null"
    />

    <UserHistoryModal
      v-if="historyForUser"
      :user="historyForUser"
      :organizations="organizations"
      :companies="companies"
      :user-types="userTypes"
      :current-user-name="currentUserName"
      @close="historyForUser = null"
    />

    <UserAccessModal
      v-if="accessUser"
      :user="accessUser"
      @close="accessUser = null"
      @updated="onAccessUpdated"
    />

    <UserAccessPlacesModal
      v-if="accessPlacesUser"
      :user="accessPlacesUser"
      @close="accessPlacesUser = null"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { ref } from 'vue';
import { mapState, mapActions } from 'pinia';
import { useOrganizationsStore } from '@/stores/organizations';
import { useCompaniesStore } from '@/stores/companies';
import { formatRussianPhone } from '@/composables/useRussianPhoneMask'
import { formatShortName } from '@/utils/formatName'
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants'
import SearchComponent from './SearchComponent.vue';
import RefreshButton from './RefreshButton.vue';
import PasswordInput from './ui/PasswordInput.vue';
import { getPasswordPolicy } from '@/api/settings';
import { evaluatePassword, passwordMeetsPolicy, generatePassword as buildPassword, DEFAULT_PASSWORD_POLICY } from '@/utils/passwordPolicy';
import ConfirmationModal from './ConfirmationModal.vue';
import BaseModal from './ui/BaseModal.vue';
import BaseDropdown from './ui/BaseDropdown.vue';
import ToggleSwitch from './ui/ToggleSwitch.vue';
import UserHistoryModal from './UserHistoryModal.vue';
import UserLoginHistory from './UserLoginHistory.vue';
import UserAccessModal from './admin/UserAccessModal.vue';
import UserAccessPlacesModal from './admin/UserAccessPlacesModal.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';
import { resetOnboardingForUser } from '@/api/onboarding';

export default {
  components: {
    SearchComponent,
    RefreshButton,
    PasswordInput,
    ConfirmationModal,
    BaseModal,
    BaseDropdown,
    ToggleSwitch,
    UserHistoryModal,
    UserLoginHistory,
    UserAccessModal,
    UserAccessPlacesModal
  },
  props: {
    allUsers: {
      type: Array,
      required: true
    }
  },
  emits: ['fetch-users', 'user-updated'],
  setup() {
    const showCreateModal = ref(false);
    const showEditModal = ref(false);

    function closeEditModal() {
      showEditModal.value = false;
    }

    return { showCreateModal, showEditModal, closeEditModal };
  },
  data() {
    return {
      userSearch: '',
      refreshing: false,
      selectedUser: null,
      activeTab: 'profile',
      historyForUser: null,
      accessUser: null,
      accessPlacesUser: null,
      currentUserName: '',
      deleteConfirmUser: null,
      showArchive: false,
      archiveOptions: [
        { value: 'active', label: 'Активные' },
        { value: 'archive', label: 'Архив' },
      ],
      showNewPass: false,
      currentLanguage: '',
      isCapsLockOn: false,
      userTypes: [],
      sortField: null,
      sortDirection: 'desc',
      passwordPolicy: { ...DEFAULT_PASSWORD_POLICY },
      newUser: {
        username: '',
        password: '',
        last_name: '',
        first_name: '',
        middle_name: '',
        email: '',
        phone: '',
        position: '',
        type_id: null,
        organization_id: null,
        company_id: null
      }
    };
  },
  computed: {
    ...mapState(useOrganizationsStore, { organizations: 'items' }),
    ...mapState(useCompaniesStore, { companies: 'items' }),
    // Опции с пунктом "Не выбрано" (null) для дропдаунов создания - орг/компания опциональны.
    orgOptionsWithNone() {
      return [{ id: null, name: 'Не выбрано' }, ...this.organizations];
    },
    companyOptionsWithNone() {
      return [{ id: null, name: 'Не выбрано' }, ...this.companies];
    },
    // Места доступа настраиваются только охранникам ЧОП — резолвим по стабильному
    // user_types.code, не по числовому type_id (нестабилен между средами).
    selectedUserIsSecurity() {
      if (!this.selectedUser) return false;
      const type = this.userTypes.find(t => t.id === this.selectedUser.type_id);
      return type?.code === 'security';
    },
    filteredUsers() {
      const variants = buildSearchVariants(this.userSearch);
      return this.allUsers
        .filter(user => (this.showArchive ? user.is_active === false : user.is_active !== false))
        .filter(user => matchesSearch(
          `${user.username} ${user.organization || ''} ${user.company || ''} `
          + `${user.user_type || ''} ${user.position || ''} ${this.formatUserName(user)}`,
          variants,
        ))
        .map(user => ({
          ...user,
          newPassword: '',
          last_name: user.last_name || '',
          first_name: user.first_name || '',
          middle_name: user.middle_name || '',
          position: user.position || '',
          email: user.email || '',
          phone: user.phone || ''
        }));
    },
    sortedUsers() {
      const users = [...this.filteredUsers];
      
      if (!this.sortField) {
        return users.sort((a, b) => a.username.localeCompare(b.username));
      }
      
      return users.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'username':
            valueA = a.username;
            valueB = b.username;
            break;
          case 'full_name':
            valueA = this.formatUserName(a);
            valueB = this.formatUserName(b);
            break;
          case 'organization':
            valueA = a.organization || '';
            valueB = b.organization || '';
            break;
          case 'company':
            valueA = a.company || '';
            valueB = b.company || '';
            break;
          case 'position':
            valueA = a.position || '';
            valueB = b.position || '';
            break;
          case 'user_type':
            valueA = a.user_type || '';
            valueB = b.user_type || '';
            break;
          default:
            return 0;
        }
        
        if (valueA < valueB) {
          return this.sortDirection === 'asc' ? 1 : -1;
        }
        if (valueA > valueB) {
          return this.sortDirection === 'asc' ? -1 : 1;
        }
        return 0;
      });
    },
    createPasswordRules() {
      return evaluatePassword(this.passwordPolicy, this.newUser.password || '');
    },
    createPasswordValid() {
      return passwordMeetsPolicy(this.passwordPolicy, this.newUser.password || '');
    },
    changePasswordRules() {
      return evaluatePassword(this.passwordPolicy, (this.selectedUser && this.selectedUser.newPassword) || '');
    },
    changePasswordValid() {
      return passwordMeetsPolicy(this.passwordPolicy, (this.selectedUser && this.selectedUser.newPassword) || '');
    },
    canCreateUser() {
      return (
        this.newUser.username &&
        this.newUser.password &&
        this.createPasswordValid &&
        this.newUser.type_id &&
        this.hasOrgOrCompany
      );
    },
    hasOrgOrCompany() {
      return Boolean(this.newUser.organization_id || this.newUser.company_id);
    }
  },
  watch: {
    // После рефетча списка (роль/группы/блокировка менялись в модалке прав)
    // пере-резолвим открытую карточку на свежий объект из allUsers. Иначе
    // selectedUser держит копию старого user и роль/права в карточке остаются
    // устаревшими до перезагрузки страницы.
    allUsers(list) {
      if (!this.selectedUser) return;
      const fresh = list.find((u) => u.username === this.selectedUser.username);
      if (fresh) this.selectedUser = { ...fresh, newPassword: this.selectedUser.newPassword || '' };
    },
  },
  async created() {
    // Подтягиваем актуальные справочники из stores. Дальнейшая синхронизация
    // (после CRUD в OrganizationsManagement/CompaniesManagement) идёт через
    // pinia reactivity - listener'ы на window.event больше не нужны.
    await Promise.all([
      this.fetchOrganizations(),
      this.fetchCompanies(),
      this.fetchUserTypes(),
      this.fetchCurrentUser()
    ]);
    this.loadPasswordPolicy();
  },
  mounted() {
    this.fetchAllUsers();
    this.loadDraft();
  },
 
  methods: {
    ...mapActions(useOrganizationsStore, {
      fetchOrganizations: 'fetchOrganizations',
      refreshOrganizations: 'refresh',
    }),
    ...mapActions(useCompaniesStore, {
      fetchCompanies: 'fetchCompanies',
      refreshCompanies: 'refresh',
    }),

    // onPhoneInput применяет российскую маску к введённому телефону и записывает
    // результат обратно в reactive model. Используется для newUser/selectedUser
    // чтобы явно триггерить saveDraft (где он нужен) отдельно.
    onPhoneInput(event, modelKey) {
      const masked = formatRussianPhone(event.target.value)
      this[modelKey].phone = masked
      if (modelKey === 'newUser') {
        this.saveDraft()
      }
    },

    async refreshAllData() {
      this.refreshing = true;
      try {
        await Promise.all([
          this.fetchOrganizations(),
          this.fetchCompanies(),
          this.fetchUserTypes(),
          this.fetchAllUsers()
        ]);
      } finally {
        this.refreshing = false;
      }
    },

    onArchiveModeChange(value) {
      this.showArchive = value === 'archive';
      this.selectedUser = null;
    },

    async restoreUser(user) {
      try {
        const response = await apiRequest(`/users/${user.username}/restore`, { method: 'POST' });
        if (response.ok) {
          this.closeEditModal();
          this.selectedUser = null;
          this.$emit('fetch-users');
          this.refreshOrganizations();
          this.refreshCompanies();
          useDeletionsStore().notify({ prefix: 'Пользователь ', bold: user.username, suffix: ' восстановлен из архива' });
        } else {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось восстановить: ', bold: errorData.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error('Ошибка сети при восстановлении пользователя:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось восстановить: ', bold: 'нет связи с сервером', type: 'error' });
      }
    },

    async resetOnboarding(user) {
      const ok = await useUiStore().confirm({
        title: 'Сбросить обучение?',
        message: `Пользователь «${user.username}» снова увидит автозапуск обучающего тура при следующем входе.`,
        confirmText: 'Сбросить',
        cancelText: 'Отмена',
        danger: false,
      });
      if (!ok) return;
      try {
        await resetOnboardingForUser(user.username);
        useDeletionsStore().notify({ prefix: 'Обучение сброшено для ', bold: user.username, suffix: ' — тур запустится снова при входе' });
      } catch (error) {
        useDeletionsStore().notify({ prefix: 'Не удалось сбросить обучение: ', bold: error?.message || 'ошибка', type: 'error' });
      }
    },

    openHistory(user) {
      this.historyForUser = user;
    },

    openAccess(user) {
      this.accessUser = user;
    },

    openAccessPlaces(user) {
      this.accessPlacesUser = user;
    },

    onAccessUpdated() {
      // Роль/группы/блокировка/индивидуальные права изменились - перечитываем список.
      this.$emit('fetch-users');
    },

    async fetchCurrentUser() {
      // Имя нужно для футера Excel-экспорта истории ("Отчёт сформировал").
      try {
        const res = await apiRequest('/users/me');
        if (!res.ok) return;
        const u = await res.json();
        const parts = [u.last_name, u.first_name, u.middle_name].filter(Boolean);
        this.currentUserName = parts.join(' ') || u.username || '';
      } catch {
        // Имя - необязательная деталь экспорта, молчим (footer покажет дефолт).
      }
    },

    formatUserName(user) {
      const formatted = formatShortName(user);
      return formatted || '-';
    },

    onSelectOrganization(id) {
      this.selectedUser.organization_id = id;
      this.updateUserOrganization(this.selectedUser);
    },

    onSelectCompany(id) {
      this.selectedUser.company_id = id;
      this.updateUserCompany(this.selectedUser);
    },

    onSelectUserType(id) {
      this.selectedUser.type_id = id;
      this.updateUserType(this.selectedUser);
    },

    onSelectNewUserType(id) {
      this.newUser.type_id = id;
      this.saveDraft();
    },

    onSelectNewUserOrg(id) {
      this.newUser.organization_id = id;
      this.saveDraft();
    },

    onSelectNewUserCompany(id) {
      this.newUser.company_id = id;
      this.saveDraft();
    },
    
    // Методы для работы с черновиком
    saveDraft() {
      localStorage.setItem('newUserDraft', JSON.stringify(this.newUser));
    },
    
    loadDraft() {
      const saved = localStorage.getItem('newUserDraft');
      if (saved) {
        try {
          this.newUser = JSON.parse(saved);
        } catch (e) {
          console.error('Error loading draft:', e);
          localStorage.removeItem('newUserDraft');
        }
      }
    },
    
    clearDraft() {
      localStorage.removeItem('newUserDraft');
    },
    
    openCreateModal() {
      this.loadDraft();
      this.showCreateModal = true;
      this.$nextTick(() => {
        if (this.$refs.usernameInput) {
          this.$refs.usernameInput.focus();
        }
      });
    },
    
    closeCreateModal() {
      this.showCreateModal = false;
    },
    
    handleUserCreated() {
      this.showCreateModal = false;
      this.clearDraft();
      this.resetNewUser();
      this.$emit('fetch-users');
      // Обновляем оба представления (items + itemsWithUsers) - чтобы синхронно
      // отрисовался user_count в OrganizationsManagement/CompaniesManagement.
      this.refreshOrganizations();
      this.refreshCompanies();
    },
    
    resetNewUser() {
      this.newUser = {
        username: '',
        password: '',
        last_name: '',
        first_name: '',
        middle_name: '',
        email: '',
        phone: '',
        position: '',
        type_id: null,
        organization_id: null,
        company_id: null
      };
      this.clearDraft();
    },
    
    async createUser() {
      if (!this.canCreateUser) return;

      try {
        // Админская регистрация через POST /users (JWT-защищённый).
        // Публичный /register намеренно не экспонируется.
        const response = await apiRequest("/users", {
          method: "POST",
          body: JSON.stringify({
            username: this.newUser.username,
            password: this.newUser.password,
            last_name: this.newUser.last_name || null,
            first_name: this.newUser.first_name || null,
            middle_name: this.newUser.middle_name || null,
            email: this.newUser.email || null,
            phone: this.newUser.phone || null,
            position: this.newUser.position || null,
            type_id: this.newUser.type_id,
            organization_id: this.newUser.organization_id || null,
            company_id: this.newUser.company_id || null
          }),
        });

        if (response.ok) {
          const createdName = this.newUser.username;
          this.handleUserCreated();
          useDeletionsStore().notify({ prefix: 'Пользователь ', bold: createdName, suffix: ' создан' });
        } else {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось создать пользователя: ', bold: errorData.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error("Ошибка сети при создании пользователя:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось создать пользователя: ', bold: 'нет связи с сервером', type: 'error' });
      }
    },
    
    async updateUserInfo(user) {
      try {
        const response = await apiRequest(`/users/${user.username}/info`,
          {
            method: "PUT",
            body: JSON.stringify({
              last_name: user.last_name || null,
              first_name: user.first_name || null,
              middle_name: user.middle_name || null,
              position: user.position || null,
              email: user.email || null,
              phone: user.phone || null,
              is_important: !!user.is_important,
            }),
          }
        );

        if (!response.ok) {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось обновить данные: ', bold: errorData.message || 'ошибка', type: 'error' });
          this.$emit('fetch-users');
        } else {
          this.$emit('user-updated', user);
        }
      } catch (error) {
        console.error("Ошибка сети при обновлении информации:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось обновить данные: ', bold: 'нет связи с сервером', type: 'error' });
        this.$emit('fetch-users');
      }
    },
    
    confirmDeleteUser(user) {
      // Открываем ConfirmationModal вместо window.confirm. Реальное удаление - performDeleteUser.
      this.deleteConfirmUser = user;
    },

    async performDeleteUser() {
      const user = this.deleteConfirmUser;
      this.deleteConfirmUser = null;
      if (!user) return;
      try {
        const response = await apiRequest(`/users/${user.username}`,
          {
            method: "DELETE",
          }
        );

        if (response.ok) {
          this.closeEditModal();
          this.selectedUser = null;
          this.$emit('fetch-users');
          // user_count меняется - подтягиваем оба представления.
          this.refreshOrganizations();
          this.refreshCompanies();
          useDeletionsStore().notify({ prefix: 'Пользователь ', bold: user.username, suffix: ' удалён' });
        } else {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось удалить пользователя: ', bold: errorData.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error("Ошибка сети при удалении пользователя:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось удалить пользователя: ', bold: 'нет связи с сервером', type: 'error' });
      }
    },
    
    generatePassword(user) {
      user.newPassword = buildPassword(this.passwordPolicy);
      this.showNewPass = true;
    },
    
    async loadPasswordPolicy() {
      try {
        this.passwordPolicy = await getPasswordPolicy();
      } catch (error) {
        console.error('Не удалось загрузить политику паролей:', error);
        this.passwordPolicy = { ...DEFAULT_PASSWORD_POLICY };
      }
    },

    async fetchUserTypes() {
      try {
        const response = await apiRequest("/user-types", {
        });
        if (response.ok) {
          this.userTypes = await response.json();
        }
      } catch (error) {
        console.error("Error fetching user types:", error);
      }
    },
    
    checkInputLanguage(event) {
      if (!event || typeof event.getModifierState !== 'function') return;
      
      const isRussian = /[а-яА-ЯЁё]/.test(event.key);
      this.currentLanguage = isRussian ? 'RU' : 'EN';
      this.isCapsLockOn = event.getModifierState('CapsLock');
    },
    
    async updateUserType(user) {
      try {
        const response = await apiRequest(`/users/${user.username}/type`,
          {
            method: "PUT",
            body: JSON.stringify({ type_id: user.type_id }),
          }
        );

        if (!response.ok) {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось сменить тип: ', bold: errorData.message || 'ошибка', type: 'error' });
          this.$emit('fetch-users');
        } else {
          const type = this.userTypes.find(t => t.id === user.type_id);
          if (type) user.user_type = type.name;
          this.$emit('user-updated', user);
        }
      } catch (error) {
        console.error("Ошибка сети при обновлении типа пользователя:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось сменить тип: ', bold: 'нет связи с сервером', type: 'error' });
        this.$emit('fetch-users');
      }
    },
    
    async updateUserOrganization(user) {
      try {
        const response = await apiRequest(`/users/${user.username}/organization`,
          {
            method: "PUT",
            body: JSON.stringify({ organization_id: user.organization_id }),
          }
        );

        if (!response.ok) {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось сменить организацию: ', bold: errorData.message || 'ошибка', type: 'error' });
          this.$emit('fetch-users');
        } else {
          const org = this.organizations.find(o => o.id === user.organization_id);
          if (org) user.organization = org.name;
          this.$emit('user-updated', user);
        }
      } catch (error) {
        console.error("Ошибка сети при обновлении организации:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось сменить организацию: ', bold: 'нет связи с сервером', type: 'error' });
        this.$emit('fetch-users');
      }
    },
    
    async updateUserCompany(user) {
      try {
        const response = await apiRequest(`/users/${user.username}/company`,
          {
            method: "PUT",
            body: JSON.stringify({ company_id: user.company_id }),
          }
        );

        if (!response.ok) {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось сменить компанию: ', bold: errorData.message || 'ошибка', type: 'error' });
          this.$emit('fetch-users');
        } else {
          const comp = this.companies.find(c => c.id === user.company_id);
          if (comp) user.company = comp.name;
          this.$emit('user-updated', user);
        }
      } catch (error) {
        console.error("Ошибка сети при обновлении компании:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось сменить компанию: ', bold: 'нет связи с сервером', type: 'error' });
        this.$emit('fetch-users');
      }
    },
    
    async changeUserPassword(user) {
      if (!user.newPassword) {
        useDeletionsStore().notify({ bold: 'Введите новый пароль', type: 'error' });
        return;
      }

      if (!passwordMeetsPolicy(this.passwordPolicy, user.newPassword)) {
        useDeletionsStore().notify({ bold: 'Пароль не соответствует требованиям', type: 'error' });
        return;
      }

      try {
        const response = await apiRequest(`/users/${user.username}/password`,
          {
            method: "PUT",
            body: JSON.stringify({ password: user.newPassword }),
          }
        );

        if (response.ok) {
          user.newPassword = "";
          this.$emit('fetch-users');
          useDeletionsStore().notify({ prefix: 'Пароль пользователя ', bold: user.username, suffix: ' изменён' });
        } else {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось изменить пароль: ', bold: errorData.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error("Ошибка сети при изменении пароля:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось изменить пароль: ', bold: 'нет связи с сервером', type: 'error' });
      }
    },
    
    fetchAllUsers() {
      this.$emit('fetch-users');
    },
    
    selectUser(user) {
      this.selectedUser = { ...user };
      this.activeTab = 'profile';
      this.showEditModal = true;
    },

    closeDetails() {
      this.closeEditModal();
      this.selectedUser = null;
    },

    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'desc';
      }
    }
  }
};
</script>

<style scoped>
.user-management {
  background-color: #fff;
  border-radius: 30px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 100%;
}

.management-header {
  border-bottom: 1px solid #e6e6e6;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0px 20px;
  height: 50px;
}

.management-title {
  font-size: 1.1em;
  margin: 0;
  font-weight: 600;
  color: #000;
}

.search-container {
  display: flex;
  align-items: center;
  gap: 8px;
}

.users-container {
  display: flex;
  height: fit-content;
  width: 100%;
}

.users-list {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.users-footer {
  padding: 6px 16px;
  border-top: 1px solid #e6e6e6;
  text-align: right;
  background: #f8fafc;
  flex-shrink: 0;
}

.users-footer .items-count {
  font-size: 12px;
  color: #a2a2a2;
  font-weight: 500;
}

.type-badge {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 500;
  background: #e8eafe;
  color: #4F5BDF;
}

.archive-dropdown {
  min-width: 140px;
}

.user-item.inactive {
  background: #fafafa;
  color: #9aa0a6;
}

.inactive-badge {
  margin-left: 6px;
  font-size: 11px;
  color: #9aa0a6;
  font-style: italic;
}

.archive-badge {
  background: #6b7280;
  color: #fff;
  padding: 4px 12px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
}

/* Заголовок таблицы */
.users-header {
  border-bottom: 1px solid #e6e6e6;
  padding: 12px 16px;
  flex-shrink: 0;
}

.header-row {
  display: flex;
  width: 100%;
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
.login-col { width: 15%; min-width: 110px; }
.name-col { width: 17%; min-width: 110px; }
.org-col { width: 20%; min-width: 110px; }
.company-col { width: 16%; min-width: 110px; }
.position-col { width: 17%; min-width: 110px; }
.type-col { width: 15%; min-width: 90px; }

/* Тело таблицы */
.users-body {
  overflow-y: auto;
  flex-grow: 1;
  height: 258px;
  max-height: 258px;
}

.user-item {
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.user-item:hover {
  background-color: #fafafa;
}

.user-row {
  display: flex;
  width: 100%;
  padding: 12px 16px;
  align-items: center;
}

.user-col {
  padding: 0 4px;
  text-align: left;
  font-size: 14px;
}

.user-login {
  color: #4F5BDF;
  font-weight: 600;
  white-space: nowrap;
  overflow: visible;
  text-overflow: clip;
}

.truncate-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
  display: block;
}

.access-icon {
  width: 15px;
  height: 15px;
}

.delete-icon-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 25px;
  height: 25px;
  border: none;
  background: none;
  cursor: pointer;
  padding: 0;
  transition: opacity 0.2s;
}

.delete-icon-btn:hover {
  opacity: 0.7;
}

.delete-icon {
  width: 20px;
  height: 20px;
}

.details-grid-two-columns {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-bottom: 16px;
}

.details-column {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.full-width-groups {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.detail-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.detail-label {
  font-size: 0.85em;
  color: #a2a2a2;
}

/* Значения дропдаунов (Организация/Компания/Тип) в деталях - как обычный текст,
   вровень с соседними инпутами (а не полужирным весом BaseDropdown по умолчанию). */
.detail-group :deep(.base-dropdown__text) {
  font-weight: 400;
}

.full-width {
  width: 100%;
}

.password-group {
  margin-top: 8px;
}

.password-checklist {
  list-style: none;
  margin: 6px 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.password-checklist__item {
  font-size: 11px;
  color: #999;
  font-family: 'Montserrat', sans-serif;
  transition: color 0.15s ease;
}

.password-checklist__item--ok {
  color: #28a745;
}

.password-input-container {
  display: flex;
  align-items: center;
  gap: 8px;
}

.password-input-sm {
  padding: 6px 10px;
  border: 1px solid #ddd;
  border-radius: 15px;
  font-size: 14px;
  height: 32px;
  width: 150px;
  transition: border-color 0.2s;
}

.password-input-sm:focus {
  border-color: #4F5BDF;
  outline: none;
}

.password-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.generate-password-btn {
  padding: 6px 10px;
  background-color: #fff;
  border: 1px solid #ddd;
  border-radius: 50px;
  cursor: pointer;
  white-space: nowrap;
  font-size: 13px;
  height: 30px;
  transition: background-color 0.2s;
  color: #4F5BDF;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 3px;
}

.generate-password-btn:hover {
  background: #eee;
}

.generate-icon {
  width: 15px;
  height: 15px;
}

.save-password-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  background-color: #4F5BDF;
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.save-password-btn:disabled {
  background-color: #e6e6e6;
  cursor: not-allowed;
}

.save-password-btn:hover:not(:disabled) {
  background-color: #3a45b2;
}

.save-icon {
  width: 16px;
  height: 16px;
}

.input-hints {
  margin-top: 4px;
  font-size: 0.75em;
}

.language-hint {
  color: #666;
}

.language-hint.warning {
  color: #e74c3c;
  font-weight: bold;
}

.no-users {
  text-align: center;
  padding: 15px;
  color: #666;
}

/* Шапка модалки редактирования */
.modal-title-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.modal-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #1a1a1a;
}

.modal-subtitle {
  margin: 0;
  font-size: 12px;
  color: #a2a2a2;
}

.modal-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* Вкладки модалки редактирования */
.modal-tabs {
  display: flex;
  gap: 4px;
  border-bottom: 1px solid #e6e6e6;
  margin-bottom: 20px;
}

.modal-tab {
  border: none;
  background: none;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  color: #a2a2a2;
  padding: 8px 16px 12px;
  cursor: pointer;
  position: relative;
  transition: color 0.15s ease;
}

.modal-tab:hover {
  color: #333;
}

.modal-tab--active {
  color: #4F5BDF;
}

.modal-tab--active::after {
  content: '';
  position: absolute;
  left: 12px;
  right: 12px;
  bottom: -1px;
  height: 3px;
  border-radius: 3px 3px 0 0;
  background: #4F5BDF;
}

.tab-panel {
  animation: tab-fade 0.18s ease;
}

@keyframes tab-fade {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}

@media (prefers-reduced-motion: reduce) {
  .tab-panel {
    animation: none;
  }
}

/* Вкладка "Доступ" */
.access-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  background: #f8fafc;
  margin-bottom: 12px;
}

.access-card__body strong {
  font-size: 14px;
  font-weight: 600;
  color: #1a1a1a;
}

.access-card__body p {
  margin: 4px 0 0;
  font-size: 12.5px;
  color: #a2a2a2;
}

/* Опасное действие внизу "Профиля" */
.danger-zone {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px dashed #e0e0e0;
}

.danger-zone__hint {
  font-size: 12px;
  color: #a2a2a2;
}

/* Тело модалки редактирования */
.modal-body-inner {
  padding: 24px;
}

/* Тело модалки создания */
.create-user-body {
  padding: 24px;
}

.form-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}

.form-hint {
  font-size: 13px;
  color: #555;
  margin-bottom: 12px;
  padding: 8px 12px;
  border-radius: 8px;
  background: #f3f3fb;
}

.form-hint--warning {
  background: #FFE0B2;
  color: #9A3412;
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.input-group.half {
  flex: 1 1 calc(50% - 8px);
  min-width: 220px;
}

.input-group.full {
  flex: 1 1 100%;
}

.input-label {
  font-size: 14px;
  font-weight: 500;
  color: #555;
}

.required {
  color: #ff4444;
}

.modal-input {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid #e0e0e0;
  border-radius: 15px;
  font-size: 14px;
  transition: border-color 0.2s ease;
  background: #fff;
}

/* Модалка создания пользователя: чуть уже (width prop 600) и скруглённее. */
:deep(.user-create-modal) {
  border-radius: 30px;
}

.modal-input:focus {
  border-color: #4F5BDF;
  outline: none;
}

.input-hint {
  font-size: 12px;
  color: #999;
  margin-top: 2px;
}

.modal-btn {
  padding: 10px 24px;
  border: none;
  border-radius: 30px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  min-width: 100px;
}

.modal-btn--cancel {
  background: #f5f5f5;
  color: #666;
  border: 1px solid #e0e0e0;
}

.modal-btn--cancel:hover {
  background: #e9e9e9;
}

.modal-btn--confirm {
  background: #4F5BDF;
  color: white;
}

.modal-btn--confirm:hover:not(.modal-btn--disabled) {
  background: #3a45b2;
}

.modal-btn--disabled {
  background: #ccc;
  cursor: not-allowed;
}

/* Скроллбары */
.select-dropdown::-webkit-scrollbar,
.users-body::-webkit-scrollbar {
  width: 6px;
}

.select-dropdown::-webkit-scrollbar-track,
.users-body::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 3px;
}

.select-dropdown::-webkit-scrollbar-thumb,
.users-body::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 3px;
}

.select-dropdown::-webkit-scrollbar-thumb:hover,
.users-body::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
}

@media (max-width: 768px) {
  .users-container {
    flex-direction: column;
    height: auto;
  }

  /*
   * Таблица - horizontal scroll вместо wrap в столбик.
   * Scroll на .users-list - оба child'а (.users-header + .users-body) получают
   * min-width 600px и двигаются синхронно.
   */
  .users-list {
    overflow-x: auto !important;
    overflow-y: hidden !important;
  }

  .users-header,
  .users-body {
    min-width: 720px;
    overflow-x: visible !important;
    overflow-y: visible !important;
    height: auto !important;
    max-height: none !important;
  }

  .header-row,
  .user-row {
    flex-wrap: nowrap !important;
    min-width: 720px;
    width: 100%;
  }

  .header-col,
  .user-col {
    width: auto !important;
    min-width: 110px !important;
    flex: 1 1 auto !important;
    margin-bottom: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  
  .details-grid-two-columns {
    grid-template-columns: 1fr;
  }
  
  .password-input-container {
    flex-direction: column;
    align-items: flex-start;
  }
  
  .password-actions {
    justify-content: flex-end;
  }
  
  .management-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
    height: auto;
    padding: 16px;
  }
  
  .search-container {
    width: 100%;
    justify-content: flex-end;
  }
  
  .form-wrap {
    gap: 12px;
  }

  .input-group.half {
    flex: 1 1 100%;
    min-width: 0;
  }
}

/* Закруглённые углы модалок пользователя — локально, без изменения глобального токена */
:deep(.user-edit-modal),
:deep(.user-create-modal) {
  border-radius: 30px;
}
</style>