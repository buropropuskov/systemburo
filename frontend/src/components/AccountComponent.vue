<template>
  <div class="account-dashboard">
    <!-- Первая строка: заголовок и заявки -->
    <div class="first-row">
      <!-- Заголовок с информацией о пользователе -->
      <UserProfileHeader
        :organization="organization"
        :company="company"
        :last-name="lastName"
        :first-name="firstName"
        :middle-name="middleName"
        :position="position"
        :email="email"
        :phone="phone"
        :user-type="user_type"
        :type-id="type_id"
        class="dashboard-card-animated"
      />

      <UserNotifications />
    </div>
    
    <div class="dashboard-row">
      <!-- Блок заявок с наблюдаемым элементом -->
      <div class="applications-wrapper">
        <UserApplications 
          :applications="applications"
          :organization="organization"
          @refresh-applications="fetchApplications"
          class="dashboard-card-animated"
        />
      </div>
    </div>
    
    
    
    <div class="settings">
      <div class="account__settings" :class="{ 'fixed': isSettingsFixed }" v-if="isBuroPropuskov">
      <div class="settings__container">
        <img src="@/assets/icons/settings.png" class="settings__icon" />
        <h2 class="settings__title">Управление и настройка системы</h2>
      </div>
      <ul class="settings__navigation">
        <li class="navigation__link">
          <a href="#users" class="link">Пользователи</a>
        </li>
        <li class="navigation__link">
          <a href="#organizations" class="link">Организации</a>
        </li>
        <li class="navigation__link">
          <a href="#companies" class="link">Компании</a>
        </li>
        <li class="navigation__link">
          <a href="#" class="link disabled">Доступ</a>
        </li>
        <li class="navigation__link">
          <a href="#tables" class="link">Таблицы</a>
        </li>
        <li class="navigation__link">
          <a href="#number" class="link">Авто-номера</a>
        </li>
        <li class="navigation__link">
          <a href="#unload_place" class="link">Разгрузка</a>
        </li>
        <li class="navigation__link">
          <a href="#" class="link disabled">Уведомления</a>
        </li>
        <li class="navigation__link">
          <a href="#" class="link disabled">Шаблоны</a>
        </li>
         <li class="navigation__link">
          <a href="#" class="link disabled">Карта</a>
        </li>
      </ul>
    </div>
    </div>
    

    <!-- Вторая строка: управление пользователями (если доступно) -->
    <div class="dashboard-row" id="users" v-if="isBuroPropuskov">
      <UserControl 
        :allUsers="allUsers"
        @fetch-users="fetchAllUsers"
        @user-updated="handleUserUpdated"
        class="dashboard-card dashboard-card-animated"
      />
    </div>

    <!-- Третья строка: управление организациями (если доступно) -->
    <div class="dashboard-row" id="organizations" v-if="isBuroPropuskov">
      <OrganizationsManagement class="dashboard-card dashboard-card-animated"/>
    </div>
    <div class="dashboard-row" id="companies" v-if="isBuroPropuskov">
      <CompaniesManagement class="dashboard-card dashboard-card-animated"/>
    </div>
    <div class="dashboard-row" id="unload" v-if="isBuroPropuskov">
      <UnloadPlacesContainer id="unload_place" class="dashboard-card dashboard-card-animated"/>
    </div>
    <div class="dashboard-row" id="number" v-if="isBuroPropuskov">
      <NumberFormat class="dashboard-card dashboard-card-animated"/>
    </div>
    <div class="dashboard-row" id="tables" v-if="isBuroPropuskov">
      <TableConstructor class="dashboard-card dashboard-card-animated" />
    </div>
  </div>
</template>

<script>
import UserControl from './UserControl.vue';
import UserApplications from './UserApplications.vue';
import UserProfileHeader from './UserProfileHeader.vue';
import UserNotifications from './UserNotifications.vue';
import OrganizationsManagement from './OrganizationsManagement.vue';
import CompaniesManagement from './CompaniesManagement.vue';
import UnloadPlacesContainer from './UnloadPlaces/UnloadPlacesContainer.vue';
import NumberFormat from './NumberFormat.vue';
import TableConstructor from './TableConstructor.vue';

