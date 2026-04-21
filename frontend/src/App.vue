<template>
  <div id="app">
    <a href="#main-content" class="skip-link">Перейти к основному содержанию</a>
    <NavMenu
      v-if="showChrome"
      :is-buropropuskov="isBuropropuskov"
      @logout="logout"
    />
    <div class="content" id="main-content">
      <TheHeader class="theheader" v-if="showChrome"/>
      <router-view v-slot="{ Component }" class="content__container">
        <transition name="page-fade" mode="out-in">
          <component :is="Component" @login-success="handleSuccessfulLogin" :key="$route.path" />
        </transition>
      </router-view>
      <ScrollTopButton v-if="showChrome" />
    </div>

    <!-- Модальное окно истекшей сессии -->
    <SessionExpiredModal 
      v-if="showSessionModal"
      :time-remaining="modalTimeRemaining"
      @extend-session="extendSession"
      @logout="logout"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { usePermissionsStore } from '@/stores/permissions'
import NavMenu from './components/NavMenu.vue';
import TheHeader from './components/TheHeader/TheHeader.vue';
import SessionExpiredModal from './components/SessionExpiredModal.vue';
import ScrollTopButton from './components/ScrollTopButton.vue';

export default {
  name: "App",
  components: {
    NavMenu,
    TheHeader,
    SessionExpiredModal,
    ScrollTopButton
  },
  data() {
    return {
      showSessionModal: false,
      expirationTimer: null,
      modalTimeRemaining: 0,
      refreshExpiryTime: null,
    };
  },
  computed: {
    isAuthenticated() {
      const authStore = useAuthStore()
      return authStore.isAuthenticated
    },
    isBuropropuskov() {
      const authStore = useAuthStore()
      return authStore.isAdmin
    },
    /**
     * Показывать шапку, навигацию и связанный chrome только когда юзер
     * аутентифицирован и находится не на странице логина. Иначе после
     * setTokens() в LoginComponent до router.push('/personal-cabinet')
     * (задержан на 1.5с из-за success-анимации) шапка и меню моргают
     * поверх формы логина.
     */
    showChrome() {
      return this.isAuthenticated && this.$route.path !== '/';
    }
  },
  methods: {
    checkAuthStatus() {
      const authStore = useAuthStore()
      if (!authStore.refreshToken) {
        this.showSessionModal = false;
        this.stopExpirationTimer();
        return;
      }

      const payload = authStore.refreshPayload;
      if (!payload || !payload.exp) {
        this.logout();
        return;
      }

      const timeUntilExpiry = payload.exp - Math.floor(Date.now() / 1000);
      if (timeUntilExpiry <= 0) {
        this.logout();
        return;
      }

      this.refreshExpiryTime = payload.exp;

      const permissionsStore = usePermissionsStore()
      if (!permissionsStore.loaded) {
        permissionsStore.fetchPermissions()
      }

      this.scheduleExpirationWarning(timeUntilExpiry);
    },

    scheduleExpirationWarning(timeUntilExpiry) {
      this.stopExpirationTimer();
      const WARNING_WINDOW_SEC = 600; // 10 минут

      if (timeUntilExpiry <= WARNING_WINDOW_SEC) {
        this.modalTimeRemaining = timeUntilExpiry;
        this.showSessionModal = true;
        this.expirationTimer = setInterval(() => {
          const remaining = this.refreshExpiryTime - Math.floor(Date.now() / 1000);
          if (remaining <= 0) {
            this.logout();
            return;
          }
          this.modalTimeRemaining = remaining;
        }, 1000);
      } else {
        const delayMs = (timeUntilExpiry - WARNING_WINDOW_SEC) * 1000;
        this.expirationTimer = setTimeout(() => this.checkAuthStatus(), delayMs);
      }
    },

    stopExpirationTimer() {
      if (this.expirationTimer) {
        clearTimeout(this.expirationTimer);
        clearInterval(this.expirationTimer);
        this.expirationTimer = null;
      }
    },

    handleSuccessfulLogin(tokenData) {
      const authStore = useAuthStore()
      authStore.setTokens(tokenData.token, tokenData.refreshToken)
      this.checkAuthStatus();
    },

    async extendSession() {
      const authStore = useAuthStore()
      if (!authStore.refreshToken) {
        this.logout();
        return;
      }
      try {
        const response = await apiRequest("/refresh-token", {
          method: "POST",
          body: JSON.stringify({ refresh_token: authStore.refreshToken }),
        });
        if (!response.ok) throw new Error(`refresh failed: ${response.status}`);
        const data = await response.json();
        authStore.setTokens(data.token, data.refreshToken);
        this.showSessionModal = false;
        this.checkAuthStatus();
        const permissionsStore = usePermissionsStore();
        permissionsStore.fetchPermissions();
      } catch {
        this.logout();
      }
    },

    async logout() {
      const authStore = useAuthStore()
      try {
        if (authStore.token && authStore.refreshToken) {
          await apiRequest("/logout", {
            method: "POST",
            body: JSON.stringify({ refresh_token: authStore.refreshToken }),
          });
        }
      } catch {
        // сеть упала — всё равно чистим клиент
      } finally {
        authStore.clearTokens()
        this.showSessionModal = false;
        this.stopExpirationTimer();

        const permissionsStore = usePermissionsStore()
        permissionsStore.clearPermissions()

        if (this.$route.path !== '/') {
          this.$router.push("/");
        }
      }
    },

    onVisibilityChange() {
      if (document.visibilityState === 'visible') {
        this.checkAuthStatus();
      }
    },
  },
  created() {
    this.checkAuthStatus();
    window.addEventListener('storage', this.checkAuthStatus);
    document.addEventListener('visibilitychange', this.onVisibilityChange);
  },
  beforeUnmount() {
    this.stopExpirationTimer();
    window.removeEventListener('storage', this.checkAuthStatus);
    document.removeEventListener('visibilitychange', this.onVisibilityChange);
  },
  watch: {
    $route() {
      this.checkAuthStatus();
    }
  }
};
</script>

<style>
.skip-link {
  position: absolute;
  top: -40px;
  left: 0;
  background: var(--color-primary);
  color: white;
  padding: 8px 16px;
  z-index: 10000;
  transition: top 0.2s;
}
.skip-link:focus {
  top: 0;
}

* {
    font-family: 'Montserrat', sans-serif;
    padding: 0;
    margin: 0;
    box-sizing: border-box;
    scroll-behavior: smooth;
}

::-webkit-scrollbar {
  width: 0;
}

body.auth-active #app {
  margin-left: 25px;
}

body:not(.auth-active) #app {
  margin-left: 0;
}

.form-input-sm {
  border-radius: 10px !important;
}

.blue {
  color: #4F5BDF;
}

.red {
  color: rgb(241, 76, 76);
}

.page-fade-enter-active {
  transition: opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}
.page-fade-leave-active {
  transition: opacity 0.15s cubic-bezier(0.4, 0, 0.2, 1);
}
.page-fade-enter-from,
.page-fade-leave-to {
  opacity: 0;
}
</style>