<template>
  <div class="applications-card">
    <div class="card-header">
      <div class="card-header__title">
        <h3 class="card-title">Мои заявки</h3>
        <p class="card-organization" v-if="userOrganization || userCompany">
          {{ userOrganization || userCompany }}
        </p>
      </div>
      <div class="card-header__settings">
        <SearchComponent
          :title="'Поиск заявок..'"
          v-model="searchQuery"
        />
        <RefreshButton @refresh="fetchUserApplications" />
      </div>
    </div>

    <!-- Кнопки фильтров -->
    <div class="filter-tabs">
      <button 
        class="filter-tab"
        :class="{ 'filter-tab--active': currentFilter === 'my' }"
        @click="setFilter('my')"
      >
        Мои заявки
      </button>
      <button 
        v-if="userOrganizationId"
        class="filter-tab"
        :class="{ 'filter-tab--active': currentFilter === 'organization' }"
        @click="setFilter('organization')"
      >
        Заявки организации (отдела)
      </button>
      <button 
        v-if="userCompanyId"
        class="filter-tab"
        :class="{ 'filter-tab--active': currentFilter === 'company' }"
        @click="setFilter('company')"
      >
        Заявки компании
      </button>
    </div>
    
    <div class="card-content">
      <div v-if="filteredApplications.length > 0" class="applications-container">
        <!-- Левая часть - таблица заявок -->
        <div class="applications-list" :class="{'with-details': selectedApplication}">
          <!-- Заголовок таблицы -->
          <div class="applications-header">
            <div class="header-row">
              <div class="header-col id-col" @click="sortBy('application_number')">
                <p :class="{ 'active-sort': sortField === 'application_number' }">Номер заявки</p>
                <img 
                  src="@/assets/icons/sort.png" 
                  class="sort-icon" 
                  :class="{ 
                    'sorted': sortField === 'application_number',
                    'desc': sortField === 'application_number' && sortDirection === 'desc'
                  }" 
                />
              </div>
              <div class="header-col date-col" @click="sortBy('sending_datetime')">
                <p :class="{ 'active-sort': sortField === 'sending_datetime' }">Дата подачи</p>
                <img 
                  src="@/assets/icons/sort.png" 
                  class="sort-icon" 
                  :class="{ 
                    'sorted': sortField === 'sending_datetime',
                    'desc': sortField === 'sending_datetime' && sortDirection === 'desc'
                  }" 
                />
              </div>
              <div class="header-col confirmation-col" @click="sortBy('confirmation')">
                <p :class="{ 'active-sort': sortField === 'confirmation' }">Подтверждение</p>
                <img 
                  src="@/assets/icons/sort.png" 
                  class="sort-icon" 
                  :class="{ 
                    'sorted': sortField === 'confirmation',
                    'desc': sortField === 'confirmation' && sortDirection === 'desc'
                  }" 
                />
              </div>
              <div class="header-col status-col" @click="sortBy('status')">
                <p :class="{ 'active-sort': sortField === 'status' }">Статус</p>
                <img 
                  src="@/assets/icons/sort.png" 
                  class="sort-icon" 
                  :class="{ 
                    'sorted': sortField === 'status',
                    'desc': sortField === 'status' && sortDirection === 'desc'
                  }" 
                />
              </div>
            </div>
          </div>
          
          <!-- Тело таблицы -->
          <div class="applications-body">
            <div 
              v-for="application in sortedApplications" 
              :key="application.id" 
              class="application-item"
              :class="{'selected': selectedApplication && selectedApplication.id === application.id}"
              @click="selectApplication(application)"
            >
              <div class="application-row">
                <div class="application-col id-col">
                  <span class="application-id">{{ application.application_number }}</span>
                </div>
                <div class="application-col date-col">
                  {{ formatDateTime(application.sending_datetime) }}
                </div>
                <div class="application-col confirmation-col">
                  <span 
                    class="confirmation-badge"
                    :class="getConfirmationClass(application.confirmation)"
                  >
                    {{ application.confirmation }}
                  </span>
                </div>
                <div class="application-col status-col">
                  <span 
                    class="status-badge"
                    :class="getStatusClass(application.status)"
                  >
                    {{ application.status }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
        
        <!-- Правая часть - детали выбранной заявки -->
        <div v-if="selectedApplication" class="application-details-panel">
          <div class="details-content">
            <div class="details-header">
              <div class="details-title-wrapper">
                <h3 class="details-title">Заявка {{ selectedApplication.application_number }}</h3>
                <span 
                  class="status-badge-lg"
                  :class="getStatusClass(selectedApplication.status)"
                >
                  {{ selectedApplication.status }}
                </span>
              </div>
              <button @click="closeDetails" class="close-btn">×</button>
            </div>
            
            <div class="details-section">
              <div class="details-grid">
                <div class="detail-item">
                  <span class="detail-label">Номер заявки:</span>
                  <span class="detail-value">{{ selectedApplication.application_number }}</span>
                </div>
                <div class="detail-item" v-if="selectedApplication.organization_name">
                  <span class="detail-label">Организация:</span>
                  <span class="detail-value">{{ selectedApplication.organization_name }}</span>
                </div>
                <div class="detail-item" v-if="selectedApplication.company_name">
                  <span class="detail-label">Компания:</span>
                  <span class="detail-value">{{ selectedApplication.company_name }}</span>
                </div>
                <div class="detail-item">
                  <span class="detail-label">Отправитель:</span>
                  <span class="detail-value">{{ selectedApplication.sender_full_name || selectedApplication.sender_name }}</span>
                </div>
                <div class="detail-item">
                  <span class="detail-label">Дата и время отправки:</span>
                  <span class="detail-value">{{ formatDateTime(selectedApplication.sending_datetime) }}</span>
                </div>
                <div class="detail-item" v-if="selectedApplication.reading_datetime">
                  <span class="detail-label">Дата прочтения:</span>
                  <span class="detail-value">{{ formatDateTime(selectedApplication.reading_datetime) }}</span>
                </div>
                <div class="detail-item" v-if="selectedApplication.confirmation_datetime">
                  <span class="detail-label">Дата подтверждения:</span>
                  <span class="detail-value">{{ formatDateTime(selectedApplication.confirmation_datetime) }}</span>
                </div>
                <div class="detail-item">
                  <span class="detail-label">Подтверждение:</span>
                  <span class="detail-value">
                    <span 
                      class="confirmation-badge"
                      :class="getConfirmationClass(selectedApplication.confirmation)"
                    >
                      {{ selectedApplication.confirmation }}
                    </span>
                  </span>
                </div>
                <div class="detail-item" v-if="selectedApplication.message">
                  <span class="detail-label">Сообщение:</span>
                  <span class="detail-value">{{ selectedApplication.message }}</span>
                </div>
                
                <!-- Блок с ответственными -->
                <div class="detail-item" v-if="responsibleUsers.length > 0">
                  <span class="detail-label">Ответственные:</span>
                  <div class="responsible-users-list">
                    <div 
                      v-for="user in responsibleUsers" 
                      :key="user.id"
                      class="responsible-user-item"
                      :class="{ 'primary-responsible': user.is_primary }"
                    >
                      <span class="user-name">
                        {{ formatUserName(user) }}
                        <span v-if="user.is_primary" class="primary-badge">★</span>
                      </span>
                      <span v-if="user.position" class="user-position">{{ user.position }}</span>
                    </div>
                  </div>
                </div>
                
                <div class="detail-item" v-if="selectedApplication.responsible_comment">
                  <span class="detail-label">Комментарий ответственного:</span>
                  <span class="detail-value">{{ selectedApplication.responsible_comment }}</span>
                </div>
                <div class="detail-item">
                  <span class="detail-label">Согласие на обработку данных:</span>
                  <span class="detail-value">{{ selectedApplication.data_approval ? 'Да' : 'Нет' }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
        
        <div v-else class="no-selection-message">
          <p>Выберите заявку для просмотра</p>
        </div>
      </div>
      <div v-else class="no-data-message">
        <p>{{ searchQuery ? 'Заявки не найдены' : 'Заявок нет' }}</p>
        <p class="hint" v-if="!searchQuery && !isLoading">
          {{ getNoDataHint() }}
        </p>
      </div>
      
      <div v-if="isLoading" class="loading-message">
        <p>Загрузка заявок...</p>
      </div>
    </div>
  </div>
</template>

<script>
import RefreshButton from './RefreshButton.vue';
import SearchComponent from './SearchComponent.vue';

export default {
  components: {
    RefreshButton,
    SearchComponent
  },
  props: {
    userOrganizationId: {
      type: Number,
      default: null
    },
    userCompanyId: {
      type: Number,
      default: null
    },
    userId: {
      type: Number,
      default: null
    },
    userOrganization: {
      type: String,
      default: ""
    },
    userCompany: {
      type: String,
      default: ""
    }
  },
  data() {
    return {
      applications: [],
      selectedApplication: null,
      responsibleUsers: [], // Новое поле для хранения ответственных
      searchQuery: '',
      sortField: null,
      sortDirection: 'desc',
      isLoading: false,
      currentFilter: 'my' // 'my', 'organization', 'company'
    };
  },
  computed: {
    filteredApplications() {
      if (!this.searchQuery) return this.applications;
      
      const searchTerm = this.searchQuery.toLowerCase();
      return this.applications.filter(application => {
        return (
          application.application_number?.toLowerCase().includes(searchTerm) ||
          application.organization_name?.toLowerCase().includes(searchTerm) ||
          application.company_name?.toLowerCase().includes(searchTerm) ||
          application.sender_name?.toLowerCase().includes(searchTerm) ||
          application.sender_full_name?.toLowerCase().includes(searchTerm) ||
          application.confirmation?.toLowerCase().includes(searchTerm) ||
          application.status?.toLowerCase().includes(searchTerm) ||
          application.message?.toLowerCase().includes(searchTerm) ||
          application.responsible_name?.toLowerCase().includes(searchTerm) ||
          application.responsible_full_name?.toLowerCase().includes(searchTerm) ||
          application.responsible_comment?.toLowerCase().includes(searchTerm)
        );
      });
    },
    
    sortedApplications() {
      const applications = [...this.filteredApplications];
      
      if (!this.sortField) {
        return applications.sort((a, b) => {
          const dateA = new Date(a.sending_datetime);
          const dateB = new Date(b.sending_datetime);
          return dateB - dateA;
        });
      }
      
      return applications.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'application_number':
            valueA = a.application_number;
            valueB = b.application_number;
            break;
            
          case 'sending_datetime':
            valueA = new Date(a.sending_datetime);
            valueB = new Date(b.sending_datetime);
            break;
            
          case 'confirmation':
            valueA = a.confirmation;
            valueB = b.confirmation;
            break;
            
          case 'status':
            valueA = a.status;
            valueB = b.status;
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
    }
  },
  methods: {
    async fetchUserApplications() {
      try {
        this.isLoading = true;
        const token = localStorage.getItem("token");
        if (!token) {
          console.error("Пользователь не авторизован.");
          this.isLoading = false;
          return;
        }

        let url = "http://localhost:8080/applications/user";
        const params = new URLSearchParams();
        
        // Добавляем search_query если есть
        if (this.searchQuery) {
          params.append('search_query', this.searchQuery);
        }

        const queryString = params.toString();
        if (queryString) {
          url += '?' + queryString;
        }

        console.log("Fetching applications from:", url);
        console.log("Current user data:", {
          userId: this.userId,
          userCompanyId: this.userCompanyId,
          userOrganizationId: this.userOrganizationId,
          currentFilter: this.currentFilter
        });

        const response = await fetch(url, {
          method: "GET",
          headers: {
            "Authorization": `Bearer ${token}`,
            "Accept": "application/json"
          },
        });

        console.log("Response status:", response.status);
        
        if (response.ok) {
          const data = await response.json();
          console.log("Total applications loaded:", data.length);
          
          // Фильтруем заявки в зависимости от выбранной вкладки
          this.applications = data.filter(app => {
            console.log(`App ${app.application_number}:`, {
              appCompanyId: app.company_id,
              userCompanyId: this.userCompanyId,
              appOrganizationId: app.organization_id,
              userOrganizationId: this.userOrganizationId,
              appSenderId: app.sender_user_id,
              userId: this.userId
            });
            
            switch (this.currentFilter) {
              case 'my':
                return app.sender_user_id === this.userId;
              case 'organization':
                // Для заявок организации проверяем и organization_id
                return app.organization_id === this.userOrganizationId;
              case 'company':
                // Для заявок компании проверяем company_id
                // Обрабатываем случай, когда company_id может быть null
                if (app.company_id && this.userCompanyId) {
                  return app.company_id === this.userCompanyId;
                }
                return false;
              default:
                return true;
            }
          });
          
          console.log("Filtered applications:", this.applications.length);
          if (this.applications.length === 0) {
            console.log("No applications after filtering. Check your IDs!");
          }
        } else {
          const errorText = await response.text();
          console.error("Ошибка при загрузке заявок:", response.status, errorText);
        }
      } catch (error) {
        console.error("Ошибка сети при загрузке заявок:", error);
      } finally {
        this.isLoading = false;
      }
    },

    async fetchResponsibleUsers(applicationId) {
      try {
        const token = localStorage.getItem("token");
        if (!token) return;

        const response = await fetch(`http://localhost:8080/applications/${applicationId}/responsible-users`, {
          method: "GET",
          headers: {
            "Authorization": `Bearer ${token}`,
            "Accept": "application/json"
          },
        });

        if (response.ok) {
          this.responsibleUsers = await response.json();
          console.log("Responsible users loaded:", this.responsibleUsers);
        } else {
          console.error("Failed to fetch responsible users:", response.status);
          this.responsibleUsers = [];
        }
      } catch (error) {
        console.error("Error fetching responsible users:", error);
        this.responsibleUsers = [];
      }
    },

    formatDateTime(dateTimeString) {
      if (!dateTimeString) return '';
      const date = new Date(dateTimeString);
      return date.toLocaleString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
      });
    },

    formatUserName(user) {
      const parts = [];
      if (user.last_name) parts.push(user.last_name);
      if (user.first_name) parts.push(user.first_name);
      if (user.middle_name) parts.push(user.middle_name);
      return parts.join(' ') || user.username;
    },

    getConfirmationClass(confirmation) {
      const classes = {
        'Согласовано': 'confirmation-approved',
        'Согласование': 'confirmation-pending',
        'Не согласовано': 'confirmation-rejected'
      };
      return classes[confirmation] || 'confirmation-default';
    },

    getStatusClass(status) {
      const statusClasses = {
        'Непрочитано': 'status-unread',
        'В обработке': 'status-processing',
        'В работе': 'status-in-progress',
        'Завершено': 'status-completed',
        'Отказано': 'status-rejected'
      };
      return statusClasses[status] || 'status-default';
    },

    getNoDataHint() {
      switch (this.currentFilter) {
        case 'my':
          return 'У вас ещё нет отправленных заявок';
        case 'organization':
          return 'Ваша организация ещё не отправляла заявки';
        case 'company':
          return 'Ваша компания ещё не отправляла заявки';
        default:
          return 'Заявок нет';
      }
    },

    async selectApplication(application) {
      this.selectedApplication = application;
      await this.fetchResponsibleUsers(application.id);
    },

    closeDetails() {
      this.selectedApplication = null;
      this.responsibleUsers = [];
    },

    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'desc';
      }
    },

    setFilter(filter) {
      this.currentFilter = filter;
      this.selectedApplication = null;
      this.responsibleUsers = [];
      this.fetchUserApplications();
    }
  },
  mounted() {
    this.fetchUserApplications();
  },
  watch: {
    searchQuery() {
      this.fetchUserApplications();
    },
    userId() {
      this.fetchUserApplications();
    }
  }
};
</script>