export default {
  components: {
    UserControl,
    UserApplications,
    UserProfileHeader,
    UserNotifications,
    OrganizationsManagement,
    CompaniesManagement,
    UnloadPlacesContainer,
    NumberFormat,
    TableConstructor
  },
  data() {
    return {
      applications: [],
      allUsers: [],
      organization: "",
      company: "",
      username: "",
      type_id: 1,
      user_type: "user",
      
      // Поля пользователя
      lastName: "",
      firstName: "",
      middleName: "",
      position: "",
      email: "",
      phone: "",
      
      // Настройки фиксации
      isSettingsFixed: false,
      fixationOffset: 0, // Отступ в пикселях (можно менять)
      showFixationControls: false, // Показывать панель управления фиксацией
      
      // Observer для отслеживания видимости элемента
      observer: null
    };
  },
  computed: {
    isBuroPropuskov() {
      return this.type_id === 6 || this.user_type === 'buropropuskov';
    }
  },
  methods: {
    async fetchUserData() {
      try {
        const token = localStorage.getItem("token");
        if (!token) {
          alert("Пользователь не авторизован.");
          return;
        }

        const response = await fetch("http://localhost:8080/users/me", {
          method: "GET",
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });

        if (response.ok) {
          const userData = await response.json();
          this.updateUserData(userData);
        } else {
          alert("Ошибка при загрузке данных пользователя.");
        }
      } catch (error) {
        console.error("Ошибка сети при загрузке данных пользователя:", error);
      }
    },
    updateUserData(userData) {
      this.organization = userData.organization || "";
      this.company = userData.company || "";
      this.username = userData.username || "";
      this.type_id = userData.type_id || 1;
      this.user_type = userData.user_type || "user";
      
      // Дополнительные поля
      this.lastName = userData.last_name || "";
      this.firstName = userData.first_name || "";
      this.middleName = userData.middle_name || "";
      this.position = userData.position || "";
      this.email = userData.email || "";
      this.phone = userData.phone || "";
    },
    async fetchApplications() {
      try {
        const token = localStorage.getItem("token");
        if (!token) {
          alert("Пользователь не авторизован.");
          return;
        }

        const response = await fetch("http://localhost:8080/applications/all-cars", {
          method: "GET",
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });

        if (response.ok) {
          const data = await response.json();
          this.applications = data;
        } else {
          alert("Ошибка при загрузке заявок.");
        }
      } catch (error) {
        console.error("Ошибка сети при загрузке заявок:", error);
      }
    },
    async fetchAllUsers() {
      try {
        const token = localStorage.getItem("token");
        const response = await fetch("http://localhost:8080/users/all", {
          method: "GET",
          headers: {
            "Authorization": `Bearer ${token}`,
          },
        });

        if (response.ok) {
          const data = await response.json();
          this.allUsers = data.map(user => ({
            ...user,
            newPassword: ""
          }));
        } else {
          alert("Ошибка при загрузке пользователей.");
        }
      } catch (error) {
        console.error("Ошибка сети при загрузке пользователей:", error);
      }
    },
    handleUserUpdated(updatedUser) {
      if (updatedUser.username === this.username) {
        this.updateUserData(updatedUser);
      }
      
      const userIndex = this.allUsers.findIndex(u => u.username === updatedUser.username);
      if (userIndex !== -1) {
        this.allUsers[userIndex] = {
          ...this.allUsers[userIndex],
          ...updatedUser
        };
      }
    },
    initIntersectionObserver() {
      if (!this.isBuroPropuskov) return;
      
      const applicationsElement = this.$refs.applicationsWrapper;
      if (!applicationsElement) {
        console.warn('Applications element not found');
        return;
      }

      console.log('Initializing Intersection Observer with offset:', this.fixationOffset);

      // Создаем наблюдатель для отслеживания нижней границы с учетом отступа
      this.observer = new IntersectionObserver(
        (entries) => {
          entries.forEach(entry => {
            console.log('Intersection observed:', {
              isIntersecting: entry.isIntersecting,
              bottom: entry.boundingClientRect.bottom,
              windowHeight: window.innerHeight,
              offset: this.fixationOffset
            });
            
            // Проверяем с учетом отступа
            const shouldFix = entry.boundingClientRect.bottom <= this.fixationOffset;
            
            if (shouldFix && !this.isSettingsFixed) {
              console.log('Making settings fixed - bottom edge reached offset');
              this.isSettingsFixed = true;
            } else if (!shouldFix && this.isSettingsFixed) {
              console.log('Making settings normal - bottom edge above offset');
              this.isSettingsFixed = false;
            }
          });
        },
        {
          root: null, // viewport
          rootMargin: `${this.fixationOffset}px 0px 0px 0px`, // Используем отступ в rootMargin
          threshold: 0 // Срабатывает при любом пересечении
        }
      );

      // Начинаем наблюдение за элементом заявок
      this.observer.observe(applicationsElement);
    },
    handleScroll() {
      // Резервный метод на случай если Intersection Observer не работает
      if (!this.isBuroPropuskov) return;
      
      const applicationsElement = this.$refs.applicationsWrapper;
      if (!applicationsElement) return;

      const rect = applicationsElement.getBoundingClientRect();
      // Проверяем с учетом отступа
      const shouldFix = rect.bottom <= this.fixationOffset;
      
      console.log('Scroll check:', {
        bottom: rect.bottom,
        offset: this.fixationOffset,
        shouldFix: shouldFix,
        isSettingsFixed: this.isSettingsFixed
      });
      
      if (shouldFix && !this.isSettingsFixed) {
        console.log('Setting fixed position with custom offset');
        this.isSettingsFixed = true;
      } else if (!shouldFix && this.isSettingsFixed) {
        console.log('Removing fixed position');
        this.isSettingsFixed = false;
      }
    },
    updateFixationOffset() {
      // Обновляем observer с новым отступом
      if (this.observer) {
        this.observer.disconnect();
      }
      this.$nextTick(() => {
        this.initIntersectionObserver();
      });
    },
    resetOffset() {
      this.fixationOffset = 20;
      this.updateFixationOffset();
    }
  },
  mounted() {
    this.fetchUserData();
    this.fetchApplications();
    if (this.isBuroPropuskov) {
      this.fetchAllUsers();
      // Включаем панель управления для разработки
      this.showFixationControls = process.env.NODE_ENV === 'development';
    }
    
    // Инициализируем observer после монтирования DOM
    this.$nextTick(() => {
      this.initIntersectionObserver();
      
      // Добавляем резервный обработчик скролла
      window.addEventListener('scroll', this.handleScroll, { passive: true });
    });
  },
  beforeUnmount() {
    // Очищаем observer при уничтожении компонента
    if (this.observer) {
      this.observer.disconnect();
    }
    window.removeEventListener('scroll', this.handleScroll);
  }
};
</script>

