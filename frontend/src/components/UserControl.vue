<template>
  <div class="user-management dashboard-card">
    <div class="management-header">
      <h3 class="management-title">
        Учётные записи пользователей
      </h3>
      <div class="search-container">
        <SearchComponent
          v-model="userSearch"
          :title="'Поиск пользователей...'"
        />
        <button
          class="create-btn"
          @click="openCreateModal"
        >
          Создать
        </button>
        <RefreshButton @refresh="refreshAllData" />
      </div>
    </div>

    <div class="users-container">
      <!-- Левая часть - таблица пользователей -->
      <div
        class="users-list"
        :class="{'with-details': selectedUser}"
      >
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
            :class="{'selected': selectedUser && selectedUser.username === user.username}"
            @click="selectUser(user)"
          >
            <div class="user-row">
              <div class="user-col login-col">
                <span class="user-login">{{ user.username }}</span>
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
              <div class="user-col type-col">
                {{ user.user_type || '-' }}
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <!-- Правая часть - детали выбранного пользователя -->
      <div
        v-if="selectedUser"
        class="user-details-panel"
      >
        <div class="details-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">
                Редактирование
              </h3>
              <p class="details-subtitle">
                учётной записи <strong>{{ selectedUser.username }}</strong>
              </p>
            </div>
            <div class="details-header-actions">
              <button
                class="access-rights-btn"
                @click="openPermissions(selectedUser)"
              >
                <img
                  src="@/assets/icons/access.png"
                  class="access-icon"
                >
                Права доступа
              </button>
              <button
                class="delete-icon-btn"
                @click="confirmDeleteUser(selectedUser)"
              >
                <img
                  src="@/assets/icons/delete.png"
                  class="delete-icon"
                >
              </button>
            </div>
          </div>
          
          <div class="details-section">
            <div class="details-grid-two-columns">
              <!-- Левый столбец -->
              <div class="details-column">
                <div class="detail-group">
                  <label class="detail-label">Фамилия:</label>
                  <input 
                    v-model="selectedUser.last_name" 
                    class="form-input-sm"
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
                    class="form-input-sm"
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
                  <div
                    class="custom-select"
                    @click="toggleOrgDropdown"
                  >
                    <div class="select-trigger">
                      <span>{{ getOrganizationName(selectedUser.organization_id) || 'Не выбрано' }}</span>
                      <img 
                        src="@/assets/icons/arrow.png" 
                        class="select-arrow" 
                        :class="{ 'open': orgDropdownOpen }"
                        width="12"
                        height="12"
                      >
                    </div>
                    <transition name="dropdown">
                      <div
                        v-if="orgDropdownOpen"
                        class="select-dropdown"
                      >
                        <div 
                          v-for="org in organizations" 
                          :key="org.id"
                          class="select-option"
                          :class="{ 'selected': selectedUser.organization_id === org.id }"
                          @click="selectOrganization(org)"
                        >
                          {{ org.name }}
                        </div>
                      </div>
                    </transition>
                  </div>
                </div>
                
                <div class="detail-group">
                  <label class="detail-label">Должность:</label>
                  <input 
                    v-model="selectedUser.position" 
                    class="form-input-sm"
                    placeholder="Введите должность"
                    autocomplete="new-password"
                    autocorrect="off"
                    autocapitalize="off"
                    spellcheck="false"
                    @change="updateUserInfo(selectedUser)"
                  >
                </div>
              </div>
              
              <!-- Правый столбец -->
              <div class="details-column">
                <div class="detail-group">
                  <label class="detail-label">Имя:</label>
                  <input 
                    v-model="selectedUser.first_name" 
                    class="form-input-sm"
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
                    class="form-input-sm"
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
                  <div
                    class="custom-select"
                    @click="toggleCompanyDropdown"
                  >
                    <div class="select-trigger">
                      <span>{{ getCompanyName(selectedUser.company_id) || 'Не выбрано' }}</span>
                      <img 
                        src="@/assets/icons/arrow.png" 
                        class="select-arrow" 
                        :class="{ 'open': companyDropdownOpen }"
                        width="12"
                        height="12"
                      >
                    </div>
                    <transition name="dropdown">
                      <div
                        v-if="companyDropdownOpen"
                        class="select-dropdown"
                      >
                        <div 
                          v-for="comp in companies" 
                          :key="comp.id"
                          class="select-option"
                          :class="{ 'selected': selectedUser.company_id === comp.id }"
                          @click="selectCompany(comp)"
                        >
                          {{ comp.name }}
                        </div>
                      </div>
                    </transition>
                  </div>
                </div>
                
                <div class="detail-group">
                  <label class="detail-label">Email:</label>
                  <input 
                    v-model="selectedUser.email" 
                    class="form-input-sm"
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
            
            <!-- Полноширинные элементы -->
            <div class="full-width-groups">
              <div class="detail-group">
                <label class="detail-label">Тип пользователя:</label>
                <div
                  class="custom-select full-width"
                  @click="toggleTypeDropdown"
                >
                  <div class="select-trigger">
                    <span>{{ getUserTypeName(selectedUser.type_id) || 'Не выбрано' }}</span>
                    <img 
                      src="@/assets/icons/arrow.png" 
                      class="select-arrow" 
                      :class="{ 'open': typeDropdownOpen }"
                      width="12"
                      height="12"
                    >
                  </div>
                  <transition name="dropdown">
                    <div
                      v-if="typeDropdownOpen"
                      class="select-dropdown"
                    >
                      <div 
                        v-for="type in userTypes" 
                        :key="type.id"
                        class="select-option"
                        :class="{ 'selected': selectedUser.type_id === type.id }"
                        @click="selectUserType(type)"
                      >
                        {{ type.name }}
                      </div>
                    </div>
                  </transition>
                </div>
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
                      :disabled="!selectedUser.newPassword" 
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
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <div
        v-else
        class="no-selection-message"
      >
        <p>Выберите пользователя для просмотра</p>
      </div>
    </div>

    <div
      v-if="filteredUsers.length === 0"
      class="no-users"
    >
      <p>{{ userSearch ? 'Пользователи не найдены' : 'Пользователи отсутствуют' }}</p>
    </div>

    <!-- Модальное окно создания пользователя - на уровне body через Teleport -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showCreateModal"
          class="modal-overlay"
          @click.self="closeCreateModal"
        >
          <div class="modal-content">
            <div class="modal-header">
              <h3 class="modal-title">
                Создание пользователя
              </h3>
              <button
                class="modal-close"
                @click="closeCreateModal"
              >
                <svg
                  width="10"
                  height="10"
                  viewBox="0 0 14 14"
                  fill="none"
                >
                  <path
                    d="M13 1L1 13M1 1L13 13"
                    stroke="#666"
                    stroke-width="2"
                    stroke-linecap="round"
                  />
                </svg>
              </button>
            </div>
            
            <div class="modal-body">
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
                  <div class="input-hint">
                    Минимум 6 символов
                  </div>
                </div>
                <div class="input-group half">
                  <label class="input-label">Организация <span class="required">*</span></label>
                  <div
                    class="custom-select"
                    @click="toggleNewUserOrgDropdown"
                  >
                    <div class="select-trigger">
                      <span>{{ getOrganizationName(newUser.organization_id) || 'Не выбрано' }}</span>
                      <img
                        src="@/assets/icons/arrow.png"
                        class="select-arrow"
                        :class="{ 'open': newUserOrgDropdownOpen }"
                        width="12"
                        height="12"
                      >
                    </div>
                    <transition name="dropdown">
                      <div
                        v-if="newUserOrgDropdownOpen"
                        class="select-dropdown"
                      >
                        <div
                          class="select-option"
                          :class="{ 'selected': newUser.organization_id === null }"
                          @click="selectNewUserOrg(null)"
                        >
                          Не выбрано
                        </div>
                        <div
                          v-for="org in organizations"
                          :key="org.id"
                          class="select-option"
                          :class="{ 'selected': newUser.organization_id === org.id }"
                          @click="selectNewUserOrg(org)"
                        >
                          {{ org.name }}
                        </div>
                      </div>
                    </transition>
                  </div>
                </div>
                <div class="input-group half">
                  <label class="input-label">Компания <span class="required">*</span></label>
                  <div
                    class="custom-select"
                    @click="toggleNewUserCompanyDropdown"
                  >
                    <div class="select-trigger">
                      <span>{{ getCompanyName(newUser.company_id) || 'Не выбрано' }}</span>
                      <img
                        src="@/assets/icons/arrow.png"
                        class="select-arrow"
                        :class="{ 'open': newUserCompanyDropdownOpen }"
                        width="12"
                        height="12"
                      >
                    </div>
                    <transition name="dropdown">
                      <div
                        v-if="newUserCompanyDropdownOpen"
                        class="select-dropdown"
                      >
                        <div
                          class="select-option"
                          :class="{ 'selected': newUser.company_id === null }"
                          @click="selectNewUserCompany(null)"
                        >
                          Не выбрано
                        </div>
                        <div
                          v-for="comp in companies"
                          :key="comp.id"
                          class="select-option"
                          :class="{ 'selected': newUser.company_id === comp.id }"
                          @click="selectNewUserCompany(comp)"
                        >
                          {{ comp.name }}
                        </div>
                      </div>
                    </transition>
                  </div>
                </div>
                <div class="input-group full">
                  <label class="input-label">Тип пользователя <span class="required">*</span></label>
                  <div
                    class="custom-select"
                    @click="toggleNewUserTypeDropdown"
                  >
                    <div class="select-trigger">
                      <span>{{ getUserTypeName(newUser.type_id) || 'Выберите тип' }}</span>
                      <img
                        src="@/assets/icons/arrow.png"
                        class="select-arrow"
                        :class="{ 'open': newUserTypeDropdownOpen }"
                        width="12"
                        height="12"
                      >
                    </div>
                    <transition name="dropdown">
                      <div
                        v-if="newUserTypeDropdownOpen"
                        class="select-dropdown"
                      >
                        <div
                          v-for="type in userTypes"
                          :key="type.id"
                          class="select-option"
                          :class="{ 'selected': newUser.type_id === type.id }"
                          @click="selectNewUserType(type)"
                        >
                          {{ type.name }}
                        </div>
                      </div>
                    </transition>
                  </div>
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
            
            <div class="modal-footer">
              <button
                class="modal-btn modal-btn--cancel"
                @click="closeCreateModal"
              >
                Отмена
              </button>
              <button 
                class="modal-btn modal-btn--confirm" 
                :disabled="!canCreateUser"
                :class="{'modal-btn--disabled': !canCreateUser}"
                @click="createUser"
              >
                Создать
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <!-- Модальное окно прав доступа -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showPermissionsModal"
          class="modal-overlay"
          @click.self="showPermissionsModal = false"
        >
          <div class="modal-content">
            <div class="modal-header">
              <h3 class="modal-title">
                Права: {{ selectedUserForPermissions ? selectedUserForPermissions.username : '' }}
              </h3>
              <button
                class="modal-close"
                @click="showPermissionsModal = false"
              >
                <svg
                  width="10"
                  height="10"
                  viewBox="0 0 14 14"
                  fill="none"
                >
                  <path
                    d="M13 1L1 13M1 1L13 13"
                    stroke="#666"
                    stroke-width="2"
                    stroke-linecap="round"
                  />
                </svg>
              </button>
            </div>

            <div class="modal-body">
              <PermissionTree
                :tree="permissionTree"
                :selected="userPermissions"
                @change="onPermissionChange"
              />
            </div>

            <div class="modal-footer">
              <button
                class="modal-btn modal-btn--cancel"
                @click="showPermissionsModal = false"
              >
                Отмена
              </button>
              <button
                :disabled="savingPermissions"
                class="modal-btn modal-btn--confirm"
                :class="{'modal-btn--disabled': savingPermissions}"
                @click="savePermissions"
              >
                {{ savingPermissions ? 'Сохранение...' : 'Сохранить' }}
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>
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
import SearchComponent from './SearchComponent.vue';
import RefreshButton from './RefreshButton.vue';
import PermissionTree from './PermissionTree.vue';
import PasswordInput from './ui/PasswordInput.vue';
import { getUserPermissions, updateUserPermissions, getPermissionTree } from '@/api/permissions';

