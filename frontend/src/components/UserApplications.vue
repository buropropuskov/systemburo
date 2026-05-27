<template>
  <div class="applications-card">
    <div class="card-header">
      <div class="card-header__title">
        <h3 class="card-title">
          Список заявок
        </h3>
        
        <!-- Кнопки фильтров в шапке -->
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
        </div>
      </div>
      
      <div class="card-header__settings">
        <SearchComponent
          v-model="searchQuery"
          :title="'Поиск заявок..'"
        />
        <RefreshButton @refresh="fetchUserApplications" />
      </div>
    </div>
    
    <div class="card-content">
      <div class="applications-container">
        <!-- Левая часть - таблица заявок -->
        <div class="applications-list">
          <!-- Заголовок таблицы -->
          <div class="applications-header">
            <div class="header-row">
              <div
                class="header-col id-col"
                @click="sortBy('application_number')"
              >
                <p :class="{ 'active-sort': sortField === 'application_number' }">
                  Номер заявки
                </p>
                <img 
                  src="@/assets/icons/sort.png" 
                  class="sort-icon" 
                  :class="{ 
                    'sorted': sortField === 'application_number',
                    'desc': sortField === 'application_number' && sortDirection === 'desc'
                  }" 
                >
              </div>
              <div
                class="header-col date-col"
                @click="sortBy('sending_datetime')"
              >
                <p :class="{ 'active-sort': sortField === 'sending_datetime' }">
                  Дата и время
                </p>
                <img 
                  src="@/assets/icons/sort.png" 
                  class="sort-icon" 
                  :class="{ 
                    'sorted': sortField === 'sending_datetime',
                    'desc': sortField === 'sending_datetime' && sortDirection === 'desc'
                  }" 
                >
              </div>
              <div
                class="header-col sender-col"
                @click="sortBy('sender_name')"
              >
                <p :class="{ 'active-sort': sortField === 'sender_name' }">
                  Отправитель
                </p>
                <img 
                  src="@/assets/icons/sort.png" 
                  class="sort-icon" 
                  :class="{ 
                    'sorted': sortField === 'sender_name',
                    'desc': sortField === 'sender_name' && sortDirection === 'desc'
                  }" 
                >
              </div>
              <div
                class="header-col confirmation-col"
                @click="sortBy('confirmation')"
              >
                <p :class="{ 'active-sort': sortField === 'confirmation' }">
                  Подтверждение
                </p>
                <img 
                  src="@/assets/icons/sort.png" 
                  class="sort-icon" 
                  :class="{ 
                    'sorted': sortField === 'confirmation',
                    'desc': sortField === 'confirmation' && sortDirection === 'desc'
                  }" 
                >
              </div>
              <div
                class="header-col status-col"
                @click="sortBy('status')"
              >
                <p :class="{ 'active-sort': sortField === 'status' }">
                  Статус
                </p>
                <img
                  src="@/assets/icons/sort.png"
                  class="sort-icon"
                  :class="{
                    'sorted': sortField === 'status',
                    'desc': sortField === 'status' && sortDirection === 'desc'
                  }"
                >
              </div>
              <div class="header-col actions-col" />
            </div>
          </div>
          
          <!-- Тело таблицы -->
          <div class="applications-body">
            <div
              v-if="isLoading"
              class="loading-message"
            >
              <LoaderSpinner label="Загрузка заявок…" />
            </div>
            
            <template v-else>
              <div
                v-if="filteredApplications.length > 0"
                class="applications-list-content"
              >
                <transition-group
                  name="fade-list"
                  tag="div"
                  class="applications-transition-group"
                >
                  <div 
                    v-for="(application) in sortedApplications" 
                    :key="application.id" 
                    class="application-item"
                    @click="openApplication(application)"
                  >
                    <div class="application-row">
                      <div class="application-col id-col">
                        <span class="application-id">{{ application.application_number }}</span>
                      </div>
                      <div class="application-col date-col">
                        {{ formatDateTime(application.sending_datetime) }}
                      </div>
                      <div class="application-col sender-col">
                        {{ application.sender_name || application.sender_full_name || '—' }}
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
                      <div class="application-col actions-col">
                        <button
                          v-if="application.has_blank_template"
                          class="download-btn"
                          title="Скачать"
                          @click.stop="downloadApplication(application)"
                        >
                          Скачать
                        </button>
                      </div>
                    </div>
                  </div>
                </transition-group>
              </div>
              
              <div
                v-else
                class="no-data-message"
              >
                <p>{{ searchQuery ? 'Заявки не найдены' : 'Заявок нет' }}</p>
                <p
                  v-if="!searchQuery"
                  class="hint"
                >
                  {{ getNoDataHint() }}
                </p>
              </div>
            </template>
          </div>
        </div>
      </div>
    </div>

    <teleport to="body">
      <ApplicationDetail
        v-if="showDetailModal"
        :application="selectedApplication"
        :current-user-id="currentUserId"
        :current-user-name="currentUserName"
        :mode="'user'"
        @close="closeApplicationDetail"
        @duplicate="handleDuplicate"
      />
      <DownloadBlanksModal
        v-if="showDownloadModal && downloadAppId"
        :application-id="downloadAppId"
        :application-info="downloadAppInfo"
        @close="showDownloadModal = false"
      />
    </teleport>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import RefreshButton from './RefreshButton.vue';