<style scoped>
.account-dashboard {
  max-width: 1400px;
  margin: 0 auto;
  padding: 15px;
  position: relative;
}

/* Стили для строк */
.first-row {
  display: flex;
  gap: 15px;
  margin-bottom: 15px;
}

.dashboard-row {
 padding: 15px 0;
}

.applications-wrapper {
  position: relative;
}

/* Стили для карточек */
.dashboard-card {
  background: white;
  border-radius: 30px;
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.05);
  border: 1px solid #e6e6e6;
}

.dashboard-card-animated {
  opacity: 0;
  transform: translateY(20px);
  animation: fadeInUp 0.5s cubic-bezier(0.16, 1, 0.3, 1) forwards;
}

.dashboard-card h3 {
  margin-top: 0;
  margin-bottom: 15px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
  color: #4F5BDF;
  font-size: 1.2em;
  font-weight: 600;
}

.reserved-area {
  background: #f8f9fa;
  border: 1px dashed rgba(0, 0, 0, 0.1) !important;
}

.placeholder-content {
  padding: 15px;
  text-align: center;
  color: #666;
  font-size: 0.9em;
  opacity: 0.8;
}

.settings {
  height: 30px;
  width: 1238px;
  margin-top: 35px;
  margin-bottom: 35px;
}