<style scoped>
/* Добавить стили для вкладок фильтров */
.filter-tabs {
  display: flex;
  gap: 10px;
  padding: 15px 20px;
  border-bottom: 1px solid #e6e6e6;
  background-color: #fafafa;
}

.filter-tab {
  padding: 8px 16px;
  border: 1px solid #e6e6e6;
  background: white;
  border-radius: 20px;
  cursor: pointer;
  font-size: 13px;
  transition: all 0.2s;
  color: #333;
  white-space: nowrap;
}

.filter-tab:hover:not(.filter-tab--active) {
  background: #f5f5f5;
  border-color: #d9d9d9;
}

.filter-tab--active {
  background: #4F5BDF;
  color: white;
  border-color: #4F5BDF;
}

.filter-tab--active:hover {
  background: #3a45c0;
  border-color: #3a45c0;
}

/* Стили для списка ответственных */
.responsible-users-list {
  margin-top: 5px;
}

.responsible-user-item {
  padding: 8px 12px;
  background: #f8f9ff;
  border-radius: 8px;
  margin-bottom: 6px;
  border-left: 3px solid #4F5BDF;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.responsible-user-item.primary-responsible {
  background: #fff7ed;
  border-left-color: #ea580c;
}

.user-name {
  font-weight: 500;
  color: #1a1a1a;
  display: flex;
  align-items: center;
  gap: 6px;
}

