<template>
  <div id="app">
    <NavMenu 
      v-if="isAuthenticated"
      :is-buropropuskov="isBuropropuskov"
      @logout="logout"
    />
    <div class="content">
      <TheHeader class="theheader" v-if="isAuthenticated"/>
      <router-view class="content__container" @login-success="handleSuccessfulLogin" /> 
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
      isAuthenticated: false,
      isBuropropuskov: false,
      showSessionModal: false,
      tokenCheckInterval: null,
      expirationTimer: null,
      modalTimeRemaining: 30,
      tokenExpiryTime: null
    };
  },
  methods: {
    async checkAuthStatus() {
      const token = localStorage.getItem("token");
      const refreshToken = localStorage.getItem("refreshToken");
      
      // Если нет хотя бы одного токена - не аутентифицирован и скрываем модалку
      if (!token || !refreshToken) {
          this.isAuthenticated = false;
          this.isBuropropuskov = false;
          this.showSessionModal = false;
          this.stopExpirationTimer();
          return;
      }
      
      try {
          const payload = JSON.parse(atob(token.split('.')[1]));
          const currentTime = Math.floor(Date.now() / 1000);
          const timeUntilExpiry = payload.exp - currentTime;
          
          /* console.log('Token expiry check:', {
              timeUntilExpiry: timeUntilExpiry + ' seconds',
              isExpired: timeUntilExpiry <= 0,
              willExpireSoon: timeUntilExpiry <= 300
          }); */
          
          // Если токен уже истек - сразу выходим
          if (timeUntilExpiry <= 0) {
              console.log('Token expired, logging out...');
              this.logout();
              return;
          }
          
          // Если дошли сюда - токен валиден
          this.isAuthenticated = true;
          this.isBuropropuskov = payload.type_id === 6;
          this.tokenExpiryTime = payload.exp;
          
          // Запускаем/обновляем таймер истечения
          this.startExpirationTimer(timeUntilExpiry);
          
      } catch (e) {
          console.error("Token decode error:", e);
          this.logout(); // При ошибке декодирования тоже выходим
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
      localStorage.setItem("token", tokenData.token);
      localStorage.setItem("refreshToken", tokenData.refreshToken);
      this.checkAuthStatus();
    },
    
    async extendSession() {
      try {
          const refreshToken = localStorage.getItem("refreshToken");
          
          console.log('🔄 Attempting token refresh with RT:', refreshToken ? 'Present' : 'Missing');
          
          if (!refreshToken) {
              console.log('❌ No refresh token found');
              this.logout();
              return;
          }
          
          const requestBody = {
              refresh_token: refreshToken
          };
          
          console.log('📤 Sending refresh request:', requestBody);
          
          const response = await apiRequest("/refresh-token", {
              method: "POST",
              body: JSON.stringify(requestBody)
          });

          console.log('📥 Refresh response status:', response.status);
          
          if (response.ok) {
              const tokenData = await response.json();
              console.log('✅ Token refresh successful:', tokenData);
              
              localStorage.setItem("token", tokenData.token);
              localStorage.setItem("refreshToken", tokenData.refreshToken);
              this.showSessionModal = false;
              this.checkAuthStatus();
              console.log('✅ Token successfully refreshed and stored');
          } else {
              const errorText = await response.text();
              console.log('❌ Refresh failed:', response.status, errorText);
              throw new Error(`Refresh failed: ${response.status} - ${errorText}`);
          }
      } catch (error) {
          console.error("🔴 Token refresh error:", error);
          this.logout();
      }
    },
    
    async logout() {
  try {
    const token = localStorage.getItem("token");
    const refreshToken = localStorage.getItem("refreshToken");
    
    if (token && refreshToken) {
      console.log('🔄 Sending logout request with specific refresh token');
      
      await apiRequest("/logout", {
        method: "POST",
        body: JSON.stringify({
          refresh_token: refreshToken  // Отправляем конкретный refresh token для удаления
        })
      });
    }
  } catch (error) {
    console.error("Logout error:", error);
  } finally {
    // Всегда очищаем localStorage
    localStorage.removeItem("token");
    localStorage.removeItem("refreshToken");
    this.isAuthenticated = false;
    this.isBuropropuskov = false;
    this.showSessionModal = false;
    this.stopExpirationTimer();
    
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
/* Стили остаются без изменений */
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
</style>