.account__settings {
  display: flex;
  width: 100%;
  gap: 0;
  position: relative;
  background: transparent;
}




.account__settings.fixed {
  padding-top: 35px;
  padding-bottom: 15px;
  position: fixed;
  top: 0;
  background: rgba(255,255,255,0.8);
  -webkit-backdrop-filter: blur(10px);
  z-index: 99;
  max-width: 1238px;
  border-bottom: 1px solid #e6e6e6;
}

.settings__container {
  min-width: 400px;
  height: 50px;
  background-color: #4F5BDF;
  border-radius: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.settings__icon {
  width: 25px;
  height: 25px;
  animation: settings_rotate 8s linear infinite;
}

.settings__title {
  font-size: 18px;
  color: #FFF;
}

.settings__navigation {
  width: 100%;
  height: 30px;
  border-top: 2px solid #4F5BDF;
  margin-top: auto;
  display: flex;
  list-style: none;
  justify-content: space-between;
  padding: 5px 10px;
}

.link {
  font-size: 12px;
  font-weight: 600;
  color: #939CFF;
  transition: .2s;
  text-decoration: none;
}

.link:hover:not(.disabled) {
  color: #4F5BDF;
}

/* Панель управления фиксацией */
.fixation-controls {
  margin: 10px 0;
  padding: 15px;
  background: #f8f9fa;
  border-radius: 10px;
  border: 1px solid #e9ecef;
}

.control-panel h4 {
  margin: 0 0 10px 0;
  color: #495057;
  font-size: 14px;
}

.control-group {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
  flex-wrap: wrap;
}

.control-group label {
  font-size: 14px;
  color: #495057;
  white-space: nowrap;
}

.offset-input {
  width: 80px;
  padding: 5px 8px;
  border: 1px solid #ced4da;
  border-radius: 4px;
  font-size: 14px;
}

.reset-btn {
  padding: 5px 10px;
  background: #6c757d;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
}

.reset-btn:hover {
  background: #5a6268;
}

.status-info {
  display: flex;
  gap: 15px;
  font-size: 12px;
  color: #6c757d;
  flex-wrap: wrap;
}

@keyframes settings_rotate {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}

/* Анимации */
@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Адаптивность */
@media (max-width: 1200px) {
  .first-row {
    flex-direction: column;
  }
  
  .account__settings.fixed {
    padding: 10px;
    width: calc(100% - 20px);
  }
  
  .settings__container {
    min-width: 300px;
  }
}

@media (max-width: 768px) {
  .account__settings {
    flex-direction: column;
    gap: 10px;
  }
  
  .account__settings.fixed {
    flex-direction: column;
    gap: 10px;
  }
  
  .settings__container {
    min-width: auto;
    width: 100%;
  }
  
  .settings__navigation {
    flex-wrap: wrap;
    height: auto;
    gap: 10px;
  }
  
  .control-group {
    flex-direction: column;
    align-items: flex-start;
  }
  
  .status-info {
    flex-direction: column;
    gap: 5px;
  }
}

@media (max-width: 480px) {
  .dashboard-card {
    padding: 15px 10px;
  }
  
  .settings__title {
    font-size: 16px;
  }
  
  .link {
    font-size: 11px;
  }
  
  .account__settings.fixed {
    width: calc(100% - 20px);
    padding: 8px 10px;
  }
  
  .fixation-controls {
    padding: 10px;
  }
}

.disabled {
  color: #e6e6e6;
  cursor: not-allowed;
}
</style>