.primary-badge {
  color: #ea580c;
  font-size: 14px;
}

.user-position {
  font-size: 12px;
  color: #666;
  font-style: italic;
}

/* Остальные стили остаются как в предыдущей версии */
.applications-card {
  background-color: #fff;
  border-radius: 30px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 100%;
  box-shadow: 0 3px 10px rgba(0,0,0,0.05);
  transition: height .2s;
}

.card-header {
  border-bottom: 1px solid #e6e6e6;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0px 20px;
  height: 50px;
}

.card-header__title {
  display: flex;
  gap: 8px;
  align-items: center;
}

.card-header__settings {
  display: flex;
  gap: 8px;
  align-items: center;
}

.card-organization {
  font-size: 14px;
  color: #a2a2a2;
  margin: 0;
}

.card-title {
  margin: 0;
  color: #000;
  font-weight: 600;
  font-size: 1.1em;
}

.card-content {
  padding: 0;
}

.applications-container {
  display: flex;
  height: fit-content;
  max-height: 292px;
  width: 100%;
}

/* Левая часть - таблица заявок */
.applications-list {
  width: 55%; /* Изменено с 100% на 55% */
  display: flex;
  flex-direction: column;
  transition: width 0.3s ease;
  border-right: 1px solid #e6e6e6;
}

