<template>
  <div class="user-management">
    <div class="management-header">
      <h3 class="management-title">Учётные записи пользователей</h3>
      <div class="search-container">
        <SearchComponent
          :title="'Поиск пользователей...'"
          v-model="userSearch"
        />
        <button @click="showCreateModal = true" class="create-btn">
          Создать
        </button>
        <RefreshButton @refresh="fetchAllUsers" />
      </div>
    </div>

    <div class="users-container">
      <!-- Левая часть - таблица пользователей -->
      <div class="users-list" :class="{'with-details': selectedUser}">
        <!-- Заголовок таблицы -->
        <div class="users-header">
          <div class="header-row">
            <div class="header-col login-col" @click="sortBy('username')">
              <p :class="{ 'active-sort': sortField === 'username' }">Логин</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'username',
                  'desc': sortField === 'username' && sortDirection === 'desc'
                }" 
              />
            </div>
            <div class="header-col name-col" @click="sortBy('full_name')">
              <p :class="{ 'active-sort': sortField === 'full_name' }">Фамилия И.О.</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'full_name',
                  'desc': sortField === 'full_name' && sortDirection === 'desc'
                }" 
              />
            </div>
            <div class="header-col org-col" @click="sortBy('organization')">
              <p :class="{ 'active-sort': sortField === 'organization' }">Организация / Отдел</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'organization',
                  'desc': sortField === 'organization' && sortDirection === 'desc'
                }" 
              />
            </div>
            <div class="header-col company-col" @click="sortBy('company')">
              <p :class="{ 'active-sort': sortField === 'company' }">Компания</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'company',
                  'desc': sortField === 'company' && sortDirection === 'desc'
                }" 
              />
            </div>
            <div class="header-col type-col" @click="sortBy('user_type')">
              <p :class="{ 'active-sort': sortField === 'user_type' }">Тип</p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'user_type',
                  'desc': sortField === 'user_type' && sortDirection === 'desc'
                }" 
              />
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
                <span class="truncate-text" :title="user.organization || '-'">
                  {{ user.organization || '-' }}
                </span>
              </div>
              <div class="user-col company-col">
                <span class="truncate-text" :title="user.company || '-'">
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
      <div v-if="selectedUser" class="user-details-panel">
        <div class="details-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">Редактирование</h3>
              <p class="details-subtitle">учётной записи <strong>{{ selectedUser.username }}</strong></p>
            </div>
            <div class="details-header-actions">
              <button class="access-rights-btn">
                <img src="@/assets/icons/access.png" class="access-icon" />
                Права доступа
              </button>
              <button @click="confirmDeleteUser(selectedUser)" class="delete-icon-btn">
                <img src="@/assets/icons/delete.png" class="delete-icon" />
              </button>
              <!--<button @click="closeDetails" class="close-btn">×</button>-->
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
                    @change="updateUserInfo(selectedUser)"
                    class="form-input-sm"
                    placeholder="Введите фамилию"
                    autocomplete="new-password"
                    autocorrect="off"
                    autocapitalize="off"
                    spellcheck="false"
                  >
                </div>
                
                <div class="detail-group">
                  <label class="detail-label">Отчество:</label>
                  <input 
                    v-model="selectedUser.middle_name" 
                    @change="updateUserInfo(selectedUser)"
                    class="form-input-sm"
                    placeholder="Введите отчество"
                    autocomplete="new-password"
                    autocorrect="off"
                    autocapitalize="off"
                    spellcheck="false"
                  >
                </div>
                
                <div class="detail-group">
                  <label class="detail-label">Организация:</label>
                  <select 
                    v-model="selectedUser.organization_id" 
                    @change="updateUserOrganization(selectedUser)"
                    class="form-select-sm"
                    autocomplete="off"
                  >
                    <option v-for="org in organizations" :key="org.id" :value="org.id">
                      {{ org.name }}
                    </option>
                  </select>
                </div>
                
                <div class="detail-group">
                  <label class="detail-label">Должность:</label>
                  <input 
                    v-model="selectedUser.position" 
                    @change="updateUserInfo(selectedUser)"
                    class="form-input-sm"
                    placeholder="Введите должность"
                    autocomplete="new-password"
                    autocorrect="off"
                    autocapitalize="off"
                    spellcheck="false"
                  >
                </div>
              </div>
              
              <!-- Правый столбец -->
              <div class="details-column">
                <div class="detail-group">
                  <label class="detail-label">Имя:</label>
                  <input 
                    v-model="selectedUser.first_name" 
                    @change="updateUserInfo(selectedUser)"
                    class="form-input-sm"
                    placeholder="Введите имя"
                    autocomplete="new-password"
                    autocorrect="off"
                    autocapitalize="off"
                    spellcheck="false"
                  >
                </div>
                
                <div class="detail-group">
                  <label class="detail-label">Телефон:</label>
                  <input 
                    v-model="selectedUser.phone" 
                    @change="updateUserInfo(selectedUser)"
                    class="form-input-sm"
                    placeholder="Введите телефон"
                    type="tel"
                    autocomplete="new-password"
                    autocorrect="off"
                    autocapitalize="off"
                    spellcheck="false"
                  >
                </div>
                
                <div class="detail-group">
                  <label class="detail-label">Компания:</label>
                  <select 
                    v-model="selectedUser.company_id" 
                    @change="updateUserCompany(selectedUser)"
                    class="form-select-sm"
                    autocomplete="off"
                  >
                    <option v-for="comp in companies" :key="comp.id" :value="comp.id">
                      {{ comp.name }}
                    </option>
                  </select>
                </div>
                
                <div class="detail-group">
                  <label class="detail-label">Email:</label>
                  <input 
                    v-model="selectedUser.email" 
                    @change="updateUserInfo(selectedUser)"
                    class="form-input-sm"
                    placeholder="Введите email"
                    type="email"
                    autocomplete="new-password"
                    autocorrect="off"
                    autocapitalize="off"
                    spellcheck="false"
                  >
                </div>
              </div>
            </div>
            
            <!-- Полноширинные элементы -->
            <div class="full-width-groups">
              <div class="detail-group">
                <label class="detail-label">Тип пользователя:</label>
                <select 
                  v-model="selectedUser.type_id" 
                  @change="updateUserType(selectedUser)"
                  class="form-select-sm full-width"
                  autocomplete="off"
                >
                  <option v-for="type in userTypes" :key="type.id" :value="type.id">
                    {{ type.name }}
                  </option>
                </select>
              </div>
              
              <div class="detail-group password-group">
                <label class="detail-label">Новый пароль:</label>
                <div class="password-input-container">
                  <input 
                    :type="showNewPass ? 'text' : 'password'" 
                    v-model="selectedUser.newPassword" 
                    @keyup="checkInputLanguage($event)"
                    @keyup.enter="changeUserPassword(selectedUser)"
                    class="password-input-sm"
                    placeholder="Новый пароль"
                    autocomplete="new-password"
                    autocorrect="off"
                    autocapitalize="off"
                    spellcheck="false"
                  >
                  <div class="password-actions">
                    <button 
                      @click="generatePassword(selectedUser)" 
                      class="generate-password-btn"
                      type="button"
                    >
                      <img src="@/assets/icons/random.png" class="generate-icon" />
                      Генерировать
                    </button>
                    <button 
                      @click="changeUserPassword(selectedUser)" 
                      :disabled="!selectedUser.newPassword"
                      class="save-password-btn"
                    >
                      <img src="@/assets/icons/save.png" class="save-icon" />
                    </button>
                  </div>
                </div>
                <div class="input-hints">
                  <span class="language-hint" :class="{ 'warning': isCapsLockOn }">
                    {{ currentLanguage }} {{ isCapsLockOn ? '| CAPS LOCK' : '' }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <div v-else class="no-selection-message">
        <p>Выберите пользователя для просмотра</p>
      </div>
    </div>

    <div v-if="filteredUsers.length === 0" class="no-users">
      <p>{{ userSearch ? 'Пользователи не найдены' : 'Пользователи отсутствуют' }}</p>
    </div>

    <teleport to="body">
      <CreateUserModal
        v-if="showCreateModal"
        :organizations="organizations"
        :companies="companies"
        :userTypes="userTypes"
        @close="showCreateModal = false"
        @user-created="handleUserCreated"
      />
    </teleport>
  </div>
</template>

<script>
import { ref } from 'vue';
import CreateUserModal from './CreateUserModal.vue';
import SearchComponent from './SearchComponent.vue';
import RefreshButton from './RefreshButton.vue';

export default {
  components: {
    CreateUserModal,
    SearchComponent,
    RefreshButton
  },
  props: {
    allUsers: {
      type: Array,
      required: true
    }
  },
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
      organizations: [],
      companies: [],
      userTypes: [],
      sortField: null,
      sortDirection: 'desc'
    };
  },
  async created() {
    await Promise.all([
      this.fetchOrganizations(), 
      this.fetchCompanies(),
      this.fetchUserTypes()
    ]);
  },
  computed: {
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
    }
  },
  mounted() {
    this.fetchAllUsers();
  },
  methods: {
    formatUserName(user) {
      if (!user.last_name && !user.first_name && !user.middle_name) return '-';
      
      let result = user.last_name || '';
      if (user.first_name) {
        result += ` ${user.first_name.charAt(0).toUpperCase()}.`;
        if (user.middle_name) {
          result += `${user.middle_name.charAt(0).toUpperCase()}.`;
        }
      }
      return result || '-';
    },
    handleUserCreated() {
      this.showCreateModal = false;
      this.$emit('fetch-users');
    },
    async updateUserInfo(user) {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch(
          `http://localhost:8080/users/${user.username}/info`,
          {
            method: "PUT",
            headers: {
              "Authorization": `Bearer ${token}`,
              "Content-Type": "application/json",
            },
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
          const token = localStorage.getItem("token");
          const response = await fetch(
            `http://localhost:8080/users/${user.username}`,
            {
              method: "DELETE",
              headers: {
                "Authorization": `Bearer ${token}`,
              },
            }
          );

          if (response.ok) {
            alert("Пользователь успешно удален");
            this.selectedUser = null;
            this.$emit('fetch-users');
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
        const token = localStorage.getItem("token");
        const response = await fetch("http://localhost:8080/user-types", {
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });
        if (response.ok) {
          this.userTypes = await response.json();
        }
      } catch (error) {
        console.error("Error fetching user types:", error);
      }
    },
    async fetchOrganizations() {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch("http://localhost:8080/organizations", {
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });
        if (response.ok) {
          this.organizations = await response.json();
        }
      } catch (error) {
        console.error("Error fetching organizations:", error);
      }
    },
    async fetchCompanies() {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch("http://localhost:8080/companies", {
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });
        if (response.ok) {
          this.companies = await response.json();
        }
      } catch (error) {
        console.error("Error fetching companies:", error);
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
        const token = localStorage.getItem("token");
        const response = await fetch(
          `http://localhost:8080/users/${user.username}/type`,
          {
            method: "PUT",
            headers: {
              "Authorization": `Bearer ${token}`,
              "Content-Type": "application/json",
            },
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
        const token = localStorage.getItem("token");
        const response = await fetch(
          `http://localhost:8080/users/${user.username}/organization`,
          {
            method: "PUT",
            headers: {
              "Authorization": `Bearer ${token}`,
              "Content-Type": "application/json",
            },
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
        const token = localStorage.getItem("token");
        const response = await fetch(
          `http://localhost:8080/users/${user.username}/company`,
          {
            method: "PUT",
            headers: {
              "Authorization": `Bearer ${token}`,
              "Content-Type": "application/json",
            },
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
        const token = localStorage.getItem("token");
        const response = await fetch(
          `http://localhost:8080/users/${user.username}/password`,
          {
            method: "PUT",
            headers: {
              "Authorization": `Bearer ${token}`,
              "Content-Type": "application/json",
            },
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
  background-color: #3a5a80;
}

.users-container {
  display: flex;
  height: fit-content;
  max-height: 400px;
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
  height: 357px;
  max-height: 357px;
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

.close-btn {
  background: none;
  border: none;
  font-size: 20px;
  cursor: pointer;
  color: #999;
  padding: 0;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: 4px;
  transition: background-color 0.2s ease;
}

.close-btn:hover {
  background-color: #f0f0f0;
  color: #666;
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
  gap: 10px;
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
  padding: 6px 5px;
  border: 1px solid #ddd;
  border-radius: 10px;
  font-size: 0.8em;
  width: 100%;
  height: 32px;
  transition: border-color 0.2s;
}

.form-input-sm:focus {
  border-color: #4F5BDF;
  outline: none;
}

.form-select-sm {
  padding: 6px 5px;
  border: 1px solid #ddd;
  border-radius: 10px;
  background-color: white;
  font-size: 0.8em;
  width: 100%;
  height: 32px;
  transition: border-color 0.2s;
}

.form-select-sm:focus {
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
  padding: 6px 5px;
  border: 1px solid #ddd;
  border-radius: 10px;
  font-size: 0.8em;
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
  font-size: 0.8em;
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
  background-color: #3a5a80;
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
  
  .header-row,
  .user-row {
    flex-wrap: wrap;
  }
  
  .header-col,
  .user-col {
    width: 50% !important;
    margin-bottom: 4px;
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
}
</style>