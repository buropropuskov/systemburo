<template>
  <div
    class="account-dashboard"
    data-testid="cabinet-page"
  >
    <!-- Первая строка: заголовок и заявки -->
    <div class="first-row">
      <SkeletonTransition :loading="loading">
        <template #skeleton>
          <div style="display: flex; gap: 20px; width: 100%;">
            <SkeletonBlock
              height="200px"
              style="flex: 1;"
            />
            <SkeletonBlock
              height="200px"
              style="flex: 1;"
            />
          </div>
        </template>

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

        <UserNotificationsInline class="dashboard-card-animated notifications" />
      </SkeletonTransition>
    </div>
    
    <div class="dashboard-row">
      <!-- Блок заявок -->
      <div class="applications-wrapper">
        <UserApplications 
          :user-organization-id="userOrganizationId"
          :user-company-id="userCompanyId"
          :user-id="userId"
          :user-organization="organization"
          :user-company="company"
          class="dashboard-card-animated"
        />
      </div>
    </div>
    
    <!-- Настройки системы -->
    <div class="settings">
      <AccountSettings v-if="isBuroPropuskov" />
    </div>

    <template v-if="isBuroPropuskov">
      <section id="users" class="account-section">
        <UserControl
          :all-users="allUsers"
          class="dashboard-card dashboard-card-animated"
          @fetch-users="fetchAllUsers"
          @user-updated="handleUserUpdated"
        />
      </section>
      <section id="organizations" class="account-section">
        <OrganizationsManagement class="dashboard-card dashboard-card-animated" />
      </section>
      <section id="companies" class="account-section">
        <CompaniesManagement class="dashboard-card dashboard-card-animated" />
      </section>
      <section id="unload_place" class="account-section">
        <UnloadPlacesContainer class="dashboard-card dashboard-card-animated" />
      </section>
      <section id="number" class="account-section">
        <NumberFormat class="dashboard-card dashboard-card-animated" />
      </section>
      <section id="citizenships" class="account-section">
        <CitizenshipManagement class="dashboard-card dashboard-card-animated" />
      </section>
      <section id="marks" class="account-section">
        <MarksManagement class="dashboard-card dashboard-card-animated" />
      </section>
      <section id="tables" class="account-section">
        <TableConstructor class="dashboard-card dashboard-card-animated" />
      </section>
      <section id="user_types" class="account-section">
        <UserTypes class="dashboard-card dashboard-card-animated" />
      </section>
      <section id="attachments" class="account-section">
        <AttachmentsManagement class="dashboard-card dashboard-card-animated" />
      </section>
      <section id="approvers" class="account-section">
        <ApplicationApprovers class="dashboard-card dashboard-card-animated" />
      </section>
    </template>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import UserControl from './UserControl.vue';
import UserApplications from './UserApplications.vue';
import UserProfileHeader from './UserProfileHeader.vue';
import UserNotificationsInline from './UserNotificationsInline.vue';
import OrganizationsManagement from './OrganizationsManagement.vue';
import CompaniesManagement from './CompaniesManagement.vue';
import UnloadPlacesContainer from './UnloadPlaces/UnloadPlacesContainer.vue';
import NumberFormat from './NumberFormat.vue';
import TableConstructor from './TableConstructor.vue';
import UserTypes from './UserTypes.vue';
import AccountSettings from './AccountSettings.vue';
import CitizenshipManagement from './CitizenshipManagement.vue';
import MarksManagement from './MarksManagement.vue';
import AttachmentsManagement from './AttachmentsManagement.vue';
import ApplicationApprovers from './ApplicationApprovers.vue';
import { SkeletonTransition, SkeletonBlock } from '@/components/ui';

export default {
  components: {
    UserControl,
    UserApplications,
    UserProfileHeader,
    UserNotificationsInline,
    OrganizationsManagement,
    CompaniesManagement,
    UnloadPlacesContainer,
    NumberFormat,
    TableConstructor,
    UserTypes,
    AccountSettings,
    CitizenshipManagement,
    MarksManagement,
    AttachmentsManagement,
    ApplicationApprovers,
    SkeletonTransition,
    SkeletonBlock,
  },
  data() {
    return {
      loading: true,
      allUsers: [],
      organization: "",
      company: "",
      username: "",
      type_id: 1,
      user_type: "user",
      is_super_admin: false,
      userId: null,
      userOrganizationId: null,
      userCompanyId: null,
      
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
      return this.is_super_admin === true;
    }
  },
  mounted() {
    this.fetchUserData();
    if (this.isBuroPropuskov) {
      this.fetchAllUsers();
    }
  },
  methods: {
    async fetchUserData() {
      try {
        const authStore = useAuthStore();
        if (!authStore.token) {
          alert("Пользователь не авторизован.");
          return;
        }

        const response = await apiRequest("/users/me", {
          method: "GET",
        });

        if (response.ok) {
          const userData = await response.json();
          this.updateUserData(userData);
        } else {
          alert("Ошибка при загрузке данных пользователя.");
        }
      } catch (error) {
        console.error("Ошибка сети при загрузке данных пользователя:", error);
      } finally {
        this.loading = false;
      }
    },
    updateUserData(userData) {
      this.organization = userData.organization || "";
      this.company = userData.company || "";
      this.username = userData.username || "";
      this.type_id = userData.type_id || 1;
      this.user_type = userData.user_type || "user";
      this.is_super_admin = userData.is_super_admin === true;
      this.userId = userData.id || null;
      this.userOrganizationId = userData.organization_id || null;
      this.userCompanyId = userData.company_id || null;
      
      // Дополнительные поля
      this.lastName = userData.last_name || "";
      this.firstName = userData.first_name || "";
      this.middleName = userData.middle_name || "";
      this.position = userData.position || "";
      this.email = userData.email || "";
      this.phone = userData.phone || "";

      console.log("userCompanyId set to:", this.userCompanyId);
    console.log("userOrganizationId set to:", this.userOrganizationId);
    },
    async fetchAllUsers() {
      try {
        const response = await apiRequest("/users/all", {
          method: "GET",
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
  }
};
</script>

<style scoped>
.account-dashboard {
  width: 100%;
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

.first-row :deep(.notifications) {
  max-width: 38%;
}

/* SkeletonTransition оборачивает детей в div — раскладываем его в flex row
   чтобы UserProfileHeader и UserNotificationsInline были side-by-side */
.first-row > div {
  display: flex;
  flex-direction: row;
  gap: 15px;
  flex: 1;
  min-width: 0;
  width: 100%;
}

.dashboard-row {
 padding: 15px 0;
}

.account-section {
  margin-top: 25px;
}

.applications-wrapper {
  position: relative;
}

/* Стили для карточек */
.dashboard-card {
  background: white;
  border-radius: 30px;
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.1);
  border: 1px solid #e6e6e6;
}

.dashboard-card-animated {
  opacity: 0;
  transform: translateY(20px);
  animation: fadeInUp 0.5s cubic-bezier(0.16, 1, 0.3, 1) forwards;
   box-shadow: 0 3px 10px rgba(0, 0, 0, 0.1);
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

.settings {
  width: 100%;
  margin-top: 35px;
  margin-bottom: 35px;
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

  .first-row > div {
    flex-direction: column;
  }

  .first-row :deep(.notifications) {
    max-width: 100%;
  }

  .settings {
    width: 100%;
  }
}

@media (max-width: 768px) {
  .dashboard-card {
    padding: 15px;
  }
}

@media (max-width: 480px) {
  .dashboard-card {
    padding: 15px 10px;
  }
}
</style>