.applications-list.with-details {
  width: 55%; /* Изменено с 50% на 55% */
}

/* Заголовок таблицы */
.applications-header {
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
.id-col {
  width: 35%; /* Увеличено для компенсации */
  min-width: 150px;
}

.date-col {
  width: 25%; /* Уменьшено с 30% на 20% */
  min-width: 150px; /* Уменьшено с 150px */
}

.confirmation-col {
  width: 30%; /* Немного увеличено */
  min-width: 150px;
}

.status-col {
  width: 23%; /* Немного увеличено */
  min-width: 100px;
}

/* Тело таблицы */
.applications-body {
  overflow-y: auto;
  flex-grow: 1;
}

.application-item {
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.application-item.selected {
  background-color: #f8f9ff;
}

.application-item:hover {
  background-color: #fafafa;
}

.application-row {
  display: flex;
  width: 100%;
  padding: 12px 16px;
  align-items: center;
}

.application-col {
  padding: 0 4px;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
}

.application-id {
  color: #4F5BDF;
  font-weight: 600;
}

/* Бейджи подтверждения */
.confirmation-badge {
  padding: 4px 8px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 500;
  display: inline-block;
}

.confirmation-approved {
  background-color: #f0f9ff;
  color: #059669;
  border: 1px solid #a7f3d0;
}

.confirmation-pending {
  background-color: #fffbeb;
  color: #d97706;
  border: 1px solid #fcd34d;
}

.confirmation-rejected {
  background-color: #fef2f2;
  color: #dc2626;
  border: 1px solid #fecaca;
}

.confirmation-default {
  background-color: #f5f5f5;
  color: #616161;
  border: 1px solid #e0e0e0;
}

/* Бейджи статуса */
.status-badge {
  display: inline-block;
  padding: 4px 8px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
  min-width: 70px;
  text-align: center;
}

.status-badge-lg {
  display: inline-block;
  padding: 6px 12px;
  border-radius: 14px;
  font-size: 13px;
  font-weight: 500;
  min-width: 80px;
  text-align: center;
  margin-left: 12px;
}

/* Статусы */
.status-unread {
  background-color: #fff7ed;
  color: #ea580c;
  border: 1px solid #fed7aa;
}

.status-processing {
  background-color: #fff3e0;
  color: #ef6c00;
  border: 1px solid #ffe0b2;
}

.status-in-progress {
  background-color: #e3f2fd;
  color: #1565c0;
  border: 1px solid #bbdefb;
}

.status-completed {
  background-color: #e8f5e8;
  color: #2e7d32;
  border: 1px solid #c8e6c9;
}

.status-rejected {
  background-color: #ffebee;
  color: #c62828;
  border: 1px solid #ffcdd2;
}

.status-default {
  background-color: #f5f5f5;
  color: #616161;
  border: 1px solid #e0e0e0;
}

.status-unread-lg {
  background-color: #fff7ed;
  color: #ea580c;
  border: 1px solid #fed7aa;
}

.status-processing-lg {
  background-color: #fff3e0;
  color: #ef6c00;
  border: 1px solid #ffe0b2;
}

.status-in-progress-lg {
  background-color: #e3f2fd;
  color: #1565c0;
  border: 1px solid #bbdefb;
}

.status-completed-lg {
  background-color: #e8f5e8;
  color: #2e7d32;
  border: 1px solid #c8e6c9;
}

.status-rejected-lg {
  background-color: #ffebee;
  color: #c62828;
  border: 1px solid #ffcdd2;
}

.status-default-lg {
  background-color: #f5f5f5;
  color: #616161;
  border: 1px solid #e0e0e0;
}

/* Правая часть - детали заявки */
.application-details-panel {
  width: 45%; /* Изменено с 50% на 45% */
  padding: 20px;
  overflow-y: auto;
  flex-shrink: 0;
  background-color: #fafafa;
}

.no-selection-message {
  width: 45%; /* Изменено с 50% на 45% */
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
  margin-bottom: 20px;
  padding-bottom: 16px;
  border-bottom: 1px solid #e6e6e6;
}

.details-title-wrapper {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.details-title {
  margin: 0;
  color: #1a1a1a;
  font-size: 1.2em;
  font-weight: 600;
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

.details-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}

.detail-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.detail-label {
  font-weight: 500;
  color: #a2a2a2;
  font-size: 13px;
}

.detail-value {
  color: #1a1a1a;
  font-size: 14px;
  font-weight: 500;
}

.no-data-message {
  text-align: center;
  color: #a2a2a2;
  padding: 40px 20px;
  margin: 0;
  font-size: 14px;
}

.hint {
  font-size: 12px;
  color: #999;
  margin-top: 8px;
}

.loading-message {
  text-align: center;
  color: #4F5BDF;
  padding: 40px 20px;
  margin: 0;
  font-size: 14px;
}

@media (max-width: 768px) {
  .applications-container {
    flex-direction: column;
    height: auto;
  }
  
  .applications-list,
  .application-details-panel,
  .no-selection-message {
    width: 100% !important;
  }
  
  .applications-list.with-details {
    border-right: none;
    border-bottom: 1px solid #e6e6e6;
    height: 300px;
  }
  
  .header-row,
  .application-row {
    flex-wrap: wrap;
  }
  
  .header-col,
  .application-col {
    width: 50% !important;
    margin-bottom: 4px;
  }
  
  .details-title-wrapper {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  
  .status-badge-lg {
    margin-left: 0;
  }
  
  .card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
    height: auto;
    padding: 16px;
  }
  
  .card-header__settings {
    width: 100%;
    justify-content: flex-end;
  }
  
  .filter-tabs {
    flex-wrap: wrap;
    justify-content: center;
  }
  
  .responsible-user-item {
    padding: 6px 8px;
  }
}
</style>