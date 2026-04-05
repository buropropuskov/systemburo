<template>
  <div id="app">
    <a href="#main-content" class="skip-link">Перейти к основному содержанию</a>
    <NavMenu
      v-if="isAuthenticated"
      :is-buropropuskov="isBuropropuskov"
      @logout="logout"
    />
    <div class="content" id="main-content">
      <TheHeader class="theheader" v-if="isAuthenticated"/>
      <router-view v-slot="{ Component }" class="content__container">
        <transition name="page-fade" mode="out-in">
          <component :is="Component" @login-success="handleSuccessfulLogin" :key="$route.path" />
        </transition>
      </router-view>
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

export default {
  name: "App",
  components: {
    NavMenu,
    TheHeader,
    SessionExpiredModal
  },
  data() {
    return {
      showSessionModal: false,
      tokenCheckInterval: null,
      expirationTimer: null,
      modalTimeRemaining: 30,
      tokenExpiryTime: null
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
    }
  },
  methods: {
    async checkAuthStatus() {
      const authStore = useAuthStore()
      const token = authStore.token
      const refreshToken = authStore.refreshToken

      // Если нет хотя бы одного токена - не аутентифицирован и скрываем модалку
      if (!token || !refreshToken) {
          this.showSessionModal = false;
          this.stopExpirationTimer();
          return;
      }

      try {
          const payload = JSON.parse(atob(token.split('.')[1]));
          const currentTime = Math.floor(Date.now() / 1000);
          const timeUntilExpiry = payload.exp - currentTime;

          // Если токен уже истек - сразу выходим
          if (timeUntilExpiry <= 0) {
              this.logout();
              return;
          }

          // Если дошли сюда - токен валиден
          this.tokenExpiryTime = payload.exp;

          // Загружаем права доступа если ещё не загружены
          const permissionsStore = usePermissionsStore()
          if (!permissionsStore.loaded) {
            permissionsStore.fetchPermissions()
          }

          // Запускаем/обновляем таймер истечения
          this.startExpirationTimer(timeUntilExpiry);

      } catch (e) {
          this.logout();
      }
    },
    
    startExpirationTimer(timeUntilExpiry) {
      this.stopExpirationTimer();
      
      // console.log('Starting expiration timer:', timeUntilExpiry + ' seconds');
      
      // Если до истечения меньше или равно 30 секунд - показываем модалку
      if (timeUntilExpiry <= 300) {
        this.modalTimeRemaining = Math.max(0, timeUntilExpiry);
        this.showSessionModal = true;
        
        // Запускаем таймер для обновления оставшегося времени в модалке
        this.expirationTimer = setInterval(() => {
          const currentTime = Math.floor(Date.now() / 1000);
          const remaining = this.tokenExpiryTime - currentTime;
          
          if (remaining <= 0) {
            this.logout();
            return;
          }
          
          this.modalTimeRemaining = Math.max(0, remaining);
          
          // Если время вышло, но модалка еще открыта - выходим
          if (remaining <= 0 && this.showSessionModal) {
            this.logout();
          }
        }, 1000);
        
      } else {
        // Запускаем таймер, который покажет модалку когда останется 30 секунд
        const delayToModal = (timeUntilExpiry - 300) * 1000;
        // console.log('Will show modal in:', delayToModal + ' ms');
        
        this.expirationTimer = setTimeout(() => {
          this.checkAuthStatus(); // Перепроверим статус
        }, delayToModal);
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
      try {
          const authStore = useAuthStore()
          const refreshToken = authStore.refreshToken

          if (!refreshToken) {
              this.logout();
              return;
          }

          const requestBody = {
              refresh_token: refreshToken
          };

          const response = await apiRequest("/refresh-token", {
              method: "POST",
              body: JSON.stringify(requestBody)
          });

          if (response.ok) {
              const tokenData = await response.json();

              authStore.setTokens(tokenData.token, tokenData.refreshToken)
              this.showSessionModal = false;
              this.checkAuthStatus();

              // Перезагружаем права доступа после обновления токена
              const permissionsStore = usePermissionsStore()
              permissionsStore.fetchPermissions()
          } else {
              const errorText = await response.text();
              throw new Error(`Refresh failed: ${response.status} - ${errorText}`);
          }
      } catch (error) {
          this.logout();
      }
    },
    
    async logout() {
      const authStore = useAuthStore()
      try {
        if (authStore.token && authStore.refreshToken) {
          await apiRequest("/logout", {
            method: "POST",
            body: JSON.stringify({
              refresh_token: authStore.refreshToken
            })
          });
        }
      } catch (error) {
        // ignore — logout cleanup happens in finally
      } finally {
        authStore.clearTokens()
        this.showSessionModal = false;
        this.stopExpirationTimer();

        // Сбрасываем права доступа
        const permissionsStore = usePermissionsStore()
        permissionsStore.clearPermissions()

        // Перенаправляем на логин только если мы не уже на странице логина
        if (this.$route.path !== '/') {
          this.$router.push("/");
        }
      }
    },
    
    startTokenMonitoring() {
      // Проверяем токен каждые 10 секунд как fallback
      this.tokenCheckInterval = setInterval(() => {
        this.checkAuthStatus();
      }, 10000);
    },
    
    stopTokenMonitoring() {
      if (this.tokenCheckInterval) {
        clearInterval(this.tokenCheckInterval);
        this.tokenCheckInterval = null;
      }
      this.stopExpirationTimer();
    }
  },
  created() {
    this.checkAuthStatus();
    if (this.isAuthenticated) {
        this.startTokenMonitoring();
    }
    
    // Слушаем изменения localStorage
    window.addEventListener('storage', this.checkAuthStatus);
  },
  beforeUnmount() {
    this.stopTokenMonitoring();
    window.removeEventListener('storage', this.checkAuthStatus);
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