export default {
  components: {
    SearchComponent,
    RefreshButton,
    PermissionTree,
    PasswordInput
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
    return { showCreateModal };
  },
  data() {
    return {
      userSearch: '',
      selectedUser: null,
      showNewPass: false,
      currentLanguage: '',
      isCapsLockOn: false,
      userTypes: [],
      sortField: null,
      sortDirection: 'desc',
      orgDropdownOpen: false,
      companyDropdownOpen: false,
      typeDropdownOpen: false,
      newUserTypeDropdownOpen: false,
      newUserOrgDropdownOpen: false,
      newUserCompanyDropdownOpen: false,
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
      },
      permissionTree: [],
      userPermissions: {},
      showPermissionsModal: false,
      selectedUserForPermissions: null,
      savingPermissions: false
    };
  },
  computed: {
    ...mapState(useOrganizationsStore, { organizations: 'items' }),
    ...mapState(useCompaniesStore, { companies: 'items' }),
    filteredUsers() {
      const searchTerm = this.userSearch.toLowerCase();
      return this.allUsers
        .filter(user => {
          return (
            user.username.toLowerCase().includes(searchTerm) ||
            (user.organization && user.organization.toLowerCase().includes(searchTerm)) ||
            (user.company && user.company.toLowerCase().includes(searchTerm)) ||
            (user.user_type && user.user_type.toLowerCase().includes(searchTerm)) ||
            (this.formatUserName(user).toLowerCase().includes(searchTerm))
          );
        })
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
    canCreateUser() {
      return (
        this.newUser.username &&
        this.newUser.password &&
        this.newUser.type_id &&
        this.hasOrgOrCompany
      );
    },
    hasOrgOrCompany() {
      return Boolean(this.newUser.organization_id || this.newUser.company_id);
    }
  },
  async created() {
    // Подтягиваем актуальные справочники из stores. Дальнейшая синхронизация
    // (после CRUD в OrganizationsManagement/CompaniesManagement) идёт через
    // pinia reactivity - listener'ы на window.event больше не нужны.
    await Promise.all([
      this.fetchOrganizations(),
      this.fetchCompanies(),
      this.fetchUserTypes()
    ]);
  },
  beforeUnmount() {
    document.removeEventListener('click', this.handleClickOutside);
  },
  mounted() {
    this.fetchAllUsers();
    
    // Добавляем обработчики для закрытия дропдаунов при клике вне
    document.addEventListener('click', this.handleClickOutside);
    
    // Восстанавливаем черновик при монтировании
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

    refreshAllData() {
      this.fetchOrganizations();
      this.fetchCompanies();
      this.fetchUserTypes();
      this.fetchAllUsers();
    },

    handleClickOutside(event) {
      if (!event.target.closest('.custom-select')) {
        this.orgDropdownOpen = false;
        this.companyDropdownOpen = false;
        this.typeDropdownOpen = false;
        this.newUserTypeDropdownOpen = false;
        this.newUserOrgDropdownOpen = false;
        this.newUserCompanyDropdownOpen = false;
      }
    },
    
    formatUserName(user) {
      const formatted = formatShortName(user);
      return formatted || '-';
    },
    
    getOrganizationName(id) {
      if (!id) return null;
      const org = this.organizations.find(o => o.id === id);
      return org ? org.name : null;
    },
    
    getCompanyName(id) {
      if (!id) return null;
      const comp = this.companies.find(c => c.id === id);
      return comp ? comp.name : null;
    },
    
    getUserTypeName(id) {
      if (!id) return null;
      const type = this.userTypes.find(t => t.id === id);
      return type ? type.name : null;
    },
    
    toggleOrgDropdown() {
      this.orgDropdownOpen = !this.orgDropdownOpen;
      this.companyDropdownOpen = false;
      this.typeDropdownOpen = false;
    },
    
    toggleCompanyDropdown() {
      this.companyDropdownOpen = !this.companyDropdownOpen;
      this.orgDropdownOpen = false;
      this.typeDropdownOpen = false;
    },
    
    toggleTypeDropdown() {
      this.typeDropdownOpen = !this.typeDropdownOpen;
      this.orgDropdownOpen = false;
      this.companyDropdownOpen = false;
    },
    
    toggleNewUserTypeDropdown() {
      this.newUserTypeDropdownOpen = !this.newUserTypeDropdownOpen;
      this.newUserOrgDropdownOpen = false;
      this.newUserCompanyDropdownOpen = false;
    },
    
    toggleNewUserOrgDropdown() {
      this.newUserOrgDropdownOpen = !this.newUserOrgDropdownOpen;
      this.newUserTypeDropdownOpen = false;
      this.newUserCompanyDropdownOpen = false;
    },
    
    toggleNewUserCompanyDropdown() {
      this.newUserCompanyDropdownOpen = !this.newUserCompanyDropdownOpen;
      this.newUserTypeDropdownOpen = false;
      this.newUserOrgDropdownOpen = false;
    },
    
    selectOrganization(org) {
      this.selectedUser.organization_id = org.id;
      this.updateUserOrganization(this.selectedUser);
      this.orgDropdownOpen = false;
    },
    
    selectCompany(comp) {
      this.selectedUser.company_id = comp.id;
      this.updateUserCompany(this.selectedUser);
      this.companyDropdownOpen = false;
    },
    
    selectUserType(type) {
      this.selectedUser.type_id = type.id;
      this.updateUserType(this.selectedUser);
      this.typeDropdownOpen = false;
    },
    
    selectNewUserType(type) {
      this.newUser.type_id = type.id;
      this.newUserTypeDropdownOpen = false;
      this.saveDraft();
    },
    
    selectNewUserOrg(org) {
      this.newUser.organization_id = org ? org.id : null;
      this.newUserOrgDropdownOpen = false;
      this.saveDraft();
    },

    selectNewUserCompany(comp) {
      this.newUser.company_id = comp ? comp.id : null;
      this.newUserCompanyDropdownOpen = false;
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
          this.handleUserCreated();
          alert("Пользователь успешно создан");
        } else {
          const errorData = await response.json();
          alert(errorData.message || "Ошибка при создании пользователя");
        }
      } catch (error) {
        console.error("Ошибка сети при создании пользователя:", error);
        alert("Не удалось создать пользователя");
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
              phone: user.phone || null
            }),
          }
        );

        if (!response.ok) {
          const errorData = await response.json();
          alert(errorData.message || "Ошибка при обновлении информации");
          this.$emit('fetch-users');
        } else {
          this.$emit('user-updated', user);
        }
      } catch (error) {
        console.error("Ошибка сети при обновлении информации:", error);
        alert("Не удалось обновить информацию о пользователе");
        this.$emit('fetch-users');
      }
    },
    
    async confirmDeleteUser(user) {
      if (confirm(`Вы уверены, что хотите удалить аккаунт «${user.username}»?`)) {
        try {
          const response = await apiRequest(`/users/${user.username}`,
            {
              method: "DELETE",
            }
          );

          if (response.ok) {
            alert("Пользователь успешно удален");
            this.selectedUser = null;
            this.$emit('fetch-users');
            // user_count меняется - подтягиваем оба представления.
            this.refreshOrganizations();
            this.refreshCompanies();
          } else {
            const errorData = await response.json();
            alert(errorData.message || "Ошибка при удалении пользователя");
          }
        } catch (error) {
          console.error("Ошибка сети при удалении пользователя:", error);
          alert("Не удалось удалить пользователя");
        }
      }
    },
    
    generatePassword(user) {
      const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!#$%&?';
      const length = Math.floor(Math.random() * 6) + 6;
      let password = '';
      for (let i = 0; i < length; i++) {
        password += chars.charAt(Math.floor(Math.random() * chars.length));
      }
      user.newPassword = password;
      this.showNewPass = true;
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
          alert(errorData.message || "Ошибка при обновлении типа пользователя");
          this.$emit('fetch-users');
        } else {
          const type = this.userTypes.find(t => t.id === user.type_id);
          if (type) user.user_type = type.name;
          this.$emit('user-updated', user);
        }
      } catch (error) {
        console.error("Ошибка сети при обновлении типа пользователя:", error);
        alert("Не удалось обновить тип пользователя");
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
          alert(errorData.message || "Ошибка при обновлении организации");
          this.$emit('fetch-users');
        } else {
          const org = this.organizations.find(o => o.id === user.organization_id);
          if (org) user.organization = org.name;
          this.$emit('user-updated', user);
        }
      } catch (error) {
        console.error("Ошибка сети при обновлении организации:", error);
        alert("Не удалось обновить организацию пользователя");
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
          alert(errorData.message || "Ошибка при обновлении компании");
          this.$emit('fetch-users');
        } else {
          const comp = this.companies.find(c => c.id === user.company_id);
          if (comp) user.company = comp.name;
          this.$emit('user-updated', user);
        }
      } catch (error) {
        console.error("Ошибка сети при обновлении компании:", error);
        alert("Не удалось обновить компанию пользователя");
        this.$emit('fetch-users');
      }
    },
    
    async changeUserPassword(user) {
      if (!user.newPassword) {
        alert("Введите новый пароль");
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
          alert("Пароль успешно изменен");
          user.newPassword = "";
          this.$emit('fetch-users');
        } else {
          const errorData = await response.json();
          alert(errorData.message || "Ошибка при изменении пароля");
        }
      } catch (error) {
        console.error("Ошибка сети при изменении пароля:", error);
        alert("Не удалось изменить пароль");
      }
    },
    
    fetchAllUsers() {
      this.$emit('fetch-users');
    },
    
    selectUser(user) {
      this.selectedUser = { ...user };
    },
    
    closeDetails() {
      this.selectedUser = null;
    },
    
    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'desc';
      }
    },

    async openPermissions(user) {
      this.selectedUserForPermissions = user;
      this.showPermissionsModal = true;
      try {
        const [treeData, permsData] = await Promise.all([
          getPermissionTree(),
          getUserPermissions(user.id),
        ]);
        this.permissionTree = Array.isArray(treeData) ? treeData : [];
        this.userPermissions = {};
        if (Array.isArray(permsData)) {
          permsData.forEach(p => { this.userPermissions[p.key] = p.value; });
        }
      } catch (e) {
        console.error('Ошибка загрузки прав:', e);
      }
    },

    onPermissionChange(key, value) {
      this.userPermissions[key] = value;
    },

    async savePermissions() {
      if (!this.selectedUserForPermissions) return;
      this.savingPermissions = true;
      const permissions = Object.entries(this.userPermissions).map(([key, value]) => ({ key, value }));
      try {
        const result = await updateUserPermissions(this.selectedUserForPermissions.id, { permissions });
        if (result && result.message) {
          console.error('Ошибка сохранения прав:', result.message);
        } else {
          this.showPermissionsModal = false;
        }
      } catch (e) {
        console.error('Ошибка сохранения прав:', e);
      } finally {
        this.savingPermissions = false;
      }
    }
  }
};
</script>

