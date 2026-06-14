<template>
  <div
    ref="root"
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
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import UserApplications from './UserApplications.vue';
import UserProfileHeader from './UserProfileHeader.vue';
import UserNotificationsInline from './UserNotificationsInline.vue';
import { SkeletonTransition, SkeletonBlock } from '@/components/ui';

export default {
  components: {
    UserApplications,
    UserProfileHeader,
    UserNotificationsInline,
    SkeletonTransition,
    SkeletonBlock,
  },
  data() {
    return {
      loading: true,
      organization: "",
      company: "",
      username: "",
      type_id: 1,
      user_type: "user",
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
  mounted() {
    this.headerObserver = null;
    this.lastDashboardHeight = -1;
    this.fetchUserData();
    this.$nextTick(this.applyDashboardHeight);
    window.addEventListener('resize', this.applyDashboardHeight);
    // Шапка in-flow и может менять высоту (анонс, перенос строки) - пересчитываем
    // под неё. Наблюдаем саму шапку, а не body, чтобы Teleport-модалки не будили
    // лишний reflow. Тот же приём, что в AdminPageShell.vue.
    const header = document.querySelector('.theheader');
    if (header && typeof ResizeObserver !== 'undefined') {
      this.headerObserver = new ResizeObserver(this.applyDashboardHeight);
      this.headerObserver.observe(header);
    }
  },
  beforeUnmount() {
    window.removeEventListener('resize', this.applyDashboardHeight);
    if (this.headerObserver) {
      this.headerObserver.disconnect();
      this.headerObserver = null;
    }
  },
  methods: {
    /**
     * Тянет дашборд кабинета на доступную высоту вьюпорта (под шапкой), чтобы
     * список заявок занимал весь экран без скролла всей страницы. Высоту меряем
     * фактически (innerHeight - top), а не 100vh минус хардкод шапки. На планшете
     * и мобильном (<=1200px) возвращаем естественный поток - там работает
     * адаптивная фикс-высота карточки и строки складываются в колонку.
     */
    applyDashboardHeight() {
      const el = this.$refs.root;
      if (!el) return;
      if (window.innerWidth <= 1200) {
        el.style.height = '';
        this.lastDashboardHeight = -1;
        return;
      }
      const top = el.getBoundingClientRect().top;
      const height = Math.max(0, Math.round(window.innerHeight - top));
      // Защита от ResizeObserver-петли: пишем стиль только при реальном изменении.
      if (height === this.lastDashboardHeight) return;
      this.lastDashboardHeight = height;
      el.style.height = `${height}px`;
    },
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
    }
  }
};
</script>

<style scoped>
.account-dashboard {
  width: 100%;
  padding: 15px;
  position: relative;
  /* Высоту на desktop задаёт applyDashboardHeight под доступный вьюпорт;
     flex-колонка тянет блок заявок на остаток высоты, overflow держит остаток
     внутри, а не на всей странице. */
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* Стили для строк */
.first-row {
  display: flex;
  gap: 15px;
  margin-bottom: 15px;
  flex-shrink: 0;
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
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.applications-wrapper {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

/* Карточки личного кабинета (профиль/уведомления/заявки) */
.dashboard-card-animated {
  opacity: 0;
  transform: translateY(20px);
  animation: fadeInUp 0.5s cubic-bezier(0.16, 1, 0.3, 1) forwards;
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
}
</style>