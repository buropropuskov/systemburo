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
    <!-- Блок заявок -->
      <UserApplications 
        :applications="applications"
        :organization="organization"
        @refresh-applications="fetchApplications"
        class="dashboard-card-animated"
      />
    </div>

    <div class="account__settings" v-if="isBuroPropuskov">
      <div class="settings__container">
        <img src="@/assets/icons/settings.png" class="settings__icon" />
        <h2 class="settings__title">Управление и настройка системы</h2>
      </div>
      <ul class="settings__navigation">
        <li class="navigation__link">
          <a href="#users" class="link">Пользователи</a>
        </li>
        <li class="navigation__link">
          <a href="#" class="link">Организации</a>
        </li>
        <li class="navigation__link">
          <a href="#" class="link">Компании</a>
        </li>
        <li class="navigation__link">
          <a href="#" class="link">Доступ</a>
        </li>
        <li class="navigation__link">
          <a href="#" class="link">Таблицы</a>
        </li>
        <li class="navigation__link">
          <a href="#" class="link">Согласование</a>
        </li>
        <li class="navigation__link">
          <a href="#" class="link">Уведомления</a>
        </li>
        <li class="navigation__link">
          <a href="#" class="link">Шаблоны</a>
        </li>
         <li class="navigation__link">
          <a href="#" class="link">Карта</a>
        </li>


      </ul>
    </div>

    <!-- Вторая строка: управление пользователями (если доступно) -->
    <div class="dashboard-row" v-if="isBuroPropuskov">
      <UserControl 
        :allUsers="allUsers"
        @fetch-users="fetchAllUsers"
        @user-updated="handleUserUpdated"
        class="dashboard-card dashboard-card-animated"
        id="users"
      />
    </div>

    <!-- Третья строка: управление организациями (если доступно) -->
    <div class="dashboard-row" v-if="isBuroPropuskov">
      <OrganizationsManagement class="dashboard-card dashboard-card-animated"/>
    </div>
    <div class="dashboard-row" v-if="isBuroPropuskov">
      <CompaniesManagement class="dashboard-card dashboard-card-animated"/>
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

export default {
  components: {
    UserControl,
    UserApplications,
    UserProfileHeader,
    UserNotifications,
    OrganizationsManagement,
    CompaniesManagement
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
      phone: ""
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
    }
  },
  mounted() {
    this.fetchUserData();
    this.fetchApplications();
    if (this.isBuroPropuskov) {
      this.fetchAllUsers();
    }
  },
};
</script>

<style scoped>
.account-dashboard {
  max-width: 1400px;
  margin: 0 auto;
  padding: 15px;
}

/* Стили для строк */
.first-row {
  display: flex;
  gap: 15px;
  margin-bottom: 15px;
}

.dashboard-row {
  margin-bottom: 15px;
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

.account__settings {
  padding-top: 30px;
  padding-bottom: 30px;
  display: flex;
  width: 100%;
  gap: 0;
}

.settings__container {
  min-width: 450px;
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
  font-size: 13px;
  font-weight: 600;
  color: #939CFF;
  transition: .2s;
  text-decoration: none;
}

.link:hover {
  color: #4F5BDF;
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
}

@media (max-width: 480px) {
  .dashboard-card {
    padding: 15px 10px;
  }
}
</style>