<style scoped>
/* Все стили остаются без изменений */
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

.create-btn {
  padding: 8px 16px;
  background-color: #4F5BDF;
  color: white;
  border: none;
  border-radius: 50px;
  cursor: pointer;
  font-size: 0.9em;
  transition: background-color 0.2s;
}

.create-btn:hover {
  background-color: #3a45b2;
}

.users-container {
  display: flex;
  height: fit-content;
  max-height: 258px;
  width: 100%;
}

/* Левая часть - таблица пользователей */
.users-list {
  width: 70%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #e6e6e6;
}

.users-list.with-details {
  width: 70%;
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
.login-col { width: 18%; min-width: 120px; }
.name-col { width: 18%; min-width: 120px; }
.org-col { width: 24%; min-width: 120px; }
.company-col { width: 20%; min-width: 120px; }
.type-col { width: 20%; min-width: 100px; }

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

.user-item.selected {
  background-color: #f8f9ff;
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

/* Правая часть - детали пользователя */
.user-details-panel {
  width: 30%;
  padding: 10px;
  overflow-y: auto;
  flex-shrink: 0;
  background-color: #fafafa;
}

.no-selection-message {
  width: 30%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a2a2a2;
  font-weight: 400;
  flex-shrink: 0;
  font-size: 14px;
}

.details-content {
  position: relative;
  height: 100%;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding-bottom: 15px;
}

.details-title-wrapper {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.details-title {
  margin: 0;
  color: #000;
  font-size: 1.2em;
  font-weight: 600;
}

.details-subtitle {
  margin: 0;
  font-size: 10px;
  color: #a2a2a2;
}

.details-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.access-rights-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 3px;
  border: 1px solid #4F5BDF;
  border-radius: 50px;
  background-color: #FFF;
  color: #000;
  font-size: 12px;
  cursor: pointer;
  width: 130px;
  height: 30px;
  transition: background-color 0.2s;
  padding: 0 5px;
}

.access-rights-btn:hover {
  background-color: #f0f2ff;
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

.details-section {
  margin-bottom: 24px;
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

/* Уменьшенные инпуты */
.form-input-sm {
  padding: 6px 10px;
  border: 1px solid #ddd;
  border-radius: 10px;
  font-size: 14px;
  width: 100%;
  height: 32px;
  transition: border-color 0.2s;
}

.form-input-sm:focus {
  border-color: #4F5BDF;
  outline: none;
}

.full-width {
  width: 100%;
}

.password-group {
  margin-top: 8px;
}

.password-input-container {
  display: flex;
  align-items: center;
  gap: 8px;
}

.password-input-sm {
  padding: 6px 10px;
  border: 1px solid #ddd;
  border-radius: 10px;
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

/* Стили для кастомного селекта */
.custom-select {
  position: relative;
  cursor: pointer;
}

.select-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border: 1px solid #e0e0e0;
  border-radius: 12px;
  background: white;
  transition: border-color 0.2s;
  font-size: 14px;
}

.select-trigger:hover {
  border-color: #4F5BDF;
}

.select-arrow {
  transition: transform 0.2s ease;
}

.select-arrow.open {
  transform: rotate(90deg);
}

.select-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  background: white;
  border: 1px solid #e0e0e0;
  border-radius: 12px;
  box-shadow: 0 4px 15px rgba(0,0,0,0.1);
  z-index: 1000;
  max-height: 200px;
  overflow-y: auto;
}

.select-option {
  padding: 8px 12px;
  cursor: pointer;
  border-bottom: 1px solid #f0f0f0;
  transition: background 0.2s;
  font-size: 14px;
}

.select-option:last-child {
  border-bottom: none;
}

.select-option:hover {
  background: #f5f7ff;
  color: #4F5BDF;
}

.select-option.selected {
  background: #f0f3ff;
  color: #4F5BDF;
  font-weight: 500;
}

/* Анимации дропдауна */
.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(-10px);
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
  align-items: center;
  justify-content: center;
  z-index: 10000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
  animation: overlayAppear 0.3s ease-out;
}

@keyframes overlayAppear {
  from {
    background: rgba(0, 0, 0, 0);
    backdrop-filter: blur(0px);
  }
  to {
    background: rgba(0, 0, 0, 0.5);
    backdrop-filter: blur(0.1px);
  }
}

.modal-content {
  background: #fff;
  border-radius: 30px;
  padding: 0;
  width: 600px;
  max-width: 90vw;
  max-height: 85vh;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  animation: modalAppear 0.3s ease-out;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

@keyframes modalAppear {
  from {
    opacity: 0;
    transform: scale(0.8) translateY(-20px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 24px;
  border-bottom: 1px solid #f0f0f0;
  flex-shrink: 0;
}

.modal-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #1a1a1a;
}

.modal-close {
  background: none;
  border: none;
  cursor: pointer;
  padding: 8px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s ease;
}

.modal-close:hover {
  background-color: #f5f5f5;
}

.modal-body {
  padding: 24px;
  overflow-y: auto;
  flex: 1;
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
  padding: 10px 12px;
  border: 1px solid #e0e0e0;
  border-radius: 12px;
  font-size: 14px;
  transition: border-color 0.2s ease;
  background: #fff;
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

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 16px 24px 20px;
  border-top: 1px solid #f0f0f0;
  flex-shrink: 0;
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
.modal-body::-webkit-scrollbar,
.select-dropdown::-webkit-scrollbar,
.users-body::-webkit-scrollbar {
  width: 6px;
}

.modal-body::-webkit-scrollbar-track,
.select-dropdown::-webkit-scrollbar-track,
.users-body::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 3px;
}

.modal-body::-webkit-scrollbar-thumb,
.select-dropdown::-webkit-scrollbar-thumb,
.users-body::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 3px;
}

.modal-body::-webkit-scrollbar-thumb:hover,
.select-dropdown::-webkit-scrollbar-thumb:hover,
.users-body::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
}

@media (max-width: 768px) {
  .users-container {
    flex-direction: column;
    height: auto;
  }
  
  .users-list,
  .user-details-panel,
  .no-selection-message {
    width: 100% !important;
  }
  
  .users-list.with-details {
    border-right: none;
    border-bottom: 1px solid #e6e6e6;
    height: 300px;
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
    min-width: 600px;
    overflow-x: visible !important;
    overflow-y: visible !important;
    height: auto !important;
    max-height: none !important;
  }

  .header-row,
  .user-row {
    flex-wrap: nowrap !important;
    min-width: 600px;
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
  
  .modal-content {
    width: 95%;
    max-height: 90vh;
  }
  
  .form-wrap {
    gap: 12px;
  }

  .input-group.half {
    flex: 1 1 100%;
    min-width: 0;
  }
}
</style>