import SearchComponent from './SearchComponent.vue';
import ApplicationDetail from './ApplicationDetail/ApplicationDetail.vue';
import DownloadBlanksModal from './applications/DownloadBlanksModal.vue';
import LoaderSpinner from './ui/LoaderSpinner.vue';

export default {
  components: {
    RefreshButton,
    SearchComponent,
    ApplicationDetail,
    DownloadBlanksModal,
    LoaderSpinner
  },
  props: {
    userOrganizationId: {
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
    }
  },
  data() {
    return {
      applications: [],
      selectedApplication: null,
      showDetailModal: false,
      responsibleUsers: [],
      searchQuery: '',
      sortField: null,
      sortDirection: 'desc',
      isLoading: false,
      currentFilter: 'my',
      currentUserId: null,
      currentUserName: '',
      showDownloadModal: false,
      downloadAppId: null,
      downloadAppInfo: null
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
            
          case 'sender_name':
            valueA = a.sender_name || a.sender_full_name || '';
            valueB = b.sender_name || b.sender_full_name || '';
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
  watch: {
    searchQuery() {
      this.fetchUserApplications();
    },
    userId() {
      this.fetchUserApplications();
    }
  },
  mounted() {
    this.fetchUserApplications();
    this.getCurrentUser();
    
    // Добавляем стили для анимации уведомлений
    const style = document.createElement('style');
    style.textContent = `
      @keyframes slideDown {
        from {
          transform: translate(-50%, -100%);
          opacity: 0;
        }
        to {
          transform: translate(-50%, 0);
          opacity: 1;
        }
      }
      @keyframes slideUp {
        from {
          transform: translate(-50%, 0);
          opacity: 1;
        }
        to {
          transform: translate(-50%, -100%);
          opacity: 0;
        }
      }
    `;
    document.head.appendChild(style);
  },
  beforeUnmount() {
    // Восстанавливаем скролл при размонтировании компонента
    document.body.style.overflow = '';
  },
  methods: {
    async fetchUserApplications() {
      try {
        this.isLoading = true;
        const authStore = useAuthStore();
        if (!authStore.token) {
          console.error("Пользователь не авторизован.");
          this.isLoading = false;
          return;
        }

        let url = "/applications/user";
        const params = new URLSearchParams();

        if (this.searchQuery) {
          params.append('search_query', this.searchQuery);
        }

        const queryString = params.toString();
        if (queryString) {
          url += '?' + queryString;
        }

        const response = await apiRequest(url, {
          method: "GET",
          headers: {
            "Accept": "application/json"
          },
        });
        
        if (response.ok) {
          const data = await response.json();
          
          this.applications = data.filter(app => {
            switch (this.currentFilter) {
              case 'my':
                return app.sender_user_id === this.userId;
              case 'organization':
                return app.organization_id === this.userOrganizationId;
              default:
                return true;
            }
          });
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
        const authStore = useAuthStore();
        if (!authStore.token) return;

        const response = await apiRequest(`/applications/${applicationId}/responsible-users`, {
          method: "GET",
          headers: {
            "Accept": "application/json"
          },
        });

        if (response.ok) {
          this.responsibleUsers = await response.json();
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
        default:
          return 'Заявок нет';
      }
    },

    async openApplication(application) {
      this.selectedApplication = application;
      await this.fetchResponsibleUsers(application.id);
      this.showDetailModal = true;
      
      // Блокируем скролл body при открытии модального окна
      document.body.style.overflow = 'hidden';
    },

    closeApplicationDetail() {
      this.showDetailModal = false;
      this.selectedApplication = null;
      this.responsibleUsers = [];
      
      // Разблокируем скролл body при закрытии модального окна
      document.body.style.overflow = '';
    },

    handleDuplicate(application) {
      console.log('Дублирование заявки из UserApplications:', application.application_number);
      this.showNotification('Функция дублирования пока не реализована', 'error');
    },

    downloadApplication(application) {
      this.downloadAppId = application.id;
      this.downloadAppInfo = application;
      this.showDownloadModal = true;
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
    },

    async getCurrentUser() {
      try {
        const response = await apiRequest("/users/me", {
          method: "GET",
        });

        if (response.ok) {
          const userData = await response.json();
          this.currentUserId = userData.id;
          this.currentUserName = `${userData.last_name} ${userData.first_name}`;
        } else {
          console.error("Ошибка при получении текущего пользователя:", await response.text());
        }
      } catch (error) {
        console.error("Ошибка сети при получении текущего пользователя:", error);
      }
    },

    showNotification(message, type = 'success') {
      // Создаем временное уведомление
      const notification = document.createElement('div');
      notification.className = `temp-notification ${type}`;
      notification.textContent = message;
      notification.style.cssText = `
        position: fixed;
        top: 20px;
        left: 50%;
        transform: translateX(-50%);
        padding: 12px 24px;
        border-radius: 8px;
        z-index: 99999;
        font-weight: 500;
        box-shadow: 0 4px 12px rgba(0,0,0,0.15);
        animation: slideDown 0.3s ease-out, slideUp 0.3s ease-out 2.7s forwards;
      `;
      
      if (type === 'success') {
        notification.style.background = '#4CAF50';
        notification.style.color = 'white';
      } else {
        notification.style.background = '#f44336';
        notification.style.color = 'white';
      }
      
      document.body.appendChild(notification);
      
      setTimeout(() => {
        if (notification.parentNode) {
          notification.parentNode.removeChild(notification);
        }
      }, 3000);
    }
  }
};
</script>

<style scoped>
.applications-card {
  background-color: #fff;
  border-radius: 30px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  width: 100%;
  box-shadow: 0 3px 10px rgba(0,0,0,0.05);
  height: 383px;
  display: flex;
  flex-direction: column;
  position: relative;
}

.card-header {
  border-bottom: 1px solid #e6e6e6;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0px 20px;
  height: auto;
  min-height: 50px;
  flex-shrink: 0;
}

.card-header__title {
  display: flex;
  gap: 15px;
  align-items: center;
  flex: 1;
}

.card-header__settings {
  display: flex;
  gap: 8px;
  align-items: center;
}

.card-title {
  margin: 0;
  color: #000;
  font-weight: 600;
  font-size: 1.1em;
}

/* Стили для кнопок фильтров в шапке */
.filter-tabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.filter-tab {
  padding: 4px 12px;
  border: 1px solid #e6e6e6;
  background: white;
  border-radius: 16px;
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

.card-content {
  padding: 0;
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.applications-container {
  display: flex;
  flex: 1;
  width: 100%;
  overflow: hidden;
}

/* Левая часть - таблица заявок */
.applications-list {
  width: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  height: 100%;
}

/* Заголовок таблицы */
.applications-header {
  border-bottom: 1px solid #e6e6e6;
  padding: 12px 16px;
  flex-shrink: 0;
  height: 44px;
  box-sizing: border-box;
}

.header-row {
  display: flex;
  width: 100%;
}

.header-col {
  font-weight: 500;
  color: #a2a2a2;
  text-align: left;
  font-size: 14px;
  display: flex;
  align-items: center;
  gap: 5px;
  transition: .2s;
  cursor: pointer;
  user-select: none;
  flex: 1;
  overflow: hidden;
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
  flex-shrink: 0;
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

/* Колонки с пропорциональной шириной для 5 столбцов */
.id-col {
  flex: 1.2; /* Немного шире для номера заявки */
  min-width: 180px;
  max-width: 180px;
}

.date-col {
  flex: 1.5; /* Шире для даты и времени */
  min-width: 180px;
  max-width: 180px;
}

.sender-col {
  flex: 1.2; /* Отправитель */
  min-width: 200px;
  max-width: 200px;
}

.confirmation-col {
  flex: 1; /* Подтверждение */
  min-width: 180px;
  max-width: 180px;
}

.status-col {
  flex: 1;
  min-width: 180px;
  max-width: 180px;
}

.actions-col {
  flex: 0 0 100px;
  min-width: 100px;
  max-width: 100px;
  justify-content: flex-end;
  cursor: default;
}

.header-col.actions-col:hover {
  color: #a2a2a2;
}

.download-btn {
  height: 25px;
  background-color: #fff;
  color: #000;
  border-radius: 50px;
  border: 1px solid var(--color-border);
  font-size: 12px;
  font-weight: 500;
  padding: 0 12px;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  white-space: nowrap;
  min-width: 80px;
}

.download-btn:hover {
  background-color: #f5f5f5;
  border-color: #d0d0d0;
}

/* Тело таблицы */
.applications-body {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  position: relative;
}

.applications-list-content {
  flex: 1;
  overflow-y: auto;
  position: relative;
}

.applications-transition-group {
  position: relative;
  width: 100%;
}

.application-item {
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
  transition: background-color 0.2s ease;
  flex-shrink: 0;
  position: relative;
}

.application-item:hover {
  background-color: #fafafa;
}

.application-row {
  display: flex;
  width: 100%;
  padding: 6px 0;
  align-items: center;
  height: 40px;
}

.application-col {
  padding: 0 16px;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  flex: 1;
  align-items: center;
  display: flex;
  height: 100%;
}

.application-col:first-child {
  padding-left: 16px;
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
  white-space: nowrap;
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
  white-space: nowrap;
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

/* Сообщения о загрузке и отсутствии данных */
.no-data-message {
  text-align: center;
  color: #a2a2a2;
  padding: 40px 20px;
  margin: 0;
  font-size: 14px;
  width: 100%;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
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
  width: 100%;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
}

.loader {
  width: 30px;
  height: 30px;
  border: 3px solid #f3f3f3;
  border-top: 3px solid #4F5BDF;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

/* Анимация для списка заявок - исправленная */
.fade-list-enter-active {
  transition: all 0.3s ease;
  position: relative;
}

.fade-list-leave-active {
  transition: all 0.3s ease;
  position: absolute !important;
  width: 100%;
  left: 0;
}

.fade-list-enter-from {
  opacity: 0;
  transform: translateY(10px);
}

.fade-list-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

.fade-list-move {
  transition: transform 0.3s ease;
}

/* Стили для прокрутки */
.applications-body::-webkit-scrollbar {
  width: 6px;
}

.applications-body::-webkit-scrollbar-track {
  background: transparent;
  margin: 2px 0;
  border-radius: 3px;
}

.applications-body::-webkit-scrollbar-thumb {
  background: #D9E2FF;
  border-radius: 3px;
  border: 1px solid transparent;
  background-clip: content-box;
  transition: all 0.3s ease;
}

.applications-body::-webkit-scrollbar-thumb:hover {
  background: #C5D1FF;
  border: 1px solid transparent;
  background-clip: content-box;
}

.applications-body {
  scrollbar-width: thin;
  scrollbar-color: #D9E2FF transparent;
}

@media (max-width: 1200px) {
  .applications-card {
    height: 450px;
  }
  
  .header-col,
  .application-col {
    padding: 0 12px;
  }
  
  .header-col:first-child,
  .application-col:first-child {
    padding-left: 12px;
  }
}

@media (max-width: 992px) {
  .applications-card {
    height: 500px;
  }
  
  .card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
    height: auto;
    padding: 16px;
  }
  
  .card-header__title {
    width: 100%;
  }
  
  .card-header__settings {
    width: 100%;
    justify-content: flex-end;
  }
  
  .filter-tabs {
    width: 100%;
    justify-content: flex-start;
  }
  
  .applications-container {
    flex-direction: column;
    height: 100%;
  }
  
  .applications-list {
    width: 100% !important;
    height: 100% !important;
  }
  
  .header-row,
  .application-row {
    flex-wrap: wrap;
  }
  
  .header-col,
  .application-col {
    width: 33.33% !important;
    margin-bottom: 4px;
    min-width: 100px !important;
    max-width: none !important;
    flex: none !important;
    padding: 0 8px;
  }
  
  .header-col:first-child,
  .application-col:first-child {
    padding-left: 8px;
  }
  
  .date-col {
    min-width: 140px !important;
  }
  
  .confirmation-badge,
  .status-badge {
    min-width: 60px;
    font-size: 10px;
  }
}

@media (max-width: 768px) {
  /*
   * Синхронный horizontal scroll: scroll на .applications-list (общий parent
   * header+body), inner header/body имеют visible overflow чтобы наследовать
   * scroll от parent'а. Headers и data двигаются вместе.
   */
  .applications-list {
    overflow-x: auto !important;
    overflow-y: hidden !important;
  }

  .applications-header,
  .applications-body,
  .applications-list-content {
    overflow-x: visible !important;
    min-width: 600px;
  }

  .applications-body {
    overflow-y: visible !important;
    height: auto !important;
    max-height: none !important;
  }

  .header-row,
  .application-row {
    flex-wrap: nowrap !important;
    min-width: 600px;
  }

  .header-col,
  .application-col {
    width: auto !important;
    min-width: 110px !important;
    flex: 1 1 auto !important;
    white-space: nowrap;
  }

  .header-col.date-col,
  .application-col.date-col {
    min-width: 140px !important;
  }

  .header-col p,
  .header-col {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .header-col.actions-col,
  .application-col.actions-col {
    display: none;
  }
}

@media (max-width: 576px) {
  /* Оставляем horizontal-scroll from 768px, просто уменьшаем min-widths для экономии места */
  .header-col,
  .application-col {
    min-width: 100px !important;
    font-size: 13px;
    padding: 0 10px !important;
  }

  .header-col.date-col,
  .application-col.date-col {
    min-width: 120px !important;
  }
}
</style>